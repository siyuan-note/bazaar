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

// 资源类型枚举常量
const (
	icons ResourceType = iota
	plugins
	templates
	themes
	widgets
)

const (
	FILE_PATH_CHECK_RESULT_TEMPLATE = "./templates/check-result.md.tpl" // 检查结果模板文件路径

	// 所有类型集市资源都需要存在的文件
	FILE_PATH_ICON_PNG    = "icon.png"
	FILE_PATH_PREVIEW_PNG = "preview.png"
	FILE_PATH_README_MD   = "README.md"

	// 各类型集市资源的清单文件
	FILE_PATH_ICON_JSON     = "icon.json"
	FILE_PATH_PLUGIN_JSON   = "plugin.json"
	FILE_PATH_TEMPLATE_JSON = "template.json"
	FILE_PATH_THEME_JSON    = "theme.json"
	FILE_PATH_WIDGET_JSON   = "widget.json"
)

var (
	// 内置主题
	BuiltinThemeNames = []string{"daylight", "midnight"}
	// 内置图标
	BuiltinIconNames = []string{"ant", "material"}
)
