// SiYuan community bazaar.
// Copyright (c) 2021-present, b3log.org
//
// Bazaar is licensed under Mulan PSL v2.
// You can use this software according to the terms and conditions of the Mulan PSL v2.
// You may obtain a copy of Mulan PSL v2 at:
//         http://license.coscl.org.cn/MulanPSL2
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR PURPOSE.
// See the Mulan PSL v2 for more details.

package main

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/88250/gulu"
	"github.com/google/go-github/v89/github"
	"github.com/siyuan-note/bazaar/actions/util"
	"github.com/siyuan-note/bazaar/rules"
	"golang.org/x/sync/errgroup"
)

/*
Stage 流程：
1. 检查 GitHub API rate limit 是否足够覆盖本轮待索引仓库数
2. 按类型依次执行 performStage；每类开始前重新加载 OccupiedNames，以便上一类本轮新写入的 name 参与后续类型的唯一性检查
3. 读取 *s.txt 与既有 stage/*.json，并发索引各 owner/repo
4. hash 未变则跳过下载；否则下载 package.zip → rules.Check → 上传 OSS（package.zip、README、preview、icon、清单 JSON）
5. 按 updated 降序排序后写出 stage/*.json（键序经 marshalSortedIndentedJSON 稳定）
6. 将本轮失败/成功同步到固定 Issue（#1921）：失败 upsert 一仓一条评论，成功则删除；hash 跳过不改动评论

换维护者（列表中 alice/foo → bob/foo，stage 仍有 alice/foo）：
- 同路径旧条目用于 hash 跳过 / 失败保留；换路径时不沿用旧 URL 条目
- rules.Check 继承旧 package.name 与 version（视同更新，须提升 version）
*/

type Set map[string]struct{} // 字符串集合

var (
	BAZAAR_ROOT_PATH = "."              // bazaar 仓库根目录（stage 工作区）
	GITHUB_TOKEN     = os.Getenv("PAT") // GitHub Token
	STAGE_POOL_SIZE  = envIntDefault("STAGE_POOL_SIZE", 80)

	REQUEST_TIMEOUT = 30 * time.Second // 请求超时时间

	logger        = gulu.Log.NewLogger(os.Stdout)
	githubContext context.Context
	githubClient  *github.Client
)

func envIntDefault(key string, defaultVal int) int {
	if s := os.Getenv(key); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return n
		}
	}
	return defaultVal
}

func main() {
	logger.Infof("Stage started")

	var stop context.CancelFunc
	githubContext, stop = signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var err error
	githubClient, err = util.NewGitHubClient(GITHUB_TOKEN, REQUEST_TIMEOUT)
	if err != nil {
		logger.Fatalf("create github client failed: %s", err)
	}

	reposByType, err := loadReposByPackageType()
	if err != nil {
		logger.Fatalf("parse repos list failed: %s", err)
	}

	if err := checkRateLimitBeforeStage(reposByType); err != nil {
		logger.Fatalf("GitHub API rate limit check failed: %s", err)
	}

	// 每类 stage 前重新加载 OccupiedNames，以便上一类本轮 performStage 新写入的 name 参与后续类型的唯一性检查。
	// 仅对新上架的包进行检查
	reports := &stageReportCollector{}
	for _, packageType := range rules.AllPackageTypes() {
		occupiedNames, err := util.LoadOccupiedNames(BAZAAR_ROOT_PATH)
		if err != nil {
			logger.Fatalf("load occupied names failed: %s", err)
		}
		performStage(packageType, occupiedNames, reposByType[packageType], reports)
	}

	// 失败汇总到固定 Issue；同步失败不阻断已写出的 stage JSON（仅记日志）。
	if err := syncStageFailReports(githubContext, githubClient, reports.snapshot()); err != nil {
		logger.Errorf("sync stage-fail comments failed: %s", err)
	}

	logger.Infof("Stage completed")
}

func loadReposByPackageType() (map[rules.PackageType][]string, error) {
	reposByType := make(map[rules.PackageType][]string, len(rules.AllPackageTypes()))
	for _, packageType := range rules.AllPackageTypes() {
		repos, err := util.ParseReposFromTxt(packageType.ReposListFile())
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", packageType.ReposListFile(), err)
		}
		reposByType[packageType] = repos
	}
	return reposByType, nil
}

// checkRateLimitBeforeStage 统计本次待检查仓库数、请求 GitHub rate_limit（该请求不计入 core），若 core 剩余请求数不足则返回错误。参考 https://docs.github.com/zh/rest/rate-limit/rate-limit
func checkRateLimitBeforeStage(reposByType map[rules.PackageType][]string) error {
	var repoCount int
	for _, repos := range reposByType {
		repoCount += len(repos)
	}
	// 2.5 为每个仓库 staging 时消耗的 GitHub REST API (core) 请求数经验值（repoStats 1 次 + getRepoLatestRelease 等）
	required := int(math.Ceil(float64(repoCount) * 2.5))
	if required == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(githubContext, 10*time.Second)
	defer cancel()
	limits, _, err := githubClient.RateLimit.Get(ctx)
	if err != nil {
		return fmt.Errorf("get rate limit: %w", err)
	}
	core := limits.GetCore()
	if core == nil {
		return fmt.Errorf("rate_limit response missing core")
	}
	remaining := core.Remaining
	limit := core.Limit
	reset := core.Reset.Unix()
	if remaining < required {
		return fmt.Errorf("GitHub REST API (core) remaining %d / %d is below required %d for %d repos (~%d requests); reset at %d", remaining, limit, required, repoCount, required, reset)
	}
	logger.Infof("GitHub API (core) remaining %d / %d, %d repos to check (~%d requests), OK", remaining, limit, repoCount, required)
	return nil
}

// loadOldStageData 加载现有的 stage 文件数据，返回以 owner/repo 为 key 的映射。
// 文件不存在时返回空映射（不报错）；读取或解析失败时返回错误，避免误把已有 stage 当作无旧数据。
func loadOldStageData(packageType rules.PackageType) (map[string]*util.StageRepo, error) {
	oldStageData := make(map[string]*util.StageRepo)
	stageFilePath := filepath.Join(BAZAAR_ROOT_PATH, "stage", packageType.StageJSONFile())

	stageFile, err := util.ReadStageFile(stageFilePath)
	if err != nil {
		return nil, fmt.Errorf("read stage [%s]: %w", stageFilePath, err)
	}

	for i := range stageFile.Repos {
		repo := &stageFile.Repos[i]
		repoKey, ok := util.OwnerRepoFromStageURL(repo.URL)
		if !ok {
			continue
		}
		oldStageData[repoKey] = repo
	}

	return oldStageData, nil
}

func performStage(packageType rules.PackageType, occupiedNames map[string]struct{}, reposSlice []string, reports *stageReportCollector) {
	logger.Infof("start stage [%s]", packageType.Plural())

	oldStageData, err := loadOldStageData(packageType)
	if err != nil {
		logger.Fatalf("load old stage [%s] failed: %s", packageType.Plural(), err)
	}
	listedRepos := make(Set, len(reposSlice))
	for _, ownerRepo := range reposSlice {
		listedRepos[ownerRepo] = struct{}{}
	}
	oldStageByRepoName := indexOldStageByRepoName(oldStageData)

	var themeJsAllowSet Set
	if packageType == rules.TypeTheme {
		paths, err := util.ParseReposFromTxt(util.ThemeJsAllowlistRelPath)
		if err != nil {
			logger.Fatalf("read or parse [%s] failed: %s", util.ThemeJsAllowlistRelPath, err)
		}
		themeJsAllowSet = make(Set, len(paths))
		for _, p := range paths {
			themeJsAllowSet[p] = struct{}{}
		}
	}

	var stageReposMu sync.Mutex
	var stageRepos []*util.StageRepo
	var g errgroup.Group
	g.SetLimit(STAGE_POOL_SIZE)

	for _, ownerRepo := range reposSlice {
		g.Go(func() error {
			repoURL := util.GitHubRepoURL(ownerRepo)
			exactOld, checkOldName, checkOldVersion := resolveStageCheckLegacy(ownerRepo, oldStageData, oldStageByRepoName, listedRepos)
			releaseInfo, releaseErr := getRepoLatestRelease(ownerRepo)
			if releaseErr != nil {
				stageReposMu.Lock()
				if exactOld != nil {
					stageRepos = append(stageRepos, exactOld)
					logger.Errorf("get [%s] latest release failed, keeping old data", repoURL)
				} else {
					logger.Errorf("get [%s] latest release failed and no old data found", repoURL)
				}
				stageReposMu.Unlock()
				if !errors.Is(releaseErr, errInvalidOwnerRepo) {
					reports.add(stageReport{
						OwnerRepo:   ownerRepo,
						PackageType: packageType,
						Kind:        stageReportFail,
						Release:     releaseInfo,
						Issues:      stageIssueFromErr(releaseErr),
						KeptOld:     exactOld != nil,
					})
				}
				return nil
			}
			hash := releaseInfo.CommitSHA
			updated := releaseInfo.Published
			packageZipAssetID := releaseInfo.PackageZipAssetID

			// Latest Release 的 hash 与已 stage 的 hash 一致则跳过，不下载、不更新，沿用旧条目
			// 仅同路径 exactOld：换维护者不得沿用旧 owner/repo@hash 条目
			if exactOld != nil {
				oldHash := parseHashFromStageURL(exactOld.URL)
				if oldHash != "" && hash == oldHash {
					if sameCommitPackageZipChanged(exactOld, packageZipAssetID) {
						logger.Errorf("repo [%s] hash unchanged [%s] but package.zip asset id changed (%d -> %d); a new release tag is required to update the staged package",
							repoURL, hash, exactOld.PackageZipAssetID, packageZipAssetID)
						stageReposMu.Lock()
						stageRepos = append(stageRepos, exactOld)
						stageReposMu.Unlock()
						reports.add(stageReport{
							OwnerRepo:   ownerRepo,
							PackageType: packageType,
							Kind:        stageReportFail,
							Release:     releaseInfo,
							Hash:        hash,
							KeptOld:     true,
							Issues: stageInternalIssue(
								fmt.Sprintf("Latest Release 仍指向同一 commit（`%s`），但 `package.zip` 资源已被替换（asset id %d → %d）。集市 Stage 需要新的 Release 标签才会更新入库。请提升清单 `version`，重新打包 `package.zip`，并发布带新 tag 的 GitHub Release（标记为 Latest）。",
									hash, exactOld.PackageZipAssetID, packageZipAssetID),
								fmt.Sprintf("The Latest Release still points to the same commit (`%s`), but the `package.zip` asset was replaced (asset id %d → %d). Stage only updates when there is a new release tag. Please bump the manifest `version`, rebuild `package.zip`, and publish a new GitHub Release with a new tag (marked as Latest).",
									hash, exactOld.PackageZipAssetID, packageZipAssetID),
							),
						})
						return nil
					}
					logger.Infof("skip repo [%s], hash unchanged [%s]", ownerRepo, hash)
					stageReposMu.Lock()
					stageRepos = append(stageRepos, exactOld)
					stageReposMu.Unlock()
					reports.add(stageReport{
						OwnerRepo:   ownerRepo,
						PackageType: packageType,
						Kind:        stageReportSkip,
						Release:     releaseInfo,
						Hash:        hash,
						KeptOld:     true,
					})
					return nil
				}
			}

			var allowThemeJS bool
			if packageType == rules.TypeTheme {
				_, allowThemeJS = themeJsAllowSet[ownerRepo]
			}
			if checkOldName != "" && exactOld == nil {
				logger.Infof("maintainer change staging [%s], inherit old name [%s] version [%s]", ownerRepo, checkOldName, checkOldVersion)
			}
			ok, size, installSize, pkg, indexIssues := indexPackage(ownerRepo, packageType, hash, packageZipAssetID, checkOldName, checkOldVersion, allowThemeJS, occupiedNames)
			if !ok || pkg == nil {
				// 索引失败或 pkg 为空时使用旧数据，避免 "package": null 的坏数据覆盖
				// 换维护者无 exactOld：不得把旧 owner/repo 条目写回
				stageReposMu.Lock()
				if exactOld != nil {
					stageRepos = append(stageRepos, exactOld)
					logger.Errorf("index failed for [%s], keeping old data", repoURL)
				} else {
					logger.Errorf("index failed for [%s] and no old data found", repoURL)
				}
				stageReposMu.Unlock()
				// 无 Issues 时多为防御性路径（如非法 owner/repo），仅日志、不写汇总 Issue
				if len(indexIssues) > 0 {
					reports.add(stageReport{
						OwnerRepo:   ownerRepo,
						PackageType: packageType,
						Kind:        stageReportFail,
						Release:     releaseInfo,
						Hash:        hash,
						Issues:      indexIssues,
						KeptOld:     exactOld != nil,
					})
				}
				return nil
			}

			stars, openIssues, ok := repoStats(ownerRepo)
			// 如果获取统计数据失败，尝试使用旧数据
			if !ok {
				stageReposMu.Lock()
				if exactOld != nil {
					stageRepos = append(stageRepos, exactOld)
					logger.Errorf("repoStats failed for [%s], keeping old data", repoURL)
				} else {
					logger.Errorf("repoStats failed for [%s] and no old data found", repoURL)
				}
				stageReposMu.Unlock()
				reports.add(stageReport{
					OwnerRepo:   ownerRepo,
					PackageType: packageType,
					Kind:        stageReportFail,
					Release:     releaseInfo,
					Hash:        hash,
					KeptOld:     exactOld != nil,
					Issues: stageInternalIssue(
						"获取仓库 Star / Open Issues 统计失败，本轮未更新入库。请稍后重试；若持续失败请联系维护者。",
						"Failed to fetch repository star / open-issue stats, so this run did not update the staged package. Please retry later; if it keeps failing, contact a maintainer.",
					),
				})
				return nil
			}

			stageReposMu.Lock()
			stageRepos = append(stageRepos, &util.StageRepo{
				URL:               ownerRepo + "@" + hash,
				Stars:             stars,
				OpenIssues:        openIssues,
				Updated:           updated,
				Size:              size,
				InstallSize:       installSize,
				PackageZipAssetID: packageZipAssetID,
				Package:           *pkg,
			})
			stageReposMu.Unlock()
			reports.add(stageReport{
				OwnerRepo:   ownerRepo,
				PackageType: packageType,
				Kind:        stageReportPass,
				Release:     releaseInfo,
				Hash:        hash,
			})
			logger.Infof("updated repo [%s]", ownerRepo)
			return nil
		})
	}
	_ = g.Wait()

	slices.SortStableFunc(stageRepos, func(a, b *util.StageRepo) int {
		return cmp.Compare(b.Updated, a.Updated)
	})

	staged := util.StageFile{Repos: make([]util.StageRepo, len(stageRepos))}
	for i, repo := range stageRepos {
		staged.Repos[i] = *repo
		// hash 跳过 / 失败保留的旧条目也可能带有 "funding": {}，写回前一并清理。
		rules.ClearEmptyFunding(&staged.Repos[i].Package)
	}

	data, err := marshalSortedIndentedJSON(staged)
	if err != nil {
		logger.Fatalf("marshal stage [%s] failed: %s", packageType.StageJSONFile(), err)
	}

	stageFilePath := filepath.Join(BAZAAR_ROOT_PATH, "stage", packageType.StageJSONFile())
	if err = os.WriteFile(stageFilePath, data, 0644); err != nil {
		logger.Fatalf("write stage [%s] failed: %s", packageType.StageJSONFile(), err)
	}

	logger.Infof("finish stage [%s]", packageType.Plural())
}

func repoStats(ownerRepo string) (stars, openIssues int, ok bool) {
	repoURL := util.GitHubRepoURL(ownerRepo)
	owner, name, cutOk := strings.Cut(ownerRepo, "/")
	if !cutOk {
		logger.Errorf("get repo stats [%s] failed: invalid owner/repo", ownerRepo)
		return
	}
	ctx, cancel := context.WithTimeout(githubContext, REQUEST_TIMEOUT)
	defer cancel()
	repo, _, err := githubClient.Repositories.Get(ctx, owner, name)
	if err != nil {
		logger.Errorf("get repo stats [%s] failed: %s", repoURL, err)
		return
	}
	stars = repo.GetStargazersCount()
	openIssues = repo.GetOpenIssuesCount()
	ok = true
	return
}

// marshalSortedIndentedJSON 序列化为键序稳定、带缩进的 JSON。
// v 可为 struct，或原始 JSON 字节（[]byte）；struct 会先经 json.Marshal / Unmarshal 转为 JSON 树（map[string]any），
// 再按字典序输出键。先输出紧凑 JSON，再用 json.Indent 缩进，避免 json.MarshalIndent 对嵌套 any 的缩进错乱。
// SetEscapeHTML(false) 避免将 & 等字符转为 \uXXXX。
func marshalSortedIndentedJSON(v any) ([]byte, error) {
	data, ok := v.([]byte)
	if !ok {
		var err error
		data, err = json.Marshal(v)
		if err != nil {
			return nil, err
		}
	}
	var tree any
	if err := json.Unmarshal(data, &tree); err != nil {
		return nil, err
	}
	var compact bytes.Buffer
	enc := json.NewEncoder(&compact)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(tree); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, bytes.TrimSpace(compact.Bytes()), "", "  "); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
