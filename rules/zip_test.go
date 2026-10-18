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
	"archive/zip"
	"bytes"
	"io/fs"
	"strings"
	"testing"
)

func TestZipPathsOK(t *testing.T) {
	data := mustZipBytes(t, map[string]string{
		"plugin.json":  "{}",
		"i18n/zh.json": "{}",
	})
	if issues := ZipPaths(data); len(issues) != 0 {
		t.Fatalf("expected no issues, got %v", issues)
	}
}

func TestZipPathsBackslash(t *testing.T) {
	data := mustZipBytes(t, map[string]string{
		`i18n\zh.json`: "{}",
		"plugin.json":  "{}",
	})
	issues := ZipPaths(data)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %v", issues)
	}
	if !strings.Contains(issues[0].MessageZh, `反斜杠`) || !strings.Contains(issues[0].MessageZh, `i18n\zh.json`) {
		t.Fatalf("unexpected zh message: %s", issues[0].MessageZh)
	}
	if !strings.Contains(issues[0].MessageEn, `backslash`) || !strings.Contains(issues[0].MessageEn, `i18n\zh.json`) {
		t.Fatalf("unexpected en message: %s", issues[0].MessageEn)
	}
}

func TestZipPathsEmptySkipped(t *testing.T) {
	if issues := ZipPaths(nil); len(issues) != 0 {
		t.Fatalf("expected nil/empty zip data to skip, got %v", issues)
	}
}

func TestStepZipPathsSkippedWithoutData(t *testing.T) {
	c := &Context{
		PackageRoot: "testdata/plugin_ok",
		OwnerRepo:   "demo/sample-plugin",
		Type:        TypePlugin,
	}
	stepZipPaths(c)
	if len(c.Issues) != 0 {
		t.Fatalf("expected skip without ZipData, got %v", c.Issues)
	}
}

func TestStepZipPathsReportsBackslash(t *testing.T) {
	c := &Context{
		ZipData: mustZipBytes(t, map[string]string{
			`foo\bar.txt`: "x",
		}),
	}
	stepZipPaths(c)
	if len(c.Issues) != 1 || !strings.Contains(c.Issues[0].MessageZh, `foo\bar.txt`) {
		t.Fatalf("expected backslash issue, got %v", c.Issues)
	}
}

func mustZipBytes(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range files {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

func TestZipEntryTypesOK(t *testing.T) {
	data := mustEntryTypeZipBytes(t, []zipProbeEntry{
		{name: "plugin.json", data: "{}"},
		{name: "i18n/", mode: fs.ModeDir | 0755},
		{name: "i18n/zh.json", data: "{}"},
	})
	if issues := ZipEntryTypes(data); len(issues) != 0 {
		t.Fatalf("expected no issues, got %v", issues)
	}
}

func TestZipEntryTypesSymlink(t *testing.T) {
	data := mustEntryTypeZipBytes(t, []zipProbeEntry{
		{name: "plugin.json", data: "{}"},
		{name: "link.js", mode: fs.ModeSymlink | 0777, data: "../shared/link.js"},
	})
	issues := ZipEntryTypes(data)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %v", issues)
	}
	if !strings.Contains(issues[0].MessageZh, "符号链接") || !strings.Contains(issues[0].MessageZh, "link.js") {
		t.Fatalf("unexpected zh message: %s", issues[0].MessageZh)
	}
	if !strings.Contains(issues[0].MessageEn, "symbolic link") || !strings.Contains(issues[0].MessageEn, "link.js") {
		t.Fatalf("unexpected en message: %s", issues[0].MessageEn)
	}
}

func TestZipEntryTypesNonRegular(t *testing.T) {
	data := mustEntryTypeZipBytes(t, []zipProbeEntry{
		{name: "pipe", mode: fs.ModeNamedPipe | 0644},
	})
	issues := ZipEntryTypes(data)
	if len(issues) != 1 || !strings.Contains(issues[0].MessageZh, "非常规条目") {
		t.Fatalf("expected non-regular entry issue, got %v", issues)
	}
}

func TestZipEntryTypesLimitsExamples(t *testing.T) {
	var entries []zipProbeEntry
	for _, name := range []string{"a", "b", "c", "d", "e", "f", "g"} {
		entries = append(entries, zipProbeEntry{name: name + ".js", mode: fs.ModeSymlink | 0777, data: "t"})
	}
	issues := ZipEntryTypes(mustEntryTypeZipBytes(t, entries))
	if len(issues) != 1 || !strings.Contains(issues[0].MessageZh, "另有 2 条未列出") {
		t.Fatalf("expected truncated example list, got %v", issues)
	}
}

func TestZipEntryTypesEmptySkipped(t *testing.T) {
	if issues := ZipEntryTypes(nil); len(issues) != 0 {
		t.Fatalf("expected nil/empty zip data to skip, got %v", issues)
	}
}

func TestZipEntryTypesInvalidZipSkipped(t *testing.T) {
	// 解析失败由 ZipPaths 报告，本函数不再重复。
	if issues := ZipEntryTypes([]byte("not a zip")); len(issues) != 0 {
		t.Fatalf("expected invalid zip to be skipped, got %v", issues)
	}
}

func TestStepZipEntryTypesSkippedWithoutData(t *testing.T) {
	c := &Context{
		PackageRoot: "testdata/plugin_ok",
		OwnerRepo:   "demo/sample-plugin",
		Type:        TypePlugin,
	}
	stepZipEntryTypes(c)
	if len(c.Issues) != 0 {
		t.Fatalf("expected skip without ZipData, got %v", c.Issues)
	}
}

func TestStepZipEntryTypesReportsSymlink(t *testing.T) {
	c := &Context{ZipData: mustEntryTypeZipBytes(t, []zipProbeEntry{
		{name: "link.js", mode: fs.ModeSymlink | 0777, data: "target.js"},
	})}
	stepZipEntryTypes(c)
	if len(c.Issues) != 1 || !strings.Contains(c.Issues[0].MessageZh, "link.js") {
		t.Fatalf("expected symlink issue, got %v", c.Issues)
	}
}

// zipProbeEntry 描述构造带文件模式的测试 zip 时的一个条目；mode 为 0 时按普通文件处理。
type zipProbeEntry struct {
	name string
	mode fs.FileMode
	data string
}

// mustEntryTypeZipBytes 构造带 Unix 文件模式的 zip。CreatorVersion 高字节设为 3（Unix），
// archive/zip 才会从外部属性解析出文件模式。
func mustEntryTypeZipBytes(t *testing.T, entries []zipProbeEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, e := range entries {
		mode := e.mode
		if mode == 0 {
			mode = 0644
		}
		name := e.name
		unixMode := uint32(mode.Perm())
		switch {
		case mode&fs.ModeDir != 0:
			unixMode |= 0x4000
			name = strings.TrimSuffix(name, "/") + "/"
		case mode&fs.ModeSymlink != 0:
			unixMode |= 0xa000
		case mode&fs.ModeNamedPipe != 0:
			unixMode |= 0x1000
		case mode&fs.ModeSocket != 0:
			unixMode |= 0xc000
		case mode&fs.ModeDevice != 0:
			unixMode |= 0x6000
		default:
			unixMode |= 0x8000
		}
		fh := &zip.FileHeader{Name: name, Method: zip.Deflate}
		fh.CreatorVersion = 3 << 8
		fh.ExternalAttrs = unixMode << 16
		fw, err := w.CreateHeader(fh)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := fw.Write([]byte(e.data)); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}
