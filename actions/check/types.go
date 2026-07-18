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
	Plugins   []PackageCheck
	Themes    []PackageCheck
	Icons     []PackageCheck
	Templates []PackageCheck
	Widgets   []PackageCheck

	// ParseError 包列表 TXT 读取或格式校验错误，非空时在 PR 评论中优先展示
	ParseError string

	// FlowError 流程层规则失败说明（如一次只能添加/更改一个包）；非空时在评论中展示，跳过包检查且不展示下架列表
	FlowError string

	PluginsDeleted   []string
	ThemesDeleted    []string
	IconsDeleted     []string
	TemplatesDeleted []string
	WidgetsDeleted   []string
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
	RepoInfo          RepoInfo
	Release           util.LatestRelease
	Issues            []rules.Issue
	MaintainerChanged bool
}

// RepoInfo 仓库信息
type RepoInfo struct {
	Path string // 仓库路径 owner/repo
	Home string // 仓库主页
}
