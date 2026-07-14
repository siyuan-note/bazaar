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

// Mode 控制检查宽严：PR 更严，Stage 保底。
type Mode int

const (
	ModeStage Mode = iota
	ModePR
)

// PackageType 集市包类型。
type PackageType int

const (
	TypePlugin PackageType = iota
	TypeTheme
	TypeIcon
	TypeTemplate
	TypeWidget
)

func (t PackageType) String() string {
	switch t {
	case TypeTheme:
		return "theme"
	case TypeIcon:
		return "icon"
	case TypeTemplate:
		return "template"
	case TypeWidget:
		return "widget"
	default:
		return "plugin"
	}
}

// ManifestFile 返回该类型清单文件名（大小写敏感）。
func (t PackageType) ManifestFile() string {
	return t.String() + ".json"
}

// ParsePackageType 解析类型字符串（plugins/plugin 等均可）。
func ParsePackageType(s string) (PackageType, bool) {
	switch s {
	case "plugin", "plugins":
		return TypePlugin, true
	case "theme", "themes":
		return TypeTheme, true
	case "icon", "icons":
		return TypeIcon, true
	case "template", "templates":
		return TypeTemplate, true
	case "widget", "widgets":
		return TypeWidget, true
	default:
		return 0, false
	}
}
