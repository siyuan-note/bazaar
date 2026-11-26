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
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/siyuan-note/bazaar/actions/util"
	"github.com/siyuan-note/bazaar/rules"
)

const packageBaseURL = "https://oss.b3logfile.com/package/"

func main() {
	ownerRepo := strings.TrimSpace(os.Getenv("BAZAAR_PACKAGE"))
	if err := auditPackage(ownerRepo); err != nil {
		fmt.Fprintf(os.Stderr, "ZIP path audit for %q: %v\n", ownerRepo, err)
		os.Exit(1)
	}
}

func auditPackage(ownerRepo string) error {
	if ownerRepo == "" {
		return errors.New("set BAZAAR_PACKAGE to a listed owner/repo")
	}
	for _, packageType := range rules.AllPackageTypes() {
		listed, err := util.FindStageRepo(".", packageType, ownerRepo)
		if err != nil {
			return err
		}
		if listed == nil {
			continue
		}
		client := &http.Client{Timeout: time.Minute}
		if err = auditDistributedZIP(client, listed); err != nil {
			return fmt.Errorf("%q: %w", listed.URL, err)
		}
		fmt.Printf("%q passed the ZIP path check\n", listed.URL)
		return nil
	}
	return errors.New("package was not found in the Stage index")
}

func auditDistributedZIP(client *http.Client, listed *util.StageRepo) error {
	// 按索引记录限制读取长度，避免把 CDN 错误响应当作发布资产。
	if listed.Size <= 0 || listed.Size > 2<<30 {
		return fmt.Errorf("invalid ZIP size in Stage index: %d", listed.Size)
	}
	response, err := client.Get(packageBaseURL + listed.URL)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("package download returned HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, listed.Size+1))
	if err != nil {
		return err
	}
	if int64(len(data)) != listed.Size {
		return fmt.Errorf("downloaded ZIP size %d does not match Stage index size %d", len(data), listed.Size)
	}
	return checkZIPPaths(data, listed.PackageZipSHA256)
}

func checkZIPPaths(data []byte, digest string) error {
	if digest != "" && util.SHA256Hex(data) != digest {
		return errors.New("downloaded ZIP digest does not match the Stage index")
	}
	if issues := rules.ZipPaths(data); len(issues) != 0 {
		return fmt.Errorf("path check failed: %q", issues[0].MessageEn)
	}
	return nil
}
