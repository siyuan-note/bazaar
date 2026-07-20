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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/88250/gulu"
	"github.com/siyuan-note/bazaar/actions/util"
	"github.com/siyuan-note/bazaar/rules"
)

var (
	RHYTHEM_TOKEN    = os.Getenv("RHYTHEM_TOKEN") // Rhy 上报 token
	logger           = gulu.Log.NewLogger(os.Stdout)
	BAZAAR_ROOT_PATH = "." // bazaar 仓库根目录（与 stage 一致，CI 中在 checkout 根目录执行）
)

// 出现任何错误都立即退出中断工作流，确保一切正常才上报 hash
func main() {
	logger.Infof("bazaar is indexing...")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	data, err := cmd.CombinedOutput()
	if err != nil {
		logger.Fatalf("get git hash failed: %s", err)
	}
	hash := strings.TrimSpace(string(data))
	logger.Infof("bazaar [%s]", hash)

	var wg sync.WaitGroup
	for _, packageType := range rules.AllPackageTypes() {
		wg.Go(func() {
			stageIndex(ctx, hash, packageType)
		})
	}
	wg.Wait()

	reportHash(ctx, hash)

	logger.Infof("indexed bazaar")
}

// stageIndex 将指定类型的集市索引上传到 OSS
func stageIndex(ctx context.Context, hash string, packageType rules.PackageType) {
	stageFileName := packageType.StageJSONFile()
	stageFilePath := filepath.Join(BAZAAR_ROOT_PATH, "stage", stageFileName)
	if _, err := os.Stat(stageFilePath); err != nil {
		logger.Fatalf("read [%s] failed: %s", stageFilePath, err)
		return
	}
	stageFile, err := util.ReadStageFile(stageFilePath)
	if err != nil {
		logger.Fatalf("read [%s] failed: %s", stageFilePath, err)
		return
	}

	// 去掉 stage 流程内部字段后重新序列化为压缩 JSON（移除空格和换行）
	data, err := json.Marshal(stageFile.ForPublicIndex())
	if err != nil {
		logger.Fatalf("marshal [%s] failed: %s", stageFilePath, err)
		return
	}

	key := "bazaar@" + hash + "/stage/" + stageFileName
	err = util.UploadOSS(ctx, key, data)
	if err != nil {
		logger.Fatalf("upload bazaar stage index [%s] failed: %s", key, err)
	}
	logger.Infof("uploaded index: %s", "https://oss.b3logfile.com/"+key)
}

// reportHash 将当前 bazaar 版本 hash 上报到 Rhy，供思源客户端获知当前集市版本
func reportHash(ctx context.Context, hash string) {
	u := "https://rhythm.b3log.org/api/siyuan/bazaar/hash"
	body, err := json.Marshal(map[string]any{
		"token": RHYTHEM_TOKEN,
		"hash":  hash,
	})
	if err != nil {
		logger.Fatalf("hash [%s] failed: marshal body: %s", u, err)
		return
	}

	client := &http.Client{Timeout: 30 * time.Second}
	var lastErr error
	for attempt := range 3 {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				logger.Fatalf("hash [%s] failed: %s", u, ctx.Err())
				return
			case <-time.After(3 * time.Second):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
		if err != nil {
			logger.Fatalf("hash [%s] failed: %s", u, err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", util.UserAgent)

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			logger.Infof("Hashed bazaar")
			return
		}
		lastErr = fmt.Errorf("status %d", resp.StatusCode)
	}
	logger.Fatalf("hash [%s] failed: %s", u, lastErr)
}
