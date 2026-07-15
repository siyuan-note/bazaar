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
	"html"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/88250/gulu"
	"github.com/google/go-github/v89/github"
	jsoniter "github.com/json-iterator/go"
	"github.com/siyuan-note/bazaar/actions/util"
	"github.com/siyuan-note/bazaar/check"
)

var (
	logger = gulu.Log.NewLogger(os.Stdout)
	// heavyStageSem 限制同时进行「下载 + 上传 OSS」的仓库数，避免大量更新时打满 GitHub/OSS；仅 hash 检查不受限。
	heavyStageSem  chan struct{}
	heavyStageOnce sync.Once

	githubContext = context.Background()
	githubClient  *github.Client
)

// getStagePoolSize 从环境变量 STAGE_POOL_SIZE 读取并发池大小，默认 80（接近 GitHub API 的并发限制），以在多数为 skip 时加快检查。
func getStagePoolSize() int {
	const defaultPool = 80
	if s := os.Getenv("STAGE_POOL_SIZE"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return n
		}
	}
	return defaultPool
}

// getStageHeavyConcurrency 从环境变量 STAGE_HEAVY_CONCURRENCY 读取重活（下载+上传）并发上限，默认 8。
func getStageHeavyConcurrency() int {
	const defaultHeavy = 8
	if s := os.Getenv("STAGE_HEAVY_CONCURRENCY"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return n
		}
	}
	return defaultHeavy
}

func initHeavyStageSem() {
	heavyStageOnce.Do(func() {
		heavyStageSem = make(chan struct{}, getStageHeavyConcurrency())
	})
}

func main() {
	logger.Infof("bazaar is staging...")

	var err error
	githubClient, err = util.NewGitHubClient(os.Getenv("PAT"), 30*time.Second)
	if err != nil {
		logger.Fatalf("create github client failed: %v", err)
	}

	if err := checkRateLimitBeforeStage(); err != nil {
		logger.Fatalf("GitHub API rate limit check failed: %v", err)
	}

	// 每类 stage 前重新加载 OccupiedNames，以便上一类本轮新写入的 name 参与后续类型的唯一性检查。
	for _, packageType := range check.AllPackageTypes() {
		occupiedNames, err := util.LoadOccupiedNames(".")
		if err != nil {
			logger.Fatalf("load occupied names failed: %v", err)
		}
		performStage(packageType, occupiedNames)
	}

	logger.Infof("bazaar staged")
}

// apiCallsPerRepo 每个仓库 staging 时消耗的 GitHub REST API (core) 请求数经验值（repoStats 1 次 + getRepoLatestRelease 等）
const apiCallsPerRepo = 2.5

// checkRateLimitBeforeStage 统计本次待检查仓库数、请求 GitHub rate_limit（该请求不计入 core），若 core 剩余请求数不足则返回错误。参考 https://docs.github.com/zh/rest/rate-limit/rate-limit
func checkRateLimitBeforeStage() error {
	var repoCount int
	for _, packageType := range check.AllPackageTypes() {
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
	if os.Getenv("PAT") == "" {
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
func loadOldStageData(packageType check.PackageType) (map[string]*util.StageRepo, error) {
	oldStageData := make(map[string]*util.StageRepo)
	stageFilePath := "stage/" + packageType.StageJSONFile()

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

// jsoniterSortKeys 使用 json-iterator 的 SortMapKeys 配置，固定键的顺序。
var jsoniterSortKeys = jsoniter.Config{SortMapKeys: true}.Froze()

// sortJSONKeys 对 JSON 反序列化后按对象键排序再序列化（带缩进），保证输出键序稳定。
// 使用 jsoniter 输出紧凑 JSON（键已排序），再用标准库 json.Indent 做缩进，避免 jsoniter.MarshalIndent 对嵌套 any 的缩进错乱。
func sortJSONKeys(data []byte) ([]byte, error) {
	var v any
	if err := jsoniterSortKeys.Unmarshal(data, &v); nil != err {
		return nil, err
	}
	compact, err := jsoniterSortKeys.Marshal(v)
	if nil != err {
		return nil, err
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, compact, "", "  "); nil != err {
		return nil, err
	}
	return buf.Bytes(), nil
}

func performStage(packageType check.PackageType, occupiedNames map[string]struct{}) {
	logger.Infof("staging [%s]", packageType.Plural())

	reposSlice, err := util.ParseReposFromTxt(packageType.ReposListFile())
	if nil != err {
		logger.Fatalf("read or parse [%s] failed: %s", packageType.ReposListFile(), err)
	}

	oldStageData, err := loadOldStageData(packageType)
	if nil != err {
		logger.Fatalf("load old stage [%s] failed: %s", packageType.Plural(), err)
	}

	var themeJsAllowSet map[string]struct{}
	if packageType == check.TypeTheme {
		paths, err := util.ParseReposFromTxt(util.ThemeJsAllowlistRelPath)
		if err != nil {
			logger.Fatalf("read or parse [%s] failed: %s", util.ThemeJsAllowlistRelPath, err)
		}
		themeJsAllowSet = make(map[string]struct{}, len(paths))
		for _, p := range paths {
			themeJsAllowSet[p] = struct{}{}
		}
	}

	initHeavyStageSem()
	lock := sync.Mutex{}
	var stageRepos []*util.StageRepo
	waitGroup := &sync.WaitGroup{}

	poolSize := getStagePoolSize()
	sem := make(chan struct{}, poolSize)
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
			ok, skipped, hash, updated, size, installSize, pkg := indexPackage(ownerRepo, packageType, oldStageURL, oldRepo, themeJsAllowSet, occupiedNames)
			if skipped {
				// hash 未变化，跳过下载，直接沿用旧 stage 条目
				lock.Lock()
				stageRepos = append(stageRepos, oldStageData[ownerRepo])
				lock.Unlock()
				return
			}
			if !ok || pkg == nil {
				// 索引失败或 pkg 为空时使用旧数据，避免 "package": null 的坏数据覆盖
				lock.Lock()
				if oldRepo, exists := oldStageData[ownerRepo]; exists {
					stageRepos = append(stageRepos, oldRepo)
					logger.Warnf("index failed for [%s], keeping old data", ownerRepo)
				} else {
					logger.Warnf("index failed for [%s] and no old data found", ownerRepo)
				}
				lock.Unlock()
				return
			}

			stars, openIssues, ok := repoStats(ownerRepo)
			// 如果获取统计数据失败，尝试使用旧数据
			if !ok {
				lock.Lock()
				if oldRepo, exists := oldStageData[ownerRepo]; exists {
					stageRepos = append(stageRepos, oldRepo)
					logger.Warnf("repoStats failed for [%s], keeping old data", ownerRepo)
				} else {
					logger.Warnf("repoStats failed for [%s] and no old data found", ownerRepo)
				}
				lock.Unlock()
				return
			}

			lock.Lock()
			defer lock.Unlock()
			stageRepos = append(stageRepos, &util.StageRepo{
				URL:         ownerRepo + "@" + hash,
				Stars:       stars,
				OpenIssues:  openIssues,
				Updated:     updated,
				Size:        size,
				InstallSize: installSize,
				Package:     *pkg,
			})
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

	if err = os.WriteFile("stage/"+packageType.StageJSONFile(), data, 0644); nil != err {
		logger.Fatalf("write stage [%s] failed: %s", packageType.StageJSONFile(), err)
	}

	logger.Infof("staged [%s]", packageType.Plural())
}

// parseHashFromStageURL 从 stage 条目的 URL（格式 owner/repo@hash）中解析出 hash 部分，若无 @ 或 @ 后为空则返回空字符串
func parseHashFromStageURL(stageURL string) string {
	idx := strings.Index(stageURL, "@")
	if idx < 0 || idx >= len(stageURL)-1 {
		return ""
	}
	return stageURL[idx+1:]
}

// getOldPackageFields 从已 stage 的条目中解析出 package 的 name、version
func getOldPackageFields(oldRepo *util.StageRepo) (name, version string) {
	if oldRepo == nil {
		return "", ""
	}
	return oldRepo.Package.Name, oldRepo.Package.Version
}

// indexPackage 索引包，返回的 pkg 为解析后的 StagePackage。
// oldStageURL 为当前已 stage 的该仓库 URL（格式 owner/repo@hash），若与 Latest Release 的 hash 一致则跳过下载并返回 skipped=true。
// oldStageRepo 用于清单校验时与旧 name/version 对比，可为 nil（如新仓库）。
// themeJsAllowSet 仅主题为 themes 时使用；为 nil 或未包含本仓库时 AllowThemeJS 为 false（禁止 theme.js），仅白名单内为 true。
// occupiedNames 为已占用 package.name 集合，供 check.Check 做跨类型唯一性检查。
func indexPackage(ownerRepo string, packageType check.PackageType, oldStageURL string, oldStageRepo *util.StageRepo, themeJsAllowSet map[string]struct{}, occupiedNames map[string]struct{}) (ok, skipped bool, hash, published string, size, installSize int64, pkg *util.StagePackage) {
	// 获取该仓库 Latest Release 信息（hash、发布时间、package.zip asset id）
	hash, published, packageZipAssetID, releaseOk := getRepoLatestRelease(ownerRepo)
	if !releaseOk {
		logger.Warnf("get [%s] latest release failed", ownerRepo)
		return
	}

	// Latest Release 的 hash 与已 stage 的 hash 一致则跳过，不下载、不更新，沿用旧条目
	if oldStageURL != "" {
		oldHash := parseHashFromStageURL(oldStageURL)
		if oldHash != "" && hash == oldHash {
			logger.Infof("skip repo [%s], hash unchanged [%s]", ownerRepo, hash)
			skipped = true
			return
		}
	}

	owner, name, parseOk := splitOwnerRepo(ownerRepo)
	if !parseOk {
		logger.Warnf("download/unzip [%s] failed: invalid owner/repo", ownerRepo)
		return
	}

	// 限制同时进行「下载 + 上传」的仓库数
	heavyStageSem <- struct{}{}
	defer func() { <-heavyStageSem }()

	tmpUnzipPath, data, cleanup, err := util.DownloadAndUnzipPackageZip(githubContext, githubClient, owner, name, packageZipAssetID)
	if err != nil {
		logger.Errorf("download/unzip [%s] asset %d failed: %s", ownerRepo, packageZipAssetID, err)
		return
	}
	defer cleanup()

	// 记录 zip 体积，用于 stage 条目的 size 字段
	size = int64(len(data))
	installSize = size

	oldName, oldVersion := getOldPackageFields(oldStageRepo)
	_, allowThemeJS := themeJsAllowSet[ownerRepo]
	result := check.Check(check.Input{
		PackageRoot:   tmpUnzipPath,
		OwnerRepo:     ownerRepo,
		Type:          packageType,
		OldName:       oldName,
		OldVersion:    oldVersion,
		OccupiedNames: occupiedNames,
		AllowThemeJS:  allowThemeJS,
	})
	if !result.OK {
		for _, issue := range result.Issues {
			logger.Warnf("check [%s] failed: [%s] %s", ownerRepo, issue.Rule, issue.MessageZh)
		}
		return
	}

	packageRoot := result.PackageRoot
	if packageRoot == "" {
		packageRoot = tmpUnzipPath
	}

	// 计算解压后目录体积，用于 stage 条目的 installSize 字段
	installSize, err = util.SizeOfDirectory(packageRoot)
	if nil != err {
		logger.Errorf("stat package [%s] size failed: %s", ownerRepo, err)
		return
	}

	// 从解压目录读取清单，以便根据 readme 字段收集要上传的文件
	pkg = getPackage(packageRoot, packageType)
	if nil == pkg {
		logger.Warnf("get package [%s] failed", ownerRepo)
		return
	}

	// 校验通过后再上传 package.zip，避免无效包写入 OSS
	key := "package/" + ownerRepo + "@" + hash
	if err := util.UploadOSS(key, "application/zip", data); nil != err {
		logger.Errorf("upload package [%s] failed: %s", ownerRepo, err)
		return
	}

	// 收集需要上传的 README 文件列表（根据包配置中的 readme 字段）
	readmeFiles := make(map[string]bool)
	if nil != pkg.Readme {
		for _, readmePath := range pkg.Readme {
			readmePath = strings.TrimSpace(readmePath) // 跟思源内核逻辑一致，TrimSpace
			if readmePath == "" {
				continue
			}
			readmeFiles["/"+readmePath] = true
		}
	}
	// 仅 README.md 始终加入上传列表（若包内存在则上传）
	readmeFiles["/README.md"] = true

	// 从解压目录读取 README、preview、icon、清单 JSON 并并发上传到 OSS；任一份上传失败则整包视为失败
	var anyUploadFailed int32
	wg := &sync.WaitGroup{}
	wg.Add(3 + len(readmeFiles))
	for readmeFile := range readmeFiles {
		go indexPackageFile(ownerRepo, hash, packageRoot, readmeFile, 0, 0, wg, &anyUploadFailed)
	}
	go indexPackageFile(ownerRepo, hash, packageRoot, "/preview.png", 0, 0, wg, &anyUploadFailed)
	go indexPackageFile(ownerRepo, hash, packageRoot, "/icon.png", 0, 0, wg, &anyUploadFailed)
	go indexPackageFile(ownerRepo, hash, packageRoot, "/"+packageType.ManifestFile(), size, installSize, wg, &anyUploadFailed)
	wg.Wait()
	if atomic.LoadInt32(&anyUploadFailed) != 0 {
		return
	}
	ok = true
	return
}

// getPackage 从解压后的包根目录 unzipRoot 读取该类型的清单 JSON（如 plugin.json），解析为 StagePackage。
func getPackage(unzipRoot string, packageType check.PackageType) *util.StagePackage {
	jsonPath := filepath.Join(unzipRoot, packageType.ManifestFile())
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		logger.Errorf("read [%s] failed: %s", jsonPath, err)
		return nil
	}

	pkg := &util.StagePackage{}
	if err := gulu.JSON.UnmarshalJSON(data, pkg); nil != err {
		logger.Errorf("unmarshal [%s] failed: %s", jsonPath, err)
		return nil
	}
	sanitizePackageDisplayStrings(pkg)
	return pkg
}

// indexPackageFile 从解压后的包根目录 unzipRoot 读取 filePath 对应文件（大小写敏感），上传到 OSS；可选文件不存在时仅记录并跳过，其它失败时设置 anyFail。filePath 为相对包根的路径，如 /README.md、/icon.png。
func indexPackageFile(ownerRepo, hash, unzipRoot, filePath string, size, installSize int64, wg *sync.WaitGroup, anyFail *int32) {
	defer wg.Done()

	relPath := strings.TrimPrefix(filepath.ToSlash(filePath), "/")
	localPath := filepath.Join(unzipRoot, filepath.FromSlash(relPath))
	data, err := os.ReadFile(localPath)
	if err != nil {
		// 可选文件（如部分 README、preview.png）可能不存在，仅记录并跳过，不导致整包失败
		if os.IsNotExist(err) {
			logger.Warnf("file not found in package, skip upload [%s]", relPath)
			return
		}
		logger.Errorf("read [%s] failed: %s", localPath, err)
		atomic.StoreInt32(anyFail, 1)
		return
	}

	// 规范化为 /path 形式用于 OSS key
	normPath := "/" + filepath.ToSlash(relPath)

	var contentType string
	if strings.HasSuffix(normPath, ".md") {
		contentType = "text/markdown"
	} else if strings.HasSuffix(normPath, ".png") {
		contentType = "image/png"
	} else if strings.HasSuffix(normPath, ".json") {
		contentType = "application/json"
		// 注入 size/installSize 到清单 JSON
		meta := map[string]any{}
		if err := gulu.JSON.UnmarshalJSON(data, &meta); err != nil {
			logger.Errorf("unmarshal manifest [%s] failed: %s", localPath, err)
			atomic.StoreInt32(anyFail, 1)
			return
		}
		meta["size"] = size
		meta["installSize"] = installSize
		data, err = gulu.JSON.MarshalIndentJSON(meta, "", "  ")
		if err != nil {
			logger.Errorf("marshal package [%s] meta json failed: %s", localPath, err)
			atomic.StoreInt32(anyFail, 1)
			return
		}
	}

	key := "package/" + ownerRepo + "@" + hash + normPath
	if err := util.UploadOSS(key, contentType, data); err != nil {
		logger.Errorf("upload package file [%s] failed: %s", key, err)
		atomic.StoreInt32(anyFail, 1)
		return
	}
}

func repoStats(ownerRepo string) (stars, openIssues int, ok bool) {
	owner, name, parseOk := splitOwnerRepo(ownerRepo)
	if !parseOk {
		logger.Warnf("get [%s] failed: invalid owner/repo", ownerRepo)
		return
	}
	ctx, cancel := context.WithTimeout(githubContext, 30*time.Second)
	defer cancel()
	repo, _, err := githubClient.Repositories.Get(ctx, owner, name)
	if err != nil {
		logger.Warnf("get [%s] failed: %s", ownerRepo, err)
		return
	}
	stars = repo.GetStargazersCount()
	openIssues = repo.GetOpenIssuesCount()
	ok = true
	return
}

// getRepoLatestRelease 获取仓库最新发布的版本
func getRepoLatestRelease(ownerRepo string) (hash, published string, packageZipAssetID int64, ok bool) {
	owner, name, parseOk := splitOwnerRepo(ownerRepo)
	if !parseOk {
		logger.Warnf("get [%s] latest release failed: invalid owner/repo", ownerRepo)
		return
	}
	ctx, cancel := context.WithTimeout(githubContext, 30*time.Second)
	defer cancel()

	// REF https://docs.github.com/en/rest/releases/releases#get-the-latest-release
	release, _, err := githubClient.Repositories.GetLatestRelease(ctx, owner, name)
	if err != nil {
		logger.Warnf("get release [%s] failed: %s", ownerRepo, err)
		return
	}

	for _, asset := range release.Assets {
		if asset.GetName() == "package.zip" {
			packageZipAssetID = asset.GetID()
			break
		}
	}
	if packageZipAssetID == 0 {
		logger.Warnf("get [%s] package.zip failed: package.zip not found in release assets", ownerRepo)
		return
	}

	published = release.GetPublishedAt().Format(time.RFC3339)
	tagName := release.GetTagName()
	if tagName == "" {
		logger.Warnf("get [%s] tag_name failed: tag_name is empty", ownerRepo)
		return
	}

	// REF https://pkg.go.dev/github.com/google/go-github/v89/github#GitService.GetRef
	ref, _, err := githubClient.Git.GetRef(ctx, owner, name, "tags/"+tagName)
	if err != nil {
		logger.Warnf("get release hash [%s] tag [%s] failed: %s", ownerRepo, tagName, err)
		return
	}

	hash = ref.GetObject().GetSHA()
	if hash == "" {
		logger.Warnf("get [%s] release hash failed: hash is empty", ownerRepo)
		return
	}
	switch ref.GetObject().GetType() {
	case "commit":
		// 轻量 tag，object.sha 即为 commit
	case "tag":
		// REF https://pkg.go.dev/github.com/google/go-github/v89/github#GitService.GetTag
		tag, _, err := githubClient.Git.GetTag(ctx, owner, name, hash)
		if err != nil {
			logger.Warnf("get release hash [%s] tag [%s:%s] failed: %s", ownerRepo, tagName, hash, err)
			return
		}
		hash = tag.GetObject().GetSHA()
		if hash == "" {
			logger.Warnf("get [%s] tag hash failed: hash is empty", ownerRepo)
			return
		}
	default:
		logger.Warnf("get [%s] release hash failed: unknown ref type [%s]", ownerRepo, ref.GetObject().GetType())
		return
	}
	ok = true
	return
}

// splitOwnerRepo 将 "owner/repo" 拆成 owner 与 repo。
func splitOwnerRepo(ownerRepo string) (owner, repo string, ok bool) {
	parts := strings.Split(ownerRepo, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// sanitizePackageDisplayStrings 对集市包直接可能显示的信息做 HTML 转义，避免 XSS。（跟思源内核 kernel/bazaar/package.go 保持一致）
// 思源旧版本没有转义，为了避免旧版本受到攻击，必须在线上进行转义。
func sanitizePackageDisplayStrings(pkg *util.StagePackage) {
	if pkg == nil {
		return
	}
	pkg.Name = html.EscapeString(pkg.Name)
	pkg.Author = html.EscapeString(pkg.Author)
	pkg.Version = html.EscapeString(pkg.Version)
	for k, v := range pkg.DisplayName {
		pkg.DisplayName[k] = html.EscapeString(v)
	}
	for k, v := range pkg.Description {
		pkg.Description[k] = html.EscapeString(v)
	}
	if pkg.Funding != nil {
		pkg.Funding.OpenCollective = html.EscapeString(pkg.Funding.OpenCollective)
		pkg.Funding.Patreon = html.EscapeString(pkg.Funding.Patreon)
		pkg.Funding.GitHub = html.EscapeString(pkg.Funding.GitHub)
		for i, v := range pkg.Funding.Custom {
			pkg.Funding.Custom[i] = html.EscapeString(v)
		}
	}
	for i, kw := range pkg.Keywords {
		pkg.Keywords[i] = html.EscapeString(kw)
	}
}
