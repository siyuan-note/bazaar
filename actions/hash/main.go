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
	logger.Infof("bazaar is hashing...")

	cmd := exec.Command("git", "rev-parse", "HEAD")
	data, err := cmd.CombinedOutput()
	if nil != err {
		logger.Fatalf("get git hash failed: %s", err)
	}
	hash := strings.TrimSpace(string(data))
	logger.Infof("bazaar [%s]", hash)

	u := "https://rhythm.b3log.org/api/siyuan/bazaar/hash"
	resp, data, errs := gorequest.New().Post(u).
		SendMap(map[string]interface{}{
			"token": os.Getenv("RHYTHEM_TOKEN"),
			"hash":  hash,
		}).
		Set("User-Agent", util.UserAgent).
		Retry(3, 3*time.Second).Timeout(30 * time.Second).EndBytes()
	if nil != errs {
		logger.Fatalf("hash [%s] failed: %s", u, errs)
		return
	}
	if 200 != resp.StatusCode {
		logger.Fatalf("hash [%s] failed: %d", u, resp.StatusCode)
		return
	}

	logger.Infof("Hashed bazaar")
}
