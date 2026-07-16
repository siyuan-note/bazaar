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
)

// ResolvePackageRoot 确定包根目录。
// 正常情况下文件应直接在解压根目录；若 path 下恰好有一个子目录且根下没有文件，则将该子目录视为包根
// （兼容「zip 内多包一层文件夹」的常见打包方式）。
// 根下多个子目录且无文件时返回错误（无法唯一确定包根）。
func ResolvePackageRoot(path string) (string, error) {
	if path == "" {
		return "", LocalizedErr(
			"内部错误：未能定位 `package.zip` 的解压目录。这通常是集市检查流程配置问题，请联系维护者重试。",
			"Internal error: could not locate the extracted `package.zip` directory. This is usually a bazaar checker configuration issue; contact a maintainer.",
			nil,
		)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", LocalizedErr(
			fmt.Sprintf("无法读取 `package.zip` 解压后的内容：%v。请确认 Latest Release 中的 `package.zip` 可正常下载且为合法 zip。", err),
			fmt.Sprintf("Cannot read the extracted `package.zip` contents: %v. Ensure `package.zip` in the Latest Release downloads correctly and is a valid zip.", err),
			err,
		)
	}
	if !info.IsDir() {
		return "", LocalizedErr(
			"无法从 `package.zip` 确定包根目录：解压结果不是有效目录。请确认 `package.zip` 为合法 zip；若仍失败请联系集市维护者。",
			"Cannot determine the package root from `package.zip`: the extraction result is not a valid directory. Ensure `package.zip` is a valid archive; contact a bazaar maintainer if this persists.",
			nil,
		)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return "", LocalizedErr(
			fmt.Sprintf("无法列出 `package.zip` 解压后的文件：%v。请确认 zip 未损坏。", err),
			fmt.Sprintf("Cannot list files inside the extracted `package.zip`: %v. Ensure the zip is not corrupted.", err),
			err,
		)
	}

	var dirs []string
	hasFile := false
	for _, e := range entries {
		name := e.Name()
		if name == ".DS_Store" || name == "__MACOSX" {
			continue
		}
		if e.IsDir() {
			dirs = append(dirs, name)
			continue
		}
		hasFile = true
	}

	if !hasFile && len(dirs) == 1 {
		return filepath.Join(path, dirs[0]), nil
	}
	if len(dirs) > 1 && !hasFile {
		return "", LocalizedErr(
			fmt.Sprintf("无法从 `package.zip` 确定包根目录：解压根下有 %d 个并列文件夹，且没有任何文件。请把 `icon.png`、清单文件等必要文件直接放在 zip 根目录，或只保留一层包装文件夹（不要并排放多个无关目录）。", len(dirs)),
			fmt.Sprintf("Cannot determine the package root from `package.zip`: found %d sibling folders under the extraction root and no files. Put required files such as `icon.png` and the manifest (e.g. `plugin.json`) at the zip root, or use exactly one wrapping folder—not multiple top-level directories.", len(dirs)),
			nil,
		)
	}
	return path, nil
}
