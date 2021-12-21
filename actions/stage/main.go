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
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/88250/gulu"
	ants "github.com/panjf2000/ants/v2"
	"github.com/parnurzeal/gorequest"
	"github.com/qiniu/go-sdk/v7/auth/qbox"
	"github.com/qiniu/go-sdk/v7/storage"
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

	verTime, _ := time.Parse("2006-01-02T15:04:05Z", "2021-07-01T00:00:00Z")
	p, _ := ants.NewPoolWithFunc(8, func(arg interface{}) {
		defer waitGroup.Done()
		repo := arg.(string)
		t := repoUpdateTime(repo)
		if "themes" == typ {
			updated, err := time.Parse("2006-01-02T15:04:05Z", t)
			if nil != err {
				logger.Errorf("parse repo updated [%s] failed: %s", t, err)
				return
			}
			if updated.Before(verTime) {
				logger.Infof("skip legacy theme package [%s], last updated at [%s]", repo, t)
				return
			}
		}

		// 包下载后上传 CDN
		zipData := downloadRepoZip(repo)
		uploadRepoZip(repo, zipData)

		stars := repoStars(repo)
		stageRepos = append(stageRepos, &stageRepo{
			URL:     repo,
			Stars:   stars,
			Updated: t,
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

func uploadRepoZip(repoURL string, data []byte) {
	bucket := os.Getenv("QINIU_BUCKET")
	ak := os.Getenv("QINIU_AK")
	sk := os.Getenv("QINIU_SK")

	key := time.Now().Format("package/" + repoURL)
	putPolicy := storage.PutPolicy{
		Scope: fmt.Sprintf("%s:%s", bucket, key), // overwrite if exists
	}
	cfg := storage.Config{}
	cfg.Zone = &storage.ZoneHuanan
	cfg.UseCdnDomains = true
	cfg.UseHTTPS = true
	formUploader := storage.NewFormUploader(&cfg)
	if err := formUploader.Put(context.Background(), nil, putPolicy.UploadToken(qbox.NewMac(ak, sk)),
		key, bytes.NewReader(data), int64(len(data)), nil); nil != err {
		logger.Fatalf("upload package [%s] failed: %s", repoURL, err)
	}

	logger.Infof("uploaded package [%s]", repoURL)
}

func downloadRepoZip(repoURL string) []byte {
	hash := strings.Split(repoURL, "@")[1]
	ownerRepo := repoURL[:strings.Index(repoURL, "@")]
	resp, data, errs := gorequest.New().Get("https://github.com/"+ownerRepo+"/archive/"+hash+".zip").
		Set("User-Agent", "bazaar/1.0.0 https://github.com/siyuan-note/bazaar").
		Timeout(30 * time.Second).EndBytes()
	if nil != errs {
		logger.Errorf("get repo zip failed: %s", errs)
		return nil
	}
	if 200 != resp.StatusCode {
		logger.Errorf("get repo zip failed: %s", errs)
		return nil
	}

	logger.Infof("downloaded package [%s], size [%d]", repoURL, len(data))
	return data
}

func repoUpdateTime(repoURL string) (t string) {
	hash := strings.Split(repoURL, "@")[1]
	ownerRepo := repoURL[:strings.Index(repoURL, "@")]
	pat := os.Getenv("PAT")
	result := map[string]interface{}{}
	request := gorequest.New().TLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	resp, _, errs := request.Get("https://api.github.com/repos/"+ownerRepo+"/git/commits/"+hash).
		Set("Authorization", "Token "+pat).
		Set("User-Agent", "bazaar/1.0.0 https://github.com/siyuan-note/bazaar").Timeout(7*time.Second).
		Retry(1, time.Second).EndStruct(&result)
	if nil != errs {
		logger.Errorf("get repo update time failed: %s", errs)
		return ""
	}
	if 200 != resp.StatusCode {
		logger.Errorf("get repo update time failed: %s", errs)
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

func repoStars(repoURL string) int {
	repoURL = repoURL[:strings.LastIndex(repoURL, "@")]
	result := map[string]interface{}{}
	request := gorequest.New().TLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	pat := os.Getenv("PAT")
	resp, _, errs := request.Get("https://api.github.com/repos/"+repoURL).
		Set("Authorization", "Token "+pat).
		Set("User-Agent", "bazaar/1.0.0 https://github.com/siyuan-note/bazaar").Timeout(7*time.Second).
		Retry(1, time.Second).EndStruct(&result)
	if nil != errs {
		logger.Errorf("get repo stars failed: %s", errs)
		return 0
	}
	if 200 != resp.StatusCode {
		logger.Errorf("get repo stars failed: %s", errs)
		return 0
	}

	logger.Infof("X-Ratelimit-Remaining=%s]", resp.Header.Get("X-Ratelimit-Remaining"))
	return int(result["stargazers_count"].(float64))
}

type stageRepo struct {
	URL     string `json:"url"`
	Updated string `json:"updated"`
	Stars   int    `json:"stars"`
}
