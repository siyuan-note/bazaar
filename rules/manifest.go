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
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/semver"
)

// LocaleStrings 表示按 locale 键（如 default、zh_CN、en_US）组织的多语言字符串。
type LocaleStrings map[string]string

// Funding 表示清单 JSON 中 funding 字段的资助信息。
type Funding struct {
	OpenCollective string   `json:"openCollective"`
	Patreon        string   `json:"patreon"`
	GitHub         string   `json:"github"`
	Custom         []string `json:"custom"`
}

// Package 集市包清单 JSON 解析后的元数据（plugin.json / theme.json 等）。
// 字段划分与思源内核 kernel/bazaar/package.go 及 plugin.go 的实际消费一致。
//
// omitempty 说明：本结构体会写入 stage 索引供客户端反序列化。
// name/author/url/version 为身份字段，始终序列化；其余可选字段在零值时省略，
// 与 v3.5.5 以前的旧版客户端（缺失键 → 零值 / nil）及现版逻辑兼容。
type Package struct {
	Name    string `json:"name"`
	Author  string `json:"author"`
	URL     string `json:"url"`
	Version string `json:"version"`

	MinAppVersion string        `json:"minAppVersion,omitempty"`
	DisplayName   LocaleStrings `json:"displayName,omitempty"`
	Description   LocaleStrings `json:"description,omitempty"`
	Readme        LocaleStrings `json:"readme,omitempty"`
	Funding       *Funding      `json:"funding,omitempty"`
	Keywords      []string      `json:"keywords,omitempty"`

	// 插件专用（仅 plugin.json；见 kernel/bazaar/plugin.go）

	Backends          []string `json:"backends,omitempty"`
	Frontends         []string `json:"frontends,omitempty"`
	Kernels           []string `json:"kernels,omitempty"`
	DisabledInPublish bool     `json:"disabledInPublish,omitempty"`

	// 主题专用（仅 theme.json；见 kernel/bazaar/package.go Modes）

	Modes []string `json:"modes,omitempty"`
}

var commonManifestKeys = []string{
	"name", "author", "url", "version",
	"displayName", "description", "readme",
	"funding", "keywords",
	"minAppVersion",
}

var allowedManifestKeys = map[PackageType]Set{
	TypePlugin:   toKeySet(commonManifestKeys, "backends", "frontends", "kernels", "disabledInPublish"), // 插件专用字段见 kernel/bazaar/plugin.go（兼容性与发布禁用判断）。
	TypeTheme:    toKeySet(commonManifestKeys, "modes"),                                                 // 主题专用字段：亮色 / 暗色模式列表。
	TypeIcon:     toKeySet(commonManifestKeys),
	TypeTemplate: toKeySet(commonManifestKeys),
	TypeWidget:   toKeySet(commonManifestKeys),
}

func toKeySet(base []string, extra ...string) Set {
	m := make(Set, len(base)+len(extra))
	for _, k := range base {
		m[k] = struct{}{}
	}
	for _, k := range extra {
		m[k] = struct{}{}
	}
	return m
}

// 内置主题名（不得被集市主题占用）。
var builtinThemeNames = Set{
	"daylight": {},
	"midnight": {},
}

// 内置图标包名（不得被集市图标占用）。
var builtinIconNames = Set{
	"ant":       {}, // 已废弃，内核自动清理图标包
	"material":  {}, // 已废弃，内核自动清理图标包
	"litheness": {},
}

// ReadPackage 读取清单 JSON，返回原始 map 与解析后的 Package。
// 校验路径需要 map（未知字段 / 类型断言）；索引写入等只需 Package 时可忽略 map。
// map→Package 转换失败时返回零值 Package（不报错），由后续 Manifest 校验报告字段问题。
func ReadPackage(path string) (map[string]any, *Package, error) {
	base := filepath.Base(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, LocalizedErr(
			fmt.Sprintf("无法读取清单文件 `%s`：%v。请确认该文件已打进 `package.zip` 包根、路径大小写完全一致。", base, err),
			fmt.Sprintf("Cannot read manifest `%s`: %v. Ensure the file is at the package root with an exact case-sensitive path.", base, err),
			err,
		)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, nil, LocalizedErr(
			fmt.Sprintf("清单 `%s` 的 JSON 解析失败：%v。请检查 JSON 语法并修正。", base, err),
			fmt.Sprintf("Failed to parse manifest `%s` as JSON: %v. Fix JSON syntax errors.", base, err),
			err,
		)
	}
	if m == nil {
		return nil, nil, LocalizedErr(
			fmt.Sprintf("清单 `%s` 的 JSON 内容为 `null`，请修正清单。", base),
			fmt.Sprintf("Manifest `%s` JSON is `null`; Fix the manifest.", base),
			nil,
		)
	}
	var pkg Package
	if encoded, err := json.Marshal(m); err == nil {
		_ = json.Unmarshal(encoded, &pkg)
	}
	return m, &pkg, nil
}

// SanitizePackage 对集市包直接可能显示的信息做 HTML 转义，避免 XSS。
// 与思源内核 kernel/bazaar/package.go 保持一致；旧版本客户端未转义，线上写入索引前须转义。
func SanitizePackage(pkg *Package) {
	if pkg == nil {
		return
	}
	pkg.Name = html.EscapeString(pkg.Name)
	pkg.Author = html.EscapeString(pkg.Author)
	pkg.Version = html.EscapeString(pkg.Version)
	for k, v := range pkg.DisplayName {
		pkg.DisplayName[k] = html.EscapeString(v)
	}
	for k, v := range pkg.Description {
		pkg.Description[k] = html.EscapeString(v)
	}
	if pkg.Funding != nil {
		pkg.Funding.OpenCollective = html.EscapeString(pkg.Funding.OpenCollective)
		pkg.Funding.Patreon = html.EscapeString(pkg.Funding.Patreon)
		pkg.Funding.GitHub = html.EscapeString(pkg.Funding.GitHub)
		for i, v := range pkg.Funding.Custom {
			pkg.Funding.Custom[i] = html.EscapeString(v)
		}
	}
	for i, kw := range pkg.Keywords {
		pkg.Keywords[i] = html.EscapeString(kw)
	}
}

// ManifestInput 清单规则所需的上下文。
type ManifestInput struct {
	PackageRoot   string
	Owner         string
	Repo          string
	Type          PackageType
	OldName       string
	OldVersion    string
	OccupiedNames Set // 键小写；nil 表示不查唯一性
}

// Manifest 校验清单字段。
func Manifest(m map[string]any, in ManifestInput) []Issue {
	var issues []Issue
	issues = append(issues, checkUnknownKeys(m, in.Type)...)
	issues = append(issues, checkName(m, in)...)
	issues = append(issues, checkAuthor(m, in.Owner)...)
	issues = append(issues, checkURL(m, in.Owner, in.Repo)...)
	issues = append(issues, checkVersion(m, in.OldVersion)...)
	issues = append(issues, checkReadme(m, in.PackageRoot)...)
	issues = append(issues, checkFunding(m)...)
	issues = append(issues, checkOptionalTypedFields(m)...)
	return issues
}

func checkUnknownKeys(m map[string]any, typ PackageType) []Issue {
	var issues []Issue
	allowed := allowedManifestKeys[typ]
	for k := range m {
		if _, ok := allowed[k]; !ok {
			issues = append(issues, issue(
				fmt.Sprintf("`%s` 中出现了预期外的字段 `%s`。请删除该字段（保留未知字段会妨碍思源日后扩展同名字段）。若确有自定义需求，请先在[思源仓库](https://github.com/siyuan-note/siyuan)提 issue 讨论。", typ.ManifestFile(), k),
				fmt.Sprintf("`%s` contains unexpected field `%s`. Please remove it (keeping unknown fields will hinder SiYuan from later extending fields with the same name). If you have a custom need, please open an issue in the [SiYuan repository](https://github.com/siyuan-note/siyuan) for discussion first.", typ.ManifestFile(), k),
			))
		}
	}
	return issues
}

func checkName(m map[string]any, in ManifestInput) []Issue {
	raw, ok := m["name"]
	if !ok {
		return []Issue{issue(
			fmt.Sprintf("清单 `%s` 缺少必填字段 `name`（集市包的包名，通常建议与 GitHub 仓库名保持一致）。请在 JSON 根级添加字符串字段 `name`。", in.Type.ManifestFile()),
			fmt.Sprintf("Manifest `%s` is missing required field `name` (the bazaar package name; usually recommended to match the GitHub repository name). Add a string field `name` at the JSON root.", in.Type.ManifestFile()),
		)}
	}
	name, ok := raw.(string)
	if !ok {
		return []Issue{issue(
			fmt.Sprintf("清单字段 `name` 的类型必须是字符串，当前不是。请改成例如 `\"name\": \"%s\"` 这种写法。", in.Repo),
			fmt.Sprintf("Manifest field `name` must be a string. Use a value like `\"name\": \"%s\"`.", in.Repo),
		)}
	}
	if name == "" {
		return []Issue{issue(
			"清单字段 `name` 不能为空。请填写非空字符串。",
			"Manifest field `name` must not be empty. Set a non-empty string.",
		)}
	}

	if in.OldName != "" {
		if name != in.OldName {
			return []Issue{issue(
				fmt.Sprintf("已上架集市包的 `name` 不可更改。清单里当前是 `%s`，集市已记录为 `%s`。请改回 `%s`；若要换名，需按「更换维护者 / 新包」流程另提 PR。", name, in.OldName, in.OldName),
				fmt.Sprintf("The listed package `name` must not change. Manifest has `%s` but the bazaar already lists `%s`. Set `name` back to `%s`; to rename, follow the maintainer-transfer / new-package PR process.", name, in.OldName, in.OldName),
			)}
		}
		return nil
	}

	switch in.Type {
	case TypeTheme:
		if _, hit := builtinThemeNames[name]; hit {
			return []Issue{issue(
				fmt.Sprintf("`name` 的值 `%s` 与思源内置主题重名，不能上架。请更换清单 `name`。", name),
				fmt.Sprintf("`name` value `%s` conflicts with a built-in SiYuan theme and cannot be listed. Change the manifest `name`.", name),
			)}
		}
	case TypeIcon:
		if _, hit := builtinIconNames[name]; hit {
			return []Issue{issue(
				fmt.Sprintf("`name` 的值 `%s` 与思源内置图标包重名，不能上架。请更换清单 `name`。", name),
				fmt.Sprintf("`name` value `%s` conflicts with a built-in SiYuan icon pack and cannot be listed. Change the manifest `name`.", name),
			)}
		}
	}
	if _, exists := in.OccupiedNames[strings.ToLower(name)]; exists {
		return []Issue{issue(
			fmt.Sprintf("`name` 的值 `%s` 已被其他集市包占用（插件/主题/图标/模板/挂件之间也不能重名，且不区分大小写）。请更换一个未被占用的 `name` 后重新提交。", name),
			fmt.Sprintf("`name` value `%s` is already used by another bazaar package (must be unique across plugins/themes/icons/templates/widgets, case-insensitive). Choose an unused `name` before resubmitting.", name),
		)}
	}

	var issues []Issue
	// 单路径组件名称的字节长度上限，Linux ext4、macOS APFS 等常见文件系统均为 255，但为了保留冗余，这里取经验值 64
	if len(name) > 64 {
		issues = append(issues, issue(
			fmt.Sprintf("清单字段 `name` 的值 `%s` 超过长度上限（64 字节）。请缩短名称。", name),
			fmt.Sprintf("Manifest field `name` value `%s` exceeds the maximum length (64 bytes). Shorten it.", name),
		))
	}
	// 集市约定：不得以句点（.）开头，避免隐藏文件夹。
	if strings.HasPrefix(name, ".") {
		issues = append(issues, issue(
			fmt.Sprintf("清单字段 `name` 的值 `%s` 以句点开头。目录名不得以 `.` 开头（会变成隐藏文件夹）。请改成不以 `.` 开头的名称。", name),
			fmt.Sprintf("Manifest field `name` value `%s` starts with a period. Names must not start with `.` (hidden folder). Choose a name without a leading period.", name),
		))
	}
	// Windows：不得以空格开头（无法手动创建此类文件夹）或结尾（见 Microsoft 文档）。
	// REF https://learn.microsoft.com/zh-cn/windows/win32/fileio/naming-a-file#naming-conventions
	if strings.HasPrefix(name, " ") || strings.HasSuffix(name, " ") {
		issues = append(issues, issue(
			fmt.Sprintf("清单字段 `name` 的值 `%s` 以空格开头或结尾，不受 Windows 系统支持，请去掉首尾空格。", name),
			fmt.Sprintf("Manifest field `name` value `%s` has leading or trailing spaces, which Windows systems do not support. Remove those spaces.", name),
		))
	}
	// 仅允许可打印 ASCII（U+0020–U+007E），降低编码、终端与跨平台工具链上的兼容风险。
	// Windows：禁止保留字符、C0 控制字符（1–31）、NUL。
	// Linux / macOS：路径分隔符 / 与 NUL 与上述保留字符一并禁止。
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
		issues = append(issues, issue(
			fmt.Sprintf("清单字段 `name` 的值 `%s` 包含不可打印 ASCII 字符（如中文、表情符号）。请改用仅含可打印 ASCII 的名称。", name),
			fmt.Sprintf("Manifest field `name` value `%s` contains non-printable ASCII characters (e.g. CJK or emoji). Use printable ASCII only.", name),
		))
	}
	if hasReserved {
		issues = append(issues, issue(
			fmt.Sprintf("清单字段 `name` 的值 `%s` 包含路径保留字符（如 `<` `>` `:` `\"` `/` `\\` `|` `?` `*`）。请从名称中移除这些字符。", name),
			fmt.Sprintf("Manifest field `name` value `%s` contains reserved path characters (e.g. `<` `>` `:` `\"` `/` `\\` `|` `?` `*`). Remove them.", name),
		))
	}
	// Windows：不得以句点结尾（见 Microsoft 文档：Shell 与用户界面不支持此类名称）。
	if strings.HasSuffix(name, ".") {
		issues = append(issues, issue(
			fmt.Sprintf("清单字段 `name` 的值 `%s` 以句点结尾，不受 Windows 系统支持，请去掉末尾句点。", name),
			fmt.Sprintf("Manifest field `name` value `%s` ends with a period, which Windows systems do not support. Remove the trailing period.", name),
		))
	}
	// Windows：保留设备名（整段 name 与保留名完全一致时拒绝，不区分大小写）。
	// 带后缀的形式（如 CON.123）在资源管理器中可作为普通文件夹名，故不按「点号前 stem」截取比对。
	// 名称已限制为 ASCII，故不含上标数字变体。
	reservedWindowsDeviceNames := map[string]struct{}{
		"CON": {}, "PRN": {}, "AUX": {}, "NUL": {},
		"COM1": {}, "COM2": {}, "COM3": {}, "COM4": {}, "COM5": {}, "COM6": {}, "COM7": {}, "COM8": {}, "COM9": {},
		"LPT1": {}, "LPT2": {}, "LPT3": {}, "LPT4": {}, "LPT5": {}, "LPT6": {}, "LPT7": {}, "LPT8": {}, "LPT9": {},
	}
	if _, ok := reservedWindowsDeviceNames[strings.ToUpper(name)]; ok {
		issues = append(issues, issue(
			fmt.Sprintf("清单字段 `name` 的值 `%s` 是 Windows 保留设备名（如 `CON`、`PRN`、`AUX`）。请改用其他名称。", name),
			fmt.Sprintf("Manifest field `name` value `%s` is a Windows reserved device name (e.g. `CON`, `PRN`, `AUX`). Choose a different name.", name),
		))
	}
	// 用于 HTML 展示的清单字面量须与 html.EscapeString(s) 相同，避免 XSS。
	if html.EscapeString(name) != name {
		issues = append(issues, issue(
			fmt.Sprintf("清单字段 `name` 的值 `%s` 包含 HTML 特殊字符（`<`、`>`、`&`、`'`、`\"`）。请从名称中移除这些字符。", name),
			fmt.Sprintf("Manifest field `name` value `%s` contains HTML-special characters (`<`, `>`, `&`, `'`, `\"`). Remove them.", name),
		))
	}
	return issues
}

func checkAuthor(m map[string]any, githubOwner string) []Issue {
	raw, ok := m["author"]
	if !ok {
		return []Issue{issue(
			fmt.Sprintf("清单缺少必填字段 `author`。请填写作者名称字符串，例如 `\"author\": \"%s\"`。", githubOwner),
			fmt.Sprintf("Manifest is missing required field `author`. Set a string such as `\"author\": \"%s\"`.", githubOwner),
		)}
	}
	s, ok := raw.(string)
	if !ok {
		return []Issue{issue(
			"清单字段 `author` 必须是字符串。",
			"Manifest field `author` must be a string.",
		)}
	}
	if strings.TrimSpace(s) == "" {
		return []Issue{issue(
			fmt.Sprintf("清单字段 `author` 不能为空或仅含空白字符。请填写普通文本作者名，例如 `\"author\": \"%s\"`。", githubOwner),
			fmt.Sprintf("Manifest field `author` must not be empty or whitespace only. Set plain text such as `\"author\": \"%s\"`.", githubOwner),
		)}
	}
	return nil
}

// checkURL 要求 url 必须为 https://github.com/owner/repo（owner/repo 大小写不敏感）。
// 禁止末尾斜杠或 .git 结尾。
// 思源从 https://github.com/siyuan-note/siyuan/issues/7775 兼容了末尾斜杠，但为了保持一致性，统一禁止末尾斜杠。
func checkURL(m map[string]any, owner, repo string) []Issue {
	want := "https://github.com/" + owner + "/" + repo
	raw, ok := m["url"]
	if !ok {
		return []Issue{issue(
			fmt.Sprintf("清单缺少必填字段 `url`。请添加 `\"url\": \"%s\"`。", want),
			fmt.Sprintf("Manifest is missing required field `url`. Set it to `\"url\": \"%s\"`.", want),
		)}
	}
	u, ok := raw.(string)
	if !ok {
		return []Issue{issue(
			fmt.Sprintf("清单字段 `url` 必须是字符串。请写成 `\"url\": \"%s\"`。", want),
			fmt.Sprintf("Manifest field `url` must be a string, e.g. `\"url\": \"%s\"`.", want),
		)}
	}
	if !strings.EqualFold(u, want) {
		return []Issue{issue(
			fmt.Sprintf("清单字段 `url` 当前为 `%s`，请改成 `%s`。", u, want),
			fmt.Sprintf("Manifest `url` is `%s` but must be `%s`. Fix it.", u, want),
		)}
	}
	return nil
}

func checkVersion(m map[string]any, oldVersion string) []Issue {
	raw, ok := m["version"]
	if !ok {
		return []Issue{issue(
			"清单缺少必填字段 `version`。请填写语义化版本字符串，例如 `1.0.0`。",
			"Manifest is missing required field `version`. Set a semantic version string such as `1.0.0`.",
		)}
	}
	ver, ok := raw.(string)
	if !ok {
		return []Issue{issue(
			"清单字段 `version` 必须是字符串，例如 `1.0.0`。",
			"Manifest field `version` must be a string, e.g. `1.0.0`.",
		)}
	}
	if strings.TrimSpace(ver) != ver || ver == "" {
		return []Issue{issue(
			fmt.Sprintf("清单字段 `version` 的值 `%s` 无效：不能为空，也不能有前后空格。请改成干净的语义化版本，如 `1.0.0`。", ver),
			fmt.Sprintf("Manifest `version` `%s` is invalid: it must be non-empty and must not have leading/trailing spaces. Use a clean semver like `1.0.0`.", ver),
		)}
	}
	if ver[0] == 'v' || ver[0] == 'V' {
		return []Issue{issue(
			fmt.Sprintf("清单字段 `version` 的值 `%s` 不应带 `v`/`V` 前缀。请使用如 `1.0.0`、`1.0.0-beta.1` 等形式。", ver),
			fmt.Sprintf("Manifest `version` `%s` must not start with a `v`/`V` prefix. Use forms like `1.0.0` or `1.0.0-beta.1`.", ver),
		)}
	}
	canon := "v" + ver
	if !semver.IsValid(canon) {
		return []Issue{issue(
			fmt.Sprintf("清单字段 `version` 的值 `%s` 不是有效的语义化版本。请使用如 `1.0.0`、`1.0.0-beta.1` 等形式（不要带 `v` 前缀），参见 `https://semver.org/lang/zh-CN/`", ver),
			fmt.Sprintf("Manifest `version` `%s` is not valid semantic versioning. Use forms like `1.0.0` or `1.0.0-beta.1` without a `v` prefix. See `https://semver.org/`", ver),
		)}
	}
	if oldVersion == "" {
		return nil
	}
	oldCanon := "v" + oldVersion
	if !semver.IsValid(oldCanon) {
		// 旧版本号无法解析、新版本号合法：视为修复版本号的更新，放行
		return nil
	}
	if semver.Compare(canon, oldCanon) <= 0 {
		return []Issue{issue(
			fmt.Sprintf("本次清单 `version` 为 `%s`，但不高于集市已上架版本 `%s`。更新包时必须提高语义化版本。", ver, oldVersion),
			fmt.Sprintf("Manifest `version` is `%s`, which is not greater than the listed version `%s`. Bump the semver.", ver, oldVersion),
		)}
	}
	return nil
}

// checkReadme 校验清单 readme 字段：值为相对包根的说明文件路径，且文件须存在（路径大小写敏感）。
// 路径须为相对路径（不能以 / 开头）、用 / 分隔、不得包含 ..（防路径穿越）、不得使用反斜杠 \。
func checkReadme(m map[string]any, packageRoot string) []Issue {
	raw, ok := m["readme"]
	if !ok {
		return []Issue{issue(
			"清单缺少必填字段 `readme`。请用对象声明各语言说明文件，例如 `\"readme\": { \"zh_CN\": \"README_zh_CN.md\", \"default\": \"README.md\" }`，并确保这些文件都在 `package.zip` 包根（或相对包根的路径）中。",
			"Manifest is missing required field `readme`. Declare locale files as an object, e.g. `\"readme\": { \"zh_CN\": \"README_zh_CN.md\", \"default\": \"README.md\" }`, and include those files in `package.zip`.",
		)}
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return []Issue{issue(
			"清单字段 `readme` 必须是对象（键为语言，值为文件名），不能是字符串或数组。",
			"Manifest field `readme` must be an object (locale → filename), not a string or array.",
		)}
	}
	if len(obj) == 0 {
		return []Issue{issue(
			"清单字段 `readme` 是空对象。请至少声明一个语言对应的 README 文件名，并确保文件存在于 `package.zip`。",
			"Manifest field `readme` is an empty object. Declare at least one locale → README filename and include that file in `package.zip`.",
		)}
	}
	var issues []Issue
	for locale, v := range obj {
		pathVal, ok := v.(string)
		if !ok {
			issues = append(issues, issue(
				fmt.Sprintf("`readme.%s` 的值必须是字符串文件名，例如 `README.md`。", locale),
				fmt.Sprintf("`readme.%s` must be a string filename, e.g. `README.md`.", locale),
			))
			continue
		}
		pathVal = strings.TrimSpace(pathVal) // 跟思源内核逻辑一致，TrimSpace
		if pathVal == "" {
			issues = append(issues, issue(
				fmt.Sprintf("`readme.%s` 是空字符串。请填写相对包根的 README 路径，或删除该语言键。", locale),
				fmt.Sprintf("`readme.%s` is an empty string. Set a path relative to the package root, or remove this locale key.", locale),
			))
			continue
		}
		if strings.HasPrefix(pathVal, "/") || strings.Contains(pathVal, `\`) || strings.Contains(pathVal, "..") {
			issues = append(issues, issue(
				fmt.Sprintf("`readme.%s` 的路径 `%s` 不合法：请使用相对包根的路径，用 `/` 分隔，不要以 `/` 开头，不要包含 `..`，不要使用反斜杠 `\\`。", locale, pathVal),
				fmt.Sprintf("`readme.%s` path `%s` is invalid: use a path relative to the package root with `/`, no leading `/`, no `..`, and no backslashes.", locale, pathVal),
			))
			continue
		}
		if !relFileExistsCaseSensitive(packageRoot, pathVal) {
			issues = append(issues, issue(
				fmt.Sprintf("`readme.%s` 声明了文件 `%s`，但 `package.zip` 中找不到该文件（路径大小写必须一致）。请把文件打进包内，或修正 `readme` 中的文件名。", locale, pathVal),
				fmt.Sprintf("`readme.%s` declares `%s`, but that file is not in `package.zip` (paths are case-sensitive). Add the file or fix the filename.", locale, pathVal),
			))
		}
	}
	return issues
}

func relFileExistsCaseSensitive(root, rel string) bool {
	rel = filepath.FromSlash(rel)
	parts := strings.Split(rel, string(filepath.Separator))
	cur := root
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		if !fileExistsCaseSensitive(cur, part) {
			return false
		}
		cur = filepath.Join(cur, part)
	}
	info, err := os.Stat(cur)
	if err != nil || info.IsDir() {
		return false
	}
	return true
}

// checkFunding 校验 funding 字段。Custom 链接仅允许 http(s) / mailto，禁止 javascript: / data: / file: 等。
func checkFunding(m map[string]any) []Issue {
	raw, ok := m["funding"]
	if !ok || raw == nil {
		return nil
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return []Issue{issue(
			"清单字段 `funding` 必须是对象。若不需要赞助信息，请删除整个 `funding` 字段。",
			"Manifest field `funding` must be an object. If you do not need funding info, remove the `funding` field entirely.",
		)}
	}
	customRaw, ok := obj["custom"]
	if !ok || customRaw == nil {
		return nil
	}
	arr, ok := customRaw.([]any)
	if !ok {
		return []Issue{issue(
			"`funding.custom` 必须是字符串数组，例如 `\"custom\": [\"https://example.com/sponsor\"]`。",
			"`funding.custom` must be an array of strings, e.g. `\"custom\": [\"https://example.com/sponsor\"]`.",
		)}
	}
	var issues []Issue
	for i, item := range arr {
		s, ok := item.(string)
		if !ok {
			issues = append(issues, issue(
				fmt.Sprintf("`funding.custom[%d]` 必须是字符串链接。", i),
				fmt.Sprintf("`funding.custom[%d]` must be a string URL.", i),
			))
			continue
		}
		if s == "" {
			continue
		}
		if !strings.HasPrefix(s, "https://") && !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "mailto:") {
			issues = append(issues, issue(
				fmt.Sprintf("`funding.custom[%d]` 的值 `%s` 不安全或不受支持。请改成以 `https://`、`http://` 或 `mailto:` 开头的链接（禁止 `javascript:`、`data:` 等）。", i, s),
				fmt.Sprintf("`funding.custom[%d]` value `%s` is unsupported. Use a link starting with `https://`, `http://`, or `mailto:` (`javascript:`/`data:` etc. are not allowed).", i, s),
			))
		}
	}
	return issues
}

// checkOptionalTypedFields 校验可选字段类型。
// 若存在 disabledInPublish，则值必须为 bool（JSON 中该键存在时不得为字符串等其它类型）。
func checkOptionalTypedFields(m map[string]any) []Issue {
	var issues []Issue
	if raw, ok := m["disabledInPublish"]; ok {
		if _, isBool := raw.(bool); !isBool {
			issues = append(issues, issue(
				"若填写 `disabledInPublish`，值必须是布尔值 `true` 或 `false`（不要用字符串 `\"true\"`）。不需要时请删除该字段。",
				"If present, `disabledInPublish` must be a boolean `true` or `false` (not the string `\"true\"`). Remove the field if you do not need it.",
			))
		}
	}
	for _, key := range []string{"backends", "frontends", "kernels", "keywords", "modes"} {
		raw, ok := m[key]
		if !ok {
			continue
		}
		arr, ok := raw.([]any)
		if !ok {
			issues = append(issues, issue(
				fmt.Sprintf("清单字段 `%s` 若存在则必须是字符串数组，例如 `\"%s\": [\"all\"]`。", key, key),
				fmt.Sprintf("Manifest field `%s`, if present, must be an array of strings, e.g. `\"%s\": [\"all\"]`.", key, key),
			))
			continue
		}
		for i, item := range arr {
			if _, ok := item.(string); !ok {
				issues = append(issues, issue(
					fmt.Sprintf("`%s[%d]` 必须是字符串。请检查数组元素类型。", key, i),
					fmt.Sprintf("`%s[%d]` must be a string. Check the array element types.", key, i),
				))
			}
		}
	}
	return issues
}
