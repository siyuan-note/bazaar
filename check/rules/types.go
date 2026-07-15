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

// Issue 单条检查问题。
// Rule 为稳定内部标识；MessageZh / MessageEn 自含路径与改法，可直接用于评论。
type Issue struct {
	Rule      string `json:"rule"`
	MessageZh string `json:"messageZh"`
	MessageEn string `json:"messageEn"`
}

func issue(rule, zh, en string) Issue {
	return Issue{Rule: rule, MessageZh: zh, MessageEn: en}
}

// PackageType 集市包类型。
type PackageType int

const (
	TypePlugin PackageType = iota
	TypeTheme
	TypeIcon
	TypeTemplate
	TypeWidget
)

type packageTypeMeta struct {
	singular string // 清单文件名前缀，如 plugin.json
	plural   string // 仓库列表 / stage 目录名，如 plugins.txt
}

// packageTypeMetas 为集市包类型的唯一元数据表；新增类型时只改此处。
var packageTypeMetas = [...]packageTypeMeta{
	TypePlugin:   {singular: "plugin", plural: "plugins"},
	TypeTheme:    {singular: "theme", plural: "themes"},
	TypeIcon:     {singular: "icon", plural: "icons"},
	TypeTemplate: {singular: "template", plural: "templates"},
	TypeWidget:   {singular: "widget", plural: "widgets"},
}

var packageTypeByName map[string]PackageType

func init() {
	packageTypeByName = make(map[string]PackageType, len(packageTypeMetas)*2)
	for i := range packageTypeMetas {
		typ := PackageType(i)
		m := packageTypeMetas[i]
		packageTypeByName[m.singular] = typ
		packageTypeByName[m.plural] = typ
	}
}

// AllPackageTypes 返回所有集市包类型（声明顺序）。
func AllPackageTypes() []PackageType {
	out := make([]PackageType, len(packageTypeMetas))
	for i := range packageTypeMetas {
		out[i] = PackageType(i)
	}
	return out
}

// StageOrderPackageTypes 返回 Stage 流水线顺序（themes 优先，与历史行为一致）。
func StageOrderPackageTypes() []PackageType {
	return []PackageType{TypeTheme, TypeTemplate, TypeIcon, TypeWidget, TypePlugin}
}

// CheckOrderPackageTypes 返回 PR Check 并发顺序（与 CheckResult JSON 分组一致）。
func CheckOrderPackageTypes() []PackageType {
	return []PackageType{TypeIcon, TypePlugin, TypeTemplate, TypeTheme, TypeWidget}
}

func (t PackageType) valid() bool {
	return t >= TypePlugin && int(t) < len(packageTypeMetas)
}

func (t PackageType) meta() (packageTypeMeta, bool) {
	if !t.valid() {
		return packageTypeMeta{}, false
	}
	return packageTypeMetas[t], true
}

func (t PackageType) String() string {
	if m, ok := t.meta(); ok {
		return m.singular
	}
	return packageTypeMetas[TypePlugin].singular
}

// Plural 返回复数形式（plugins、themes 等），用于仓库列表与 stage 路径。
func (t PackageType) Plural() string {
	if m, ok := t.meta(); ok {
		return m.plural
	}
	return packageTypeMetas[TypePlugin].plural
}

// ManifestFile 返回该类型清单文件名（大小写敏感）。
func (t PackageType) ManifestFile() string {
	return t.String() + ".json"
}

// ReposListFile 返回仓库列表文件名（如 plugins.txt）。
func (t PackageType) ReposListFile() string {
	return t.Plural() + ".txt"
}

// StageJSONFile 返回 stage 索引文件名（如 plugins.json）。
func (t PackageType) StageJSONFile() string {
	return t.Plural() + ".json"
}

// ParsePackageType 解析类型字符串（plugins/plugin 等均可）。
func ParsePackageType(s string) (PackageType, bool) {
	t, ok := packageTypeByName[s]
	return t, ok
}
