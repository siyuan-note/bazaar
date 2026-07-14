// SiYuan community bazaar.
// Copyright (c) 2021-present, b3log.org
//
// Bazaar is licensed under Mulan PSL v2.
// You can use this software according to the terms and conditions of the Mulan PSL v2.
// You may obtain a copy of Mulan PSL v2 at:
//         http://license.coscl.org.cn/MulanPSL2
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR PURPOSE.
// See the Mulan PSL v2 for more details.

package check

import (
	"os"
	"path/filepath"
	"testing"
)

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
	if r.Manifest == nil {
		t.Fatal("expected manifest")
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
	if !hasRule(r, "files/required") {
		t.Fatalf("expected files/required, issues=%v", r.Issues)
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
	if !hasRule(r, "names/whitespace") {
		t.Fatalf("expected names/whitespace, issues=%v", r.Issues)
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
	if !hasRule(r, "manifest/name") {
		t.Fatalf("expected manifest/name, issues=%v", r.Issues)
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
	if !hasRule(r, "manifest/version") {
		t.Fatalf("expected manifest/version, issues=%v", r.Issues)
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
	if r.OK || !hasRule(r, "manifest/unknown_field") {
		t.Fatalf("expected manifest/unknown_field, issues=%v", r.Issues)
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
	if r.OK || !hasRule(r, "manifest/url") {
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
	if r.OK || !hasRule(r, "files/required") {
		t.Fatalf("expected files/required for missing index.js, issues=%v", r.Issues)
	}
}

func TestCheckAuthorHTML(t *testing.T) {
	dir := t.TempDir()
	copyTree(t, filepath.Join("testdata", "plugin_ok"), dir)
	content := `{
  "name": "sample-plugin",
  "author": "demo<script>",
  "url": "https://github.com/demo/sample-plugin",
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
	if r.OK || !hasRule(r, "manifest/author") {
		t.Fatalf("expected manifest/author, issues=%v", r.Issues)
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
	if r.OK || !hasRule(r, "manifest/name") {
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
  "readme": { "default": "README.md" }
}`
	if err := os.WriteFile(filepath.Join(dir, "template.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	r := Check(Input{
		PackageRoot: dir,
		OwnerRepo:   "demo/sample-template",
		Type:        TypeTemplate,
	})
	if r.OK || !hasRule(r, "files/template_md") {
		t.Fatalf("expected files/template_md, issues=%v", r.Issues)
	}

	if err := os.WriteFile(filepath.Join(dir, "doc.md"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	r2 := Check(Input{
		PackageRoot: dir,
		OwnerRepo:   "demo/sample-template",
		Type:        TypeTemplate,
	})
	if !r2.OK {
		t.Fatalf("expected OK after adding doc.md, issues=%v", r2.Issues)
	}
}

func TestCheckFundingProtocol(t *testing.T) {
	dir := t.TempDir()
	copyTree(t, filepath.Join("testdata", "plugin_ok"), dir)
	content := `{
  "name": "sample-plugin",
  "author": "demo",
  "url": "https://github.com/demo/sample-plugin",
  "version": "1.0.0",
  "readme": { "default": "README.md" },
  "funding": { "custom": ["javascript:alert(1)"] }
}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	r := Check(Input{
		PackageRoot: dir,
		OwnerRepo:   "demo/sample-plugin",
		Type:        TypePlugin,
	})
	if r.OK || !hasRule(r, "manifest/funding") {
		t.Fatalf("expected manifest/funding, issues=%v", r.Issues)
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
	if r.OK || !hasRule(r, "manifest/name_unique") {
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

	// 大小写不敏感
	r3 := Check(Input{
		PackageRoot: root,
		OwnerRepo:   "demo/sample-plugin",
		Type:        TypePlugin,
		OccupiedNames: map[string]struct{}{
			"Sample-Plugin": {},
		},
	})
	if r3.OK || !hasRule(r3, "manifest/name_unique") {
		t.Fatalf("expected case-insensitive unique clash, issues=%v", r3.Issues)
	}
}

func TestSanitizeDisplayStrings(t *testing.T) {
	m := map[string]any{
		"displayName": map[string]any{"default": "<b>x</b>"},
		"description": map[string]any{"zh_CN": "a&b"},
	}
	SanitizeDisplayStrings(m)
	dn := m["displayName"].(map[string]any)["default"]
	if dn != "&lt;b&gt;x&lt;/b&gt;" {
		t.Fatalf("displayName not escaped: %v", dn)
	}
	desc := m["description"].(map[string]any)["zh_CN"]
	if desc != "a&amp;b" {
		t.Fatalf("description not escaped: %v", desc)
	}
}

func hasRule(r *Result, rule string) bool {
	for _, i := range r.Issues {
		if i.Rule == rule {
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
