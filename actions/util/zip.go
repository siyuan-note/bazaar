// SiYuan community bazaar.
// Copyright (c) 2021-present, b3log.org
//
// Bazaar is licensed under Mulan PSL v2.
// You can use this software according to the terms and conditions of the Mulan PSL v2.
// You may obtain a copy of Mulan PSL v2 at:
//         http://license.coscl.org.cn/MulanPSL2
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR PURPOSE.
// See the Mulan PSL v2 for more details.

package util

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/88250/gulu"
	"github.com/google/go-github/v89/github"
)

// DownloadAndUnzipPackageZip 通过 GitHub Release Asset API 下载 package.zip 并解压到临时目录。
// zipData 供 Stage 上传 OSS；cleanup 删除临时 zip 与解压目录，调用方应 defer cleanup()。
func DownloadAndUnzipPackageZip(ctx context.Context, client *github.Client, owner, repo string, assetID int64) (unzipDir string, zipData []byte, cleanup func(), err error) {
	cleanup = func() {}
	if client == nil {
		return "", nil, cleanup, fmt.Errorf("download package.zip: github client is nil")
	}
	if assetID == 0 {
		return "", nil, cleanup, fmt.Errorf("download package.zip: asset id is empty")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}

	// followRedirectsClient 需非 nil 才能拿到内容流；私有仓还需带鉴权 Transport。
	// REF https://pkg.go.dev/github.com/google/go-github/v89/github#RepositoriesService.DownloadReleaseAsset
	followClient := &http.Client{
		Timeout:   30 * time.Second,
		Transport: client.Client().Transport,
	}
	rc, redirectURL, err := client.Repositories.DownloadReleaseAsset(ctx, owner, repo, assetID, followClient)
	if err != nil {
		return "", nil, cleanup, fmt.Errorf("download package.zip: %w", err)
	}
	if rc == nil {
		return "", nil, cleanup, fmt.Errorf("download package.zip: empty body (redirect=%q)", redirectURL)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return "", nil, cleanup, fmt.Errorf("download package.zip: read body: %w", err)
	}

	tmpDir := filepath.Join(os.TempDir(), "bazaar")
	if err = os.MkdirAll(tmpDir, 0755); nil != err {
		return "", nil, cleanup, fmt.Errorf("mkdir temp: %w", err)
	}

	tmpBase := gulu.Rand.String(16)
	tmpZipPath := filepath.Join(tmpDir, tmpBase+".zip")
	if err = os.WriteFile(tmpZipPath, data, 0644); nil != err {
		return "", nil, cleanup, fmt.Errorf("write package.zip: %w", err)
	}

	tmpUnzipPath := filepath.Join(tmpDir, tmpBase)
	if err = gulu.Zip.Unzip(tmpZipPath, tmpUnzipPath); nil != err {
		os.RemoveAll(tmpZipPath)
		return "", nil, cleanup, fmt.Errorf("unzip package.zip: %w", err)
	}

	cleanup = func() {
		os.RemoveAll(tmpZipPath)
		os.RemoveAll(tmpUnzipPath)
	}
	return tmpUnzipPath, data, cleanup, nil
}
