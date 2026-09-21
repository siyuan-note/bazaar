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

type Set map[string]struct{} // 字符串集合

// Issue 单条检查问题。
// MessageZh / MessageEn 自含路径与改法，可直接用于评论。
type Issue struct {
	MessageZh string
	MessageEn string
}

func issue(zh, en string) Issue {
	return Issue{MessageZh: zh, MessageEn: en}
}

// PackageType 集市包类型。
type PackageType int

const (
	_ PackageType = iota
	TypePlugin
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

// AllPackageTypes 返回所有集市包类型（插件、主题、图标、模板、挂件）。
func AllPackageTypes() []PackageType {
	return []PackageType{TypePlugin, TypeTheme, TypeIcon, TypeTemplate, TypeWidget}
}

func (t PackageType) valid() bool {
	return t >= TypePlugin && t <= TypeWidget
}

func (t PackageType) String() string {
	if !t.valid() {
		panic("rules: invalid PackageType")
	}
	return packageTypeMetas[t].singular
}

// Plural 返回复数形式（plugins、themes 等），用于仓库列表与 stage 路径。
func (t PackageType) Plural() string {
	if !t.valid() {
		panic("rules: invalid PackageType")
	}
	return packageTypeMetas[t].plural
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
