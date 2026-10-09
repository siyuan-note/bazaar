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
	"io/fs"
	"strings"
)

// issueExampleLimit 单条 Issue 中最多列出的条目示例数。
const issueExampleLimit = 5

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

	listedZh, listedEn := formatExampleList(bad)
	return []Issue{issue(
		fmt.Sprintf("`package.zip` 内有 %d 个条目路径使用了反斜杠 `\\`，例如 %s。ZIP 规范要求路径分隔符必须是正斜杠 `/`。请改用会写入 `/` 的打包方式重新生成 `package.zip`（不要用会写入 `\\` 的 Windows 压缩方式），并更新 GitHub Release。", len(bad), listedZh),
		fmt.Sprintf("`package.zip` has %d entries whose paths use backslash `\\`, e.g. %s. The ZIP format requires forward slash `/` as the path separator. Please rebuild `package.zip` with a tool that writes `/` (avoid Windows zip methods that emit `\\`), then update the GitHub Release.", len(bad), listedEn),
	)}
}

// ZipEntryTypes 检查 package.zip 内条目类型是否全部为普通文件或目录。
// 符号链接、FIFO、套接字、设备节点都无法跨平台可靠解压，思源内核会拒绝安装包含这类条目的包
// （kernel/bazaar/local.go validateLocalPackageArchive）；
// 而条目类型在解压后不可见（解压工具会把符号链接落成内容为链接目标的普通文件），只能在原始 zip 字节上判定。
func ZipEntryTypes(zipData []byte) []Issue {
	if len(zipData) == 0 {
		return nil
	}
	r, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		// 解析失败已由 ZipPaths 报告，此处不重复。
		return nil
	}

	var links, others []string
	for _, f := range r.File {
		mode := f.Mode()
		switch {
		case mode&fs.ModeSymlink != 0:
			links = append(links, f.Name)
		case !mode.IsRegular() && !mode.IsDir():
			others = append(others, f.Name)
		}
	}

	var issues []Issue
	if len(links) > 0 {
		listedZh, listedEn := formatExampleList(links)
		issues = append(issues, issue(
			fmt.Sprintf("`package.zip` 内有 %d 个符号链接条目，例如 %s。符号链接无法跨平台可靠解压，思源客户端会拒绝安装包含符号链接的包。请改用会跟随链接的打包方式重新生成 `package.zip`（例如 `zip -r`，不要加 `-y`），或把链接替换为真实文件或目录。", len(links), listedZh),
			fmt.Sprintf("`package.zip` has %d symbolic link entries, e.g. %s. Symbolic links can't be extracted reliably across platforms, and the SiYuan client rejects a package containing them. Please rebuild `package.zip` with a tool that follows links (e.g. `zip -r`, without `-y`), or replace the links with real files or folders.", len(links), listedEn),
		))
	}
	if len(others) > 0 {
		listedZh, listedEn := formatExampleList(others)
		issues = append(issues, issue(
			fmt.Sprintf("`package.zip` 内有 %d 个非常规条目（既不是普通文件也不是目录），例如 %s。思源客户端会拒绝安装包含这类条目的包，请从 `package.zip` 中移除。", len(others), listedZh),
			fmt.Sprintf("`package.zip` has %d entries that are neither regular files nor directories, e.g. %s. The SiYuan client rejects a package containing them. Please remove them from `package.zip`.", len(others), listedEn),
		))
	}
	return issues
}

// formatExampleList 把条目名格式化为中英文示例列表：最多列出 issueExampleLimit 项，超出部分附数量说明。
func formatExampleList(names []string) (zh, en string) {
	examples, remaining := names, 0
	if len(examples) > issueExampleLimit {
		remaining = len(examples) - issueExampleLimit
		examples = examples[:issueExampleLimit]
	}
	zh = "`" + strings.Join(examples, "`、`") + "`"
	en = "`" + strings.Join(examples, "`, `") + "`"
	if remaining > 0 {
		zh += fmt.Sprintf("（另有 %d 条未列出）", remaining)
		en += fmt.Sprintf(" (%d more not listed)", remaining)
	}
	return
}
