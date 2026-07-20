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

func TestFormatByteSize(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{0, "0B"},
		{512, "512B"},
		{1024, "1.0KB"},
		{20 * 1024, "20.0KB"},
		{200 * 1024, "200.0KB"},
		{20*1024 + 1, "20.0KB"},
		{25000, "24.4KB"},
		{31232, "30.5KB"},  // 30.5 * 1024
		{2202009, "2.1MB"}, // ≈ 2.1 * 1024 * 1024
	}
	for _, tc := range cases {
		if got := formatByteSize(tc.n); got != tc.want {
			t.Fatalf("formatByteSize(%d)=%q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestRequiredFilesIconSizeLimit(t *testing.T) {
	dir := t.TempDir()
	writeMinimalPNGs(t, dir)
	mustWrite(t, filepath.Join(dir, "README.md"), "# demo\n")
	mustWrite(t, filepath.Join(dir, "plugin.json"), `{"name":"demo"}`)
	mustWrite(t, filepath.Join(dir, "index.js"), "")

	// 刚好 20 KB：通过
	mustWriteBytes(t, filepath.Join(dir, "icon.png"), make([]byte, maxIconPNGBytes))
	if issues := RequiredFiles(dir, TypePlugin); issuesContain(issues, "超过上限") {
		t.Fatalf("icon at exact limit should pass, issues=%v", issues)
	}

	// 超过 1 字节：失败
	mustWriteBytes(t, filepath.Join(dir, "icon.png"), make([]byte, maxIconPNGBytes+1))
	issues := RequiredFiles(dir, TypePlugin)
	if !issuesContain(issues, "`icon.png`") || !issuesContain(issues, "超过上限") {
		t.Fatalf("expected icon size issue, issues=%v", issues)
	}
	if !issuesContain(issues, "20.0KB") {
		t.Fatalf("expected limit wording, issues=%v", issues)
	}
}

func TestRequiredFilesPreviewSizeLimit(t *testing.T) {
	dir := t.TempDir()
	writeMinimalPNGs(t, dir)
	mustWrite(t, filepath.Join(dir, "README.md"), "# demo\n")
	mustWrite(t, filepath.Join(dir, "plugin.json"), `{"name":"demo"}`)
	mustWrite(t, filepath.Join(dir, "index.js"), "")

	mustWriteBytes(t, filepath.Join(dir, "preview.png"), make([]byte, maxPreviewPNGBytes))
	if issues := RequiredFiles(dir, TypePlugin); issuesContain(issues, "超过上限") {
		t.Fatalf("preview at exact limit should pass, issues=%v", issues)
	}

	mustWriteBytes(t, filepath.Join(dir, "preview.png"), make([]byte, maxPreviewPNGBytes+1))
	issues := RequiredFiles(dir, TypePlugin)
	if !issuesContain(issues, "`preview.png`") || !issuesContain(issues, "超过上限") {
		t.Fatalf("expected preview size issue, issues=%v", issues)
	}
	if !issuesContain(issues, "200.0KB") {
		t.Fatalf("expected limit wording, issues=%v", issues)
	}
}

func TestCheckRejectsOversizedIcon(t *testing.T) {
	dir := t.TempDir()
	copyTree(t, filepath.Join("testdata", "plugin_ok"), dir)
	mustWriteBytes(t, filepath.Join(dir, "icon.png"), make([]byte, maxIconPNGBytes+1))

	r := Check(Input{
		PackageRoot: dir,
		OwnerRepo:   "demo/sample-plugin",
		Type:        TypePlugin,
	})
	if r.OK || !hasIssueMsg(r, "超过上限") || !hasIssueMsg(r, "icon.png") {
		t.Fatalf("expected oversized icon failure, OK=%v issues=%v", r.OK, r.Issues)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	mustWriteBytes(t, path, []byte(content))
}

func mustWriteBytes(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestRequiredFilesSizeMessagesBilingual(t *testing.T) {
	dir := t.TempDir()
	writeMinimalPNGs(t, dir)
	mustWrite(t, filepath.Join(dir, "README.md"), "# demo\n")
	mustWrite(t, filepath.Join(dir, "plugin.json"), `{"name":"demo"}`)
	mustWrite(t, filepath.Join(dir, "index.js"), "")
	mustWriteBytes(t, filepath.Join(dir, "preview.png"), make([]byte, maxPreviewPNGBytes+512))

	issues := RequiredFiles(dir, TypePlugin)
	var found bool
	for _, iss := range issues {
		if strings.Contains(iss.MessageZh, "preview.png") && strings.Contains(iss.MessageEn, "preview.png") {
			found = true
			if !strings.Contains(iss.MessageEn, "exceeds the limit") {
				t.Fatalf("english message missing exceeds wording: %q", iss.MessageEn)
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected bilingual preview size issue, issues=%v", issues)
	}
}
