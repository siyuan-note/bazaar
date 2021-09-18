package main

import (
	"crypto/tls"
	"os"
	"sort"
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

	p, _ := ants.NewPoolWithFunc(8, func(arg interface{}) {
		defer waitGroup.Done()
		repo := arg.(string)
		t := repoUpdateTime(repo)
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

func repoStars(repoURL string) int {
	repoURL = repoURL[:strings.LastIndex(repoURL, "@")]
	result := map[string]interface{}{}
	request := gorequest.New().TLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	pat := os.Getenv("secrets.PAT")
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
