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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckRejectsInvalidPackageType(t *testing.T) {
	r := Check(Input{
		PackageRoot: filepath.Join("testdata", "plugin_ok"),
		OwnerRepo:   "demo/sample-plugin",
	})
	if r.OK || len(r.Issues) != 1 || !hasIssueMsg(r, "集市包类型") {
		t.Fatalf("expected invalid package type issue, got OK=%v issues=%v", r.OK, r.Issues)
	}
}

func TestCheckPluginOK_PR(t *testing.T) {
	root := filepath.Join("testdata", "plugin_ok")
	r := Check(Input{
		PackageRoot: root,
		OwnerRepo:   "demo/sample-plugin",
		Type:        TypePlugin,
	})
	if !r.OK {
		t.Fatalf("expected OK, issues=%v", r.Issues)
	}
	if r.Package.Name == "" {
		t.Fatal("expected package name")
	}
}

func TestCheckPluginMissingIcon(t *testing.T) {
	root := filepath.Join("testdata", "plugin_missing_icon")
	r := Check(Input{
		PackageRoot: root,
		OwnerRepo:   "demo/sample-plugin",
		Type:        TypePlugin,
	})
	if r.OK {
		t.Fatal("expected failure")
	}
	if !hasIssueMsg(r, "缺少必要文件") {
		t.Fatalf("expected missing required file issue, issues=%v", r.Issues)
	}
}

func TestCheckNestedRoot(t *testing.T) {
	root := filepath.Join("testdata", "plugin_nested")
	r := Check(Input{
		PackageRoot: root,
		OwnerRepo:   "demo/sample-plugin",
		Type:        TypePlugin,
	})
	if !r.OK {
		t.Fatalf("expected OK for nested root, issues=%v", r.Issues)
	}
	wantSuffix := filepath.Join("plugin_nested", "wrapper")
	if !filepath.IsAbs(r.PackageRoot) && !containsPathSuffix(r.PackageRoot, wantSuffix) {
		// PackageRoot may be absolute or relative
		if filepath.Base(r.PackageRoot) != "wrapper" {
			t.Fatalf("expected package root wrapper, got %s", r.PackageRoot)
		}
	}
}

func TestCheckSpaceName(t *testing.T) {
	root := filepath.Join("testdata", "plugin_space_name")
	r := Check(Input{
		PackageRoot: root,
		OwnerRepo:   "demo/sample-plugin",
		Type:        TypePlugin,
	})
	if r.OK {
		t.Fatal("expected failure for leading-space filename")
	}
	if !hasIssueMsg(r, "以空格开头或结尾") {
		t.Fatalf("expected whitespace name issue, issues=%v", r.Issues)
	}
}

func TestCheckNameImmutable(t *testing.T) {
	root := filepath.Join("testdata", "plugin_ok")
	r := Check(Input{
		PackageRoot: root,
		OwnerRepo:   "demo/sample-plugin",
		Type:        TypePlugin,
		OldName:     "other-name",
	})
	if r.OK {
		t.Fatal("expected name_immutable")
	}
	if !hasIssueMsg(r, "不可更改") {
		t.Fatalf("expected name immutable issue, issues=%v", r.Issues)
	}
}

func TestCheckVersionMustIncrease(t *testing.T) {
	root := filepath.Join("testdata", "plugin_ok")
	r := Check(Input{
		PackageRoot: root,
		OwnerRepo:   "demo/sample-plugin",
		Type:        TypePlugin,
		OldVersion:  "1.0.0",
	})
	if r.OK {
		t.Fatal("expected version_not_greater when equal")
	}
	if !hasIssueMsg(r, "version") {
		t.Fatalf("expected version issue, issues=%v", r.Issues)
	}

	r2 := Check(Input{
		PackageRoot: root,
		OwnerRepo:   "demo/sample-plugin",
		Type:        TypePlugin,
		OldVersion:  "0.9.0",
	})
	if !r2.OK {
		t.Fatalf("expected OK when version greater, issues=%v", r2.Issues)
	}

	// 旧版本号无法解析、新版本号合法时，视为修复版本号，放行
	r3 := Check(Input{
		PackageRoot: root,
		OwnerRepo:   "demo/sample-plugin",
		Type:        TypePlugin,
		OldVersion:  "0.9.6.2",
	})
	if !r3.OK {
		t.Fatalf("expected OK when fixing unparsable old version, issues=%v", r3.Issues)
	}
}

func TestCheckVersionRejectsVPrefix(t *testing.T) {
	dir := t.TempDir()
	copyTree(t, filepath.Join("testdata", "plugin_ok"), dir)
	content := `{
  "name": "sample-plugin",
  "author": "demo",
  "url": "https://github.com/demo/sample-plugin",
  "version": "v1.0.0",
  "readme": { "default": "README.md" }
}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	r := Check(Input{
		PackageRoot: dir,
		OwnerRepo:   "demo/sample-plugin",
		Type:        TypePlugin,
	})
	if r.OK || !hasIssueMsg(r, "v`/`V` 前缀") {
		t.Fatalf("expected version v-prefix issue, issues=%v", r.Issues)
	}
}

func TestCheckUnknownField(t *testing.T) {
	dir := t.TempDir()
	copyTree(t, filepath.Join("testdata", "plugin_ok"), dir)
	manifest := filepath.Join(dir, "plugin.json")
	data, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	// inject unknown field
	patched := data[:len(data)-2] // strip trailing }\n roughly — use rewrite instead
	_ = patched
	content := `{
  "name": "sample-plugin",
  "author": "demo",
  "url": "https://github.com/demo/sample-plugin",
  "version": "1.0.0",
  "readme": { "default": "README.md" },
  "i18n": {}
}`
	if err := os.WriteFile(manifest, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	r := Check(Input{
		PackageRoot: dir,
		OwnerRepo:   "demo/sample-plugin",
		Type:        TypePlugin,
	})
	if r.OK || !hasIssueMsg(r, "预期外的字段") {
		t.Fatalf("expected manifest/unknown_field, issues=%v", r.Issues)
	}
}

func TestManifestKeysByPackageType(t *testing.T) {
	// 插件不得带 modes；主题不得带 backends；图标等仅通用字段
	cases := []struct {
		typ     PackageType
		extra   string
		wantSub string
	}{
		{TypePlugin, "modes", "modes"},
		{TypeTheme, "backends", "backends"},
		{TypeTheme, "disabledInPublish", "disabledInPublish"},
		{TypeIcon, "frontends", "frontends"},
		{TypeTemplate, "kernels", "kernels"},
		{TypeWidget, "modes", "modes"},
	}
	for _, tc := range cases {
		m := map[string]any{tc.extra: true}
		issues := checkUnknownKeys(m, tc.typ)
		if !issuesContain(issues, "预期外的字段") || !issuesContain(issues, tc.wantSub) {
			t.Fatalf("%v with %s: want unexpected-field issue, got %v", tc.typ, tc.extra, issues)
		}
	}

	// 各类型允许的字段不应被拒
	okCases := []struct {
		typ PackageType
		key string
	}{
		{TypePlugin, "backends"},
		{TypePlugin, "disabledInPublish"},
		{TypeTheme, "modes"},
		{TypeIcon, "keywords"},
		{TypeTemplate, "minAppVersion"},
		{TypeWidget, "displayName"},
	}
	for _, tc := range okCases {
		issues := checkUnknownKeys(map[string]any{tc.key: true}, tc.typ)
		if len(issues) != 0 {
			t.Fatalf("%v with allowed key %s: unexpected issues %v", tc.typ, tc.key, issues)
		}
	}
}

func TestCheckURL(t *testing.T) {
	dir := t.TempDir()
	copyTree(t, filepath.Join("testdata", "plugin_ok"), dir)
	content := `{
  "name": "sample-plugin",
  "author": "demo",
  "url": "https://github.com/demo/sample-plugin.git",
  "version": "1.0.0",
  "readme": { "default": "README.md" }
}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	r := Check(Input{
		PackageRoot: dir,
		OwnerRepo:   "demo/sample-plugin",
		Type:        TypePlugin,
	})
	if r.OK || !hasIssueMsg(r, "url") {
		t.Fatalf("expected manifest/url, issues=%v", r.Issues)
	}
}

func TestCheckMissingIndexJS(t *testing.T) {
	dir := t.TempDir()
	copyTree(t, filepath.Join("testdata", "plugin_ok"), dir)
	if err := os.Remove(filepath.Join(dir, "index.js")); err != nil {
		t.Fatal(err)
	}
	r := Check(Input{
		PackageRoot: dir,
		OwnerRepo:   "demo/sample-plugin",
		Type:        TypePlugin,
	})
	if r.OK || !hasIssueMsg(r, "缺少必要文件") {
		t.Fatalf("expected files/required for missing index.js, issues=%v", r.Issues)
	}
}

func TestCheckNameStrict(t *testing.T) {
	dir := t.TempDir()
	copyTree(t, filepath.Join("testdata", "plugin_ok"), dir)
	content := `{
  "name": ".hidden",
  "author": "demo",
  "url": "https://github.com/demo/.hidden",
  "version": "1.0.0",
  "readme": { "default": "README.md" }
}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	r := Check(Input{
		PackageRoot: dir,
		OwnerRepo:   "demo/.hidden",
		Type:        TypePlugin,
	})
	if r.OK || !hasIssueMsg(r, "以句点开头") {
		t.Fatalf("expected manifest/name for leading dot, issues=%v", r.Issues)
	}
}

func TestCheckTemplateNeedsContentMD(t *testing.T) {
	dir := t.TempDir()
	// minimal template package without content .md
	writeMinimalPNGs(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# t"), 0644); err != nil {
		t.Fatal(err)
	}
	manifest := `{
  "name": "sample-template",
  "author": "demo",
  "url": "https://github.com/demo/sample-template",
  "version": "1.0.0",
  "readme": { "default": "README.md", "zh_CN": "README_zh_CN.md" }
}`
	if err := os.WriteFile(filepath.Join(dir, "template.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	r := Check(Input{
		PackageRoot: dir,
		OwnerRepo:   "demo/sample-template",
		Type:        TypeTemplate,
	})
	if r.OK || !hasIssueMsg(r, "模板内容") {
		t.Fatalf("expected files/template_md, issues=%v", r.Issues)
	}

	// 清单 readme 声明的说明文件不算模板正文；以 readme 开头的文件名可以算正文
	if err := os.WriteFile(filepath.Join(dir, "README_zh_CN.md"), []byte("readme locale"), 0644); err != nil {
		t.Fatal(err)
	}
	rLocale := Check(Input{
		PackageRoot: dir,
		OwnerRepo:   "demo/sample-template",
		Type:        TypeTemplate,
	})
	if rLocale.OK || !hasIssueMsg(rLocale, "模板内容") {
		t.Fatalf("expected still missing content md when only declared readme files exist, issues=%v", rLocale.Issues)
	}

	if err := os.WriteFile(filepath.Join(dir, "readme-note.md"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	r2 := Check(Input{
		PackageRoot: dir,
		OwnerRepo:   "demo/sample-template",
		Type:        TypeTemplate,
	})
	if !r2.OK {
		t.Fatalf("expected OK after adding readme-note.md, issues=%v", r2.Issues)
	}
}

func TestCheckOptionalTypedFields(t *testing.T) {
	dir := t.TempDir()
	copyTree(t, filepath.Join("testdata", "plugin_ok"), dir)
	writePlugin := func(extra string) {
		t.Helper()
		content := `{
  "name": "sample-plugin",
  "author": "demo",
  "url": "https://github.com/demo/sample-plugin",
  "version": "1.0.0",
  "readme": { "default": "README.md" }` + extra + `
}`
		if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	check := func() *Result {
		t.Helper()
		return Check(Input{
			PackageRoot: dir,
			OwnerRepo:   "demo/sample-plugin",
			Type:        TypePlugin,
		})
	}

	writePlugin(`,
  "minAppVersion": "3.7.0",
  "displayName": { "default": "Sample" },
  "description": { "default": "Demo plugin" },
  "keywords": ["sample"],
  "backends": ["all"],
  "frontends": ["all"],
  "disabledInPublish": false`)
	if r := check(); !r.OK {
		t.Fatalf("expected OK for valid optional fields, issues=%v", r.Issues)
	}

	writePlugin(`,
  "minAppVersion": 3`)
	if r := check(); r.OK || !hasIssueMsg(r, "minAppVersion") {
		t.Fatalf("expected non-string minAppVersion to fail, issues=%v", r.Issues)
	}

	writePlugin(`,
  "displayName": "Sample"`)
	if r := check(); r.OK || !hasIssueMsg(r, "displayName") {
		t.Fatalf("expected non-object displayName to fail, issues=%v", r.Issues)
	}

	writePlugin(`,
  "description": { "default": 1 }`)
	if r := check(); r.OK || !hasIssueMsg(r, "description.default") {
		t.Fatalf("expected non-string description value to fail, issues=%v", r.Issues)
	}

	writePlugin(`,
  "keywords": "sample"`)
	if r := check(); r.OK || !hasIssueMsg(r, "keywords") {
		t.Fatalf("expected non-array keywords to fail, issues=%v", r.Issues)
	}

	writePlugin(`,
  "disabledInPublish": "true"`)
	if r := check(); r.OK || !hasIssueMsg(r, "disabledInPublish") {
		t.Fatalf("expected non-bool disabledInPublish to fail, issues=%v", r.Issues)
	}

	writePlugin(`,
  "backends": ["all", "windows"]`)
	if r := check(); r.OK || !hasIssueMsg(r, "backends") || !hasIssueMsg(r, "all") {
		t.Fatalf("expected backends mixing all with others to fail, issues=%v", r.Issues)
	}

	writePlugin(`,
  "frontends": ["desktop", "all"]`)
	if r := check(); r.OK || !hasIssueMsg(r, "frontends") || !hasIssueMsg(r, "all") {
		t.Fatalf("expected frontends mixing all with others to fail, issues=%v", r.Issues)
	}

	writePlugin(`,
  "kernels": ["all", "linux"]`)
	if r := check(); r.OK || !hasIssueMsg(r, "kernels") || !hasIssueMsg(r, "all") {
		t.Fatalf("expected kernels mixing all with others to fail, issues=%v", r.Issues)
	}
}

func TestCheckFunding(t *testing.T) {
	dir := t.TempDir()
	copyTree(t, filepath.Join("testdata", "plugin_ok"), dir)
	writePlugin := func(funding string) {
		t.Helper()
		content := `{
  "name": "sample-plugin",
  "author": "demo",
  "url": "https://github.com/demo/sample-plugin",
  "version": "1.0.0",
  "readme": { "default": "README.md" },
  "funding": ` + funding + `
}`
		if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	check := func() *Result {
		t.Helper()
		return Check(Input{
			PackageRoot: dir,
			OwnerRepo:   "demo/sample-plugin",
			Type:        TypePlugin,
		})
	}

	writePlugin(`{ "openCollective": "b3log", "patreon": "hongster85", "github": "https://github.com/demo", "custom": ["微信打赏", "https://example.com/sponsor"] }`)
	if r := check(); !r.OK {
		t.Fatalf("expected OK for valid funding fields, issues=%v", r.Issues)
	}

	writePlugin(`{ "custom": [123] }`)
	if r := check(); r.OK || !hasIssueMsg(r, "funding") {
		t.Fatalf("expected non-string funding.custom to fail, issues=%v", r.Issues)
	}

	writePlugin(`{ "github": 123 }`)
	if r := check(); r.OK || !hasIssueMsg(r, "funding.github") {
		t.Fatalf("expected non-string funding.github to fail, issues=%v", r.Issues)
	}

	writePlugin(`{ "openCollective": "javascript:alert(1)" }`)
	if r := check(); r.OK || !hasIssueMsg(r, "funding.openCollective") {
		t.Fatalf("expected unsupported funding.openCollective scheme to fail, issues=%v", r.Issues)
	}

	writePlugin(`{ "custom": ["javascript:alert(1)"] }`)
	if r := check(); r.OK || !hasIssueMsg(r, "funding.custom") {
		t.Fatalf("expected unsafe funding.custom scheme to fail, issues=%v", r.Issues)
	}

	writePlugin(`{ "custom": ["data:text/html,hi", "file:///tmp/x"] }`)
	if r := check(); r.OK || !hasIssueMsg(r, "funding.custom") {
		t.Fatalf("expected data:/file: funding.custom to fail, issues=%v", r.Issues)
	}

	writePlugin(`{ "custom": ["mailto:dev@example.com"] }`)
	if r := check(); !r.OK {
		t.Fatalf("expected mailto funding.custom to pass, issues=%v", r.Issues)
	}

	writePlugin(`{ "buyMeACoffee": "demo" }`)
	if r := check(); r.OK || !hasIssueMsg(r, "funding") {
		t.Fatalf("expected unknown funding key to fail, issues=%v", r.Issues)
	}

	writePlugin(`{ "custom": ["https://ld246.com/sponsor"] }`)
	if r := check(); r.OK || !hasIssueMsg(r, "ld246.com/sponsor") {
		t.Fatalf("expected placeholder funding.custom ld246.com/sponsor to fail, issues=%v", r.Issues)
	}
}

func TestCheckNameUnique(t *testing.T) {
	root := filepath.Join("testdata", "plugin_ok")
	occupied := map[string]struct{}{
		"sample-plugin": {},
	}
	r := Check(Input{
		PackageRoot:   root,
		OwnerRepo:     "demo/sample-plugin",
		Type:          TypePlugin,
		OccupiedNames: occupied,
	})
	if r.OK || !hasIssueMsg(r, "已被其他集市包占用") {
		t.Fatalf("expected manifest/name_unique, issues=%v", r.Issues)
	}

	// 已上架更新：有 OldName 时不查唯一性（名称本就在占用集合中）
	r2 := Check(Input{
		PackageRoot:   root,
		OwnerRepo:     "demo/sample-plugin",
		Type:          TypePlugin,
		OldName:       "sample-plugin",
		OldVersion:    "0.9.0",
		OccupiedNames: occupied,
	})
	if !r2.OK {
		t.Fatalf("update with OldName should skip uniqueness, issues=%v", r2.Issues)
	}

	// OccupiedNames 键约定为小写（与 LoadOccupiedNames 一致）；查找时对候选 name 做 ToLower
	dir := t.TempDir()
	copyTree(t, root, dir)
	content := `{
  "name": "Sample-Plugin",
  "author": "demo",
  "url": "https://github.com/demo/Sample-Plugin",
  "version": "1.0.0",
  "readme": { "default": "README.md" }
}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	r3 := Check(Input{
		PackageRoot: dir,
		OwnerRepo:   "demo/Sample-Plugin",
		Type:        TypePlugin,
		OccupiedNames: map[string]struct{}{
			"sample-plugin": {},
		},
	})
	if r3.OK || !hasIssueMsg(r3, "已被其他集市包占用") {
		t.Fatalf("expected case-insensitive unique clash, issues=%v", r3.Issues)
	}
}

func hasIssueMsg(r *Result, substr string) bool {
	return issuesContain(r.Issues, substr)
}

func issuesContain(issues []Issue, substr string) bool {
	for _, i := range issues {
		if strings.Contains(i.MessageZh, substr) || strings.Contains(i.MessageEn, substr) {
			return true
		}
	}
	return false
}

func containsPathSuffix(path, suffix string) bool {
	return len(path) >= len(suffix) && (path == suffix || filepath.ToSlash(path)[len(filepath.ToSlash(path))-len(filepath.ToSlash(suffix)):] == filepath.ToSlash(suffix))
}

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func writeMinimalPNGs(t *testing.T, dir string) {
	t.Helper()
	png := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D,
		0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xDE, 0x00, 0x00, 0x00,
		0x0C, 0x49, 0x44, 0x41, 0x54, 0x08, 0xD7, 0x63, 0xF8, 0xCF, 0xC0, 0x00,
		0x00, 0x00, 0x03, 0x00, 0x01, 0x00, 0x05, 0xFE, 0x02, 0xFE, 0xDC, 0xCC,
		0x59, 0xE7, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42,
		0x60, 0x82,
	}
	for _, name := range []string{"icon.png", "preview.png"} {
		if err := os.WriteFile(filepath.Join(dir, name), png, 0644); err != nil {
			t.Fatal(err)
		}
	}
}
