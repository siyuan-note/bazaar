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
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/88250/gulu"
	"github.com/parnurzeal/gorequest"
)

// DownloadAndUnzipPackageZip 下载 package.zip 并解压到临时目录。
// zipData 供 Stage 上传 OSS；cleanup 删除临时 zip 与解压目录，调用方应 defer cleanup()。
func DownloadAndUnzipPackageZip(url string) (unzipDir string, zipData []byte, cleanup func(), err error) {
	cleanup = func() {}

	resp, data, errs := gorequest.New().Get(url).
		Set("User-Agent", UserAgent).
		Retry(1, 3*time.Second).Timeout(30 * time.Second).EndBytes()
	if nil != errs {
		return "", nil, cleanup, fmt.Errorf("download package.zip: %v", errs)
	}
	if 200 != resp.StatusCode {
		return "", nil, cleanup, fmt.Errorf("download package.zip: HTTP %d", resp.StatusCode)
	}

	osTmpDir := filepath.Join(os.TempDir(), "bazaar")
	if err = os.MkdirAll(osTmpDir, 0755); nil != err {
		return "", nil, cleanup, fmt.Errorf("mkdir temp: %w", err)
	}

	tmpZipPath := filepath.Join(osTmpDir, gulu.Rand.String(7)+".zip")
	if err = os.WriteFile(tmpZipPath, data, 0644); nil != err {
		return "", nil, cleanup, fmt.Errorf("write package.zip: %w", err)
	}

	tmpUnzipPath := filepath.Join(osTmpDir, gulu.Rand.String(7))
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
