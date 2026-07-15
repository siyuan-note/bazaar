// SiYuan community bazaar.
// Copyright (c) 2021-present, b3log.org
//
// Bazaar is licensed under Mulan PSL v2.
// You can use this software according to the terms and conditions of the Mulan PSL v2.
// You may obtain a copy of Mulan PSL v2 at:
//         http://license.coscl.org.cn/MulanPSL2
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR PURPOSE.
// See the Mulan PSL v2 for more details.

package main

import (
	"github.com/siyuan-note/bazaar/actions/util"
	"github.com/siyuan-note/bazaar/rules"
)

type Set map[string]struct{} // 字符串集合

// CheckResult 检查结果（字段顺序：插件、主题、图标、模板、挂件）。
type CheckResult struct {
	Plugins   []PackageCheck `json:"plugins"`
	Themes    []PackageCheck `json:"themes"`
	Icons     []PackageCheck `json:"icons"`
	Templates []PackageCheck `json:"templates"`
	Widgets   []PackageCheck `json:"widgets"`

	// ParseError 包列表 TXT 读取或格式校验错误，非空时在 PR 评论中优先展示
	ParseError string `json:"parse_error"`

	PluginsDeleted   []string `json:"plugins_deleted"`
	ThemesDeleted    []string `json:"themes_deleted"`
	IconsDeleted     []string `json:"icons_deleted"`
	TemplatesDeleted []string `json:"templates_deleted"`
	WidgetsDeleted   []string `json:"widgets_deleted"`
}

// appendCheck 将单仓检查结果写入对应类型分组。
func (r *CheckResult) appendCheck(typ rules.PackageType, pc PackageCheck) bool {
	switch typ {
	case rules.TypePlugin:
		r.Plugins = append(r.Plugins, pc)
	case rules.TypeTheme:
		r.Themes = append(r.Themes, pc)
	case rules.TypeIcon:
		r.Icons = append(r.Icons, pc)
	case rules.TypeTemplate:
		r.Templates = append(r.Templates, pc)
	case rules.TypeWidget:
		r.Widgets = append(r.Widgets, pc)
	default:
		return false
	}
	return true
}

// setDeleted 将本 PR 删除列表写入对应类型分组。
func (r *CheckResult) setDeleted(typ rules.PackageType, paths []string) bool {
	switch typ {
	case rules.TypePlugin:
		r.PluginsDeleted = paths
	case rules.TypeTheme:
		r.ThemesDeleted = paths
	case rules.TypeIcon:
		r.IconsDeleted = paths
	case rules.TypeTemplate:
		r.TemplatesDeleted = paths
	case rules.TypeWidget:
		r.WidgetsDeleted = paths
	default:
		return false
	}
	return true
}

// PackageCheck 单个仓库的流程层检查结果。
// 失败一律写入 Issues（含 release/* 与 Pkg Check）；Release 仅保留展示/下载所需的 Latest Release 字段。
type PackageCheck struct {
	RepoInfo          RepoInfo           `json:"repo"`
	Release           util.LatestRelease `json:"release"`
	Issues            []rules.Issue      `json:"issues"`
	MaintainerChanged bool               `json:"maintainer_changed"`
}

// RepoInfo 仓库信息
type RepoInfo struct {
	Path string `json:"path"` // 仓库路径 owner/repo
	Home string `json:"home"` // 仓库主页
}

// checkOutput 并发检查结果通道载荷
type checkOutput struct {
	packageType  rules.PackageType
	packageCheck PackageCheck
}
