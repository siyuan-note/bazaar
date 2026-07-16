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
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/88250/gulu"
	"github.com/google/go-github/v89/github"
	"github.com/siyuan-note/bazaar/actions/util"
	"github.com/siyuan-note/bazaar/rules"
)

/*
Stage 流程：
1. 检查 GitHub API rate limit 是否足够覆盖本轮待索引仓库数
2. 按类型依次执行 performStage；每类开始前重新加载 OccupiedNames，以便上一类本轮新写入的 name 参与后续类型的唯一性检查
3. 读取 *s.txt 与既有 stage/*.json，并发索引各 owner/repo
4. hash 未变则跳过下载；否则下载 package.zip → rules.Check → 上传 OSS（package.zip、README、preview、icon、清单 JSON）
5. 按 updated 降序排序后写出 stage/*.json（键序经 marshalSortedIndentedJSON 稳定）
*/

type Set map[string]struct{} // 字符串集合

var (
	BAZAAR_ROOT_PATH = "."              // bazaar 仓库根目录（stage 工作区）
	GITHUB_TOKEN     = os.Getenv("PAT") // GitHub Token
	STAGE_POOL_SIZE  = envIntDefault("STAGE_POOL_SIZE", 80)

	REQUEST_TIMEOUT = 30 * time.Second // 请求超时时间

	logger        = gulu.Log.NewLogger(os.Stdout)
	githubContext = context.Background()
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
	for _, packageType := range rules.AllPackageTypes() {
		occupiedNames, err := util.LoadOccupiedNames(BAZAAR_ROOT_PATH)
		if err != nil {
			logger.Fatalf("load occupied names failed: %s", err)
		}
		performStage(packageType, occupiedNames, reposByType[packageType])
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
	if nil != err {
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

func performStage(packageType rules.PackageType, occupiedNames map[string]struct{}, reposSlice []string) {
	logger.Infof("start stage [%s]", packageType.Plural())

	oldStageData, err := loadOldStageData(packageType)
	if nil != err {
		logger.Fatalf("load old stage [%s] failed: %s", packageType.Plural(), err)
	}

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
	waitGroup := &sync.WaitGroup{}
	sem := make(chan struct{}, STAGE_POOL_SIZE)

	for _, ownerRepo := range reposSlice {
		waitGroup.Add(1)
		go func(ownerRepo string) {
			defer waitGroup.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			var oldStageURL string
			var oldRepo *util.StageRepo
			if o, exists := oldStageData[ownerRepo]; exists {
				oldStageURL = o.URL
				oldRepo = o
			}
			var allowThemeJS bool
			if packageType == rules.TypeTheme {
				_, allowThemeJS = themeJsAllowSet[ownerRepo]
			}
			ok, skipped, hash, updated, size, installSize, packageZipAssetID, pkg := indexPackage(ownerRepo, packageType, oldStageURL, oldRepo, allowThemeJS, occupiedNames)
			if skipped {
				// hash 未变化，跳过下载，直接沿用旧 stage 条目
				stageReposMu.Lock()
				stageRepos = append(stageRepos, oldStageData[ownerRepo])
				stageReposMu.Unlock()
				return
			}
			if !ok || pkg == nil {
				// 索引失败或 pkg 为空时使用旧数据，避免 "package": null 的坏数据覆盖
				stageReposMu.Lock()
				if oldRepo, exists := oldStageData[ownerRepo]; exists {
					stageRepos = append(stageRepos, oldRepo)
					logger.Errorf("index failed for [%s], keeping old data", ownerRepo)
				} else {
					logger.Errorf("index failed for [%s] and no old data found", ownerRepo)
				}
				stageReposMu.Unlock()
				return
			}

			stars, openIssues, ok := repoStats(ownerRepo)
			// 如果获取统计数据失败，尝试使用旧数据
			if !ok {
				stageReposMu.Lock()
				if oldRepo, exists := oldStageData[ownerRepo]; exists {
					stageRepos = append(stageRepos, oldRepo)
					logger.Errorf("repoStats failed for [%s], keeping old data", ownerRepo)
				} else {
					logger.Errorf("repoStats failed for [%s] and no old data found", ownerRepo)
				}
				stageReposMu.Unlock()
				return
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
			logger.Infof("updated repo [%s]", ownerRepo)
		}(ownerRepo)
	}
	waitGroup.Wait()

	sort.SliceStable(stageRepos, func(i, j int) bool {
		return stageRepos[i].Updated > stageRepos[j].Updated
	})

	staged := util.StageFile{Repos: make([]util.StageRepo, len(stageRepos))}
	for i, repo := range stageRepos {
		staged.Repos[i] = *repo
	}

	data, err := marshalSortedIndentedJSON(staged)
	if nil != err {
		logger.Fatalf("marshal stage [%s] failed: %s", packageType.StageJSONFile(), err)
	}

	stageFilePath := filepath.Join(BAZAAR_ROOT_PATH, "stage", packageType.StageJSONFile())
	if err = os.WriteFile(stageFilePath, data, 0644); nil != err {
		logger.Fatalf("write stage [%s] failed: %s", packageType.StageJSONFile(), err)
	}

	logger.Infof("finish stage [%s]", packageType.Plural())
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
		if nil != err {
			return nil, err
		}
	}
	var tree any
	if err := json.Unmarshal(data, &tree); nil != err {
		return nil, err
	}
	var compact bytes.Buffer
	enc := json.NewEncoder(&compact)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(tree); nil != err {
		return nil, err
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, bytes.TrimSpace(compact.Bytes()), "", "  "); nil != err {
		return nil, err
	}
	return buf.Bytes(), nil
}
