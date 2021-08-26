package main

import (
	"crypto/tls"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/88250/gulu"
	ants "github.com/panjf2000/ants/v2"
	"github.com/parnurzeal/gorequest"
)

var logger = gulu.Log.NewLogger(os.Stdout)

func main() {
	logger.Infof("bazaar is staging...")

	data, err := os.ReadFile("themes.json")
	if nil != err {
		logger.Fatalf("read themes.json failed: %s", err)
	}

	themes := map[string]interface{}{}
	if err = gulu.JSON.UnmarshalJSON(data, &themes); nil != err {
		logger.Fatalf("unmarshal themes.json failed: %s", err)
	}

	repos := themes["repos"].([]interface{})
	var stageRepos []interface{}
	waitGroup := &sync.WaitGroup{}

	p, _ := ants.NewPoolWithFunc(8, func(arg interface{}) {
		defer waitGroup.Done()
		repo := arg.(string)
		t := repoUpdateTime(repo)
		stageRepos = append(stageRepos, &stageRepo{
			URL:     repo,
			Updated: t,
		})
	})
	for _, repo := range repos {
		waitGroup.Add(1)
		p.Invoke(repo)
	}
	waitGroup.Wait()
	p.Release()

	stageThemes := map[string]interface{}{
		"repos": stageRepos,
	}

	data, err = gulu.JSON.MarshalIndentJSON(stageThemes, "", "  ")
	if nil != err {
		logger.Fatalf("marshal stage themes.json failed: %s", err)
	}

	if err = os.WriteFile("stage/themes.json", data, 0644); nil != err {
		logger.Fatalf("write stage themes.json failed: %s", err)
	}

	logger.Infof("bazaar staged")
}

func repoUpdateTime(repoURL string) string {
	result := filetree{}
	request := gorequest.New().TLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	resp, _, errs := request.Get("https://data.jsdelivr.com/v1/package/gh/"+repoURL).
		Set("User-Agent", "bazaar/1.0.0 https://github.com/siyuan-note/bazaar").Timeout(7*time.Second).
		Retry(1, time.Second).EndStruct(&result)
	if nil != errs {
		//util.LogErrorf("get repo file tree failed: %s", errs)
		return ""
	}
	if 200 != resp.StatusCode {
		logger.Errorf("get repo file tree failed: %s", errs)
		return ""
	}

	for _, f := range result.Files {
		if strings.HasSuffix(f.Name, ".json") {
			return f.Time
		}
	}
	return ""
}

type stageRepo struct {
	URL     string `json:"url"`
	Updated string `json:"updated"`
}

type filetree struct {
	Files []*file `json:"files"`
}

type file struct {
	Type  string  `json:"type"`
	Name  string  `json:"name"`
	Time  string  `json:"time"`
	Data  []byte  `json:"data"`
	Files []*file `json:"files"`
}
