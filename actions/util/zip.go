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
	"github.com/siyuan-note/bazaar/rules"
)

// DownloadAndUnzipPackageZip 通过 GitHub Release Asset API 下载 package.zip 并解压到临时目录。
// zipData 供 Stage 上传 OSS；cleanup 删除临时工作目录，调用方应 defer cleanup()。
func DownloadAndUnzipPackageZip(ctx context.Context, client *github.Client, owner, repo string, assetID int64) (unzipDir string, zipData []byte, cleanup func(), err error) {
	cleanup = func() {}
	if client == nil {
		return "", nil, cleanup, rules.LocalizedErr(
			"内部错误：下载 package.zip 时 GitHub 客户端未初始化。这通常是集市检查流程配置问题，请联系维护者重试。",
			"Internal error: GitHub client is not initialized while downloading package.zip. This is usually a bazaar checker configuration issue; contact a maintainer.",
			nil,
		)
	}
	if assetID == 0 {
		return "", nil, cleanup, rules.LocalizedErr(
			"无法下载 package.zip：Latest Release 中未找到有效的 package.zip 资源。请把打包好的 package.zip 作为 Release Asset 上传（文件名必须是 package.zip）。",
			"Cannot download package.zip: no valid package.zip asset was found in the Latest Release. Upload package.zip as a Release asset (the filename must be package.zip).",
			nil,
		)
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
	rc, _, err := client.Repositories.DownloadReleaseAsset(ctx, owner, repo, assetID, followClient)
	if err != nil {
		return "", nil, cleanup, rules.LocalizedErr(
			fmt.Sprintf("下载 package.zip 失败：%v。请确认仓库已公开，且 Latest Release 中的 package.zip 可正常下载，然后重新打包并更新 Release。", err),
			fmt.Sprintf("Failed to download package.zip: %v. Ensure the repository is public and package.zip in the Latest Release can be downloaded, then rebuild and update the Release.", err),
			err,
		)
	}
	if rc == nil {
		return "", nil, cleanup, rules.LocalizedErr(
			"下载 package.zip 失败：GitHub 返回了空响应。请重新上传 package.zip 到 Latest Release；若仍失败请联系集市维护者。",
			"Failed to download package.zip: GitHub returned an empty response. Re-upload package.zip to the Latest Release; contact a bazaar maintainer if it persists.",
			nil,
		)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return "", nil, cleanup, rules.LocalizedErr(
			fmt.Sprintf("读取 package.zip 失败：%v。请确认 Release 中的 zip 未损坏，重新打包并更新 GitHub Release 中的 package.zip；若仍失败请联系集市维护者。", err),
			fmt.Sprintf("Failed to read package.zip: %v. Ensure the Release zip is not corrupted, rebuild and update package.zip on the GitHub Release; contact a bazaar maintainer if it persists.", err),
			err,
		)
	}

	workDir, err := os.MkdirTemp("", "bazaar-*")
	if err != nil {
		return "", nil, cleanup, rules.LocalizedErr(
			fmt.Sprintf("内部错误：解压 package.zip 时无法创建临时目录：%v。请联系集市维护者。", err),
			fmt.Sprintf("Internal error: cannot create temp directory while extracting package.zip: %v. Contact a bazaar maintainer.", err),
			err,
		)
	}
	cleanup = func() {
		os.RemoveAll(workDir)
	}

	tmpZipPath := filepath.Join(workDir, "package.zip")
	if err = os.WriteFile(tmpZipPath, data, 0644); err != nil {
		cleanup()
		cleanup = func() {}
		return "", nil, cleanup, rules.LocalizedErr(
			fmt.Sprintf("内部错误：保存 package.zip 时写入失败：%v。请联系集市维护者。", err),
			fmt.Sprintf("Internal error: failed to save package.zip: %v. Contact a bazaar maintainer.", err),
			err,
		)
	}

	tmpUnzipPath := filepath.Join(workDir, "unzip")
	if err = gulu.Zip.Unzip(tmpZipPath, tmpUnzipPath); err != nil {
		cleanup()
		cleanup = func() {}
		return "", nil, cleanup, rules.LocalizedErr(
			fmt.Sprintf("解压 package.zip 失败：%v。请确认 zip 内结构正确且未损坏，重新打包后更新 GitHub Release 中的 package.zip。", err),
			fmt.Sprintf("Failed to unzip package.zip: %v. Ensure the archive structure is valid and not corrupted, then rebuild and update package.zip on the GitHub Release.", err),
			err,
		)
	}

	return tmpUnzipPath, data, cleanup, nil
}
