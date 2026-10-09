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
	"path/filepath"
	"strings"
	"testing"
)

// buildTree 在临时目录下按相对路径写入文件，路径以 "/" 结尾时创建目录。
func buildTree(t *testing.T, paths ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, rel := range paths {
		target := filepath.Join(root, filepath.FromSlash(strings.TrimSuffix(rel, "/")))
		if strings.HasSuffix(rel, "/") {
			mustMkdir(t, target)
			continue
		}
		mustMkdir(t, filepath.Dir(target))
		mustWrite(t, target, "x")
	}
	return root
}

func TestSyncIgnoredEntriesClean(t *testing.T) {
	root := buildTree(t,
		"plugin.json", "index.js", "README.md",
		"i18n/zh_CN.json",
		"node_modules/dep/package.json",
		"__MACOSX/whatever",
		".siyuan/inner.json",
		"storage/local.json",
		"storage/view-state.json",
		"draft.tmp.txt",
	)
	if issues := SyncIgnoredEntries(root, "/plugins/demo"); len(issues) != 0 {
		t.Fatalf("expected no issues, got %v", issues)
	}
}

func TestSyncIgnoredEntriesDotEntries(t *testing.T) {
	root := buildTree(t,
		"plugin.json",
		".git/config",
		".github/workflows/ci.yml",
		".gitignore",
		".env",
		".vscode/settings.json",
	)
	issues := SyncIgnoredEntries(root, "/plugins/demo")
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %v", issues)
	}
	if !strings.Contains(issues[0].MessageZh, "以 `.` 开头") || !strings.Contains(issues[0].MessageZh, ".gitignore") {
		t.Fatalf("unexpected zh message: %s", issues[0].MessageZh)
	}
	if !strings.Contains(issues[0].MessageEn, "start with `.`") || !strings.Contains(issues[0].MessageEn, ".gitignore") {
		t.Fatalf("unexpected en message: %s", issues[0].MessageEn)
	}
	// 点开头目录整棵剪枝，条目数量只按目录本身计：.git、.github、.vscode 各一个，加两个点开头的文件。
	if !strings.Contains(issues[0].MessageZh, "5 个") {
		t.Fatalf("expected 5 collapsed entries, got %s", issues[0].MessageZh)
	}
}

func TestSyncIgnoredEntriesTempFiles(t *testing.T) {
	root := buildTree(t, "plugin.json", "draft.tmp", "nested/cache.tmp")
	issues := SyncIgnoredEntries(root, "/plugins/demo")
	if len(issues) != 1 || !strings.Contains(issues[0].MessageZh, "draft.tmp") {
		t.Fatalf("expected tmp issue, got %v", issues)
	}
}

// 名为 filesys_status_check 的目录不检查：概率极低，且内核正在收窄为按实际路径判定。
// 参见 `syncignore.go` 中那段说明与 https://github.com/siyuan-note/siyuan/issues/20353
func TestSyncIgnoredEntriesStatusDirNotChecked(t *testing.T) {
	root := buildTree(t, "plugin.json", "filesys_status_check/status", "nested/filesys_status_check/status")
	if issues := SyncIgnoredEntries(root, "/plugins/demo"); len(issues) != 0 {
		t.Fatalf("expected status dir to be skipped, got %v", issues)
	}
}

func TestSyncIgnoredEntriesLegacyStorage(t *testing.T) {
	root := buildTree(t,
		"plugin.json",
		"data/storage/local.json",
		"fixtures/data/storage/ref-used.json",
	)
	issues := SyncIgnoredEntries(root, "/plugins/demo")
	if len(issues) != 1 || !strings.Contains(issues[0].MessageZh, "data/storage/local.json") {
		t.Fatalf("expected legacy storage issue, got %v", issues)
	}
}

// 包名为 data 时，包内 storage/local.json 拼接出的完整路径同样命中后缀规则。
func TestSyncIgnoredEntriesLegacyStorageUsesInstallPath(t *testing.T) {
	root := buildTree(t, "plugin.json", "storage/local.json")
	if issues := SyncIgnoredEntries(root, "/plugins/data"); len(issues) != 1 {
		t.Fatalf("expected legacy storage issue, got %v", issues)
	}
	if issues := SyncIgnoredEntries(root, "/plugins/demo"); len(issues) != 0 {
		t.Fatalf("expected no issues for a package not named data, got %v", issues)
	}
}

func TestSyncIgnoredEntriesMultipleKinds(t *testing.T) {
	root := buildTree(t,
		"plugin.json",
		".gitignore",
		"draft.tmp",
		"data/storage/local.json",
	)
	issues := SyncIgnoredEntries(root, "/plugins/demo")
	if len(issues) != 3 {
		t.Fatalf("expected 3 issues, got %v", issues)
	}
}

func TestSyncIgnoredEntriesEmptyRoot(t *testing.T) {
	if issues := SyncIgnoredEntries("", "/plugins/demo"); len(issues) != 0 {
		t.Fatalf("expected empty root to skip, got %v", issues)
	}
}

func TestStepSyncIgnoredUsesManifestName(t *testing.T) {
	root := buildTree(t, "plugin.json", "storage/local.json")
	c := &Context{Root: root, Type: TypePlugin}
	c.Package = Package{Name: "data"}
	stepSyncIgnored(c)
	if len(c.Issues) != 1 {
		t.Fatalf("expected 1 issue from install path, got %v", c.Issues)
	}
}

func TestStepSyncIgnoredSkippedWhenHalted(t *testing.T) {
	c := &Context{Root: buildTree(t, ".gitignore"), Type: TypePlugin}
	c.Halt()
	stepSyncIgnored(c)
	if len(c.Issues) != 0 {
		t.Fatalf("expected halted pipeline to skip, got %v", c.Issues)
	}
}

// 包目录名本身也会导致整个包不同步，检查必须覆盖清单 name，而不只是包内条目。
func TestCheckReportsSyncIgnoredEntries(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "README.md"), "# demo\n")
	mustWrite(t, filepath.Join(dir, "index.js"), "")
	mustWrite(t, filepath.Join(dir, ".gitignore"), "node_modules\n")
	mustWrite(t, filepath.Join(dir, "plugin.json"),
		`{"name":"demo","author":"demo","url":"https://github.com/demo/demo","version":"1.0.0","readme":{"default":"README.md"}}`)

	r := Check(Input{PackageRoot: dir, OwnerRepo: "demo/demo", Type: TypePlugin})
	if r.OK || !hasIssueMsg(r, ".gitignore") {
		t.Fatalf("expected sync-ignored entry rejection, OK=%v issues=%v", r.OK, r.Issues)
	}
}
