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
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/88250/gulu"
	"github.com/parnurzeal/gorequest"
	"github.com/siyuan-note/bazaar/actions/util"
	"github.com/siyuan-note/bazaar/rules"
)

var (
	logger           = gulu.Log.NewLogger(os.Stdout)
	BAZAAR_ROOT_PATH = "." // bazaar 仓库根目录（与 stage 一致，CI 中在 checkout 根目录执行）
)

// 出现任何错误都立即退出中断工作流，确保一切正常才上报 hash
func main() {
	logger.Infof("bazaar is indexing...")

	cmd := exec.Command("git", "rev-parse", "HEAD")
	data, err := cmd.CombinedOutput()
	if nil != err {
		logger.Fatalf("get git hash failed: %s", err)
	}
	hash := strings.TrimSpace(string(data))
	logger.Infof("bazaar [%s]", hash)

	indexTypes := rules.AllPackageTypes()
	var wg sync.WaitGroup
	wg.Add(len(indexTypes))
	for _, packageType := range indexTypes {
		go func(pt rules.PackageType) {
			defer wg.Done()
			stageIndex(hash, pt)
		}(packageType)
	}
	wg.Wait()

	reportHash(hash)

	logger.Infof("indexed bazaar")
}

// stageIndex 将指定类型的集市索引上传到 OSS
func stageIndex(hash string, packageType rules.PackageType) {
	stageFileName := packageType.StageJSONFile()
	stageFilePath := filepath.Join(BAZAAR_ROOT_PATH, "stage", stageFileName)
	if _, err := os.Stat(stageFilePath); nil != err {
		logger.Fatalf("read [%s] failed: %s", stageFilePath, err)
		return
	}
	stageFile, err := util.ReadStageFile(stageFilePath)
	if nil != err {
		logger.Fatalf("read [%s] failed: %s", stageFilePath, err)
		return
	}

	// 去掉 stage 流程内部字段后重新序列化为压缩 JSON（移除空格和换行）
	data, err := json.Marshal(stageFile.ForPublicIndex())
	if nil != err {
		logger.Fatalf("marshal [%s] failed: %s", stageFilePath, err)
		return
	}

	key := "bazaar@" + hash + "/stage/" + stageFileName
	err = util.UploadOSS(context.Background(), key, data)
	if nil != err {
		logger.Fatalf("upload bazaar stage index [%s] failed: %s", key, err)
	}
	logger.Infof("uploaded index: %s", "https://oss.b3logfile.com/"+key)
}

// reportHash 将当前 bazaar 版本 hash 上报到 Rhy，供思源客户端获知当前集市版本
func reportHash(hash string) {
	u := "https://rhythm.b3log.org/api/siyuan/bazaar/hash"
	resp, _, errs := gorequest.New().Post(u).
		SendMap(map[string]any{
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
