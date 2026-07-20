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
