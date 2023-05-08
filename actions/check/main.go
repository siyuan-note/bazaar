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
	"os"
	"text/template"

	"github.com/88250/gulu"
)

/*
检查更改的文件内的仓库是否有不在 stage/*.json 对应文件的仓库列表中
	获取 PR 中 *.json 中的仓库列表
	获取 stage/*.json 中的仓库列表
	判断是否有仅存在 *.json 而不存在 stage/*.json 中的仓库
如果有
	获取仓库最新 release
	获取仓库最新 release 的 tag
	获取仓库最新 release 的 hash
	获取配置文件 *.json
		检查配置文件是否具有必要的字段
		检查资源名称是否与 stage/*.json 中的重复
	检查必要的文件是否存在
	生成检查结果并输出文件 (使用 go 模板)
	使用 thollander/actions-comment-pull-request@v2.3.1 将检查结果输出到到 PR 中
*/

var logger = gulu.Log.NewLogger(os.Stdout)

func main() {
	logger.Infof("PR Check started")

	const commentTemplateFilePath = "./templates/comment.md.tpl"
	const commentOutputFilePath = "./templates/comment.md"

	commentTpl, err := template.ParseFiles(commentTemplateFilePath)
	if nil != err {
		logger.Fatalf("load template file [%s] failed: %s", commentTemplateFilePath, err)
		panic(err)
	}

	commentFile, err := os.OpenFile(commentOutputFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if nil != err {
		logger.Fatalf("open comment file [%s] failed: %s", commentOutputFilePath, err)
		panic(err)
	}

	commentTpl.Execute(commentFile, CheckResultTestExample)
	logger.Infof("PR Check finished")
}

/* 检查结果 */
type CheckResult struct {
	Icons     []IconRepo     `json:"icons"`
	Plugins   []PluginRepo   `json:"plugins"`
	Templates []TemplateRepo `json:"templates"`
	Themes    []ThemeRepo    `json:"themes"`
	Widgets   []WidgetRepo   `json:"widgets"`
}

/* 图标 */
type IconRepo struct {
	Name string `json:"name"` // 仓库名
	Path string `json:"path"` // 仓库路径
	Home string `json:"home"` // 仓库主页

	Release Release   `json:"release"` // 仓库发行版
	Files   IconFiles `json:"files"`   // 仓库文件
	Attrs   Attrs     `json:"attrs"`   // 仓库属性
}

type IconFiles struct {
	Pass bool `json:"pass"`

	IconJs     File `json:"icon.js"`
	IconJson   File `json:"icon.json"`
	PreviewPng File `json:"preview.png"`
	ReadmeMd   File `json:"README.md"`
}

/* 插件 */
type PluginRepo struct {
	Name string `json:"name"` // 仓库名
	Path string `json:"path"` // 仓库路径
	Home string `json:"home"` // 仓库主页

	Release Release     `json:"release"` // 仓库发行版
	Files   PluginFiles `json:"files"`   // 仓库文件
	Attrs   Attrs       `json:"attrs"`   // 仓库属性
}

type PluginFiles struct {
	Pass bool `json:"pass"`

	IndexJs    File `json:"index.js"`
	PluginJson File `json:"plugin.json"`
	PreviewPng File `json:"preview.png"`
	ReadmeMd   File `json:"README.md"`
}

/* 模板 */
type TemplateRepo struct {
	Name string `json:"name"` // 仓库名
	Path string `json:"path"` // 仓库路径
	Home string `json:"home"` // 仓库主页

	Release Release       `json:"release"` // 仓库发行版
	Files   TemplateFiles `json:"files"`   // 仓库文件
	Attrs   Attrs         `json:"attrs"`   // 仓库属性
}

type TemplateFiles struct {
	Pass bool `json:"pass"`

	PreviewPng   File `json:"preview.png"`
	ReadmeMd     File `json:"README.md"`
	TemplateJson File `json:"template.json"`
}

/* 主题 */
type ThemeRepo struct {
	Name string `json:"name"` // 仓库名
	Path string `json:"path"` // 仓库路径
	Home string `json:"home"` // 仓库主页

	Release Release    `json:"release"` // 仓库发行版
	Files   ThemeFiles `json:"files"`   // 仓库文件
	Attrs   Attrs      `json:"attrs"`   // 仓库属性
}

type ThemeFiles struct {
	Pass bool `json:"pass"`

	PreviewPng File `json:"preview.png"`
	ReadmeMd   File `json:"README.md"`
	ThemeCss   File `json:"theme.css"`
	ThemeJson  File `json:"theme.json"`
}

/* 挂件 */
type WidgetRepo struct {
	Name string `json:"name"` // 仓库名
	Path string `json:"path"` // 仓库路径
	Home string `json:"home"` // 仓库主页

	Release Release     `json:"release"` // 仓库发行版
	Files   WidgetFiles `json:"files"`   // 仓库文件
	Attrs   Attrs       `json:"attrs"`   // 仓库属性
}

type WidgetFiles struct {
	Pass bool `json:"pass"`

	IndexHtml  File `json:"index.html"`
	PreviewPng File `json:"preview.png"`
	ReadmeMd   File `json:"README.md"`
	WidgetJson File `json:"widget.json"`
}

/* 发行版 */
type Release struct {
	Pass          bool          `json:"pass"`           // 必要的发行版是否检查通过
	LatestRelease LatestRelease `json:"latest_release"` // 最新发行版
}

/* 最新发行版 */
type LatestRelease struct {
	Pass       bool       `json:"pass"`        // 最新发行版是否存在
	URL        string     `json:"url"`         // 最新发行版 URL
	PackageZip PackageZip `json:"package_zip"` // package.zip 包
}

/* 最新发行版的 package.zip 包 */
type PackageZip struct {
	Pass bool   `json:"pass"` // package.zip 包是否存在
	URL  string `json:"url"`  // package.zip 包 URL
}

/* 文件 */
type File struct {
	Pass bool   `json:"pass"` // 文件是否存在
	Name string `json:"name"` // 文件名
	URL  string `json:"url"`  // 文件 URL
}

/* 配置文件属性 */
type Attrs struct {
	Pass    bool `json:"pass"` // 配置文件必要属性相关内容是否都存在
	Name    Attr `json:"name"`
	Version Attr `json:"version"`
}

type Attr struct {
	Pass   bool   `json:"pass"`   // 配置文件属性是否存在
	Unique bool   `json:"unique"` // 配置文件属性是否唯一
	Value  string `json:"value"`  // 配置文件属性值
}
