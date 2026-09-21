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
	"testing"
)

func TestNormalizePackageImagesSupportedFormats(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
		data     []byte
		mimeType string
	}{
		{name: "png", fileName: "icon.png", data: minimalPNGBytes(), mimeType: "image/png"},
		{name: "jpg", fileName: "icon.jpg", data: minimalJPEGBytes(), mimeType: "image/jpeg"},
		{name: "jpeg", fileName: "icon.jpeg", data: minimalJPEGBytes(), mimeType: "image/jpeg"},
		{name: "webp", fileName: "icon.webp", data: minimalWebPBytes(), mimeType: "image/webp"},
		{name: "uppercase webp extension", fileName: "icon.WEBP", data: minimalWebPBytes(), mimeType: "image/webp"},
		{name: "avif", fileName: "icon.avif", data: minimalAVIFBytes(), mimeType: "image/avif"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, test.fileName), test.data, 0644); err != nil {
				t.Fatal(err)
			}
			pkg := &Package{}
			issues := normalizePackageImages(map[string]any{"icon": test.fileName}, pkg, dir)
			if len(issues) != 0 {
				t.Fatalf("expected supported image, issues=%v", issues)
			}
			if pkg.Icon == nil || *pkg.Icon != test.fileName {
				t.Fatalf("normalized icon = %#v, want %q", pkg.Icon, test.fileName)
			}
			if pkg.Preview == nil || *pkg.Preview != "" {
				t.Fatalf("normalized preview = %#v, want explicit empty", pkg.Preview)
			}
			if got := ImageMIMEType(test.fileName); got != test.mimeType {
				t.Fatalf("ImageMIMEType(%q) = %q, want %q", test.fileName, got, test.mimeType)
			}
		})
	}
}

func TestCheckUsesDeclaredPackageImages(t *testing.T) {
	dir := t.TempDir()
	copyTree(t, filepath.Join("testdata", "plugin_ok"), dir)
	if err := os.Remove(filepath.Join(dir, "icon.png")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "brand.webp"), minimalWebPBytes(), 0644); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, "plugin.json"), `{
  "name": "sample-plugin",
  "author": "demo",
  "url": "https://github.com/demo/sample-plugin",
  "version": "1.0.0",
  "readme": { "default": "README.md" },
  "icon": "brand.webp"
}`)

	result := Check(Input{PackageRoot: dir, OwnerRepo: "demo/sample-plugin", Type: TypePlugin})
	if !result.OK {
		t.Fatalf("declared package image should pass, issues=%v", result.Issues)
	}
	if result.Package.Icon == nil || *result.Package.Icon != "brand.webp" {
		t.Fatalf("normalized icon = %#v, want brand.webp", result.Package.Icon)
	}
}

func TestNormalizePackageImagesLegacyFallbackAndOptional(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "icon.png"), minimalPNGBytes(), 0644); err != nil {
		t.Fatal(err)
	}
	pkg := &Package{}
	issues := normalizePackageImages(map[string]any{}, pkg, dir)
	if len(issues) != 0 {
		t.Fatalf("legacy fallback should pass, issues=%v", issues)
	}
	if pkg.Icon == nil || *pkg.Icon != "icon.png" {
		t.Fatalf("legacy icon = %#v, want icon.png", pkg.Icon)
	}
	if pkg.Preview == nil || *pkg.Preview != "" {
		t.Fatalf("missing preview = %#v, want explicit empty", pkg.Preview)
	}

	emptyDir := t.TempDir()
	emptyPkg := &Package{}
	if emptyIssues := normalizePackageImages(map[string]any{}, emptyPkg, emptyDir); len(emptyIssues) != 0 {
		t.Fatalf("package without images should pass, issues=%v", emptyIssues)
	}
	if emptyPkg.Icon == nil || *emptyPkg.Icon != "" || emptyPkg.Preview == nil || *emptyPkg.Preview != "" {
		t.Fatalf("missing images must normalize to explicit empty values: icon=%#v preview=%#v", emptyPkg.Icon, emptyPkg.Preview)
	}
}

func TestNormalizePackageImagesRejectsInvalidDeclarations(t *testing.T) {
	tests := []struct {
		name     string
		manifest map[string]any
		files    map[string][]byte
		want     string
	}{
		{name: "wrong field type", manifest: map[string]any{"icon": 1}, want: "必须是字符串"},
		{name: "empty filename", manifest: map[string]any{"icon": ""}, want: "非空文件名"},
		{name: "nested path", manifest: map[string]any{"icon": "images/icon.png"}, want: "包根文件名"},
		{name: "missing declared file", manifest: map[string]any{"icon": "icon.webp"}, want: "找不到"},
		{name: "svg", manifest: map[string]any{"icon": "icon.svg"}, files: map[string][]byte{"icon.svg": []byte("<svg></svg>")}, want: "不支持 SVG"},
		{name: "extension mismatch", manifest: map[string]any{"icon": "icon.webp"}, files: map[string][]byte{"icon.webp": minimalPNGBytes()}, want: "扩展名对应"},
		{name: "invalid contents", manifest: map[string]any{"icon": "icon.png"}, files: map[string][]byte{"icon.png": []byte("not an image")}, want: "未知格式"},
		{name: "case-sensitive file", manifest: map[string]any{"icon": "Icon.png"}, files: map[string][]byte{"icon.png": minimalPNGBytes()}, want: "找不到"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, data := range test.files {
				if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
					t.Fatal(err)
				}
			}
			pkg := &Package{}
			issues := normalizePackageImages(test.manifest, pkg, dir)
			if !issuesContain(issues, test.want) {
				t.Fatalf("expected issue containing %q, got %v", test.want, issues)
			}
		})
	}
}

func TestNormalizePackageImagesSizeLimits(t *testing.T) {
	tests := []struct {
		name      string
		field     string
		fileName  string
		limit     int
		wantIssue bool
	}{
		{name: "icon exact", field: "icon", fileName: "icon.png", limit: maxIconImageBytes},
		{name: "icon over", field: "icon", fileName: "icon.png", limit: maxIconImageBytes + 1, wantIssue: true},
		{name: "preview exact", field: "preview", fileName: "preview.png", limit: maxPreviewImageBytes},
		{name: "preview over", field: "preview", fileName: "preview.png", limit: maxPreviewImageBytes + 1, wantIssue: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			data := append(minimalPNGBytes(), make([]byte, test.limit-len(minimalPNGBytes()))...)
			if err := os.WriteFile(filepath.Join(dir, test.fileName), data, 0644); err != nil {
				t.Fatal(err)
			}
			issues := normalizePackageImages(map[string]any{test.field: test.fileName}, &Package{}, dir)
			gotIssue := issuesContain(issues, "超过上限")
			if gotIssue != test.wantIssue {
				t.Fatalf("size issues=%v, wantIssue=%v", issues, test.wantIssue)
			}
		})
	}
}

func minimalJPEGBytes() []byte {
	return []byte{0xFF, 0xD8, 0xFF, 0xDB, 0x00, 0x04, 0x00, 0x00, 0xFF, 0xD9}
}

func minimalWebPBytes() []byte {
	return []byte{'R', 'I', 'F', 'F', 0x04, 0x00, 0x00, 0x00, 'W', 'E', 'B', 'P'}
}

func minimalAVIFBytes() []byte {
	return []byte{
		0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', 'm', 'i', 'f', '1', 0x00, 0x00, 0x00, 0x00,
		'a', 'v', 'i', 'f', 'm', 'i', 'f', '1',
	}
}
