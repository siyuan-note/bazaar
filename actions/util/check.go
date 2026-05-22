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
	"errors"
	"fmt"
	"html"
	"strings"
)

// ValidatePlainStringForHTML 校验用于 HTML 展示的清单字面量：去掉首尾空白后须非空，且整段字符串与 html.EscapeString(s) 相同，避免 XSS。
func ValidatePlainStringForHTML(s string) error {
	if strings.TrimSpace(s) == "" {
		return errors.New("value is empty or whitespace only")
	}
	if html.EscapeString(s) != s {
		return errors.New(`contains HTML-special character: <, >, &, ' and "`)
	}
	return nil
}

// maxNameBytesPOSIX 为单路径组件名称的字节长度上限（NAME_MAX），Linux ext4、macOS APFS 等常见文件系统均为 255。
const maxNameBytesPOSIX = 255

// ValidateName 在 Windows 与类 Unix（Linux、macOS）常见规则下校验集市包 name。
// Windows：保留字符、C0 控制字符（1–31）、NUL、保留设备名（整段 name 与保留名完全一致时拒绝，不区分大小写）、不得以空格或句点结尾（见 Microsoft 文档）、不得以空格开头（无法手动创建此类文件夹）。
// REF https://learn.microsoft.com/zh-cn/windows/win32/fileio/naming-a-file#naming-conventions
// Linux / macOS：路径分隔符 / 与 NUL 与上述保留字符一并禁止；名称 UTF-8 编码长度不超过 NAME_MAX（255 字节）。
// 仅允许可打印 ASCII（U+0020–U+007E），降低编码、终端与跨平台工具链上的兼容风险。
// 集市约定：不得以句点（.）开头，避免隐藏文件夹。
func ValidateName(name string) error {
	if name == "" {
		return errors.New("[name] is empty")
	}
	if len(name) > maxNameBytesPOSIX {
		return errors.New("[name] exceeds maximum length in bytes (Linux/macOS NAME_MAX)")
	}
	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, " ") {
		return errors.New("[name] must not start with period or space")
	}
	if err := validateDisallowedRunes(name); err != nil {
		return err
	}
	if err := validateNoTrailingSpaceOrPeriod(name); err != nil {
		return err
	}
	if err := checkReservedWindowsDeviceName(name); err != nil {
		return err
	}
	if err := ValidatePlainStringForHTML(name); err != nil {
		return fmt.Errorf("[name] %w", err)
	}
	return nil
}

// validateDisallowedRunes：仅允许可打印 ASCII（含空格，即 U+0020–U+007E）；并禁止其中 Windows / POSIX 路径保留字符。
func validateDisallowedRunes(name string) error {
	for _, r := range name {
		if r < 0x20 || r > 0x7E {
			return errors.New("[name] contains characters other than printable ASCII characters")
		}
		switch r {
		case '<', '>', ':', '"', '/', '\\', '|', '?', '*':
			return errors.New("[name] contains reserved character")
		}
	}
	return nil
}

// validateNoTrailingSpaceOrPeriod：不得以空格或句点结束名称（见 Microsoft 文档：Shell 与用户界面不支持此类名称）。
func validateNoTrailingSpaceOrPeriod(name string) error {
	runes := []rune(name)
	last := runes[len(runes)-1]
	if last == ' ' || last == '.' {
		return errors.New("[name] must not end with space or period")
	}
	return nil
}

// checkReservedWindowsDeviceName：若整段 name 与 Windows 保留设备名相同则拒绝（不区分大小写）。
// 带后缀的形式（如 CON.123）在资源管理器中可作为普通文件夹名，故不按「点号前 stem」截取比对。
func checkReservedWindowsDeviceName(name string) error {
	if _, ok := reservedWords[strings.ToUpper(name)]; ok {
		return errors.New("[name] is a reserved word")
	}
	return nil
}

// reservedWords 为 Windows 保留设备名（整名精确匹配，不区分大小写）。名称已限制为 ASCII，故不含上标数字变体。
var reservedWords = map[string]any{
	"CON": nil, "PRN": nil, "AUX": nil, "NUL": nil,
	"COM1": nil, "COM2": nil, "COM3": nil, "COM4": nil, "COM5": nil,
	"COM6": nil, "COM7": nil, "COM8": nil, "COM9": nil,
	"LPT1": nil, "LPT2": nil, "LPT3": nil, "LPT4": nil, "LPT5": nil,
	"LPT6": nil, "LPT7": nil, "LPT8": nil, "LPT9": nil,
}
