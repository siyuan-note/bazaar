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
	"crypto/tls"
	"encoding/json"
	"errors"
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
	jsoniter "github.com/json-iterator/go"
	"github.com/panjf2000/ants/v2"
	"github.com/parnurzeal/gorequest"
	"github.com/siyuan-note/bazaar/actions/util"
	"golang.org/x/mod/semver"
)

var (
	logger = gulu.Log.NewLogger(os.Stdout)
	// heavyStageSem 限制同时进行「下载 + 上传 OSS」的仓库数，避免大量更新时打满 GitHub/OSS；仅 hash 检查不受限。
	heavyStageSem  chan struct{}
	heavyStageOnce sync.Once
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

	if err := checkRateLimitBeforeStage(); err != nil {
		logger.Fatalf("GitHub API rate limit check failed: %v", err)
	}

	performStage("themes")
	performStage("templates")
	performStage("icons")
	performStage("widgets")
	performStage("plugins")

	logger.Infof("bazaar staged")
}

// apiCallsPerRepo 每个仓库 staging 时消耗的 GitHub REST API (core) 请求数经验值（repoStats 1 次 + getRepoLatestRelease 等）
const apiCallsPerRepo = 2.5

// checkRateLimitBeforeStage 统计本次待检查仓库数、请求 GitHub rate_limit（该请求不计入 core），若 core 剩余请求数不足则返回错误。参考 https://docs.github.com/zh/rest/rate-limit/rate-limit
func checkRateLimitBeforeStage() error {
	types := []string{"themes", "templates", "icons", "widgets", "plugins"}
	var repoCount int
	for _, typ := range types {
		repos, err := util.ParseReposFromTxt(typ + ".txt")
		if err != nil {
			return fmt.Errorf("count staging repos: %w", err)
		}
		repoCount += len(repos)
	}
	required := int(math.Ceil(float64(repoCount) * apiCallsPerRepo))
	if required == 0 {
		return nil
	}
	pat := os.Getenv("PAT")
	if pat == "" {
		return fmt.Errorf("env PAT is not set")
	}
	var out struct {
		Resources struct {
			Core struct {
				Limit     int   `json:"limit"`
				Remaining int   `json:"remaining"`
				Reset     int64 `json:"reset"`
			} `json:"core"`
		} `json:"resources"`
	}
	request := gorequest.New().TLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	resp, _, errs := request.Get("https://api.github.com/rate_limit").
		Set("Accept", "application/vnd.github+json").
		Set("Authorization", "Token "+pat).
		Set("User-Agent", util.UserAgent).
		Set("X-GitHub-Api-Version", "2022-11-28").
		Timeout(10 * time.Second).
		EndStruct(&out)
	if len(errs) > 0 {
		return fmt.Errorf("get rate limit: %w", errs[0])
	}
	if resp != nil && resp.StatusCode != 200 {
		return fmt.Errorf("rate_limit returned %d", resp.StatusCode)
	}
	remaining := out.Resources.Core.Remaining
	limit := out.Resources.Core.Limit
	reset := out.Resources.Core.Reset
	if remaining < required {
		return fmt.Errorf("GitHub REST API (core) remaining %d / %d is below required %d for %d repos (~%d requests); reset at %d", remaining, limit, required, repoCount, required, reset)
	}
	logger.Infof("GitHub API (core) remaining %d / %d, %d repos to check (~%d requests), OK", remaining, limit, repoCount, required)
	return nil
}

// loadOldStageData 加载现有的 stage 文件数据，返回以 owner/repo 为 key 的映射
func loadOldStageData(typ string) map[string]*StageRepo {
	oldStageData := make(map[string]*StageRepo)
	stageFilePath := "stage/" + typ + ".json"

	stageData, err := os.ReadFile(stageFilePath)
	if nil != err {
		return oldStageData
	}

	oldStaged := map[string]any{}
	if err = gulu.JSON.UnmarshalJSON(stageData, &oldStaged); nil != err {
		return oldStageData
	}

	oldRepos, ok := oldStaged["repos"].([]any)
	if !ok {
		return oldStageData
	}

	for _, repo := range oldRepos {
		repoMap, ok := repo.(map[string]any)
		if !ok {
			continue
		}

		url, ok := repoMap["url"].(string)
		if !ok {
			continue
		}

		// 从 URL 中提取 owner/repo（去掉 @hash 部分）
		idx := strings.Index(url, "@")
		if idx <= 0 {
			continue
		}

		repoKey := url[:idx]
		stageRepo := &StageRepo{}
		repoJSON, marshalErr := gulu.JSON.MarshalJSON(repoMap)
		if nil != marshalErr {
			continue
		}

		if err = gulu.JSON.UnmarshalJSON(repoJSON, stageRepo); nil != err {
			continue
		}

		oldStageData[repoKey] = stageRepo
	}

	return oldStageData
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

func performStage(typ string) {
	logger.Infof("staging [%s]", typ)

	reposSlice, err := util.ParseReposFromTxt(typ + ".txt")
	if nil != err {
		logger.Fatalf("read or parse [%s.txt] failed: %s", typ, err)
	}
	// 与后续 Invoke(arg) 的 arg.(string) 兼容，转为 []any
	repos := make([]any, len(reposSlice))
	for i, s := range reposSlice {
		repos[i] = s
	}

	oldStageData := loadOldStageData(typ)

	initHeavyStageSem()
	lock := sync.Mutex{}
	var stageRepos []any
	waitGroup := &sync.WaitGroup{}

	poolSize := getStagePoolSize()
	p, _ := ants.NewPoolWithFunc(poolSize, func(arg any) {
		defer waitGroup.Done()
		repo := arg.(string)
		var oldStageURL string
		var oldRepo *StageRepo
		if o, exists := oldStageData[repo]; exists {
			oldStageURL = o.URL
			oldRepo = o
		}
		ok, skipped, hash, updated, size, installSize, pkg := indexPackage(repo, typ, oldStageURL, oldRepo)
		if skipped {
			// hash 未变化，跳过下载，直接沿用旧 stage 条目
			lock.Lock()
			stageRepos = append(stageRepos, oldStageData[repo])
			lock.Unlock()
			return
		}
		if !ok || pkg == nil {
			// 索引失败或 pkg 为空时使用旧数据，避免 "package": null 的坏数据覆盖
			lock.Lock()
			if oldRepo, exists := oldStageData[repo]; exists {
				stageRepos = append(stageRepos, oldRepo)
				logger.Warnf("index failed for [%s], keeping old data", repo)
			} else {
				logger.Warnf("index failed for [%s] and no old data found", repo)
			}
			lock.Unlock()
			return
		}

		stars, openIssues, ok := repoStats(repo)
		// 如果获取统计数据失败，尝试使用旧数据
		if !ok {
			lock.Lock()
			if oldRepo, exists := oldStageData[repo]; exists {
				stageRepos = append(stageRepos, oldRepo)
				logger.Warnf("repoStats failed for [%s], keeping old data", repo)
			} else {
				logger.Warnf("repoStats failed for [%s] and no old data found", repo)
			}
			lock.Unlock()
			return
		}

		lock.Lock()
		defer lock.Unlock()
		stageRepos = append(stageRepos, &StageRepo{
			URL:         repo + "@" + hash,
			Stars:       stars,
			OpenIssues:  openIssues,
			Updated:     updated,
			Size:        size,
			InstallSize: installSize,
			Package:     pkg,
		})
		logger.Infof("updated repo [%s]", repo)
	})
	for _, repo := range repos {
		waitGroup.Add(1)
		p.Invoke(repo)
	}
	waitGroup.Wait()
	p.Release()

	sort.SliceStable(stageRepos, func(i, j int) bool {
		return stageRepos[i].(*StageRepo).Updated > stageRepos[j].(*StageRepo).Updated
	})

	staged := map[string]any{
		"repos": stageRepos,
	}

	data, err := gulu.JSON.MarshalIndentJSON(staged, "", "  ")
	if nil != err {
		logger.Fatalf("marshal stage [%s.json] failed: %s", typ, err)
	}
	data, err = sortJSONKeys(data)
	if nil != err {
		logger.Fatalf("sort stage [%s.json] keys failed: %s", typ, err)
	}

	if err = os.WriteFile("stage/"+typ+".json", data, 0644); nil != err {
		logger.Fatalf("write stage [%s.json] failed: %s", typ, err)
	}

	logger.Infof("staged [%s]", typ)
}

// parseHashFromStageURL 从 stage 条目的 URL（格式 owner/repo@hash）中解析出 hash 部分，若无 @ 或 @ 后为空则返回空字符串
func parseHashFromStageURL(stageURL string) string {
	idx := strings.Index(stageURL, "@")
	if idx < 0 || idx >= len(stageURL)-1 {
		return ""
	}
	return stageURL[idx+1:]
}

// requiredFilesByType 各类型集市包在包根目录下必须存在的文件（大小写敏感）
var requiredFilesByType = map[string][]string{
	"plugins":   {"plugin.json", "index.js"},
	"themes":    {"theme.json", "theme.css"},
	"icons":     {"icon.json", "icon.js"},
	"templates": {"template.json"},
	"widgets":   {"widget.json", "index.html"},
}

// validateUnzipRoot 检查解压根目录结构：正常情况下所有文件应在根目录；若根目录仅有一个子目录，则将该子目录视为包根目录。返回包根目录的绝对路径。
func validateUnzipRoot(unzipPath string) (packageRoot string, err error) {
	entries, err := os.ReadDir(unzipPath)
	if err != nil {
		return "", err
	}
	srcPath := unzipPath
	if len(entries) == 1 && entries[0].IsDir() {
		srcPath = filepath.Join(unzipPath, entries[0].Name())
	}
	return srcPath, nil
}

// fileExistsInDir 判断 dir 下是否存在名为 name 的文件或目录（大小写敏感，通过列目录比对）。
func fileExistsInDir(dir, name string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.Name() == name {
			return true
		}
	}
	return false
}

// validatePackageRoot 按类型检查包根目录下是否存在所有必要文件
func validatePackageRoot(packageRoot, typ string) error {
	required, ok := requiredFilesByType[typ]
	if !ok {
		return nil
	}
	for _, name := range required {
		if !fileExistsInDir(packageRoot, name) {
			return errors.New("missing required file: " + name)
		}
	}
	// 特殊检查
	if typ == "templates" {
		// 模板：至少包含一个模板文件
		var foundNonReadmeMd bool
		filepath.Walk(packageRoot, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			base := strings.ToLower(filepath.Base(path))
			if strings.HasSuffix(base, ".md") && !strings.HasPrefix(base, "readme") { // 跟思源内核判断逻辑一致
				foundNonReadmeMd = true
				return filepath.SkipAll
			}
			return nil
		})
		if !foundNonReadmeMd {
			return errors.New("template must contain at least one .md file not starting with readme (case-insensitive)")
		}
	}
	return nil
}

// getOldPackageFields 从已 stage 的条目中解析出 package 的 name、version
func getOldPackageFields(oldRepo *StageRepo) (name, version string) {
	if oldRepo == nil || oldRepo.Package == nil {
		return "", ""
	}
	m, ok := oldRepo.Package.(map[string]any)
	if !ok {
		return "", ""
	}
	if v, _ := m["name"].(string); v != "" {
		name = v
	}
	if v, _ := m["version"].(string); v != "" {
		version = v
	}
	return name, version
}

// packageValidationMeta 聚合元数据与校验所需上下文
type packageValidationMeta struct {
	repoURL     string
	packageRoot string
	typ         string
	basePkg     *Package
	oldRepo     *StageRepo
}

// validatePackageMetadata 校验元数据
//   - 如果涉及文件，文件名大小写敏感
func validatePackageMetadata(meta *packageValidationMeta) error {
	// 必须为 https://github.com/owner/repo ，禁止末尾斜杠或 .git 结尾
	expectedURL := "https://github.com/" + meta.repoURL
	if meta.basePkg.URL != expectedURL {
		return fmt.Errorf("url must be exactly %s (no trailing slash, no .git)", expectedURL)
	}

	// 不存在 oldRepo 时（新上架集市包），oldName 和 oldVersion 都为空
	oldName, oldVersion := getOldPackageFields(meta.oldRepo)

	if oldName != "" && meta.basePkg.Name != oldName {
		return fmt.Errorf("name must be identical to current stage: got %q, expected %q", meta.basePkg.Name, oldName)
	}
	if meta.basePkg.Name == "" {
		return fmt.Errorf("name is required")
	}

	newVer := meta.basePkg.Version
	if !strings.HasPrefix(newVer, "v") {
		newVer = "v" + newVer
	}
	// version 需为合法语义化版本，且若存在旧 version 则必须高于旧 version
	if !semver.IsValid(newVer) {
		return fmt.Errorf("version must be valid semver: %q", meta.basePkg.Version)
	}
	if oldVersion != "" {
		oldVer := oldVersion
		if !strings.HasPrefix(oldVer, "v") {
			oldVer = "v" + oldVer
		}
		if semver.Compare(newVer, oldVer) <= 0 {
			return fmt.Errorf("version must be higher than current stage: %s <= %s", meta.basePkg.Version, oldVersion)
		}
	}

	// readme：声明的 README 文件在解压后的包内存在（路径大小写敏感）
	if meta.basePkg.Readme != nil {
		for _, readmePath := range meta.basePkg.Readme {
			if readmePath == "" {
				continue
			}
			readmePath = strings.TrimSpace(readmePath) // 跟思源内核逻辑一致，TrimSpace
			// 必须是相对路径，不能以 / 开头
			if strings.HasPrefix(readmePath, "/") {
				return fmt.Errorf("readme path invalid: %q", readmePath)
			}
			// 禁止包含 ..，防止路径穿越
			if strings.Contains(readmePath, "..") {
				return fmt.Errorf("readme path invalid: %q", readmePath)
			}
			// 要求使用 / 作为分隔符，拒绝 \
			if strings.Contains(readmePath, "\\") {
				return fmt.Errorf("readme path invalid: %q", readmePath)
			}
			absPath := filepath.Join(meta.packageRoot, filepath.FromSlash(readmePath))
			info, err := os.Stat(absPath)
			if err != nil || info.IsDir() {
				return fmt.Errorf("readme file declared but not found in package: %s", readmePath)
			}
		}
	}

	// 插件：若存在 disabledInPublish 则校验为 bool（JSON 中该键存在时值必须为 bool，通过 raw 校验）
	if meta.typ == "plugins" {
		name := strings.TrimSuffix(meta.typ, "s")
		jsonPath := filepath.Join(meta.packageRoot, name+".json")
		data, err := os.ReadFile(jsonPath)
		if err != nil {
			return fmt.Errorf("read plugin.json for disabledInPublish check: %w", err)
		}
		var raw map[string]any
		if err := gulu.JSON.UnmarshalJSON(data, &raw); err != nil {
			return fmt.Errorf("unmarshal plugin.json: %w", err)
		}
		if v, has := raw["disabledInPublish"]; has && v != nil {
			if _, ok := v.(bool); !ok {
				return fmt.Errorf("disabledInPublish must be bool when present, got %T", v)
			}
		}
	}

	return nil
}

// indexPackage 索引包，返回的 pkg 为 *Package / *PluginPackage / *ThemePackage 之一。
// oldStageURL 为当前已 stage 的该仓库 URL（格式 owner/repo@hash），若与 Latest Release 的 hash 一致则跳过下载并返回 skipped=true。
// oldStageRepo 用于元数据校验时与旧 name/url/version 对比，可为 nil（如新仓库）。
func indexPackage(repoURL, typ, oldStageURL string, oldStageRepo *StageRepo) (ok, skipped bool, hash, published string, size, installSize int64, pkg any) {
	// 获取该仓库 Latest Release 信息（hash、发布时间、package.zip 下载地址）
	hash, published, packageZip, releaseOk := getRepoLatestRelease(repoURL)
	if !releaseOk {
		logger.Warnf("get [%s] latest release failed", repoURL)
		return
	}

	// Latest Release 的 hash 与已 stage 的 hash 一致则跳过，不下载、不更新，沿用旧条目
	if oldStageURL != "" {
		oldHash := parseHashFromStageURL(oldStageURL)
		if oldHash != "" && hash == oldHash {
			logger.Infof("skip repo [%s], hash unchanged [%s]", repoURL, hash)
			skipped = true
			return
		}
	}

	// 限制同时进行「下载 + 上传」的仓库数
	heavyStageSem <- struct{}{}
	defer func() { <-heavyStageSem }()

	// 下载 package.zip
	resp, data, errs := gorequest.New().Get(packageZip).
		Set("User-Agent", util.UserAgent).
		Retry(1, 3*time.Second).Timeout(30 * time.Second).EndBytes()
	if nil != errs {
		logger.Errorf("get [%s] failed: %s", packageZip, errs)
		return
	}
	if 200 != resp.StatusCode {
		logger.Errorf("get [%s] failed: %d", packageZip, resp.StatusCode)
		return
	}

	// 记录 zip 体积，用于 stage 条目的 size 字段
	size = int64(len(data))

	// 将 zip 写入临时文件并解压到临时目录
	installSize = size
	var err error
	osTmpDir := filepath.Join(os.TempDir(), "bazaar")
	if err = os.MkdirAll(osTmpDir, 0755); nil != err {
		logger.Errorf("mkdir [%s] failed: %s", osTmpDir, err)
		return
	}
	tmpZipPath := filepath.Join(os.TempDir(), "bazaar", gulu.Rand.String(7)+".zip")
	if err = os.WriteFile(tmpZipPath, data, 0644); nil != err {
		logger.Errorf("write package.zip failed: %s", err)
		return
	}
	defer os.RemoveAll(tmpZipPath)

	tmpUnzipPath := filepath.Join(os.TempDir(), "bazaar", gulu.Rand.String(7))
	if err = gulu.Zip.Unzip(tmpZipPath, tmpUnzipPath); nil != err {
		logger.Errorf("unzip package.zip failed: %s", err)
		return
	}
	defer os.RemoveAll(tmpUnzipPath)

	// 校验解压根目录结构（仅有一个子目录视为打包错误），且包根下存在该类型必要文件
	packageRoot, err := validateUnzipRoot(tmpUnzipPath)
	if err != nil {
		logger.Warnf("validate unzip root [%s] failed: %s", repoURL, err)
		return
	}
	if err = validatePackageRoot(packageRoot, typ); err != nil {
		logger.Warnf("validate package root [%s] failed: %s", repoURL, err)
		return
	}

	// 计算解压后目录体积，用于 stage 条目的 installSize 字段
	installSize, err = util.SizeOfDirectory(packageRoot)
	if nil != err {
		logger.Errorf("stat package [%s] size failed: %s", repoURL, err)
		return
	}

	// 从解压目录读取元数据，以便根据 readme 字段收集要上传的文件
	var basePkg *Package
	pkg, basePkg = getPackage(packageRoot, typ)
	if nil == pkg || nil == basePkg {
		logger.Warnf("get package [%s] failed", repoURL)
		return
	}

	// 元数据校验：name/url/version/readme 及类型相关字段；失败则本轮索引失败，沿用旧数据
	if err := validatePackageMetadata(&packageValidationMeta{
		repoURL:     repoURL,
		packageRoot: packageRoot,
		typ:         typ,
		basePkg:     basePkg,
		oldRepo:     oldStageRepo,
	}); err != nil {
		logger.Warnf("validate package metadata [%s] failed: %v", repoURL, err)
		return
	}

	// 校验通过后再上传 package.zip，避免无效包写入 OSS
	key := "package/" + repoURL + "@" + hash
	if err := util.UploadOSS(key, "application/zip", data); nil != err {
		logger.Errorf("upload package [%s] failed: %s", repoURL, err)
		return
	}

	// 收集需要上传的 README 文件列表（根据包配置中的 readme 字段）
	readmeFiles := make(map[string]bool)
	if nil != basePkg.Readme {
		for _, readmePath := range basePkg.Readme {
			readmePath = strings.TrimSpace(readmePath) // 跟思源内核逻辑一致，TrimSpace
			if readmePath == "" {
				continue
			}
			readmeFiles["/"+readmePath] = true
		}
	}
	// 仅 README.md 始终加入上传列表（若包内存在则上传）
	readmeFiles["/README.md"] = true

	// 从解压目录读取 README、preview、icon、元数据 JSON 并并发上传到 OSS；任一份上传失败则整包视为失败
	var anyUploadFailed int32
	wg := &sync.WaitGroup{}
	wg.Add(3 + len(readmeFiles))
	for readmeFile := range readmeFiles {
		go indexPackageFile(repoURL, hash, packageRoot, readmeFile, 0, 0, wg, &anyUploadFailed)
	}
	go indexPackageFile(repoURL, hash, packageRoot, "/preview.png", 0, 0, wg, &anyUploadFailed)
	go indexPackageFile(repoURL, hash, packageRoot, "/icon.png", 0, 0, wg, &anyUploadFailed)
	go indexPackageFile(repoURL, hash, packageRoot, "/"+strings.TrimSuffix(typ, "s")+".json", size, installSize, wg, &anyUploadFailed)
	wg.Wait()
	if atomic.LoadInt32(&anyUploadFailed) != 0 {
		return
	}
	ok = true
	return
}

// getPackage 从解压后的包根目录 unzipRoot 读取该类型的元数据 JSON（如 plugin.json），按 typ 解析为 Package / PluginPackage / ThemePackage，并返回用于 Readme 等的 *Package。
func getPackage(unzipRoot, typ string) (pkgVal any, basePkg *Package) {
	name := strings.TrimSuffix(typ, "s")
	jsonPath := filepath.Join(unzipRoot, name+".json")
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		logger.Errorf("read [%s] failed: %s", jsonPath, err)
		return nil, nil
	}

	switch typ {
	case "plugins":
		p := &PluginPackage{Package: &Package{}}
		if err := gulu.JSON.UnmarshalJSON(data, p); nil != err {
			logger.Errorf("unmarshal [%s] failed: %s", jsonPath, err)
			return nil, nil
		}
		sanitizePackageDisplayStrings(p.Package)
		return p, p.Package
	case "themes":
		p := &ThemePackage{Package: &Package{}}
		if err := gulu.JSON.UnmarshalJSON(data, p); nil != err {
			logger.Errorf("unmarshal [%s] failed: %s", jsonPath, err)
			return nil, nil
		}
		sanitizePackageDisplayStrings(p.Package)
		return p, p.Package
	default:
		ret := &Package{}
		if err := gulu.JSON.UnmarshalJSON(data, ret); nil != err {
			logger.Errorf("unmarshal [%s] failed: %s", jsonPath, err)
			return nil, nil
		}
		sanitizePackageDisplayStrings(ret)
		return ret, ret
	}
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
			logger.Errorf("unmarshal package meta [%s] failed: %s", localPath, err)
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

func repoStats(repoURL string) (stars, openIssues int, ok bool) {
	result := map[string]any{}
	request := gorequest.New().TLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	pat := os.Getenv("PAT")
	u := "https://api.github.com/repos/" + repoURL
	resp, _, errs := request.Get(u).
		Set("Authorization", "Token "+pat).
		Set("User-Agent", util.UserAgent).Timeout(30*time.Second).
		Retry(1, 3*time.Second).EndStruct(&result)
	if nil != errs {
		logger.Warnf("get [%s] failed: %s", u, errs)
		return
	}
	if 200 != resp.StatusCode {
		logger.Warnf("get [%s] failed: %d", u, resp.StatusCode)
		return
	}

	//logger.Infof("X-Ratelimit-Remaining=%s]", resp.Header.Get("X-Ratelimit-Remaining"))
	stars = int(result["stargazers_count"].(float64))
	openIssues = int(result["open_issues_count"].(float64))
	ok = true
	return
}

// getRepoLatestRelease 获取仓库最新发布的版本
func getRepoLatestRelease(repoURL string) (hash, published, packageZip string, ok bool) {
	result := map[string]any{}
	request := gorequest.New().TLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	pat := os.Getenv("PAT")
	// REF https://docs.github.com/en/rest/releases/releases#get-the-latest-release
	u := "https://api.github.com/repos/" + repoURL + "/releases/latest"
	resp, _, errs := request.Get(u).
		Set("Authorization", "Token "+pat).
		Set("User-Agent", util.UserAgent).Timeout(30*time.Second).
		Retry(3, 3*time.Second).EndStruct(&result)
	if nil != errs {
		logger.Warnf("get release hash [%s] failed: %s", u, errs)
		return
	}
	if 200 != resp.StatusCode {
		logger.Warnf("get release hash [%s] failed: %d", u, resp.StatusCode)
		return
	}

	// 获取 package.zip 下载 url packageZip
	assets := result["assets"].([]any)
	if 0 < len(assets) {
		for _, asset := range assets {
			asset := asset.(map[string]any)
			if name := asset["name"].(string); "package.zip" == name {
				packageZip = asset["browser_download_url"].(string)
			}
		}
	}

	if "" == packageZip {
		logger.Warnf("get [%s] package.zip failed: package.zip not found in release assets", repoURL)
		return
	}

	// 获取 release 对应的 tag
	published = result["published_at"].(string)
	tagName := result["tag_name"].(string)
	if "" == tagName {
		logger.Warnf("get [%s] tag_name failed: tag_name is empty", repoURL)
		return
	}
	// REF https://docs.github.com/en/rest/git/refs#get-a-reference
	u = "https://api.github.com/repos/" + repoURL + "/git/ref/tags/" + tagName
	resp, _, errs = request.Get(u).
		Set("Authorization", "Token "+pat).
		Set("User-Agent", util.UserAgent).Timeout(30*time.Second).
		Retry(1, 3*time.Second).EndStruct(&result)
	if nil != errs {
		logger.Warnf("get release hash [%s] failed: %s", u, errs)
		return
	}
	if 200 != resp.StatusCode {
		logger.Warnf("get release hash [%s] failed: %d", u, resp.StatusCode)
		return
	}

	// 获取 release 对应的提交的 hash
	hash = result["object"].(map[string]any)["sha"].(string)
	if "" == hash {
		logger.Warnf("get [%s] release hash failed: hash is empty", repoURL)
		return
	}
	typ := result["object"].(map[string]any)["type"].(string)
	if "tag" == typ {
		// REF https://docs.github.com/en/rest/git/tags#get-a-tag
		u = "https://api.github.com/repos/" + repoURL + "/git/tags/" + hash
		resp, _, errs = request.Get(u).
			Set("Authorization", "Token "+pat).
			Set("User-Agent", util.UserAgent).Timeout(30*time.Second).
			Retry(1, 3*time.Second).EndStruct(&result)
		if nil != errs {
			logger.Warnf("get release hash [%s] failed: %s", u, errs)
			return
		}
		if 200 != resp.StatusCode {
			logger.Warnf("get release hash [%s] failed: %d", u, resp.StatusCode)
			return
		}

		hash = result["object"].(map[string]any)["sha"].(string)
		if "" == hash {
			logger.Warnf("get [%s] tag hash failed: hash is empty", repoURL)
			return
		}
	}
	ok = true
	return
}

// sanitizePackageDisplayStrings 对集市包直接显示的信息做 HTML 转义，避免 XSS。（跟思源内核代码保持一致）
func sanitizePackageDisplayStrings(pkg *Package) {
	if pkg == nil {
		return
	}
	for k, v := range pkg.DisplayName {
		pkg.DisplayName[k] = html.EscapeString(v)
	}
	for k, v := range pkg.Description {
		pkg.Description[k] = html.EscapeString(v)
	}
}

// LocaleStrings 表示按 locale 键（如 default、zh_CN、en_US）组织的多语言字符串
type LocaleStrings map[string]string

type Funding struct {
	OpenCollective string   `json:"openCollective"`
	Patreon        string   `json:"patreon"`
	GitHub         string   `json:"github"`
	Custom         []string `json:"custom"`
}

type Package struct {
	Name          string        `json:"name"`
	Author        string        `json:"author"`
	URL           string        `json:"url"`
	Version       string        `json:"version"`
	MinAppVersion string        `json:"minAppVersion"`
	DisplayName   LocaleStrings `json:"displayName"`
	Description   LocaleStrings `json:"description"`
	Readme        LocaleStrings `json:"readme"`
	Funding       *Funding      `json:"funding"`
	Keywords      []string      `json:"keywords"`
}

// PluginPackage 插件的 package
type PluginPackage struct {
	*Package
	Backends          []string `json:"backends"`
	Frontends         []string `json:"frontends"`
	DisabledInPublish bool     `json:"disabledInPublish"`
}

// ThemePackage 主题的 package
type ThemePackage struct {
	*Package
	Modes []string `json:"modes"`
}

type StageRepo struct {
	URL         string `json:"url"`
	Updated     string `json:"updated"`
	Stars       int    `json:"stars"`
	OpenIssues  int    `json:"openIssues"`
	Size        int64  `json:"size"`
	InstallSize int64  `json:"installSize"`

	// Package 为 *Package（模板/图标/挂件）、*PluginPackage（插件）或 *ThemePackage（主题）
	Package any `json:"package"`
}
