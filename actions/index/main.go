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
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/88250/gulu"
	"github.com/parnurzeal/gorequest"
	"github.com/siyuan-note/bazaar/actions/util"
)

var logger = gulu.Log.NewLogger(os.Stdout)

func main() {
	logger.Infof("bazaar is indexing...")

	cmd := exec.Command("git", "rev-parse", "HEAD")
	data, err := cmd.CombinedOutput()
	if nil != err {
		logger.Fatalf("get git hash failed: %s", err)
	}
	hash := strings.TrimSpace(string(data))
	logger.Infof("bazaar [%s]", hash)

	stageIndex(hash, "themes")
	stageIndex(hash, "templates")
	stageIndex(hash, "icons")
	stageIndex(hash, "widgets")
	stageIndex(hash, "plugins")

	logger.Infof("indexed bazaar")
}

func stageIndex(hash string, index string) {
	u := "https://raw.githubusercontent.com/siyuan-note/bazaar/" + hash + "/stage/" + index + ".json"
	resp, data, errs := gorequest.New().Get(u).
		Set("User-Agent", util.UserAgent).
		Retry(1, 3*time.Second).Timeout(30 * time.Second).EndBytes()
	if nil != errs {
		logger.Fatalf("get [%s] failed: %s", u, errs)
		return
	}
	if 200 != resp.StatusCode {
		logger.Fatalf("get [%s] failed: %d", u, resp.StatusCode)
		return
	}

	// 压缩 JSON：解析后重新序列化为压缩格式（移除空格和换行）
	var jsonData interface{}
	err := json.Unmarshal(data, &jsonData)
	if nil != err {
		logger.Fatalf("unmarshal [%s] failed: %s", u, err)
		return
	}
	data, err = json.Marshal(jsonData)
	if nil != err {
		logger.Fatalf("marshal [%s] failed: %s", u, err)
		return
	}

	key := "bazaar@" + hash + "/stage/" + index + ".json"
	err = util.UploadOSS(key, "application/json", data)
	if nil != err {
		logger.Fatalf("upload bazaar stage index [%s] failed: %s", key, err)
	}
}
