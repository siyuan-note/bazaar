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

type ResourceType int                 // 资源类型
type StringSet map[string]interface{} // 字符串集合

// CheckResult 检查结果
type CheckResult struct {
	Icons     []Icon     `json:"icons"`
	Plugins   []Plugin   `json:"plugins"`
	Templates []Template `json:"templates"`
	Themes    []Theme    `json:"themes"`
	Widgets   []Widget   `json:"widgets"`
}

// Icon 图标
type Icon struct {
	RepoInfo RepoInfo  `json:"repo"`    // 仓库
	Release  Release   `json:"release"` // 发行版
	Files    IconFiles `json:"files"`   // 文件
	Attrs    Attrs     `json:"attrs"`   // 属性
}

type IconFiles struct {
	Pass bool `json:"pass"`

	IconJson   File `json:"icon.json"`
	IconPng    File `json:"icon.png"`
	PreviewPng File `json:"preview.png"`
	ReadmeMd   File `json:"README.md"`
}

// Plugin 插件
type Plugin struct {
	RepoInfo RepoInfo    `json:"repo"`    // 仓库
	Release  Release     `json:"release"` // 发行版
	Files    PluginFiles `json:"files"`   // 文件
	Attrs    Attrs       `json:"attrs"`   // 属性
}

type PluginFiles struct {
	Pass bool `json:"pass"`

	IconPng    File `json:"icon.png"`
	PluginJson File `json:"plugin.json"`
	PreviewPng File `json:"preview.png"`
	ReadmeMd   File `json:"README.md"`
}

// Template 模板
type Template struct {
	RepoInfo RepoInfo      `json:"repo"`    // 仓库
	Release  Release       `json:"release"` // 发行版
	Files    TemplateFiles `json:"files"`   // 文件
	Attrs    Attrs         `json:"attrs"`   // 属性
}

type TemplateFiles struct {
	Pass bool `json:"pass"`

	IconPng      File `json:"icon.png"`
	PreviewPng   File `json:"preview.png"`
	ReadmeMd     File `json:"README.md"`
	TemplateJson File `json:"template.json"`
}

// Theme 主题
type Theme struct {
	RepoInfo RepoInfo   `json:"repo"`    // 仓库
	Release  Release    `json:"release"` // 发行版
	Files    ThemeFiles `json:"files"`   // 文件
	Attrs    Attrs      `json:"attrs"`   // 属性
}

type ThemeFiles struct {
	Pass bool `json:"pass"`

	IconPng    File `json:"icon.png"`
	PreviewPng File `json:"preview.png"`
	ReadmeMd   File `json:"README.md"`
	ThemeJson  File `json:"theme.json"`
}

// Widget 挂件
type Widget struct {
	RepoInfo RepoInfo    `json:"repo"`    // 仓库
	Release  Release     `json:"release"` // 发行版
	Files    WidgetFiles `json:"files"`   // 文件
	Attrs    Attrs       `json:"attrs"`   // 属性
}

type WidgetFiles struct {
	Pass bool `json:"pass"`

	IconPng    File `json:"icon.png"`
	PreviewPng File `json:"preview.png"`
	ReadmeMd   File `json:"README.md"`
	WidgetJson File `json:"widget.json"`
}

// RepoInfo 仓库信息
type RepoInfo struct {
	Owner string `json:"owner"` // 仓库拥有者
	Name  string `json:"name"`  // 仓库名
	Path  string `json:"path"`  // 仓库路径
	Home  string `json:"home"`  // 仓库主页
}

// Release 发行版
type Release struct {
	Pass          bool          `json:"pass"`           // 必要的发行版是否检查通过
	LatestRelease LatestRelease `json:"latest_release"` // 最新发行版
}

// LatestRelease 最新发行版
type LatestRelease struct {
	Pass       bool       `json:"pass"`        // 最新发行版是否存在
	URL        string     `json:"url"`         // 最新发行版 URL
	Tag        string     `json:"tag"`         // 标签名
	Hash       string     `json:"hash"`        // SHA1
	PackageZip PackageZip `json:"package_zip"` // package.zip 包
}

// PackageZip 最新发行版的 package.zip 包
type PackageZip struct {
	Pass bool   `json:"pass"` // package.zip 包是否存在
	URL  string `json:"url"`  // package.zip 包 URL
}

// File 文件
type File struct {
	Pass bool `json:"pass"` // 文件是否存在

	URL string `json:"url"` // 文件 URL
}

// Attrs 清单文件属性
type Attrs struct {
	Pass bool `json:"pass"` // 配置文件属性检查是否通过

	Name    Attr `json:"name"`
	Version Attr `json:"version"`
	Author  Attr `json:"author"`
	URL     Attr `json:"url"`
}

type Attr struct {
	Pass   bool   `json:"pass"`   // 配置文件属性是否存在
	Unique bool   `json:"unique"` // 配置文件属性是否唯一
	Value  string `json:"value"`  // 配置文件属性值
}
