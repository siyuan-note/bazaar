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

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// requiredFile 描述包根下的一个必要文件及其缺失时的补充说明。
type requiredFile struct {
	name   string
	hintZh string
	hintEn string
}

// requiredFilesFor 返回指定类型在包根下必须存在的文件（文件名大小写敏感）。
// 模板另有「至少一个非说明文档的 .md」规则，不在此列表中。
func requiredFilesFor(typ PackageType) []requiredFile {
	files := []requiredFile{
		{"icon.png", "这是集市列表里显示的图标。", "This is the icon shown in the bazaar list. "},
		{"preview.png", "这是集市包详情页里显示的预览图。", "This is the preview image shown on the bazaar package detail page. "},
		{"README.md", "这是集市包的默认说明文档。", "This is the default documentation file for the bazaar package. "},
		{
			typ.ManifestFile(),
			fmt.Sprintf("这是 %s 的清单文件。", typ.String()),
			fmt.Sprintf("This is the %s manifest file. ", typ.String()),
		},
	}
	switch typ {
	case TypePlugin:
		files = append(files, requiredFile{"index.js", "这是插件的前端入口脚本文件。", "This is the plugin frontend entry script file. "})
	case TypeTheme:
		files = append(files, requiredFile{"theme.css", "这是主题的样式入口文件。", "This is the theme stylesheet entry file. "})
	case TypeIcon:
		files = append(files, requiredFile{"icon.js", "这是图标包的脚本入口文件。", "This is the icon pack script entry file. "})
	case TypeWidget:
		files = append(files, requiredFile{"index.html", "这是挂件的页面入口文件。", "This is the widget page entry file. "})
	}
	return files
}

// RequiredFiles 检查必要文件是否存在（文件名大小写敏感）。
func RequiredFiles(root string, typ PackageType) []Issue {
	var issues []Issue
	for _, f := range requiredFilesFor(typ) {
		issues = append(issues, checkRequiredFile(root, f)...)
	}
	if typ == TypeTemplate {
		issues = append(issues, checkTemplateHasContentMD(root)...)
	}
	return issues
}

func checkRequiredFile(root string, f requiredFile) []Issue {
	p := filepath.Join(root, f.name)
	if !fileExistsCaseSensitive(root, f.name) {
		return []Issue{issue(fmt.Sprintf("`package.zip` 包根目录缺少必要文件 `%s`。%s文件名大小写必须完全一致。", f.name, f.hintZh),
			fmt.Sprintf("Required file `%s` is missing from the `package.zip` root. %sThe filename is case-sensitive.", f.name, f.hintEn),
		)}
	}
	info, err := os.Stat(p)
	if err != nil {
		return []Issue{issue(fmt.Sprintf("包根下存在 `%s`，但无法读取其信息：%v。请检查 `package.zip` 解压后的文件是否完整、权限是否正常。", f.name, err),
			fmt.Sprintf("`%s` is present at the package root, but its metadata could not be read: %v. Check that the extracted `package.zip` is intact and file permissions are correct.", f.name, err),
		)}
	}
	if info.IsDir() {
		return []Issue{issue(fmt.Sprintf("`%s` 目前是一个目录，但集市要求它是普通文件。请放到包根下的同名文件，而不是文件夹。", f.name),
			fmt.Sprintf("`%s` is a directory, but the bazaar expects a regular file at the package root. Put a file with this exact name there, not a folder.", f.name),
		)}
	}
	return nil
}

// checkTemplateHasContentMD：模板包至少包含一个可作为模板正文的 .md 文件。
// 排除 `README.md` 以及清单 `readme` 字段声明的说明文件（相对包根路径，大小写敏感；对齐思源内核）。
func checkTemplateHasContentMD(root string) []Issue {
	excluded := getTemplateReadmePaths(root)
	found := false
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if _, skip := excluded[rel]; skip {
			return nil
		}
		found = true
		return fs.SkipAll
	})
	if found {
		return nil
	}
	if err != nil {
		return []Issue{issue(
			fmt.Sprintf("检查模板内容文件时遍历目录失败：%v。请确认 `package.zip` 解压后的目录结构完整。", err),
			fmt.Sprintf("Failed to walk the template package while looking for content files: %v. Ensure the extracted `package.zip` layout is intact.", err),
		)}
	}
	return []Issue{issue("模板包里除了 `README.md` 和清单 `readme` 字段声明的说明文件外，还至少需要一个可作为模板内容的 `.md` 文件。请把模板正文 md 打进 `package.zip`。",
		"Besides `README.md` and files declared in the manifest `readme` field, a template package must include at least one content `.md` file. Add that file to `package.zip`.",
	)}
}

// getTemplateReadmePaths 返回不应计为模板正文的相对包根路径集合：恒含 `README.md`，并合并清单 `readme` 各语言值。
func getTemplateReadmePaths(root string) map[string]struct{} {
	excluded := map[string]struct{}{"README.md": {}}
	data, err := os.ReadFile(filepath.Join(root, "template.json"))
	if err != nil {
		return excluded
	}
	var pkg Package
	if err = json.Unmarshal(data, &pkg); err != nil {
		return excluded
	}
	for _, v := range pkg.Readme {
		v = strings.TrimSpace(v)
		if v != "" {
			excluded[v] = struct{}{}
		}
	}
	return excluded
}

// ThemeJS 检查主题是否允许包含 theme.js。
// 仅 config/themes-theme-js-allowlist.txt 中的旧仓库允许在包根目录包含 theme.js（存量豁免），
// 其余新上架主题必须移除。REF https://github.com/siyuan-note/bazaar/issues/1821
// allow 由调用方根据白名单决定（对应 Input.AllowThemeJS）。
func ThemeJS(root string, allow bool) []Issue {
	if !fileExistsCaseSensitive(root, "theme.js") {
		return nil
	}
	if allow {
		return nil
	}
	return []Issue{issue("包根目录含有 `theme.js`。新上架主题默认不允许使用 `theme.js`（历史白名单仓库除外）。请从 `package.zip` 中删除 `theme.js`，改用纯 CSS 方式或者单独制作插件实现。",
		"`package.zip` contains `theme.js` at the package root. New themes must not ship `theme.js` (except legacy allowlisted repos). Remove `theme.js` from the package and use pure CSS, or implement the behavior as a separate plugin.",
	)}
}

// fileExistsCaseSensitive 在 dir 下查找名为 name 的文件或目录（对 name 大小写敏感，通过列目录比对）。
// 集市包文件名校验均按大小写敏感处理（对齐思源内核在 Mac / Linux 上的行为）。
func fileExistsCaseSensitive(dir, name string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.Name() == name {
			return true
		}
	}
	return false
}
