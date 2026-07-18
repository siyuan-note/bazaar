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

// PathNames 在 Windows 与类 Unix（Linux、macOS）常见规则下递归检查包内路径/文件名是否规范。
// Windows：保留设备名（基名与保留名一致时拒绝，不区分大小写）、不得以空格开头或结尾（见 Microsoft 文档）、不得以空格开头（无法手动创建此类文件夹）。
// REF https://learn.microsoft.com/zh-cn/windows/win32/fileio/naming-a-file#naming-conventions
func PathNames(root string) []Issue {
	var issues []Issue
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			issues = append(issues, issue(fmt.Sprintf("检查包内文件时无法访问路径 `%s`：%v。请确认 `package.zip` 完整、未损坏。", path, err),
				fmt.Sprintf("Couldn't access path `%s` while inspecting the package: %v. Please make sure `package.zip` is complete and not corrupted.", path, err),
			))
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}
		if rel == "." {
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		name := d.Name()

		if strings.HasPrefix(name, " ") || strings.HasSuffix(name, " ") {
			issues = append(issues, issue(fmt.Sprintf("包内路径 `%s` 的文件/目录名 `%s` 以空格开头或结尾，不受 Windows 系统支持，请去掉首尾空格。", relSlash, name),
				fmt.Sprintf("In package path `%s`, the file/folder name `%s` has leading or trailing spaces, which Windows doesn't support. Please trim them.", relSlash, name),
			))
		}
		if IsReservedWindowsDeviceName(name) {
			issues = append(issues, issue(fmt.Sprintf("包内路径 `%s` 的名称 `%s` 是 Windows 系统保留设备名（如 `CON`、`PRN`、`AUX`、`NUL`、`COM1`、`LPT1` 等），请改成其他名称。", relSlash, name),
				fmt.Sprintf("In package path `%s`, the name `%s` is a Windows reserved device name (e.g. `CON`, `PRN`, `AUX`, `NUL`, `COM1`, `LPT1`). Please rename it.", relSlash, name),
			))
		}
		return nil
	})
	return issues
}

// IsReservedWindowsDeviceName 检查名称是否为 Windows 保留设备名（不区分大小写）。
// 按 Microsoft 文档：保留名后紧跟扩展名也等价于保留名（如 NUL.txt、NUL.tar.gz 均等同于 NUL），故取第一个句点前的基名比对。
// REF https://learn.microsoft.com/zh-cn/windows/win32/fileio/naming-a-file#naming-conventions
func IsReservedWindowsDeviceName(name string) bool {
	base := name
	if before, _, ok := strings.Cut(name, "."); ok {
		base = before
	}
	base = strings.ToUpper(base)
	switch base {
	case "CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		return true
	default:
		return false
	}
}
