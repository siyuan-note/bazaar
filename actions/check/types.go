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

import "github.com/siyuan-note/bazaar/check"

type ResourceType int                 // 资源类型
type StringSet map[string]interface{} // 字符串集合

// CheckResult 检查结果
type CheckResult struct {
	Icons     []PackageCheck `json:"icons"`
	Plugins   []PackageCheck `json:"plugins"`
	Templates []PackageCheck `json:"templates"`
	Themes    []PackageCheck `json:"themes"`
	Widgets   []PackageCheck `json:"widgets"`

	// ParseError 包列表 TXT 读取或格式校验错误，非空时在 PR 评论中优先展示
	ParseError string `json:"parse_error"`

	IconsDeleted     []string `json:"icons_deleted"`
	PluginsDeleted   []string `json:"plugins_deleted"`
	TemplatesDeleted []string `json:"templates_deleted"`
	ThemesDeleted    []string `json:"themes_deleted"`
	WidgetsDeleted   []string `json:"widgets_deleted"`
}

// PackageCheck 单个仓库的流程层检查结果。
// 失败一律写入 Issues（含 release/* 与 Pkg Check）；Release 仅保留通过后展示/下载所需信息。
type PackageCheck struct {
	RepoInfo          RepoInfo      `json:"repo"`
	Release           ReleaseInfo   `json:"release"`
	Issues            []check.Issue `json:"issues"`
	MaintainerChanged bool          `json:"maintainer_changed"`
}

// RepoInfo 仓库信息
type RepoInfo struct {
	Path string `json:"path"` // 仓库路径 owner/repo
	Home string `json:"home"` // 仓库主页
}

// ReleaseInfo Latest Release 摘要（无 Pass 标志；失败用 Issues）。
type ReleaseInfo struct {
	Tag               string `json:"tag,omitempty"`               // 标签名
	URL               string `json:"url,omitempty"`               // Latest Release 页面
	PackageZipAssetID int64  `json:"packageZipAssetId,omitempty"` // package.zip 的 Release Asset ID（下载用）
}

// checkOutput 并发检查结果通道载荷
type checkOutput struct {
	resourceType ResourceType
	pkg          PackageCheck
}
