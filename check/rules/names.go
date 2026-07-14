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

// PathNames 递归检查路径/文件名是否规范。
func PathNames(root string) []Issue {
	var issues []Issue
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			issues = append(issues, issue("names/walk",
				fmt.Sprintf("检查包内文件时无法访问路径 %s：%v。请确认 package.zip 完整、未损坏，且其中没有异常的权限或损坏条目。", path, err),
				fmt.Sprintf("Could not access path %s while inspecting the package: %v. Ensure package.zip is complete and not corrupted.", path, err),
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
			issues = append(issues, issue("names/whitespace",
				fmt.Sprintf("包内路径 %s 的文件/目录名 %q 以空格开头或结尾。请去掉首尾空格后重新打包 package.zip（部分系统上此类名称会导致安装失败）。", relSlash, name),
				fmt.Sprintf("Entry %s uses name %q with leading or trailing spaces. Remove those spaces and rebuild package.zip (such names can break installs on some systems).", relSlash, name),
			))
		}
		if IsReservedWindowsDeviceName(name) {
			issues = append(issues, issue("names/reserved",
				fmt.Sprintf("包内路径 %s 的名称 %q 是 Windows 保留设备名（如 CON、PRN、AUX、NUL、COM1、LPT1 等）。请改成普通名称后重新打包，否则在 Windows 上可能无法解压或安装。", relSlash, name),
				fmt.Sprintf("Entry %s uses name %q, which is a Windows reserved device name (CON, PRN, AUX, NUL, COM1, LPT1, etc.). Rename it and rebuild package.zip, or Windows installs may fail.", relSlash, name),
			))
		}
		return nil
	})
	return issues
}

// IsReservedWindowsDeviceName 检查基名（去掉扩展名之前）是否为保留设备名。
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
