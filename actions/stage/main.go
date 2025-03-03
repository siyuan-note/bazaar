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
	lock := sync.Mutex{}
	var stageRepos []interface{}
	waitGroup := &sync.WaitGroup{}

	p, _ := ants.NewPoolWithFunc(8, func(arg interface{}) {
		defer waitGroup.Done()
		repo := arg.(string)
		var hash, updated string
		var size, installSize int64
		var ok bool
		var pkg *Package

		if ok, hash, updated, size, installSize, pkg = indexPackage(repo, typ); !ok {
			return
		}

		stars, openIssues := repoStats(repo, hash)

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

// indexPackage 索引包
func indexPackage(repoURL, typ string) (ok bool, hash, published string, size, installSize int64, pkg *Package) {
	hash, published, packageZip := getRepoLatestRelease(repoURL)
	if "" == hash {
		logger.Warnf("get [%s] latest release failed", repoURL)
		return
	}

	if "" == packageZip {
		logger.Warnf("get [%s] package.zip failed", repoURL)
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

	wg := &sync.WaitGroup{}
	wg.Add(7)
	go func() {
		defer wg.Done()
		pkg = getPackage(repoURL, hash, typ)
	}()
	go indexPackageFile(repoURL, hash, "/README.md", 0, 0, wg)
	go indexPackageFile(repoURL, hash, "/README_zh_CN.md", 0, 0, wg)
	go indexPackageFile(repoURL, hash, "/README_en_US.md", 0, 0, wg)
	go indexPackageFile(repoURL, hash, "/preview.png", 0, 0, wg)
	go indexPackageFile(repoURL, hash, "/icon.png", 0, 0, wg)
	go indexPackageFile(repoURL, hash, "/"+strings.TrimSuffix(typ, "s")+".json", size, installSize, wg)
	wg.Wait()
	ok = true
	return
}

// getPackage 获取 release 对应提交中的 *.json 配置文件
func getPackage(ownerRepo, hash, typ string) (ret *Package) {
	name := strings.TrimSuffix(typ, "s")
	u := "https://raw.githubusercontent.com/" + ownerRepo + "/" + hash + "/" + name + ".json"
	resp, data, errs := gorequest.New().Get(u).
		Set("User-Agent", util.UserAgent).
		Retry(1, 3*time.Second).Timeout(30 * time.Second).EndBytes()
	if nil != errs {
		logger.Errorf("get [%s] failed: %s", u, errs)
		return
	}
	if 200 != resp.StatusCode {
		return
	}

	ret = &Package{}
	if err := gulu.JSON.UnmarshalJSON(data, ret); nil != err {
		logger.Errorf("unmarshal [%s] failed: %s", u, err)
		ret = nil
		return
	}

	sanitizePackage(ret)
	return
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

func repoStats(repoURL, hash string) (stars, openIssues int) {
	result := map[string]interface{}{}
	request := gorequest.New().TLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	pat := os.Getenv("PAT")
	u := "https://api.github.com/repos/" + repoURL
	resp, _, errs := request.Get(u).
		Set("Authorization", "Token "+pat).
		Set("User-Agent", util.UserAgent).Timeout(30*time.Second).
		Retry(1, 3*time.Second).EndStruct(&result)
	if nil != errs {
		logger.Fatalf("get [%s] failed: %s", u, errs)
		return 0, 0
	}
	if 200 != resp.StatusCode {
		logger.Fatalf("get [%s] failed: %d", u, resp.StatusCode)
		return 0, 0
	}

	//logger.Infof("X-Ratelimit-Remaining=%s]", resp.Header.Get("X-Ratelimit-Remaining"))
	stars = int(result["stargazers_count"].(float64))
	openIssues = int(result["open_issues_count"].(float64))
	return
}

// getRepoLatestRelease 获取仓库最新发布的版本
func getRepoLatestRelease(repoURL string) (hash, published, packageZip string) {
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
		logger.Fatalf("get release hash [%s] failed: %s", u, errs)
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
		return
	}

	// 获取 release 对应的 tag
	published = result["published_at"].(string)
	tagName := result["tag_name"].(string)
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
	typ := result["object"].(map[string]interface{})["type"].(string)
	if "tag" == typ {
		// REF https://docs.github.com/en/rest/git/tags#get-a-tag
		u = "https://api.github.com/repos/" + repoURL + "/git/tags/" + hash
		resp, _, errs = request.Get(u).
			Set("Authorization", "Token "+pat).
			Set("User-Agent", util.UserAgent).Timeout(30*time.Second).
			Retry(1, 3*time.Second).EndStruct(&result)
		if nil != errs {
			logger.Fatalf("get release hash [%s] failed: %s", u, errs)
			return
		}
		if 200 != resp.StatusCode {
			logger.Fatalf("get release hash [%s] failed: %d", u, resp.StatusCode)
			return
		}

		hash = result["object"].(map[string]interface{})["sha"].(string)
	}
	return
}

// sanitizePackage 对 Package 中部分字段消毒
func sanitizePackage(pkg *Package) {
	// REF: https://pkg.go.dev/github.com/microcosm-cc/bluemonday#Policy.Sanitize
	pkg.Name = sterilizer.Sanitize(pkg.Name)
	pkg.Author = sterilizer.Sanitize(pkg.Author)

	if nil != pkg.DisplayName {
		pkg.DisplayName.Default = sterilizer.Sanitize(pkg.DisplayName.Default)
		pkg.DisplayName.ZhCN = sterilizer.Sanitize(pkg.DisplayName.ZhCN)
		pkg.DisplayName.EnUS = sterilizer.Sanitize(pkg.DisplayName.EnUS)
	}

	if nil != pkg.Description {
		pkg.Description.Default = sterilizer.Sanitize(pkg.Description.Default)
		pkg.Description.ZhCN = sterilizer.Sanitize(pkg.Description.ZhCN)
		pkg.Description.EnUS = sterilizer.Sanitize(pkg.Description.EnUS)
	}
}

type DisplayName struct {
	Default string `json:"default"`
	ZhCN    string `json:"zh_CN"`
	EnUS    string `json:"en_US"`
}

type Description struct {
	Default string `json:"default"`
	ZhCN    string `json:"zh_CN"`
	EnUS    string `json:"en_US"`
}

type Readme struct {
	Default string `json:"default"`
	ZhCN    string `json:"zh_CN"`
	EnUS    string `json:"en_US"`
}

type Funding struct {
	OpenCollective string   `json:"openCollective"`
	Patreon        string   `json:"patreon"`
	GitHub         string   `json:"github"`
	Custom         []string `json:"custom"`
}

type Package struct {
	Name          string       `json:"name"`
	Author        string       `json:"author"`
	URL           string       `json:"url"`
	Version       string       `json:"version"`
	MinAppVersion string       `json:"minAppVersion"`
	Backends      []string     `json:"backends"`
	Frontends     []string     `json:"frontends"`
	DisplayName   *DisplayName `json:"displayName"`
	Description   *Description `json:"description"`
	Readme        *Readme      `json:"readme"`
	Funding       *Funding     `json:"funding"`
	Keywords      []string     `json:"keywords"`
}

type StageRepo struct {
	URL         string `json:"url"`
	Updated     string `json:"updated"`
	Stars       int    `json:"stars"`
	OpenIssues  int    `json:"openIssues"`
	Size        int64  `json:"size"`
	InstallSize int64  `json:"installSize"`

	Package *Package `json:"package"`
}
