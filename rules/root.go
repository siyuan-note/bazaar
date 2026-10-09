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
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolvePackageRoot 确定包根目录。
// 判定必须与思源内核 `kernel/bazaar/install.go` 保持一致：解压根下恰好只有一个条目且该条目是目录时，
// 剥离这层包装目录（兼容「zip 内多包一层文件夹」的常见打包方式）；否则解压根本身就是包根。
// 内核用 `os.ReadDir` 直接计数、不跳过任何条目，因此这里也不能跳过 macOS 压缩残留，
// 否则会出现「集市检查通过、客户端安装失败」。
// 根下多个并列子目录且没有任何文件时返回错误（无法唯一确定包根）。
func ResolvePackageRoot(path string) (string, error) {
	if path == "" {
		return "", LocalizedErr(
			"内部错误：未能定位 `package.zip` 的解压目录。这通常是集市检查流程配置问题，请联系维护者。",
			"Internal error: couldn't locate the extracted `package.zip` directory. This usually means a bazaar checker config problem — please contact a maintainer.",
			nil,
		)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", LocalizedErr(
			fmt.Sprintf("无法读取 `package.zip` 解压后的内容：%v。请确认 Latest Release 中的 `package.zip` 可正常下载且为合法 zip。", err),
			fmt.Sprintf("Couldn't read the extracted `package.zip` contents: %v. Please make sure `package.zip` in the Latest Release downloads fine and is a valid zip.", err),
			err,
		)
	}
	if !info.IsDir() {
		return "", LocalizedErr(
			"无法从 `package.zip` 确定包根目录：解压结果不是有效目录。请确认 `package.zip` 为合法 zip。",
			"Couldn't determine the package root from `package.zip`: the extraction result isn't a valid directory. Please make sure `package.zip` is a valid zip.",
			nil,
		)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return "", LocalizedErr(
			fmt.Sprintf("无法列出 `package.zip` 解压后的文件：%v。请确认 zip 未损坏。", err),
			fmt.Sprintf("Couldn't list files inside the extracted `package.zip`: %v. Please make sure the zip isn't corrupted.", err),
			err,
		)
	}

	var dirs []string
	hasFile := false
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
			continue
		}
		hasFile = true
	}

	// 与内核一致：解压根下只有一个子目录时剥离该层。
	if len(entries) == 1 && entries[0].IsDir() {
		return filepath.Join(path, entries[0].Name()), nil
	}
	// 内核不跳过 macOS 残留，因此这层包装目录不会被剥离；提前给出原因，避免作者只看到「缺少必要文件」。
	if residue, wrapper, ok := macOSArchiveResidue(entries); ok {
		return "", LocalizedErr(
			fmt.Sprintf("无法从 `package.zip` 确定包根目录：解压根下有 macOS 压缩残留（`%s`）和唯一的子目录 `%s`。思源内核按解压根下的实际条目判断包根、不会跳过这些残留，因此不会剥离 `%s` 这层，客户端安装时会找不到清单文件。请删除残留后重新打包（例如使用 `zip -r`，不要直接压缩整个文件夹）。", strings.Join(residue, "`、`"), wrapper, wrapper),
			fmt.Sprintf("Couldn't determine the package root from `package.zip`: the extraction root has macOS archive residue (`%s`) next to a single subfolder `%s`. The SiYuan kernel determines the package root from the actual entries under the extraction root and doesn't skip this residue, so it won't strip `%s`, and the client will fail to find the manifest. Please remove the residue and repackage (e.g. use `zip -r` instead of compressing the whole folder).", strings.Join(residue, "`, `"), wrapper, wrapper),
			nil,
		)
	}
	if len(dirs) > 1 && !hasFile {
		return "", LocalizedErr(
			fmt.Sprintf("无法从 `package.zip` 确定包根目录：解压根下有 %d 个并列文件夹，且没有任何文件。请把 `README.md`、清单文件等必要文件直接放在 zip 根目录，或只保留一层包装文件夹（不要并排放多个无关目录）。", len(dirs)),
			fmt.Sprintf("Couldn't determine the package root from `package.zip`: there are %d sibling folders under the extraction root and no files. Please put required files like `README.md` and the manifest directly at the zip root, or keep exactly one wrapping folder — not several top-level directories side by side.", len(dirs)),
			nil,
		)
	}
	return path, nil
}

// macOSArchiveResidue 报告会干扰包根判定的 macOS 压缩残留。
// 仅当剔除残留后解压根恰好只剩一个子目录、且没有任何普通文件时返回 ok，
// 因为此时作者通常是想多包一层文件夹，而内核按实际条目计数不会剥离，两者结果不一致。
// residue 为残留条目名，wrapper 为那个唯一的子目录名。
func macOSArchiveResidue(entries []os.DirEntry) (residue []string, wrapper string, ok bool) {
	dirs, files := 0, 0
	for _, e := range entries {
		if isMacOSArchiveResidue(e.Name()) {
			residue = append(residue, e.Name())
			continue
		}
		if e.IsDir() {
			dirs++
			wrapper = e.Name()
			continue
		}
		files++
	}
	if len(residue) == 0 || dirs != 1 || files != 0 {
		return nil, "", false
	}
	return residue, wrapper, true
}

// isMacOSArchiveResidue 判断名称是否为 macOS 压缩工具（Finder、ditto -c -k 等）写入的残留条目。
func isMacOSArchiveResidue(name string) bool {
	return name == ".DS_Store" || name == "__MACOSX"
}
