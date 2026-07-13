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
	"errors"
	"fmt"
	"html"
	"strings"
)

// maxNameBytesPOSIX 为单路径组件名称的字节长度上限（NAME_MAX）。
const maxNameBytesPOSIX = 255

// validatePlainStringForHTML 校验用于 HTML 展示的字面量：去首尾空白后非空，且不含需转义的 HTML 特殊字符。
func validatePlainStringForHTML(s string) error {
	if strings.TrimSpace(s) == "" {
		return errors.New("value is empty or whitespace only")
	}
	if html.EscapeString(s) != s {
		return errors.New(`contains HTML-special character: <, >, &, ' and "`)
	}
	return nil
}

// validatePackageName 在 Windows 与类 Unix 常见规则下校验集市包 name（对齐旧 util.ValidateName）。
func validatePackageName(name string) error {
	if name == "" {
		return errors.New("[name] is empty")
	}
	if len(name) > maxNameBytesPOSIX {
		return errors.New("[name] exceeds maximum length in bytes (Linux/macOS NAME_MAX)")
	}
	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, " ") {
		return errors.New("[name] must not start with period or space")
	}
	if err := validateNameDisallowedRunes(name); err != nil {
		return err
	}
	if err := validateNameNoTrailingSpaceOrPeriod(name); err != nil {
		return err
	}
	if err := validateNameNotReservedDevice(name); err != nil {
		return err
	}
	if err := validatePlainStringForHTML(name); err != nil {
		return fmt.Errorf("[name] %w", err)
	}
	return nil
}

func validateNameDisallowedRunes(name string) error {
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

func validateNameNoTrailingSpaceOrPeriod(name string) error {
	runes := []rune(name)
	last := runes[len(runes)-1]
	if last == ' ' || last == '.' {
		return errors.New("[name] must not end with space or period")
	}
	return nil
}

// validateNameNotReservedDevice：整段 name 与 Windows 保留设备名相同则拒绝（不截取扩展名）。
func validateNameNotReservedDevice(name string) error {
	switch strings.ToUpper(name) {
	case "CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		return errors.New("[name] is a reserved word")
	default:
		return nil
	}
}
