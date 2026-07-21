// SiYuan community bazaar.
// Copyright (c) 2021-present, b3log.org
//
// Bazaar is licensed under Mulan PSL v2.
// You can use this software according to the terms and conditions of the Mulan PSL v2.
// You may obtain a copy of Mulan PSL v2 at:
//         http://license.coscl.org.cn/MulanPSL2
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR PURPOSE.
// See the Mulan PSL v2 for more details.

package rules

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
)

// zipPathSepExampleLimit 单条 Issue 中最多列出的反斜杠路径示例数。
const zipPathSepExampleLimit = 5

// ZipPaths 检查 package.zip 内条目路径是否全部使用正斜杠 `/`。
// ZIP 规范要求路径分隔符为 `/`；Windows 部分压缩工具会写入 `\`，导致跨平台解压异常。
func ZipPaths(zipData []byte) []Issue {
	if len(zipData) == 0 {
		return nil
	}
	r, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return []Issue{issue(
			fmt.Sprintf("无法解析 `package.zip`：%v。请确认 Release 中的 zip 未损坏，并用标准 zip 工具重新打包。", err),
			fmt.Sprintf("Couldn't parse `package.zip`: %v. Please make sure the Release zip isn't corrupted, and rebuild it with a standard zip tool.", err),
		)}
	}

	var bad []string
	for _, f := range r.File {
		if strings.Contains(f.Name, `\`) {
			bad = append(bad, f.Name)
		}
	}
	if len(bad) == 0 {
		return nil
	}

	examples := bad
	if len(examples) > zipPathSepExampleLimit {
		examples = examples[:zipPathSepExampleLimit]
	}
	listed := "`" + strings.Join(examples, "`、`") + "`"
	listedEn := "`" + strings.Join(examples, "`, `") + "`"
	moreZh, moreEn := "", ""
	if len(bad) > zipPathSepExampleLimit {
		moreZh = fmt.Sprintf("（另有 %d 条未列出）", len(bad)-zipPathSepExampleLimit)
		moreEn = fmt.Sprintf(" (%d more not listed)", len(bad)-zipPathSepExampleLimit)
	}

	return []Issue{issue(
		fmt.Sprintf("`package.zip` 内有 %d 个条目路径使用了反斜杠 `\\`，例如 %s%s。ZIP 规范要求路径分隔符必须是正斜杠 `/`。请改用会写入 `/` 的打包方式重新生成 `package.zip`（不要用会写入 `\\` 的 Windows 压缩方式），并更新 GitHub Release。", len(bad), listed, moreZh),
		fmt.Sprintf("`package.zip` has %d entries whose paths use backslash `\\`, e.g. %s%s. The ZIP format requires forward slash `/` as the path separator. Please rebuild `package.zip` with a tool that writes `/` (avoid Windows zip methods that emit `\\`), then update the GitHub Release.", len(bad), listedEn, moreEn),
	)}
}
