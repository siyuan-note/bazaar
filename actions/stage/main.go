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
	"sync/atomic"
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
1. 按 STAGE_MODE 决定范围：push → 增量（仅检查本次 *.txt 相对 STAGE_BEFORE_SHA 新增的 owner/repo，且只重建有变更的类型）；
   schedule / workflow_dispatch → 全量。增量 before 无效或 diff 失败则回退全量。
2. 并发前串行 GET /repos/{owner}/{repo}：用响应头检查 PAT core 剩余是否够本轮估算量，并锚定 PAT / GITHUB_TOKEN 观测起点（不用 GET /rate_limit；不用 GET /user，以免 GITHUB_TOKEN 403）
3. 按类型依次执行 performStage；每类开始前重新加载 OccupiedNames，以便上一类本轮新写入的 name 参与后续类型的唯一性检查
4. 读取 *s.txt 与既有 stage/*.json；增量时未列入 check 的仓沿用旧条目（不打 API、不写 report），下架随当前列表重建自然消失
5. 先比 packageZipAssetId：相同则跳过；不同则取 SHA-256（优先 GitHub digest，否则下载 zip 计算）。内容未变则只回写 asset id；内容变了则校验并上传 OSS（package.zip、README、preview、icon、清单 JSON）。url 的 @hash 为 SHA-256 前 7 位
6. 按 updated 降序排序后写出 stage/*.json（键序经 marshalSortedIndentedJSON 稳定；新上架用索引时间，更新用 Release 发布时间）
7. 将本轮失败/成功同步为按仓独立 Issue（标签 stage-fail）：失败 upsert（正文未变则跳过 Edit）；
   成功入库或 hash 跳过（已能正常取到 Release）则先评论说明再关闭；
   已不在任一 *s.txt 的仓（下架 / 换维护者旧仓）按 delisted 关闭（对照完整列表，非本轮 reports）
   （本仓 Issue 用 GITHUB_TOKEN；跨仓 Release / repoStats 用 PAT）
8. 运行中若遇 GitHub API 主限流 / 次级限流：保留旧数据、不写 stage-fail，写完当前类型后中止后续类型（退出码 0，便于提交已完成进度）
9. 运行中若遇 GitHub API 5xx 服务端错误：保留旧数据、不写 stage-fail（不归咎作者；下轮索引再试），不中止其他仓库
10. 结束后用响应头记录开始/结束剩余配额与当次实际 core 请求次数（samples）

换维护者（列表中 alice/foo → bob/foo，stage 仍有 alice/foo）：
- 同路径旧条目用于 hash 跳过 / 失败保留；换路径时不沿用旧 URL 条目
- rules.Check 继承旧 package.name 与 version（视同更新，须提升 version）
*/

type Set map[string]struct{} // 字符串集合

var (
	BAZAAR_ROOT_PATH = "."                       // bazaar 仓库根目录（stage 工作区）
	PAT              = os.Getenv("PAT")          // 个人访问令牌（跨仓 Release / repoStats）
	GITHUB_TOKEN     = os.Getenv("GITHUB_TOKEN") // Actions 令牌（本仓 Stage 失败 Issue）
	STAGE_POOL_SIZE  = envIntDefault("STAGE_POOL_SIZE", 80)

	REQUEST_TIMEOUT = 30 * time.Second // 请求超时时间

	logger           = gulu.Log.NewLogger(os.Stdout)
	githubContext    context.Context
	githubClient     *github.Client // PAT：跨仓 Release / repoStats
	githubRepoClient *github.Client // GITHUB_TOKEN：本仓 Stage 失败 Issue
	patRateObs       *util.RateHeaderObserver
	repoRateObs      *util.RateHeaderObserver
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

	// TEMP: 见 migrate_hash.go；跑完迁移后删除此分支并恢复常规流程。
	if stageHashMigrateOnly {
		runStageHashMigrate()
		logger.Infof("Stage completed (hash migrate only)")
		return
	}

	var err error
	githubClient, patRateObs, err = util.NewGitHubClientWithRateObserver(PAT, REQUEST_TIMEOUT)
	if err != nil {
		logger.Fatalf("create github client failed: %s", err)
	}
	repoToken := GITHUB_TOKEN
	if repoToken == "" {
		repoToken = PAT
		logger.Infof("GITHUB_TOKEN empty, fall back to PAT for stage-fail issues")
	}
	githubRepoClient, repoRateObs, err = util.NewGitHubClientWithRateObserver(repoToken, REQUEST_TIMEOUT)
	if err != nil {
		logger.Fatalf("create github repo client failed: %s", err)
	}

	reposByType, err := loadReposByPackageType()
	if err != nil {
		logger.Fatalf("parse repos list failed: %s", err)
	}

	jobs, mode := resolveStageJobs(githubContext, BAZAAR_ROOT_PATH, reposByType)
	if mode == stageModeIncremental && len(jobs) == 0 {
		logger.Infof("incremental stage: no package list changes; nothing to do")
		logger.Infof("Stage completed")
		return
	}

	if err := checkRateLimitBeforeStage(countCheckRepos(jobs)); err != nil {
		logger.Fatalf("GitHub API rate limit check failed: %s", err)
	}

	// 每类 stage 前重新加载 OccupiedNames，以便上一类本轮 performStage 新写入的 name 参与后续类型的唯一性检查。
	// 仅对新上架的包进行检查
	reports := &stageReportCollector{}
	var abortedByRateLimit bool
	for _, packageType := range rules.AllPackageTypes() {
		job, ok := jobs[packageType]
		if !ok {
			logger.Infof("skip stage [%s], list unchanged in this push", packageType.Plural())
			continue
		}
		occupiedNames, err := util.LoadOccupiedNames(BAZAAR_ROOT_PATH)
		if err != nil {
			logger.Fatalf("load occupied names failed: %s", err)
		}
		if performStage(packageType, occupiedNames, job.repos, job.checkRepos, reports) {
			abortedByRateLimit = true
			logger.Errorf("abort remaining package types due to GitHub API rate limit")
			break
		}
	}

	// 失败同步为按仓独立 Issue；同步失败不阻断已写出的 stage JSON（仅记日志）。
	// 限流导致的失败不会进入 reports，故不会误刷 stage-fail Issue。
	// listed 用完整 *s.txt，避免增量跳过类型 / 限流中止时把未检查仓误判为下架。
	if err := syncStageFailReports(githubContext, githubRepoClient, reports.snapshot(), ownerRepoListedSet(reposByType)); err != nil {
		logger.Errorf("sync stage-fail issues failed: %s", err)
	}

	logger.Infof("%s", util.FormatRateHeaderObservation("PAT", patRateObs))
	logger.Infof("%s", util.FormatRateHeaderObservation("GITHUB_TOKEN", repoRateObs))
	if abortedByRateLimit {
		logger.Errorf("Stage completed with GitHub API rate limit abort; retry after the quota resets")
		return
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

// stageAPIRequestsPerRepo 为每个仓库 staging 时消耗的 GitHub REST API (core) 请求数经验值。
// 近几轮以 hash skip 为主：仅 GetLatestRelease ≈ 1；全量更新另加 DownloadReleaseAsset + repoStats。
// 取 1.3 覆盖少量更新与余量（不再解析 tag→commit）。
const stageAPIRequestsPerRepo = 1.3

// checkRateLimitBeforeStage 按本轮待 API 检查的仓库数估算请求量；串行 GET 本仓 repos 读取真实 core 响应头做门槛，并锚定观测起点。
// 不用 GET /rate_limit（其实测 remaining 可能偏高）。探测本身计入 samples，响应头 Remaining 为扣减后值。
func checkRateLimitBeforeStage(repoCount int) error {
	required := int(math.Ceil(float64(repoCount) * stageAPIRequestsPerRepo))
	ctx, cancel := context.WithTimeout(githubContext, 10*time.Second)
	defer cancel()

	owner, repo, ok := bazaarOwnerRepo()
	if !ok {
		return fmt.Errorf("probe rate headers: GITHUB_REPOSITORY not set / invalid")
	}
	rate, err := util.SeedRateHeaderBaseline(ctx, githubClient, owner, repo)
	if err != nil {
		return fmt.Errorf("probe PAT core rate headers: %w", err)
	}
	if _, err := util.SeedRateHeaderBaseline(ctx, githubRepoClient, owner, repo); err != nil {
		logger.Errorf("seed GITHUB_TOKEN rate header baseline failed: %s", err)
	}

	remaining := rate.Remaining
	limit := rate.Limit
	reset := rate.Reset.Unix()
	if required == 0 {
		logger.Infof("GitHub API (core via headers) remaining %d / %d, 0 repos to check, OK", remaining, limit)
		return nil
	}
	if remaining < required {
		return fmt.Errorf("GitHub REST API (core) remaining %d / %d is below required %d for %d repos (~%d requests); reset at %d", remaining, limit, required, repoCount, required, reset)
	}
	logger.Infof("GitHub API (core via headers) remaining %d / %d, %d repos to check (~%d requests), OK", remaining, limit, repoCount, required)
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

// performStage 执行单一类型的 staging。若遇 GitHub API 限流返回 true（调用方应中止后续类型）。
// checkRepos == nil 表示对 reposSlice 全量打 API；非 nil 时仅检查集合内路径，其余沿用同路径旧条目且不写 report。
func performStage(packageType rules.PackageType, occupiedNames map[string]struct{}, reposSlice []string, checkRepos Set, reports *stageReportCollector) (abortedByRateLimit bool) {
	if checkRepos == nil {
		logger.Infof("start stage [%s] (check all %d)", packageType.Plural(), len(reposSlice))
	} else {
		logger.Infof("start stage [%s] (check %d / listed %d)", packageType.Plural(), len(checkRepos), len(reposSlice))
	}

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
	var rateLimited atomic.Bool
	var g errgroup.Group
	g.SetLimit(STAGE_POOL_SIZE)

	appendKeptOld := func(repoURL, reason string, exactOld *util.StageRepo) {
		stageReposMu.Lock()
		defer stageReposMu.Unlock()
		if exactOld != nil {
			stageRepos = append(stageRepos, exactOld)
			logger.Errorf("%s for [%s], keeping old data", reason, repoURL)
			return
		}
		logger.Errorf("%s for [%s] and no old data found", reason, repoURL)
	}

	markRateLimited := func(ownerRepo string, err error) {
		if rateLimited.CompareAndSwap(false, true) {
			logger.Errorf("GitHub API rate limited while staging [%s]: %s; aborting remaining work this run (not reporting as stage-fail issue)", ownerRepo, err)
		}
	}

	for _, ownerRepo := range reposSlice {
		g.Go(func() error {
			repoURL := util.GitHubRepoURL(ownerRepo)
			exactOld, checkOldName, checkOldVersion := resolveStageCheckLegacy(ownerRepo, oldStageData, oldStageByRepoName, listedRepos)

			if !shouldCheck(ownerRepo, checkRepos) {
				if exactOld != nil {
					stageReposMu.Lock()
					stageRepos = append(stageRepos, exactOld)
					stageReposMu.Unlock()
					logger.Infof("keep staged [%s] without recheck (incremental)", ownerRepo)
				} else {
					logger.Infof("skip unstaged [%s] without recheck (incremental); leave for full stage", ownerRepo)
				}
				return nil
			}

			if rateLimited.Load() {
				appendKeptOld(repoURL, "skipped after rate limit", exactOld)
				return nil
			}

			releaseInfo, releaseErr := getRepoLatestRelease(ownerRepo)
			if releaseErr != nil {
				appendKeptOld(repoURL, "get latest release failed", exactOld)
				if util.IsGitHubRateLimit(releaseErr) {
					markRateLimited(ownerRepo, releaseErr)
					return nil
				}
				if util.IsGitHubServerError(releaseErr) {
					// 5xx 多为 GitHub 瞬时故障，不向作者开 stage-fail（避免误报 Release 获取失败）。
					logger.Errorf("GitHub API server error while staging [%s]: %s; keeping old data (not reporting as stage-fail issue)", ownerRepo, releaseErr)
					return nil
				}
				if !errors.Is(releaseErr, errInvalidOwnerRepo) {
					reports.add(stageReport{
						OwnerRepo:   ownerRepo,
						PackageType: packageType,
						Kind:        stageReportFail,
						Release:     releaseInfo,
						Issues:      stageIssueFromErr(releaseErr),
					})
				}
				return nil
			}
			packageZipAssetID := releaseInfo.PackageZipAssetID
			// 新上架（无旧清单可继承）用 Stage 索引时间；已有包更新仍用 Release 发布时间。
			updated := releaseInfo.Published
			if checkOldName == "" {
				updated = time.Now().UTC().Format(time.RFC3339)
			}

			// 快路径：asset id 未变则跳过（无需看 SHA-256）
			// 仅同路径 exactOld：换维护者不得沿用旧 owner/repo@hash 条目
			if exactOld != nil && exactOld.PackageZipAssetID != 0 && exactOld.PackageZipAssetID == packageZipAssetID {
				logger.Infof("skip repo [%s], package.zip asset id unchanged [%d]", ownerRepo, packageZipAssetID)
				stageReposMu.Lock()
				stageRepos = append(stageRepos, exactOld)
				stageReposMu.Unlock()
				reports.add(stageReport{
					OwnerRepo:   ownerRepo,
					PackageType: packageType,
					Kind:        stageReportSkip,
					Release:     releaseInfo,
					Hash:        parseHashFromStageURL(exactOld.URL),
				})
				return nil
			}

			sha256Hex, zipData, shaErr := resolvePackageZipSHA256(ownerRepo, releaseInfo)
			if shaErr != nil {
				appendKeptOld(repoURL, "resolve package.zip sha256 failed", exactOld)
				if util.IsGitHubRateLimit(shaErr) {
					markRateLimited(ownerRepo, shaErr)
					return nil
				}
				if util.IsGitHubServerError(shaErr) {
					logger.Errorf("GitHub API server error while hashing [%s]: %s; keeping old data", ownerRepo, shaErr)
					return nil
				}
				reports.add(stageReport{
					OwnerRepo:   ownerRepo,
					PackageType: packageType,
					Kind:        stageReportFail,
					Release:     releaseInfo,
					Issues:      stageIssueFromErr(shaErr),
				})
				return nil
			}
			hash := util.PackageHashFromSHA256(sha256Hex)
			if hash == "" {
				appendKeptOld(repoURL, "empty package hash", exactOld)
				reports.add(stageReport{
					OwnerRepo:   ownerRepo,
					PackageType: packageType,
					Kind:        stageReportFail,
					Release:     releaseInfo,
					Issues: stageInternalIssue(
						"无法由 `package.zip` 计算包 hash。请确认 Latest Release 中的 `package.zip` 可正常下载。",
						"Couldn't derive a package hash from `package.zip`. Please make sure `package.zip` in the Latest Release can be downloaded.",
					),
				})
				return nil
			}

			// asset id 变了但内容 SHA-256 未变：只回写 asset id，不重跑校验/上传
			if exactOld != nil && exactOld.PackageZipSHA256 != "" && exactOld.PackageZipSHA256 == sha256Hex {
				kept := *exactOld
				kept.PackageZipAssetID = packageZipAssetID
				kept.PackageZipSHA256 = sha256Hex
				logger.Infof("skip repo [%s], package.zip sha256 unchanged [%s] (asset id %d -> %d)", ownerRepo, hash, exactOld.PackageZipAssetID, packageZipAssetID)
				stageReposMu.Lock()
				stageRepos = append(stageRepos, &kept)
				stageReposMu.Unlock()
				reports.add(stageReport{
					OwnerRepo:   ownerRepo,
					PackageType: packageType,
					Kind:        stageReportSkip,
					Release:     releaseInfo,
					Hash:        hash,
				})
				return nil
			}

			var allowThemeJS bool
			if packageType == rules.TypeTheme {
				_, allowThemeJS = themeJsAllowSet[ownerRepo]
			}
			if checkOldName != "" && exactOld == nil {
				logger.Infof("maintainer change staging [%s], inherit old name [%s] version [%s]", ownerRepo, checkOldName, checkOldVersion)
			}
			ok, size, installSize, pkg, indexIssues, indexErr := indexPackage(ownerRepo, packageType, hash, packageZipAssetID, zipData, checkOldName, checkOldVersion, allowThemeJS, occupiedNames)
			if indexErr != nil && util.IsGitHubRateLimit(indexErr) {
				appendKeptOld(repoURL, "index failed due to rate limit", exactOld)
				markRateLimited(ownerRepo, indexErr)
				return nil
			}
			if !ok || pkg == nil {
				// 索引失败或 pkg 为空时使用旧数据，避免 "package": null 的坏数据覆盖
				// 换维护者无 exactOld：不得把旧 owner/repo 条目写回
				appendKeptOld(repoURL, "index failed", exactOld)
				// 无 Issues 时多为防御性路径（如非法 owner/repo），仅日志、不写汇总 Issue
				if len(indexIssues) > 0 {
					reports.add(stageReport{
						OwnerRepo:   ownerRepo,
						PackageType: packageType,
						Kind:        stageReportFail,
						Release:     releaseInfo,
						Hash:        hash,
						Issues:      indexIssues,
					})
				}
				return nil
			}

			stars, openIssues, ok, statsErr := repoStats(ownerRepo)
			// 如果获取统计数据失败，尝试使用旧数据
			if !ok {
				appendKeptOld(repoURL, "repoStats failed", exactOld)
				if util.IsGitHubRateLimit(statsErr) {
					markRateLimited(ownerRepo, statsErr)
					return nil
				}
				reports.add(stageReport{
					OwnerRepo:   ownerRepo,
					PackageType: packageType,
					Kind:        stageReportFail,
					Release:     releaseInfo,
					Hash:        hash,
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
				PackageZipSHA256:  sha256Hex,
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

	abortedByRateLimit = rateLimited.Load()
	if abortedByRateLimit {
		stageRepos = backfillUnprocessedStageRepos(reposSlice, stageRepos, oldStageData)
		logger.Errorf("stage [%s] aborted due to GitHub API rate limit; kept old data for unchecked repos", packageType.Plural())
	}

	slices.SortStableFunc(stageRepos, func(a, b *util.StageRepo) int {
		return cmp.Compare(b.Updated, a.Updated)
	})

	staged := util.StageFile{Repos: make([]util.StageRepo, len(stageRepos))}
	for i, repo := range stageRepos {
		staged.Repos[i] = *repo
		// hash 跳过 / 失败保留的旧条目也可能带有 "funding": {} 或冗余 locale，写回前一并清理。
		rules.ClearEmptyFunding(&staged.Repos[i].Package)
		rules.ClearRedundantLocales(&staged.Repos[i].Package)
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
	return abortedByRateLimit
}

// backfillUnprocessedStageRepos 在限流中止后，为尚未写入结果的列表仓库补回同路径旧条目，避免 stage JSON 丢仓。
func backfillUnprocessedStageRepos(reposSlice []string, stageRepos []*util.StageRepo, oldStageData map[string]*util.StageRepo) []*util.StageRepo {
	processed := make(Set, len(stageRepos))
	for _, repo := range stageRepos {
		if key, ok := util.OwnerRepoFromStageURL(repo.URL); ok {
			processed[key] = struct{}{}
		}
	}
	for _, ownerRepo := range reposSlice {
		if _, ok := processed[ownerRepo]; ok {
			continue
		}
		if old := oldStageData[ownerRepo]; old != nil {
			stageRepos = append(stageRepos, old)
			logger.Errorf("backfill old stage data for [%s] after rate limit abort", ownerRepo)
		}
	}
	return stageRepos
}

// resolvePackageZipSHA256 解析 package.zip 内容 SHA-256。
// 优先使用 GitHub asset digest；缺失时下载 zip 计算，并返回 zip 字节供后续入库复用。
func resolvePackageZipSHA256(ownerRepo string, release util.LatestRelease) (sha256Hex string, zipData []byte, err error) {
	if hex := util.NormalizeAssetDigest(release.PackageZipDigest); hex != "" {
		return hex, nil, nil
	}
	owner, name, cutOk := strings.Cut(ownerRepo, "/")
	if !cutOk {
		return "", nil, errInvalidOwnerRepo
	}
	data, downloadErr := util.DownloadPackageZip(githubContext, githubClient, owner, name, release.PackageZipAssetID)
	if downloadErr != nil {
		return "", nil, downloadErr
	}
	return util.SHA256Hex(data), data, nil
}

func repoStats(ownerRepo string) (stars, openIssues int, ok bool, err error) {
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
