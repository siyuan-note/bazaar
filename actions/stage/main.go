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
	"crypto/tls"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/88250/gulu"
	"github.com/microcosm-cc/bluemonday"
	"github.com/panjf2000/ants/v2"
	"github.com/parnurzeal/gorequest"
	"github.com/siyuan-note/bazaar/actions/util"
)

var (
	logger     = gulu.Log.NewLogger(os.Stdout)
	sterilizer = bluemonday.UGCPolicy()
)

func main() {
	logger.Infof("bazaar is staging...")

	performStage("themes")
	performStage("templates")
	performStage("icons")
	performStage("widgets")
	performStage("plugins")

	logger.Infof("bazaar staged")
}

// loadOldStageData 加载现有的 stage 文件数据，返回以 owner/repo 为 key 的映射
func loadOldStageData(typ string) map[string]*StageRepo {
	oldStageData := make(map[string]*StageRepo)
	stageFilePath := "stage/" + typ + ".json"

	stageData, err := os.ReadFile(stageFilePath)
	if nil != err {
		return oldStageData
	}

	oldStaged := map[string]interface{}{}
	if err = gulu.JSON.UnmarshalJSON(stageData, &oldStaged); nil != err {
		return oldStageData
	}

	oldRepos, ok := oldStaged["repos"].([]interface{})
	if !ok {
		return oldStageData
	}

	for _, repo := range oldRepos {
		repoMap, ok := repo.(map[string]interface{})
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

func performStage(typ string) {
	logger.Infof("staging [%s]", typ)

	data, err := os.ReadFile(typ + ".json") // 读取配置文件
	if nil != err {
		logger.Fatalf("read [%s.json] failed: %s", typ, err)
	}

	original := map[string]interface{}{} // 解析配置文件
	if err = gulu.JSON.UnmarshalJSON(data, &original); nil != err {
		logger.Fatalf("unmarshal [%s.json] failed: %s", typ, err)
	}

	repos := original["repos"].([]interface{})

	oldStageData := loadOldStageData(typ)

	lock := sync.Mutex{}
	var stageRepos []interface{}
	waitGroup := &sync.WaitGroup{}

	p, _ := ants.NewPoolWithFunc(8, func(arg interface{}) {
		defer waitGroup.Done()
		repo := arg.(string)
		var hash, updated string
		var size, installSize int64
		var ok bool
		var pkg interface{}

		ok, hash, updated, size, installSize, pkg = indexPackage(repo, typ)
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

	staged := map[string]interface{}{
		"repos": stageRepos,
	}

	data, err = gulu.JSON.MarshalIndentJSON(staged, "", "  ")
	if nil != err {
		logger.Fatalf("marshal stage [%s.json] failed: %s", typ, err)
	}

	if err = os.WriteFile("stage/"+typ+".json", data, 0644); nil != err {
		logger.Fatalf("write stage [%s.json] failed: %s", typ, err)
	}

	logger.Infof("staged [%s]", typ)
}

// indexPackage 索引包，返回的 pkg 为 *Package / *PluginPackage / *ThemePackage 之一
func indexPackage(repoURL, typ string) (ok bool, hash, published string, size, installSize int64, pkg interface{}) {
	hash, published, packageZip, releaseOk := getRepoLatestRelease(repoURL)
	if !releaseOk {
		logger.Warnf("get [%s] latest release failed", repoURL)
		return
	}

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

	// 将 package.zip 上传到 OSS
	key := "package/" + repoURL + "@" + hash
	err := util.UploadOSS(key, "application/zip", data)
	if nil != err {
		logger.Fatalf("upload package [%s] failed: %s", repoURL, err)
	}

	size = int64(len(data)) // 计算包大小

	// 解压 package.zip 以计算实际占用空间大小
	installSize = size
	osTmpDir := filepath.Join(os.TempDir(), "bazaar")
	if err = os.MkdirAll(osTmpDir, 0755); nil != err {
		logger.Errorf("mkdir [%s] failed: %s", osTmpDir, err)
	} else {
		tmpZipPath := filepath.Join(os.TempDir(), "bazaar", gulu.Rand.String(7)+".zip")
		if err = os.WriteFile(tmpZipPath, data, 0644); nil != err {
			logger.Errorf("write package.zip failed: %s", err)
		} else {
			tmpUnzipPath := filepath.Join(os.TempDir(), "bazaar", gulu.Rand.String(7))
			if err = gulu.Zip.Unzip(tmpZipPath, tmpUnzipPath); nil != err {
				logger.Errorf("unzip package.zip failed: %s", err)
			} else {
				installSize, err = util.SizeOfDirectory(tmpUnzipPath)
				if nil != err {
					logger.Errorf("stat package [%s] size failed: %s", repoURL, err)
				}
			}
			os.RemoveAll(tmpUnzipPath)
		}
		os.RemoveAll(tmpZipPath)
	}

	// 先获取包配置，以便根据配置上传对应的 README 文件
	var basePkg *Package
	pkg, basePkg = getPackage(repoURL, hash, typ)
	if nil == pkg || nil == basePkg {
		logger.Warnf("get package [%s] failed", repoURL)
		return
	}

	// 收集需要上传的 README 文件列表（根据包配置中的 readme 字段）
	readmeFiles := make(map[string]bool)
	if nil != basePkg.Readme {
		for _, readmePath := range basePkg.Readme {
			if normalized, ok := normalizeReadmePath(readmePath); ok {
				readmeFiles["/"+normalized] = true
			}
		}
	}
	// 如果没有配置 readme 字段或所有字段都为空，则上传默认的 README 文件（向后兼容）
	if 0 == len(readmeFiles) {
		readmeFiles["/README_zh_CN.md"] = true
		readmeFiles["/README_en_US.md"] = true
	}
	// 无论是否收集到 README.md 文件，都需要上传
	readmeFiles["/README.md"] = true

	// 并发上传文件
	wg := &sync.WaitGroup{}
	wg.Add(3 + len(readmeFiles))
	// 上传 README 文件
	for readmeFile := range readmeFiles {
		go indexPackageFile(repoURL, hash, readmeFile, 0, 0, wg)
	}
	// 上传其他固定文件
	go indexPackageFile(repoURL, hash, "/preview.png", 0, 0, wg)
	go indexPackageFile(repoURL, hash, "/icon.png", 0, 0, wg)
	go indexPackageFile(repoURL, hash, "/"+strings.TrimSuffix(typ, "s")+".json", size, installSize, wg)
	wg.Wait()
	ok = true
	return
}

// getPackage 获取 release 对应提交中的 *.json 配置文件，按 typ 解析为 Package / PluginPackage / ThemePackage，并返回用于 Readme 等的 *Package
func getPackage(ownerRepo, hash, typ string) (pkgVal interface{}, basePkg *Package) {
	name := strings.TrimSuffix(typ, "s")
	u := "https://raw.githubusercontent.com/" + ownerRepo + "/" + hash + "/" + name + ".json"
	resp, data, errs := gorequest.New().Get(u).
		Set("User-Agent", util.UserAgent).
		Retry(1, 3*time.Second).Timeout(30 * time.Second).EndBytes()
	if nil != errs {
		logger.Errorf("get [%s] failed: %s", u, errs)
		return nil, nil
	}
	if 200 != resp.StatusCode {
		return nil, nil
	}

	switch typ {
	case "plugins":
		p := &PluginPackage{Package: &Package{}}
		if err := gulu.JSON.UnmarshalJSON(data, p); nil != err {
			logger.Errorf("unmarshal [%s] failed: %s", u, err)
			return nil, nil
		}
		sanitizePackage(p.Package)
		return p, p.Package
	case "themes":
		p := &ThemePackage{Package: &Package{}}
		if err := gulu.JSON.UnmarshalJSON(data, p); nil != err {
			logger.Errorf("unmarshal [%s] failed: %s", u, err)
			return nil, nil
		}
		sanitizePackage(p.Package)
		return p, p.Package
	default:
		ret := &Package{}
		if err := gulu.JSON.UnmarshalJSON(data, ret); nil != err {
			logger.Errorf("unmarshal [%s] failed: %s", u, err)
			return nil, nil
		}
		sanitizePackage(ret)
		return ret, ret
	}
}

// normalizeReadmePath 规范化并校验 readme 路径，防止路径穿越；返回规范化后的相对路径（无前导斜杠）及是否合法
func normalizeReadmePath(readmePath string) (string, bool) {
	readmePath = strings.TrimSpace(readmePath)
	if readmePath == "" {
		return "", false
	}
	// 去掉前导斜杠/反斜杠，视为相对路径
	readmePath = strings.TrimLeft(readmePath, "/\\")
	// 统一为正向斜杠后交给 filepath 做跨平台清理
	cleaned := filepath.Clean(filepath.FromSlash(readmePath))
	normalized := filepath.ToSlash(cleaned)
	// 拒绝含 .. 的路径，防止路径穿越
	if strings.Contains(normalized, "..") {
		return "", false
	}
	return normalized, true
}

// indexPackageFile 索引文件
func indexPackageFile(ownerRepo, hash, filePath string, size, installSize int64, wg *sync.WaitGroup) bool {
	defer wg.Done()

	u := "https://raw.githubusercontent.com/" + ownerRepo + "/" + hash + filePath
	resp, data, errs := gorequest.New().Get(u).
		Set("User-Agent", util.UserAgent).
		Retry(1, 3*time.Second).Timeout(30 * time.Second).EndBytes()
	if nil != errs {
		logger.Errorf("get [%s] failed: %s", u, errs)
		return false
	}
	if 200 != resp.StatusCode {
		return false
	}

	var contentType string
	if strings.HasSuffix(filePath, ".md") {
		contentType = "text/markdown"
	} else if strings.HasSuffix(filePath, ".json") {
		contentType = "application/json"
		// 统计包大小
		meta := map[string]interface{}{}
		if err := gulu.JSON.UnmarshalJSON(data, &meta); nil != err {
			logger.Errorf("stat package [%s] size failed: %s", u, err)
			return false
		}
		meta["size"] = size
		meta["installSize"] = installSize
		var err error
		data, err = gulu.JSON.MarshalIndentJSON(meta, "", "  ")
		if nil != err {
			logger.Errorf("marshal package [%s] meta json failed: %s", u, err)
			return false
		}
	}

	key := "package/" + ownerRepo + "@" + hash + filePath
	err := util.UploadOSS(key, contentType, data)
	if nil != err {
		logger.Errorf("upload package file [%s] failed: %s", key, err)
		return false
	}
	return true
}

func repoStats(repoURL string) (stars, openIssues int, ok bool) {
	result := map[string]interface{}{}
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
	result := map[string]interface{}{}
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
	assets := result["assets"].([]interface{})
	if 0 < len(assets) {
		for _, asset := range assets {
			asset := asset.(map[string]interface{})
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
	hash = result["object"].(map[string]interface{})["sha"].(string)
	if "" == hash {
		logger.Warnf("get [%s] release hash failed: hash is empty", repoURL)
		return
	}
	typ := result["object"].(map[string]interface{})["type"].(string)
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

		hash = result["object"].(map[string]interface{})["sha"].(string)
		if "" == hash {
			logger.Warnf("get [%s] tag hash failed: hash is empty", repoURL)
			return
		}
	}
	ok = true
	return
}

// sanitizePackage 对 Package 中部分字段消毒
func sanitizePackage(pkg *Package) {
	// REF: https://pkg.go.dev/github.com/microcosm-cc/bluemonday#Policy.Sanitize
	pkg.Name = sterilizer.Sanitize(pkg.Name)
	pkg.Author = sterilizer.Sanitize(pkg.Author)

	if nil != pkg.DisplayName {
		for k, v := range pkg.DisplayName {
			pkg.DisplayName[k] = sterilizer.Sanitize(v)
		}
	}

	if nil != pkg.Description {
		for k, v := range pkg.Description {
			pkg.Description[k] = sterilizer.Sanitize(v)
		}
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
	Package interface{} `json:"package"`
}
