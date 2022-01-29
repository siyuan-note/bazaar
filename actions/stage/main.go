// SiYuan community bazaar.
// Copyright (c) 2021-present, b3log.org
//
// Pipe is licensed under Mulan PSL v2.
// You can use this software according to the terms and conditions of the Mulan PSL v2.
// You may obtain a copy of Mulan PSL v2 at:
//         http://license.coscl.org.cn/MulanPSL2
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR PURPOSE.
// See the Mulan PSL v2 for more details.

package main

import (
	"crypto/tls"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/88250/gulu"
	"github.com/panjf2000/ants/v2"
	"github.com/parnurzeal/gorequest"
	"github.com/siyuan-note/bazaar/actions/util"
)

var logger = gulu.Log.NewLogger(os.Stdout)

func main() {
	logger.Infof("bazaar is staging...")

	performStage("themes")
	performStage("templates")
	performStage("icons")
	performStage("widgets")

	logger.Infof("bazaar staged")
}

func performStage(typ string) {
	logger.Infof("staging [%s]", typ)

	data, err := os.ReadFile(typ + ".json")
	if nil != err {
		logger.Fatalf("read [%s.json] failed: %s", typ, err)
	}

	original := map[string]interface{}{}
	if err = gulu.JSON.UnmarshalJSON(data, &original); nil != err {
		logger.Fatalf("unmarshal [%s.json] failed: %s", typ, err)
	}

	repos := original["repos"].([]interface{})
	var stageRepos []interface{}
	waitGroup := &sync.WaitGroup{}

	verTime, _ := time.Parse("2006-01-02T15:04:05Z", "2021-12-01T00:00:00Z")
	p, _ := ants.NewPoolWithFunc(8, func(arg interface{}) {
		defer waitGroup.Done()
		repo := arg.(string)
		t := repoUpdateTime(repo)
		if "themes" == typ {
			updated, err := time.Parse("2006-01-02T15:04:05Z", t)
			if nil != err {
				logger.Fatalf("parse repo updated [%s] failed: %s", t, err)
				return
			}
			if updated.Before(verTime) {
				//logger.Infof("skip legacy theme package [%s], last updated at [%s]", repo, t)
				return
			}
		}

		var size int64
		var ok bool
		// 索引包上传 CDN
		if ok, size = indexPackage(repo, typ); !ok {
			return
		}

		stars, openIssues := repoStats(repo)
		stageRepos = append(stageRepos, &stageRepo{
			URL:        repo,
			Stars:      stars,
			OpenIssues: openIssues,
			Updated:    t,
			Size:       size,
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
		return stageRepos[i].(*stageRepo).Updated > stageRepos[j].(*stageRepo).Updated
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

func indexPackage(repoURL, typ string) (ok bool, size int64) {
	hash := strings.Split(repoURL, "@")[1]
	ownerRepo := repoURL[:strings.Index(repoURL, "@")]
	u := "https://github.com/" + ownerRepo + "/archive/" + hash + ".zip"
	resp, data, errs := gorequest.New().Get(u).
		Set("User-Agent", "bazaar/1.0.0 https://github.com/siyuan-note/bazaar").
		Retry(1, 3*time.Second).Timeout(30 * time.Second).EndBytes()
	if nil != errs {
		logger.Errorf("get [%s] failed: %s", u, errs)
		return
	}
	if 200 != resp.StatusCode {
		logger.Errorf("get [%s] failed: %d", u, resp.StatusCode)
		return
	}

	key := "package/" + repoURL
	err := util.UploadOSS(key, "application/octet-stream", data)
	if nil != err {
		logger.Fatalf("upload package [%s] failed: %s", repoURL, err)
	}

	size = int64(len(data))
	if ok = indexPackageFile(ownerRepo, hash, "/README.md", 0); !ok {
		return
	}
	if ok = indexPackageFile(ownerRepo, hash, "/preview.png", 0); !ok {
		return
	}
	if ok = indexPackageFile(ownerRepo, hash, "/"+strings.TrimSuffix(typ, "s")+".json", size); !ok {
		return
	}
	return
}

func indexPackageFile(ownerRepo, hash, filePath string, size int64) bool {
	u := "https://raw.githubusercontent.com/" + ownerRepo + "/" + hash + filePath
	resp, data, errs := gorequest.New().Get(u).
		Set("User-Agent", "bazaar/1.0.0 https://github.com/siyuan-note/bazaar").
		Retry(1, 3*time.Second).Timeout(30 * time.Second).EndBytes()
	if nil != errs {
		logger.Errorf("get [%s] failed: %s", u, errs)
		return false
	}
	if 200 != resp.StatusCode {
		logger.Errorf("get [%s] failed: %d", u, resp.StatusCode)
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
			logger.Errorf("stat package size failed: %s", err)
			return false
		}
		meta["size"] = size
		var err error
		data, err = gulu.JSON.MarshalIndentJSON(meta, "", "  ")
		if nil != err {
			logger.Errorf("marshal package meta json failed: %s", err)
			return false
		}
	}

	key := "package/" + ownerRepo + "@" + hash + filePath
	err := util.UploadOSS(key, contentType, data)
	if nil != err {
		logger.Errorf("upload package file [%s] failed: %s", key)
		return false
	}
	return true
}

func repoUpdateTime(repoURL string) (t string) {
	hash := strings.Split(repoURL, "@")[1]
	ownerRepo := repoURL[:strings.Index(repoURL, "@")]
	pat := os.Getenv("PAT")
	result := map[string]interface{}{}
	request := gorequest.New().TLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	u := "https://api.github.com/repos/" + ownerRepo + "/git/commits/" + hash
	resp, _, errs := request.Get(u).
		Set("Authorization", "Token "+pat).
		Set("User-Agent", "bazaar/1.0.0 https://github.com/siyuan-note/bazaar").Timeout(7*time.Second).
		Retry(1, 3*time.Second).EndStruct(&result)
	if nil != errs {
		logger.Fatalf("get [%s] failed: %s", u, errs)
		return ""
	}
	if 200 != resp.StatusCode {
		logger.Fatalf("get [%s] failed: %d", u, resp.StatusCode)
		return ""
	}

	if nil != result["author"] {
		author := result["author"].(map[string]interface{})
		if date := author["date"]; nil != date {
			return date.(string)
		}
	}
	if nil != result["committer"] {
		committer := result["committer"].(map[string]interface{})
		if date := committer["date"]; nil != date {
			return date.(string)
		}
	}
	return ""
}

func repoStats(repoURL string) (stars, openIssues int) {
	repoURL = repoURL[:strings.LastIndex(repoURL, "@")]
	result := map[string]interface{}{}
	request := gorequest.New().TLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	pat := os.Getenv("PAT")
	u := "https://api.github.com/repos/" + repoURL
	resp, _, errs := request.Get(u).
		Set("Authorization", "Token "+pat).
		Set("User-Agent", "bazaar/1.0.0 https://github.com/siyuan-note/bazaar").Timeout(7*time.Second).
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

type stageRepo struct {
	URL        string `json:"url"`
	Updated    string `json:"updated"`
	Stars      int    `json:"stars"`
	OpenIssues int    `json:"openIssues"`
	Size       int64  `json:"size"`
}
