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
	"html"
	"strings"
)

// validatePackageName 在 Windows 与类 Unix（Linux、macOS）常见规则下校验集市包 name。
// Windows：保留字符、C0 控制字符（1–31）、NUL、保留设备名（整段 name 与保留名完全一致时拒绝，不区分大小写）、不得以空格或句点结尾（见 Microsoft 文档）、不得以空格开头（无法手动创建此类文件夹）。
// REF https://learn.microsoft.com/zh-cn/windows/win32/fileio/naming-a-file#naming-conventions
// Linux / macOS：路径分隔符 / 与 NUL 与上述保留字符一并禁止；名称 UTF-8 编码长度不超过 NAME_MAX（255 字节）。
// 仅允许可打印 ASCII（U+0020–U+007E），降低编码、终端与跨平台工具链上的兼容风险。
// 集市约定：不得以句点（.）开头，避免隐藏文件夹。
// 返回可直接用于 PR 评论的双语错误（尽量列全所有问题）。
func validatePackageName(name string) []error {
	var errs []error
	if name == "" {
		errs = append(errs, LocalizedErr(
			"清单字段 `name` 不能为空。请在 JSON 根级填写与 GitHub 仓库名一致的字符串。",
			"Manifest field `name` must not be empty. Set a string at the JSON root that matches the GitHub repository name.",
			nil,
		))
		return errs
	}
	// 单路径组件名称的字节长度上限（NAME_MAX），Linux ext4、macOS APFS 等常见文件系统均为 255。
	if len(name) > 255 {
		errs = append(errs, LocalizedErr(
			fmt.Sprintf("清单字段 `name` 的值 `%s` 超过长度上限（255 字节）。请缩短名称，使其与 GitHub 仓库名一致。", name),
			fmt.Sprintf("Manifest field `name` value `%s` exceeds the maximum length (255 bytes). Shorten it to match the GitHub repository name.", name),
			nil,
		))
	}
	if strings.HasPrefix(name, ".") {
		errs = append(errs, LocalizedErr(
			fmt.Sprintf("清单字段 `name` 的值 `%s` 以句点开头。目录名不得以 `.` 开头（会变成隐藏文件夹）。请改成不以 `.` 开头的名称。", name),
			fmt.Sprintf("Manifest field `name` value `%s` starts with a period. Names must not start with `.` (hidden folder). Choose a name without a leading period.", name),
			nil,
		))
	}
	if strings.HasPrefix(name, " ") {
		errs = append(errs, LocalizedErr(
			fmt.Sprintf("清单字段 `name` 的值 `%s` 以空格开头。请去掉开头空格，使名称与 GitHub 仓库名一致。", name),
			fmt.Sprintf("Manifest field `name` value `%s` starts with a space. Remove the leading space so it matches the GitHub repository name.", name),
			nil,
		))
	}
	appendNameDisallowedRunesIssues(name, &errs)
	// validateNameNoTrailingSpaceOrPeriod：不得以空格或句点结束名称（见 Microsoft 文档：Shell 与用户界面不支持此类名称）。
	if strings.HasSuffix(name, " ") {
		errs = append(errs, LocalizedErr(
			fmt.Sprintf("清单字段 `name` 的值 `%s` 以空格结尾。请去掉末尾空格。", name),
			fmt.Sprintf("Manifest field `name` value `%s` ends with a space. Remove the trailing space.", name),
			nil,
		))
	}
	if strings.HasSuffix(name, ".") {
		errs = append(errs, LocalizedErr(
			fmt.Sprintf("清单字段 `name` 的值 `%s` 以句点结尾。请去掉末尾句点。", name),
			fmt.Sprintf("Manifest field `name` value `%s` ends with a period. Remove the trailing period.", name),
			nil,
		))
	}
	// validateNameNotReservedDevice：若整段 name 与 Windows 保留设备名相同则拒绝（不区分大小写）。
	// 带后缀的形式（如 CON.123）在资源管理器中可作为普通文件夹名，故不按「点号前 stem」截取比对。
	if _, ok := reservedWindowsDeviceNames[strings.ToUpper(name)]; ok {
		errs = append(errs, LocalizedErr(
			fmt.Sprintf("清单字段 `name` 的值 `%s` 是 Windows 保留设备名（如 `CON`、`PRN`、`AUX`）。请改用其他名称。", name),
			fmt.Sprintf("Manifest field `name` value `%s` is a Windows reserved device name (e.g. `CON`, `PRN`, `AUX`). Choose a different name.", name),
			nil,
		))
	}
	// validatePlainStringForHTML：用于 HTML 展示的清单字面量须与 html.EscapeString(s) 相同，避免 XSS。
	if html.EscapeString(name) != name {
		errs = append(errs, LocalizedErr(
			fmt.Sprintf("清单字段 `name` 的值 `%s` 包含 HTML 特殊字符（`<`、`>`、`&`、`'`、`\"`）。请从名称中移除这些字符。", name),
			fmt.Sprintf("Manifest field `name` value `%s` contains HTML-special characters (`<`, `>`, `&`, `'`, `\"`). Remove them.", name),
			nil,
		))
	}
	return errs
}

// validateManifestAuthor 校验用于 HTML 展示的清单字段 author：去掉首尾空白后须非空，且整段字符串与 html.EscapeString(s) 相同，避免 XSS。
// 返回可直接用于 PR 评论的双语错误。
func validateManifestAuthor(author string) []error {
	var errs []error
	if strings.TrimSpace(author) == "" {
		errs = append(errs, LocalizedErr(
			"清单字段 `author` 不能为空或仅含空白字符。请填写普通文本作者名，例如 `\"author\": \"your-name\"`。",
			"Manifest field `author` must not be empty or whitespace only. Set plain text such as `\"author\": \"your-name\"`.",
			nil,
		))
	}
	if html.EscapeString(author) != author {
		errs = append(errs, LocalizedErr(
			fmt.Sprintf("清单字段 `author` 的值 `%s` 包含 HTML 特殊字符（`<`、`>`、`&`、`'`、`\"`）。请改成普通文本。", author),
			fmt.Sprintf("Manifest field `author` value `%s` contains HTML-special characters (`<`, `>`, `&`, `'`, `\"`). Use plain text.", author),
			nil,
		))
	}
	return errs
}

// appendNameDisallowedRunesIssues：仅允许可打印 ASCII（含空格，即 U+0020–U+007E）；并禁止其中 Windows / POSIX 路径保留字符。
func appendNameDisallowedRunesIssues(name string, errs *[]error) {
	var hasNonPrintable, hasReserved bool
	for _, r := range name {
		if r < 0x20 || r > 0x7E {
			hasNonPrintable = true
		}
		switch r {
		case '<', '>', ':', '"', '/', '\\', '|', '?', '*':
			hasReserved = true
		}
	}
	if hasNonPrintable {
		*errs = append(*errs, LocalizedErr(
			fmt.Sprintf("清单字段 `name` 的值 `%s` 包含不可打印 ASCII 字符（如中文、表情符号）。请改用仅含可打印 ASCII 的名称，与 GitHub 仓库名保持一致。", name),
			fmt.Sprintf("Manifest field `name` value `%s` contains non-printable ASCII characters (e.g. CJK or emoji). Use printable ASCII only, matching the GitHub repository name.", name),
			nil,
		))
	}
	if hasReserved {
		*errs = append(*errs, LocalizedErr(
			fmt.Sprintf("清单字段 `name` 的值 `%s` 包含路径保留字符（如 `<` `>` `:` `\"` `/` `\\` `|` `?` `*`）。请从名称中移除这些字符。", name),
			fmt.Sprintf("Manifest field `name` value `%s` contains reserved path characters (e.g. `<` `>` `:` `\"` `/` `\\` `|` `?` `*`). Remove them.", name),
			nil,
		))
	}
}

// reservedWindowsDeviceNames 为 Windows 保留设备名（整名精确匹配，不区分大小写）。名称已限制为 ASCII，故不含上标数字变体。
var reservedWindowsDeviceNames = map[string]struct{}{
	"CON": {}, "PRN": {}, "AUX": {}, "NUL": {},
	"COM1": {}, "COM2": {}, "COM3": {}, "COM4": {}, "COM5": {}, "COM6": {}, "COM7": {}, "COM8": {}, "COM9": {},
	"LPT1": {}, "LPT2": {}, "LPT3": {}, "LPT4": {}, "LPT5": {}, "LPT6": {}, "LPT7": {}, "LPT8": {}, "LPT9": {},
}
