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
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/siyuan-note/bazaar/rules"
)

const (
	// DeprecatedRegistryRelPath 是集市弃用注册表在仓库根目录下的相对路径。
	DeprecatedRegistryRelPath = "deprecated.json"
	deprecatedRegistryMaxSize = 1 << 20
	maxDeprecatedReasonRunes  = 1000
	maxDeprecatedAlternatives = 20
)

// DeprecatedEntry 描述一个仍上架、但不再建议新用户选用的集市包。
type DeprecatedEntry struct {
	Reason       rules.LocaleStrings `json:"reason,omitempty"`
	Alternatives []string            `json:"alternatives,omitempty"`
}

// DeprecatedRegistry 按集市包类型保存弃用元数据。
type DeprecatedRegistry struct {
	Plugins   map[string]DeprecatedEntry `json:"plugins"`
	Themes    map[string]DeprecatedEntry `json:"themes"`
	Icons     map[string]DeprecatedEntry `json:"icons"`
	Templates map[string]DeprecatedEntry `json:"templates"`
	Widgets   map[string]DeprecatedEntry `json:"widgets"`
}

// Entries 返回指定包类型的弃用条目。
func (r *DeprecatedRegistry) Entries(packageType rules.PackageType) map[string]DeprecatedEntry {
	if r == nil {
		return nil
	}
	switch packageType {
	case rules.TypePlugin:
		return r.Plugins
	case rules.TypeTheme:
		return r.Themes
	case rules.TypeIcon:
		return r.Icons
	case rules.TypeTemplate:
		return r.Templates
	case rules.TypeWidget:
		return r.Widgets
	default:
		return nil
	}
}

// ReadDeprecatedRegistry 读取并严格解析弃用注册表。
func ReadDeprecatedRegistry(filePath string) (*DeprecatedRegistry, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, rules.LocalizedErr(
			fmt.Sprintf("无法读取 `%s`：%v。请确认弃用注册表存在于仓库根目录。", filepath.Base(filePath), err),
			fmt.Sprintf("Couldn't read `%s`: %v. Please make sure the deprecation registry exists at the repository root.", filepath.Base(filePath), err),
			err,
		)
	}
	return ParseDeprecatedRegistry(filepath.Base(filePath), data)
}

// ParseDeprecatedRegistry 严格解析弃用注册表，拒绝重复键、未知字段、缺失类型与显式 null。
func ParseDeprecatedRegistry(fileLabel string, data []byte) (*DeprecatedRegistry, error) {
	if len(data) > deprecatedRegistryMaxSize {
		return nil, deprecatedRegistryError(fileLabel, "文件超过 1 MiB 上限", "the file exceeds the 1 MiB limit", nil)
	}
	if !utf8.Valid(data) {
		return nil, deprecatedRegistryError(fileLabel, "文件不是有效的 UTF-8", "the file isn't valid UTF-8", nil)
	}
	if err := scanDeprecatedJSON(data); err != nil {
		return nil, deprecatedRegistryError(fileLabel, fmt.Sprintf("JSON 结构无效：%v", err), fmt.Sprintf("the JSON structure is invalid: %v", err), err)
	}
	trimmed := bytes.TrimSpace(data)
	if bytes.Equal(trimmed, []byte("null")) {
		return nil, deprecatedRegistryError(fileLabel, "根值不能为 `null`", "the root value can't be `null`", nil)
	}

	var registry DeprecatedRegistry
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&registry); err != nil {
		return nil, deprecatedRegistryError(fileLabel, fmt.Sprintf("JSON 解析失败：%v", err), fmt.Sprintf("the JSON couldn't be parsed: %v", err), err)
	}

	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil || root == nil {
		return nil, deprecatedRegistryError(fileLabel, "根值必须是对象", "the root value must be an object", err)
	}
	for _, packageType := range rules.AllPackageTypes() {
		key := packageType.Plural()
		raw, ok := root[key]
		if !ok {
			return nil, deprecatedRegistryError(fileLabel, fmt.Sprintf("缺少必填类型字段 `%s`", key), fmt.Sprintf("required type field `%s` is missing", key), nil)
		}
		if isJSONNull(raw) || registry.Entries(packageType) == nil {
			return nil, deprecatedRegistryError(fileLabel, fmt.Sprintf("类型字段 `%s` 必须是对象，不能为 `null`", key), fmt.Sprintf("type field `%s` must be an object, not `null`", key), nil)
		}
		var entries map[string]json.RawMessage
		if err := json.Unmarshal(raw, &entries); err != nil {
			return nil, deprecatedRegistryError(fileLabel, fmt.Sprintf("类型字段 `%s` 必须是对象", key), fmt.Sprintf("type field `%s` must be an object", key), err)
		}
		for ownerRepo, entryRaw := range entries {
			if isJSONNull(entryRaw) {
				return nil, deprecatedRegistryError(fileLabel, fmt.Sprintf("条目 `%s.%s` 必须是对象，不能为 `null`", key, ownerRepo), fmt.Sprintf("entry `%s.%s` must be an object, not `null`", key, ownerRepo), nil)
			}
			var entryObject map[string]json.RawMessage
			if err := json.Unmarshal(entryRaw, &entryObject); err != nil || entryObject == nil {
				return nil, deprecatedRegistryError(fileLabel, fmt.Sprintf("条目 `%s.%s` 必须是对象", key, ownerRepo), fmt.Sprintf("entry `%s.%s` must be an object", key, ownerRepo), err)
			}
			for _, field := range []string{"reason", "alternatives"} {
				if fieldRaw, exists := entryObject[field]; exists && isJSONNull(fieldRaw) {
					return nil, deprecatedRegistryError(fileLabel, fmt.Sprintf("字段 `%s.%s.%s` 不能为 `null`，请删除该字段或填写有效值", key, ownerRepo, field), fmt.Sprintf("field `%s.%s.%s` can't be `null`; remove it or provide a valid value", key, ownerRepo, field), nil)
				}
			}
		}
	}
	return &registry, nil
}

func deprecatedRegistryError(fileLabel, zhDetail, enDetail string, cause error) error {
	return rules.LocalizedErr(
		fmt.Sprintf("弃用注册表 `%s` 无效：%s。", fileLabel, zhDetail),
		fmt.Sprintf("Deprecation registry `%s` is invalid: %s.", fileLabel, enDetail),
		cause,
	)
}

func isJSONNull(data []byte) bool {
	return bytes.Equal(bytes.TrimSpace(data), []byte("null"))
}

// scanDeprecatedJSON 校验 JSON 语法并拒绝任意层级的重复对象键。
func scanDeprecatedJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := scanDeprecatedJSONValue(decoder, "$", 0); err != nil {
		return err
	}
	if token, err := decoder.Token(); err != io.EOF {
		if err != nil {
			return err
		}
		return fmt.Errorf("根值后还有额外内容 `%v`", token)
	}
	return nil
}

func scanDeprecatedJSONValue(decoder *json.Decoder, path string, depth int) error {
	if depth > 32 {
		return fmt.Errorf("嵌套层级超过 32 层")
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]struct{}{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("对象 `%s` 包含非字符串键", path)
			}
			if _, duplicate := seen[key]; duplicate {
				return fmt.Errorf("对象 `%s` 包含重复键 `%s`", path, key)
			}
			seen[key] = struct{}{}
			if err := scanDeprecatedJSONValue(decoder, path+"."+key, depth+1); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil {
			return err
		}
		if closing != json.Delim('}') {
			return fmt.Errorf("对象 `%s` 未正确结束", path)
		}
	case '[':
		index := 0
		for decoder.More() {
			if err := scanDeprecatedJSONValue(decoder, fmt.Sprintf("%s[%d]", path, index), depth+1); err != nil {
				return err
			}
			index++
		}
		closing, err := decoder.Token()
		if err != nil {
			return err
		}
		if closing != json.Delim(']') {
			return fmt.Errorf("数组 `%s` 未正确结束", path)
		}
	default:
		return fmt.Errorf("路径 `%s` 出现意外分隔符 `%c`", path, delim)
	}
	return nil
}

// LoadReposByPackageType 读取仓库根目录下的五个集市包列表。
func LoadReposByPackageType(repoRoot string) (map[rules.PackageType][]string, error) {
	reposByType := make(map[rules.PackageType][]string, len(rules.AllPackageTypes()))
	for _, packageType := range rules.AllPackageTypes() {
		listPath := filepath.Join(repoRoot, packageType.ReposListFile())
		repos, err := ParseReposFromTxt(listPath)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", packageType.ReposListFile(), err)
		}
		reposByType[packageType] = repos
	}
	return reposByType, nil
}

// ValidateDeprecatedRegistry 校验源包、替代包与多语言原因；源包须在同类列表中，替代包须恰属于一个类型列表。
func ValidateDeprecatedRegistry(registry *DeprecatedRegistry, reposByType map[rules.PackageType][]string) []rules.Issue {
	return ValidateDeprecatedRegistryTypes(registry, reposByType, rules.AllPackageTypes())
}

// ValidateDeprecatedRegistryTypes 只校验指定包类型，供 PR Check 避免被其它类型的既有防御性警告阻断。
func ValidateDeprecatedRegistryTypes(registry *DeprecatedRegistry, reposByType map[rules.PackageType][]string, packageTypes []rules.PackageType) []rules.Issue {
	if registry == nil {
		return []rules.Issue{deprecatedIssue("弃用注册表为空。", "The deprecation registry is empty.")}
	}
	// 替代包允许跨类型，故按全部五个列表建立 owner/repo → 所属类型 的反查表；
	// 与本次只校验的类型无关，必须取全量，否则跨类型替代包会被误判为不存在。
	typesByOwnerRepo := make(map[string][]rules.PackageType, len(reposByType))
	for _, packageType := range rules.AllPackageTypes() {
		for _, ownerRepo := range reposByType[packageType] {
			typesByOwnerRepo[ownerRepo] = append(typesByOwnerRepo[ownerRepo], packageType)
		}
	}
	var issues []rules.Issue
	for _, packageType := range packageTypes {
		listed := make(map[string]struct{}, len(reposByType[packageType]))
		for _, ownerRepo := range reposByType[packageType] {
			listed[ownerRepo] = struct{}{}
		}
		entries := registry.Entries(packageType)
		sources := make([]string, 0, len(entries))
		for source := range entries {
			sources = append(sources, source)
		}
		slices.Sort(sources)
		for _, source := range sources {
			entry := entries[source]
			path := packageType.Plural() + "." + source
			if !validOwnerRepo(source) {
				issues = append(issues, deprecatedIssue(
					fmt.Sprintf("弃用条目 `%s` 的键必须严格使用 `owner/repo` 格式，且不能包含空白字符。", path),
					fmt.Sprintf("Deprecation entry `%s` must use the exact `owner/repo` format without whitespace.", path),
				))
			}
			if _, ok := listed[source]; !ok {
				issues = append(issues, deprecatedIssue(
					fmt.Sprintf("弃用条目 `%s` 对应的源包不在 `%s` 中；请先保持包上架，或删除这条过期元数据。", path, packageType.ReposListFile()),
					fmt.Sprintf("The source package for deprecation entry `%s` isn't listed in `%s`; keep it listed or remove this stale metadata.", path, packageType.ReposListFile()),
				))
			}
			issues = append(issues, validateDeprecatedReason(path, entry.Reason)...)
			issues = append(issues, validateDeprecatedAlternatives(path, source, entry.Alternatives, typesByOwnerRepo)...)
		}
	}
	return issues
}

func validateDeprecatedReason(path string, reason rules.LocaleStrings) []rules.Issue {
	if len(reason) == 0 {
		return nil
	}
	var issues []rules.Issue
	if _, ok := reason["default"]; !ok {
		issues = append(issues, deprecatedIssue(
			fmt.Sprintf("弃用条目 `%s.reason` 包含本地化原因时必须提供 `default`。", path),
			fmt.Sprintf("Deprecation entry `%s.reason` must provide `default` when localized reasons are present.", path),
		))
	}
	locales := make([]string, 0, len(reason))
	for locale := range reason {
		locales = append(locales, locale)
	}
	slices.Sort(locales)
	for _, locale := range locales {
		value := reason[locale]
		if !validDeprecatedLocale(locale) {
			issues = append(issues, deprecatedIssue(
				fmt.Sprintf("弃用条目 `%s.reason` 包含无效语言键 `%s`；语言键不能为空、含空白或超过 64 字节。", path, locale),
				fmt.Sprintf("Deprecation entry `%s.reason` has an invalid locale key `%s`; locale keys can't be empty, contain whitespace, or exceed 64 bytes.", path, locale),
			))
		}
		if strings.TrimSpace(value) == "" {
			issues = append(issues, deprecatedIssue(
				fmt.Sprintf("弃用条目 `%s.reason.%s` 不能为空或仅含空白字符。", path, locale),
				fmt.Sprintf("Deprecation entry `%s.reason.%s` can't be empty or whitespace-only.", path, locale),
			))
		} else if value != strings.TrimSpace(value) || strings.IndexFunc(value, unicode.IsControl) >= 0 {
			issues = append(issues, deprecatedIssue(
				fmt.Sprintf("弃用条目 `%s.reason.%s` 不能包含首尾空白或控制字符。", path, locale),
				fmt.Sprintf("Deprecation entry `%s.reason.%s` can't contain leading or trailing whitespace or control characters.", path, locale),
			))
		} else if utf8.RuneCountInString(value) > maxDeprecatedReasonRunes {
			issues = append(issues, deprecatedIssue(
				fmt.Sprintf("弃用条目 `%s.reason.%s` 超过 %d 个字符上限。", path, locale, maxDeprecatedReasonRunes),
				fmt.Sprintf("Deprecation entry `%s.reason.%s` exceeds the %d-character limit.", path, locale, maxDeprecatedReasonRunes),
			))
		}
	}
	return issues
}

func validDeprecatedLocale(locale string) bool {
	return locale != "" && locale == strings.TrimSpace(locale) && strings.IndexFunc(locale, unicode.IsSpace) < 0 && len(locale) <= 64
}

func validDeprecatedReasonValue(value string) bool {
	return strings.TrimSpace(value) != "" && value == strings.TrimSpace(value) && strings.IndexFunc(value, unicode.IsControl) < 0 && utf8.RuneCountInString(value) <= maxDeprecatedReasonRunes
}

// validateDeprecatedAlternatives 校验替代包列表。
// typesByOwnerRepo 为全量列表中 owner/repo → 所属类型 的反查表：替代包可以跨类型，但必须恰属于一个类型，
// 否则 Stage 无法确定包名（而客户端也必须知道包类型才能打开）。
func validateDeprecatedAlternatives(path, source string, alternatives []string, typesByOwnerRepo map[string][]rules.PackageType) []rules.Issue {
	var issues []rules.Issue
	if len(alternatives) > maxDeprecatedAlternatives {
		issues = append(issues, deprecatedIssue(
			fmt.Sprintf("弃用条目 `%s.alternatives` 最多只能包含 %d 个替代包。", path, maxDeprecatedAlternatives),
			fmt.Sprintf("Deprecation entry `%s.alternatives` can contain at most %d alternatives.", path, maxDeprecatedAlternatives),
		))
	}
	seen := map[string]struct{}{}
	for index, alternative := range alternatives {
		field := fmt.Sprintf("%s.alternatives[%d]", path, index)
		if alternative != strings.TrimSpace(alternative) || !validOwnerRepo(alternative) {
			issues = append(issues, deprecatedIssue(
				fmt.Sprintf("弃用条目 `%s` 必须严格使用 `owner/repo` 格式，且不能包含首尾空格。", field),
				fmt.Sprintf("Deprecation entry `%s` must use the exact `owner/repo` format, without leading or trailing spaces.", field),
			))
		}
		identity := strings.ToLower(strings.TrimSpace(alternative))
		if identity == strings.ToLower(source) {
			issues = append(issues, deprecatedIssue(
				fmt.Sprintf("弃用条目 `%s` 不能把源包自身列为替代包。", field),
				fmt.Sprintf("Deprecation entry `%s` can't list the source package itself as an alternative.", field),
			))
		}
		if _, duplicate := seen[identity]; duplicate {
			issues = append(issues, deprecatedIssue(
				fmt.Sprintf("弃用条目 `%s` 与前面的替代包重复。", field),
				fmt.Sprintf("Deprecation entry `%s` duplicates an earlier alternative.", field),
			))
		} else {
			seen[identity] = struct{}{}
		}
		if listed := typesByOwnerRepo[alternative]; len(listed) == 0 {
			issues = append(issues, deprecatedIssue(
				fmt.Sprintf("弃用条目 `%s` 指向的替代包不在任何集市包列表中。", field),
				fmt.Sprintf("The alternative referenced by deprecation entry `%s` isn't in any bazaar package list.", field),
			))
		} else if len(listed) > 1 {
			issues = append(issues, deprecatedIssue(
				fmt.Sprintf("弃用条目 `%s` 指向的替代包在多个类型列表中重复出现，无法确定其包类型。", field),
				fmt.Sprintf("The alternative referenced by deprecation entry `%s` appears in multiple package type lists, so its type is ambiguous.", field),
			))
		}
	}
	return issues
}

func validOwnerRepo(ownerRepo string) bool {
	if ownerRepo == "" || ownerRepo != strings.TrimSpace(ownerRepo) || strings.IndexFunc(ownerRepo, unicode.IsSpace) >= 0 {
		return false
	}
	owner, repo, ok := strings.Cut(ownerRepo, "/")
	return ok && owner != "" && repo != "" && !strings.Contains(repo, "/")
}

func deprecatedIssue(zh, en string) rules.Issue {
	return rules.IssueFromErr(rules.LocalizedErr(zh, en, nil))
}

// ChangedDeprecatedTypes 返回两个注册表之间内容发生变化的包类型。
func ChangedDeprecatedTypes(before, after *DeprecatedRegistry) []rules.PackageType {
	var changed []rules.PackageType
	for _, packageType := range rules.AllPackageTypes() {
		if !reflect.DeepEqual(before.Entries(packageType), after.Entries(packageType)) {
			changed = append(changed, packageType)
		}
	}
	return changed
}

// DeprecatedTarget 为一个 owner/repo 解析出的替代包：来源类型与下发给客户端的选项。
type DeprecatedTarget struct {
	PackageType rules.PackageType
	Choice      rules.PackageChoice
}

// NewDeprecatedTargets 从各类 Stage 条目建立 owner/repo → 替代包 的解析表。
// 必须传入全部类型的 Stage 数据：替代包可以跨类型，只看本类型会漏掉跨类型目标。
// 同一 owner/repo 只应属于一个类型（列表唯一性由 rules.Check 保证），重复时保留先出现的类型。
func NewDeprecatedTargets(reposByType map[rules.PackageType][]StageRepo) map[string]DeprecatedTarget {
	targets := make(map[string]DeprecatedTarget)
	for _, packageType := range rules.AllPackageTypes() {
		repos := reposByType[packageType]
		for i := range repos {
			ownerRepo, ok := OwnerRepoFromStageURL(repos[i].URL)
			if !ok || repos[i].Package.Name == "" {
				continue
			}
			if _, exists := targets[ownerRepo]; exists {
				continue
			}
			targets[ownerRepo] = DeprecatedTarget{
				PackageType: packageType,
				Choice:      rules.PackageChoice{PackageType: packageType.Plural(), Name: repos[i].Package.Name},
			}
		}
	}
	return targets
}

// ApplyDeprecatedRegistry 先清除全部 Stage 生成字段，再按注册表覆盖当前类型条目。
// targets 提供跨类型替代包解析（见 NewDeprecatedTargets）。
// Alternatives 只收同类型目标：旧客户端按源包类型打开替代包，跨类型目标会查不到；
// Choices 收全部目标（含同类型），是客户端跨类型跳转的依据。
// 返回的警告表示 Stage 数据不足或注册表语义异常；调用方应记录并继续生成安全的索引。
func ApplyDeprecatedRegistry(packageType rules.PackageType, repos []StageRepo, registry *DeprecatedRegistry, targets map[string]DeprecatedTarget) []string {
	byOwnerRepo := make(map[string]int, len(repos))
	for i := range repos {
		repos[i].Package.Deprecated = false
		repos[i].Package.DeprecatedReason = nil
		repos[i].Package.Alternatives = nil
		repos[i].Package.Choices = nil
		if ownerRepo, ok := OwnerRepoFromStageURL(repos[i].URL); ok {
			byOwnerRepo[ownerRepo] = i
		}
	}

	entries := registry.Entries(packageType)
	sources := make([]string, 0, len(entries))
	for source := range entries {
		sources = append(sources, source)
	}
	slices.Sort(sources)
	var warnings []string
	for _, source := range sources {
		entry := entries[source]
		index, ok := byOwnerRepo[source]
		if !ok {
			warnings = append(warnings, fmt.Sprintf("ignore deprecated source [%s/%s]: package is absent from stage", packageType.Plural(), source))
			continue
		}
		pkg := &repos[index].Package
		pkg.Deprecated = true
		pkg.DeprecatedReason = sanitizeDeprecatedReason(entry.Reason)

		seen := map[string]struct{}{}
		choices := make([]rules.PackageChoice, 0, len(entry.Alternatives))
		for alternativeIndex, alternative := range entry.Alternatives {
			if alternativeIndex >= maxDeprecatedAlternatives {
				warnings = append(warnings, fmt.Sprintf("truncate alternatives for deprecated package [%s] at %d entries", source, maxDeprecatedAlternatives))
				break
			}
			trimmedAlternative := strings.TrimSpace(alternative)
			if alternative != trimmedAlternative || !validOwnerRepo(alternative) {
				warnings = append(warnings, fmt.Sprintf("ignore malformed alternative [%s] for deprecated package [%s]", alternative, source))
				continue
			}
			identity := strings.ToLower(alternative)
			if identity == strings.ToLower(source) {
				warnings = append(warnings, fmt.Sprintf("ignore self alternative [%s] for deprecated package [%s]", alternative, source))
				continue
			}
			if _, duplicate := seen[identity]; duplicate {
				warnings = append(warnings, fmt.Sprintf("ignore duplicate alternative [%s] for deprecated package [%s]", alternative, source))
				continue
			}
			seen[identity] = struct{}{}
			target, exists := targets[alternative]
			if !exists {
				warnings = append(warnings, fmt.Sprintf("ignore missing alternative [%s] for deprecated package [%s]", alternative, source))
				continue
			}
			choices = append(choices, target.Choice)
			if target.PackageType == packageType {
				pkg.Alternatives = append(pkg.Alternatives, target.Choice.Name)
			}
		}
		pkg.Choices = choices
	}
	return warnings
}

func sanitizeDeprecatedReason(reason rules.LocaleStrings) rules.LocaleStrings {
	if len(reason) == 0 {
		return nil
	}
	defaultValue, hasDefault := reason["default"]
	if !hasDefault || !validDeprecatedReasonValue(defaultValue) {
		return nil
	}
	out := make(rules.LocaleStrings, len(reason))
	for locale, value := range reason {
		if !validDeprecatedLocale(locale) || !validDeprecatedReasonValue(value) {
			continue
		}
		out[locale] = html.EscapeString(value)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
