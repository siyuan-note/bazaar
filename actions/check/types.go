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
	// MetaJSON 调度元数据的紧凑 JSON，写入评论开头 HTML 注释；空则不渲染
	MetaJSON string

	Plugins   []PackageCheck
	Themes    []PackageCheck
	Icons     []PackageCheck
	Templates []PackageCheck
	Widgets   []PackageCheck

	// Deprecation 为 deprecated.json 的独立校验结果；nil 表示相对当前 main 无有效元数据变更
	Deprecation *DeprecationCheck

	// ParseError 包列表 TXT 读取或格式校验错误，非空时在 PR 评论中优先展示。
	// 每条都带出错副本的来源（本 PR 的提交 / 合并基点 / bazaar 主分支），据此决定提示谁去修
	ParseError []ListParseError

	// FlowError 流程层规则失败说明（如一次只能添加/更改一个包，或新增与无关下架混用）；非空时在评论中展示，跳过包检查且不展示下架列表
	FlowError string

	PluginsDeleted   []string
	ThemesDeleted    []string
	IconsDeleted     []string
	TemplatesDeleted []string
	WidgetsDeleted   []string
}

// ListSource 指明解析失败的是列表文件的哪一份副本。
// 评论需要据此说清「这份副本由谁维护、该由谁修」：只有 PR head 归本 PR 的贡献者。
type ListSource int

const (
	// ListSourceBazaarHead bazaar 主分支当前最新提交
	ListSourceBazaarHead ListSource = iota
	// ListSourcePRBase 本 PR 的合并基点，即 PR 开立时 main 上的提交
	ListSourcePRBase
	// ListSourcePRHead 本 PR 自己的提交
	ListSourcePRHead
)

// Key 返回用于结果摘要与日志的稳定英文标识。
func (s ListSource) Key() string {
	switch s {
	case ListSourceBazaarHead:
		return "bazaar-head"
	case ListSourcePRBase:
		return "pr-base"
	default:
		return "pr-head"
	}
}

// Title 返回评论小标题里的副本说明（双语）。模板只能调用导出方法。
func (s ListSource) Title() string {
	switch s {
	case ListSourceBazaarHead:
		return "bazaar 主分支最新版本 / latest bazaar main"
	case ListSourcePRBase:
		return "本 PR 的合并基点 / this PR's merge base"
	default:
		return "本 PR 的提交 / this PR's commit"
	}
}

// fixableByContributor 判断该副本是否由本 PR 的贡献者负责。
func (s ListSource) fixableByContributor() bool {
	return s == ListSourcePRHead
}

// Hint 返回评论里的行动指引（双语），按「这份副本该谁修」区分。
// bazaar head 与合并基点都是 main 上的历史内容，贡献者既无权也无从修改。
func (s ListSource) Hint() string {
	if s.fixableByContributor() {
		return "请修正本 PR 中的该文件（每行一个 `owner/repo`，不要有多余空行），然后推送新提交。\n\n" +
			"Fix this file in the PR (one `owner/repo` per line, no extra blank lines), then push a new commit."
	}
	return "这不是本 PR 引入的问题：这份副本由 bazaar 维护者维护。请在本 PR 中留言说明，等待维护者修复；修复后维护者会打 `Check` 标签重跑检查。\n\n" +
		"This is not caused by this PR: bazaar maintainers own this copy. Please leave a comment and wait for a maintainer to fix it; afterwards a maintainer will add the `Check` label to rerun the check."
}

// ListParseError 单个列表文件的解析错误。
type ListParseError struct {
	File   string     // 出错的文件名，如 plugins.txt
	Source ListSource // 出错的是哪一份副本
	Issue  rules.Issue
}

// DeprecationCheck 是弃用注册表变更与校验结果。
type DeprecationCheck struct {
	Types   []rules.PackageType
	Changes []DeprecationChange
	Issues  []rules.Issue
}

// DeprecationChange 是单个源包的弃用注册表增删改。
type DeprecationChange struct {
	PackageType rules.PackageType
	OwnerRepo   string
	Action      string
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
