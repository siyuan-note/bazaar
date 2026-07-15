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
5. 按 updated 降序排序后写出 stage/*.json（键序经 sortJSONKeys 稳定）
*/

type Set map[string]struct{} // 字符串集合

var (
	BAZAAR_ROOT_PATH        = "."              // bazaar 仓库根目录（stage 工作区）
	GITHUB_TOKEN            = os.Getenv("PAT") // GitHub Token
	STAGE_POOL_SIZE         = envIntDefault("STAGE_POOL_SIZE", 80)
	STAGE_HEAVY_CONCURRENCY = envIntDefault("STAGE_HEAVY_CONCURRENCY", 8) // 限制同时进行「下载 + 上传 OSS」的仓库数，避免大量更新时打满 GitHub/OSS；仅 hash 检查不受限。

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

	if err := checkRateLimitBeforeStage(); err != nil {
		logger.Fatalf("GitHub API rate limit check failed: %s", err)
	}

	// 每类 stage 前重新加载 OccupiedNames，以便上一类本轮新写入的 name 参与后续类型的唯一性检查。
	for _, packageType := range rules.AllPackageTypes() {
		occupiedNames, err := util.LoadOccupiedNames(BAZAAR_ROOT_PATH)
		if err != nil {
			logger.Fatalf("load occupied names failed: %s", err)
		}
		performStage(packageType, occupiedNames)
	}

	logger.Infof("Stage completed")
}

// apiCallsPerRepo 每个仓库 staging 时消耗的 GitHub REST API (core) 请求数经验值（repoStats 1 次 + getRepoLatestRelease 等）
const apiCallsPerRepo = 2.5

// checkRateLimitBeforeStage 统计本次待检查仓库数、请求 GitHub rate_limit（该请求不计入 core），若 core 剩余请求数不足则返回错误。参考 https://docs.github.com/zh/rest/rate-limit/rate-limit
func checkRateLimitBeforeStage() error {
	var repoCount int
	for _, packageType := range rules.AllPackageTypes() {
		repos, err := util.ParseReposFromTxt(packageType.ReposListFile())
		if err != nil {
			return fmt.Errorf("count staging repos: %w", err)
		}
		repoCount += len(repos)
	}
	required := int(math.Ceil(float64(repoCount) * apiCallsPerRepo))
	if required == 0 {
		return nil
	}
	if GITHUB_TOKEN == "" {
		return fmt.Errorf("env PAT is not set")
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

// sortJSONKeys 对 JSON 反序列化后按对象键排序再序列化（带缩进），保证输出键序稳定。
// 经 Unmarshal 到 any 后，对象均为 map[string]any；json.Marshal 会按字典序输出键。
// 先输出紧凑 JSON，再用 json.Indent 做缩进，避免 json.MarshalIndent 对嵌套 any 的缩进错乱。
func sortJSONKeys(data []byte) ([]byte, error) {
	var v any
	if err := json.Unmarshal(data, &v); nil != err {
		return nil, err
	}
	compact, err := marshalSortedCompact(v)
	if nil != err {
		return nil, err
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, compact, "", "  "); nil != err {
		return nil, err
	}
	return buf.Bytes(), nil
}

// marshalSortedCompact 序列化为紧凑 JSON；map 键由 json.Marshal 按字典序输出。
// SetEscapeHTML(false) 避免将 & 等字符转为 \uXXXX，与既有 stage 文件字符串格式一致。
func marshalSortedCompact(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); nil != err {
		return nil, err
	}
	return bytes.TrimSpace(buf.Bytes()), nil
}

func performStage(packageType rules.PackageType, occupiedNames map[string]struct{}) {
	logger.Infof("start stage [%s]", packageType.Plural())

	reposSlice, err := util.ParseReposFromTxt(packageType.ReposListFile())
	if nil != err {
		logger.Fatalf("read or parse [%s] failed: %s", packageType.ReposListFile(), err)
	}

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
	heavySem := make(chan struct{}, STAGE_HEAVY_CONCURRENCY)
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
			ok, skipped, hash, updated, size, installSize, pkg := indexPackage(ownerRepo, packageType, oldStageURL, oldRepo, allowThemeJS, occupiedNames, heavySem)
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
				URL:         ownerRepo + "@" + hash,
				Stars:       stars,
				OpenIssues:  openIssues,
				Updated:     updated,
				Size:        size,
				InstallSize: installSize,
				Package:     *pkg,
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

	data, err := gulu.JSON.MarshalIndentJSON(staged, "", "  ")
	if nil != err {
		logger.Fatalf("marshal stage [%s] failed: %s", packageType.StageJSONFile(), err)
	}
	data, err = sortJSONKeys(data)
	if nil != err {
		logger.Fatalf("sort stage [%s] keys failed: %s", packageType.StageJSONFile(), err)
	}

	stageFilePath := filepath.Join(BAZAAR_ROOT_PATH, "stage", packageType.StageJSONFile())
	if err = os.WriteFile(stageFilePath, data, 0644); nil != err {
		logger.Fatalf("write stage [%s] failed: %s", packageType.StageJSONFile(), err)
	}

	logger.Infof("finish stage [%s]", packageType.Plural())
}
