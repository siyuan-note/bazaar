// SiYuan community bazaar.
// Copyright (c) 2021-present, b3log.org
//
// Bazaar is licensed under Mulan PSL v2.
// You can use this software according to the terms and conditions of the Mulan PSL v2.
// You may obtain a copy of Mulan PSL v2 at:
//         http://license.coscl.org.cn/MulanPSL2
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR PURPOSE.
// See the Mulan PSL v2 for more details.

package check

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
type Package struct {
	Name              string        `json:"name"`
	Author            string        `json:"author"`
	URL               string        `json:"url"`
	Version           string        `json:"version"`
	MinAppVersion     string        `json:"minAppVersion"`
	DisplayName       LocaleStrings `json:"displayName"`
	Description       LocaleStrings `json:"description"`
	Readme            LocaleStrings `json:"readme"`
	Funding           *Funding      `json:"funding"`
	Keywords          []string      `json:"keywords"`
	Backends          []string      `json:"backends,omitempty"`
	Frontends         []string      `json:"frontends,omitempty"`
	DisabledInPublish bool          `json:"disabledInPublish,omitempty"`
	Modes             []string      `json:"modes,omitempty"`
}

// packageFromMap 将已解析的清单 map 转为 Package；解析失败时返回零值。
func packageFromMap(m map[string]any) Package {
	var pkg Package
	if m == nil {
		return pkg
	}
	data, err := json.Marshal(m)
	if err != nil {
		return pkg
	}
	_ = json.Unmarshal(data, &pkg)
	return pkg
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
	OccupiedNames map[string]struct{} // 键小写；nil 表示不查唯一性
}

var allowedManifestKeys = map[PackageType]map[string]struct{}{
	TypePlugin:   toKeySet(commonManifestKeys, pluginExtraKeys...),
	TypeTheme:    toKeySet(commonManifestKeys, themeExtraKeys...),
	TypeIcon:     toKeySet(commonManifestKeys, iconExtraKeys...),
	TypeTemplate: toKeySet(commonManifestKeys, templateExtraKeys...),
	TypeWidget:   toKeySet(commonManifestKeys, widgetExtraKeys...),
}

var commonManifestKeys = []string{
	"name", "author", "url", "version",
	"displayName", "description", "readme",
	"funding", "keywords",
	"minAppVersion",
}

var pluginExtraKeys = []string{
	"backends", "frontends", "disabledInPublish",
}

var themeExtraKeys = []string{
	"modes",
}

var iconExtraKeys = []string{}

var templateExtraKeys = []string{}

var widgetExtraKeys = []string{
	"backends", "frontends",
}

// 内置主题名（不得被集市主题占用）。
var builtinThemeNames = map[string]struct{}{
	"daylight": {},
	"midnight": {},
}

// 内置图标包名（不得被集市图标占用）。
var builtinIconNames = map[string]struct{}{
	"ant":       {}, // 已废弃，内核自动清理图标包
	"material":  {}, // 已废弃，内核自动清理图标包
	"litheness": {},
}

func toKeySet(base []string, extra ...string) map[string]struct{} {
	m := make(map[string]struct{}, len(base)+len(extra))
	for _, k := range base {
		m[k] = struct{}{}
	}
	for _, k := range extra {
		m[k] = struct{}{}
	}
	return m
}

// ReadManifest 读取并解析清单 JSON。
func ReadManifest(path string) (map[string]any, error) {
	base := filepath.Base(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, LocalizedErr(
			fmt.Sprintf("无法读取清单文件 %s：%v。请确认该文件已打进 package.zip 包根、路径大小写完全一致，然后重新打包并更新 Release。", base, err),
			fmt.Sprintf("Cannot read manifest %s: %v. Ensure the file is at the package root with an exact case-sensitive path, then rebuild and update the Release.", base, err),
			err,
		)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, LocalizedErr(
			fmt.Sprintf("清单 %s 的 JSON 解析失败：%v。请检查 JSON 语法，修正后重新打包并更新 GitHub Release 中的 package.zip。", base, err),
			fmt.Sprintf("Failed to parse manifest %s as JSON: %v. Fix JSON syntax errors, rebuild the package, and update package.zip on the GitHub Release.", base, err),
			err,
		)
	}
	if m == nil {
		return nil, LocalizedErr(
			fmt.Sprintf("清单 %s 的 JSON 内容为 null，必须是一个对象（例如以 { 开头的键值对）。请修正清单后重新打包并更新 Release。", base),
			fmt.Sprintf("Manifest %s JSON is null; it must be a JSON object (e.g. key-value pairs starting with {). Fix the manifest, rebuild the package, and update the Release.", base),
			nil,
		)
	}
	return m, nil
}

// ReadPackage 读取清单 JSON 并解析为 Package。
func ReadPackage(path string) (*Package, error) {
	m, err := ReadManifest(path)
	if err != nil {
		return nil, err
	}
	pkg := packageFromMap(m)
	return &pkg, nil
}

// Manifest 校验清单字段。
func Manifest(m map[string]any, in ManifestInput) []Issue {
	var issues []Issue
	issues = append(issues, checkUnknownKeys(m, in.Type)...)
	issues = append(issues, checkName(m, in)...)
	issues = append(issues, checkURL(m, in.Owner, in.Repo)...)
	issues = append(issues, checkVersion(m, in.OldVersion)...)
	issues = append(issues, checkAuthor(m)...)
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
				fmt.Sprintf("%s 中出现了集市规范未收录的字段 %q。请删除该字段后重新打包（保留未知字段会妨碍官方日后扩展同名字段）。若确有自定义需求，请先在集市仓库提 issue 讨论。", typ.ManifestFile(), k),
				fmt.Sprintf("%s contains unsupported field %q. Remove it and rebuild the package (unknown fields block future official schema additions). If you need a new official field, open an issue in the bazaar repository first.", typ.ManifestFile(), k),
			))
		}
	}
	return issues
}

func checkName(m map[string]any, in ManifestInput) []Issue {
	raw, ok := m["name"]
	if !ok {
		return []Issue{issue(
			fmt.Sprintf("清单 %s 缺少必填字段 name。请在 JSON 根级添加字符串字段 name，且其值必须与 GitHub 仓库名完全一致。", in.Type.ManifestFile()),
			fmt.Sprintf("Manifest %s is missing required field name. Add a string field name at the JSON root; it must exactly match the GitHub repository name.", in.Type.ManifestFile()),
		)}
	}
	name, ok := raw.(string)
	if !ok {
		return []Issue{issue(
			fmt.Sprintf("清单字段 name 的类型必须是字符串，当前不是。请改成例如 \"name\": %q 这种写法。", in.Repo),
			fmt.Sprintf("Manifest field name must be a string. Use a value like \"name\": %q.", in.Repo),
		)}
	}

	if in.OldName != "" {
		if name != in.OldName {
			return []Issue{issue(
				fmt.Sprintf("已上架集市包的 name 不可更改。清单里当前是 %q，集市已记录为 %q。请改回 %q 后重新发布；若要换名，需按「更换维护者 / 新包」流程另提 PR。", name, in.OldName, in.OldName),
				fmt.Sprintf("The listed package name must not change. Manifest has %q but the bazaar already lists %q. Set name back to %q and republish; to rename, follow the maintainer-transfer / new-package PR process.", name, in.OldName, in.OldName),
			)}
		}
		return nil
	}

	var issues []Issue
	for _, err := range validatePackageName(name) {
		issues = append(issues, IssueFromErr(err))
	}
	if name != in.Repo {
		issues = append(issues, issue(
			fmt.Sprintf("清单字段 name 为 %q，但 GitHub 仓库名是 %q。二者必须完全一致。请把 name 改成 %q（或把仓库改名为当前 name），然后重新打包并更新 Release。", name, in.Repo, in.Repo),
			fmt.Sprintf("Manifest name is %q but the GitHub repository name is %q. They must match exactly. Set name to %q (or rename the repository), then rebuild and update the Release.", name, in.Repo, in.Repo),
		))
	}
	if in.Type == TypeTheme {
		if _, hit := builtinThemeNames[name]; hit {
			issues = append(issues, issue(
				fmt.Sprintf("name %q 与思源内置主题重名，不能上架。请更换仓库名与清单 name（例如加上作者前缀），并同步修改 package.zip。", name),
				fmt.Sprintf("name %q conflicts with a built-in SiYuan theme and cannot be listed. Rename the repository and manifest name (e.g. add an author prefix), then update package.zip.", name),
			))
		}
	}
	if in.Type == TypeIcon {
		if _, hit := builtinIconNames[name]; hit {
			issues = append(issues, issue(
				fmt.Sprintf("name %q 与思源内置图标包重名，不能上架。请更换仓库名与清单 name，并同步修改 package.zip。", name),
				fmt.Sprintf("name %q conflicts with a built-in SiYuan icon pack and cannot be listed. Rename the repository and manifest name, then update package.zip.", name),
			))
		}
	}
	if len(in.OccupiedNames) > 0 {
		for occupied := range in.OccupiedNames {
			if strings.EqualFold(occupied, name) {
				issues = append(issues, issue(
					fmt.Sprintf("name %q 已被其他集市包占用（插件/主题/图标/模板/挂件之间也不能重名，且不区分大小写）。请更换一个未被占用的仓库名，并把清单 name、url 一并改成新名后重新提交。", name),
					fmt.Sprintf("name %q is already used by another bazaar package (must be unique across plugins/themes/icons/templates/widgets, case-insensitive). Choose an unused repository name and update manifest name and url accordingly before resubmitting.", name),
				))
				break
			}
		}
	}
	return issues
}

// checkURL 要求 url 必须为 https://github.com/owner/repo（owner/repo 大小写不敏感）。
// 禁止末尾斜杠或 .git 结尾。
// https://github.com/siyuan-note/siyuan/issues/7775 兼容了末尾斜杠，但思源内核未必完全兼容，故统一禁止末尾斜杠以避免产生问题。
func checkURL(m map[string]any, owner, repo string) []Issue {
	raw, ok := m["url"]
	if !ok {
		return []Issue{issue(
			"清单缺少必填字段 url。请添加指向本仓库的地址，格式必须是 https://github.com/<owner>/<repo>（不要末尾斜杠，不要 .git）。",
			"Manifest is missing required field url. Set it to https://github.com/<owner>/<repo> for this repository (no trailing slash, no .git).",
		)}
	}
	u, ok := raw.(string)
	if !ok {
		return []Issue{issue(
			"清单字段 url 必须是字符串。请写成 \"url\": \"https://github.com/owner/repo\"。",
			"Manifest field url must be a string, e.g. \"url\": \"https://github.com/owner/repo\".",
		)}
	}
	want := "https://github.com/" + owner + "/" + repo
	if !strings.EqualFold(u, want) {
		return []Issue{issue(
			fmt.Sprintf("清单字段 url 当前为 %q，正确值应为 %s。请改成该地址（不要加 .git，不要末尾 /；owner/repo 大小写可与 GitHub 显示略有不同）。改完后重新打包 package.zip。", u, want),
			fmt.Sprintf("Manifest url is %q but must be exactly %s (no .git, no trailing slash; owner/repo matching is case-insensitive). Fix it and rebuild package.zip.", u, want),
		)}
	}
	return nil
}

func checkVersion(m map[string]any, oldVersion string) []Issue {
	raw, ok := m["version"]
	if !ok {
		return []Issue{issue(
			"清单缺少必填字段 version。请填写语义化版本字符串，例如 \"1.0.0\"，并在每次更新 package.zip 时提高版本号。",
			"Manifest is missing required field version. Set a semantic version string such as \"1.0.0\", and bump it whenever you publish a new package.zip.",
		)}
	}
	ver, ok := raw.(string)
	if !ok {
		return []Issue{issue(
			"清单字段 version 必须是字符串，例如 \"1.2.3\"。",
			"Manifest field version must be a string, e.g. \"1.2.3\".",
		)}
	}
	if strings.TrimSpace(ver) != ver || ver == "" {
		return []Issue{issue(
			fmt.Sprintf("清单字段 version 的值 %q 无效：不能为空，也不能有前后空格。请改成干净的语义化版本，如 1.0.0。", ver),
			fmt.Sprintf("Manifest version %q is invalid: it must be non-empty and must not have leading/trailing spaces. Use a clean semver like 1.0.0.", ver),
		)}
	}
	canon := canonicalSemver(ver)
	if !semver.IsValid(canon) {
		return []Issue{issue(
			fmt.Sprintf("清单字段 version 的值 %q 不是有效的语义化版本。请使用如 1.0.0、1.2.3-beta.1 等形式（不要带 v 前缀），参见 https://semver.org/lang/zh-CN/", ver),
			fmt.Sprintf("Manifest version %q is not valid semantic versioning. Use forms like 1.0.0 or 1.2.3-beta.1 without a v prefix. See https://semver.org/", ver),
		)}
	}
	if oldVersion == "" {
		return nil
	}
	oldCanon := canonicalSemver(oldVersion)
	if !semver.IsValid(oldCanon) {
		return []Issue{issue(
			fmt.Sprintf("集市已记录的旧 version %q 无法解析，自动化无法比较升降。请联系集市维护者处理后再发版。", oldVersion),
			fmt.Sprintf("The previously listed version %q cannot be parsed, so the checker cannot compare versions. Contact a bazaar maintainer before publishing.", oldVersion),
		)}
	}
	if semver.Compare(canon, oldCanon) <= 0 {
		return []Issue{issue(
			fmt.Sprintf("本次清单 version 为 %q，但不高于集市已上架版本 %q。更新包时必须提高语义化版本（例如 %s → 更高版本），然后重新打包并发布 Release。", ver, oldVersion, oldVersion),
			fmt.Sprintf("Manifest version is %q, which is not greater than the listed version %q. Bump the semver above %s, rebuild package.zip, and publish a new Release.", ver, oldVersion, oldVersion),
		)}
	}
	return nil
}

func canonicalSemver(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "v"
	}
	if v[0] == 'v' || v[0] == 'V' {
		return "v" + v[1:]
	}
	return "v" + v
}

func checkAuthor(m map[string]any) []Issue {
	raw, ok := m["author"]
	if !ok {
		return []Issue{issue(
			"清单缺少必填字段 author。请填写作者名称字符串，例如 \"author\": \"your-name\"。",
			"Manifest is missing required field author. Set a string such as \"author\": \"your-name\".",
		)}
	}
	s, ok := raw.(string)
	if !ok {
		return []Issue{issue(
			"清单字段 author 必须是字符串。",
			"Manifest field author must be a string.",
		)}
	}
	var issues []Issue
	for _, err := range validateManifestAuthor(s) {
		issues = append(issues, IssueFromErr(err))
	}
	return issues
}

// checkReadme 校验清单 readme 字段：值为相对包根的说明文件路径，且文件须存在（路径大小写敏感）。
// 路径须为相对路径（不能以 / 开头）、用 / 分隔、不得包含 ..（防路径穿越）、不得使用反斜杠 \。
func checkReadme(m map[string]any, packageRoot string) []Issue {
	raw, ok := m["readme"]
	if !ok {
		return []Issue{issue(
			"清单缺少必填字段 readme。请用对象声明各语言说明文件，例如 \"readme\": { \"zh_CN\": \"README_zh_CN.md\", \"default\": \"README.md\" }，并确保这些文件都在 package.zip 包根（或相对包根的路径）中。",
			"Manifest is missing required field readme. Declare locale files as an object, e.g. \"readme\": { \"zh_CN\": \"README_zh_CN.md\", \"default\": \"README.md\" }, and include those files in package.zip.",
		)}
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return []Issue{issue(
			"清单字段 readme 必须是对象（键为语言，值为文件名），不能是字符串或数组。",
			"Manifest field readme must be an object (locale → filename), not a string or array.",
		)}
	}
	if len(obj) == 0 {
		return []Issue{issue(
			"清单字段 readme 是空对象。请至少声明一个语言对应的 README 文件名，并确保文件存在于 package.zip。",
			"Manifest field readme is an empty object. Declare at least one locale → README filename and include that file in package.zip.",
		)}
	}
	var issues []Issue
	for locale, v := range obj {
		pathVal, ok := v.(string)
		if !ok {
			issues = append(issues, issue(
				fmt.Sprintf("readme.%s 的值必须是字符串文件名，例如 \"README.md\"。", locale),
				fmt.Sprintf("readme.%s must be a string filename, e.g. \"README.md\".", locale),
			))
			continue
		}
		pathVal = strings.TrimSpace(pathVal) // 跟思源内核逻辑一致，TrimSpace
		if pathVal == "" {
			issues = append(issues, issue(
				fmt.Sprintf("readme.%s 是空字符串。请填写相对包根的 README 路径，或删除该语言键。", locale),
				fmt.Sprintf("readme.%s is an empty string. Set a path relative to the package root, or remove this locale key.", locale),
			))
			continue
		}
		if strings.HasPrefix(pathVal, "/") || strings.Contains(pathVal, `\`) || strings.Contains(pathVal, "..") {
			issues = append(issues, issue(
				fmt.Sprintf("readme.%s 的路径 %q 不合法：请使用相对包根的路径，用 / 分隔，不要以 / 开头，不要包含 ..，不要使用反斜杠 \\。", locale, pathVal),
				fmt.Sprintf("readme.%s path %q is invalid: use a path relative to the package root with /, no leading /, no .., and no backslashes.", locale, pathVal),
			))
			continue
		}
		if !relFileExistsCaseSensitive(packageRoot, pathVal) {
			issues = append(issues, issue(
				fmt.Sprintf("readme.%s 声明了文件 %q，但 package.zip 中找不到该文件（路径大小写必须一致）。请把文件打进包内，或修正 readme 中的文件名后重新发布。", locale, pathVal),
				fmt.Sprintf("readme.%s declares %q, but that file is not in package.zip (paths are case-sensitive). Add the file or fix the filename, then republish.", locale, pathVal),
			))
		}
	}
	return issues
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
			"清单字段 funding 必须是对象。若不需要赞助信息，请删除整个 funding 字段。",
			"Manifest field funding must be an object. If you do not need funding info, remove the funding field entirely.",
		)}
	}
	customRaw, ok := obj["custom"]
	if !ok || customRaw == nil {
		return nil
	}
	arr, ok := customRaw.([]any)
	if !ok {
		return []Issue{issue(
			"funding.custom 必须是字符串数组，例如 \"custom\": [\"https://example.com/sponsor\"]。",
			"funding.custom must be an array of strings, e.g. \"custom\": [\"https://example.com/sponsor\"].",
		)}
	}
	var issues []Issue
	for i, item := range arr {
		s, ok := item.(string)
		if !ok {
			issues = append(issues, issue(
				fmt.Sprintf("funding.custom[%d] 必须是字符串链接。", i),
				fmt.Sprintf("funding.custom[%d] must be a string URL.", i),
			))
			continue
		}
		if s == "" {
			continue
		}
		if !strings.HasPrefix(s, "https://") && !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "mailto:") {
			issues = append(issues, issue(
				fmt.Sprintf("funding.custom[%d] 的值 %q 不安全或不受支持。请改成以 https://、http:// 或 mailto: 开头的链接（禁止 javascript:、data: 等）。", i, s),
				fmt.Sprintf("funding.custom[%d] value %q is unsupported. Use a link starting with https://, http://, or mailto: (javascript:/data: etc. are not allowed).", i, s),
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
				"若填写 disabledInPublish，值必须是布尔值 true 或 false（不要用字符串 \"true\"）。不需要时请删除该字段。",
				"If present, disabledInPublish must be a boolean true or false (not the string \"true\"). Remove the field if you do not need it.",
			))
		}
	}
	for _, key := range []string{"backends", "frontends", "keywords", "modes"} {
		raw, ok := m[key]
		if !ok {
			continue
		}
		arr, ok := raw.([]any)
		if !ok {
			issues = append(issues, issue(
				fmt.Sprintf("清单字段 %s 若存在则必须是字符串数组，例如 \"%s\": [\"all\"]。", key, key),
				fmt.Sprintf("Manifest field %s, if present, must be an array of strings, e.g. \"%s\": [\"all\"].", key, key),
			))
			continue
		}
		for i, item := range arr {
			if _, ok := item.(string); !ok {
				issues = append(issues, issue(
					fmt.Sprintf("%s[%d] 必须是字符串。请检查数组元素类型。", key, i),
					fmt.Sprintf("%s[%d] must be a string. Check the array element types.", key, i),
				))
			}
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

// SanitizeDisplayStrings 对清单中 displayName / description 的字符串值做 HTML 转义。
// 与思源内核 kernel/bazaar/package.go 的展示字段转义对齐；思源旧版本未转义，为避免旧客户端 XSS，须在线上转义后再写入索引。
// 注：Stage 写入 stage 条目时仍会对 name/author 等做更广的转义（见 SanitizePackage）。
func SanitizeDisplayStrings(m map[string]any) {
	if m == nil {
		return
	}
	for _, key := range []string{"displayName", "description"} {
		raw, ok := m[key]
		if !ok {
			continue
		}
		obj, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		for k, v := range obj {
			if s, ok := v.(string); ok {
				obj[k] = html.EscapeString(s)
			}
		}
	}
}
