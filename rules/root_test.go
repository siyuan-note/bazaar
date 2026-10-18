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

func TestResolvePackageRootStripsSingleWrapper(t *testing.T) {
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, "wrapper"))
	mustWrite(t, filepath.Join(dir, "wrapper", "plugin.json"), "{}")

	root, err := ResolvePackageRoot(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filepath.Base(root) != "wrapper" {
		t.Fatalf("expected wrapper root, got %s", root)
	}
}

func TestResolvePackageRootKeepsFilesAtRoot(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "plugin.json"), "{}")
	mustMkdir(t, filepath.Join(dir, "i18n"))

	root, err := ResolvePackageRoot(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root != dir {
		t.Fatalf("expected extraction root, got %s", root)
	}
}

// 与思源内核一致：macOS 压缩残留也参与条目计数，因此这层包装目录不会被剥离，
// 同时要给出比「缺少必要文件」更明确的原因。
func TestResolvePackageRootRejectsMacOSResidue(t *testing.T) {
	for _, residue := range []string{".DS_Store", "__MACOSX"} {
		t.Run(residue, func(t *testing.T) {
			dir := t.TempDir()
			mustMkdir(t, filepath.Join(dir, "wrapper"))
			mustWrite(t, filepath.Join(dir, "wrapper", "plugin.json"), "{}")
			if residue == "__MACOSX" {
				mustMkdir(t, filepath.Join(dir, residue))
			} else {
				mustWrite(t, filepath.Join(dir, residue), "x")
			}

			root, err := ResolvePackageRoot(dir)
			if err == nil {
				t.Fatalf("expected error for %s residue, got root %s", residue, root)
			}
			zh, en := LocalizedMessages(err)
			if !strings.Contains(zh, residue) || !strings.Contains(zh, "wrapper") {
				t.Fatalf("unexpected zh message: %s", zh)
			}
			if !strings.Contains(en, residue) || !strings.Contains(en, "wrapper") {
				t.Fatalf("unexpected en message: %s", en)
			}
		})
	}
}

// 残留与普通文件同时存在时不构成「只多包一层文件夹」，按内核结果返回解压根。
func TestResolvePackageRootAllowsResidueWithRootFile(t *testing.T) {
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, "wrapper"))
	mustWrite(t, filepath.Join(dir, "README.md"), "# demo\n")
	mustWrite(t, filepath.Join(dir, ".DS_Store"), "x")

	root, err := ResolvePackageRoot(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root != dir {
		t.Fatalf("expected extraction root, got %s", root)
	}
}

func TestResolvePackageRootRejectsSiblingDirs(t *testing.T) {
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, "a"))
	mustMkdir(t, filepath.Join(dir, "b"))

	if _, err := ResolvePackageRoot(dir); err == nil {
		t.Fatal("expected error for sibling folders without files")
	}
}

func TestResolvePackageRootKeepsEmptyDir(t *testing.T) {
	dir := t.TempDir()

	root, err := ResolvePackageRoot(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root != dir {
		t.Fatalf("expected extraction root, got %s", root)
	}
}

// macOSArchiveResidue 只在「剔除残留后恰好一个子目录、且无普通文件」时才报告。
func TestMacOSArchiveResidue(t *testing.T) {
	mk := func(t *testing.T, names ...string) []os.DirEntry {
		t.Helper()
		dir := t.TempDir()
		for _, name := range names {
			if strings.HasSuffix(name, "/") {
				mustMkdir(t, filepath.Join(dir, strings.TrimSuffix(name, "/")))
				continue
			}
			mustWrite(t, filepath.Join(dir, name), "x")
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		return entries
	}

	for _, tc := range []struct {
		name        string
		entries     []string
		wantOK      bool
		wantWrapper string
	}{
		{"residue plus single wrapper", []string{".DS_Store", "wrapper/"}, true, "wrapper"},
		{"macosx plus single wrapper", []string{"__MACOSX/", "wrapper/"}, true, "wrapper"},
		{"no residue", []string{"wrapper/"}, false, ""},
		{"residue plus root file", []string{".DS_Store", "README.md", "wrapper/"}, false, ""},
		{"residue without wrapper", []string{".DS_Store"}, false, ""},
		{"residue plus sibling wrappers", []string{".DS_Store", "a/", "b/"}, false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			residue, wrapper, ok := macOSArchiveResidue(mk(t, tc.entries...))
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v (residue=%v wrapper=%q)", ok, tc.wantOK, residue, wrapper)
			}
			if ok && wrapper != tc.wantWrapper {
				t.Fatalf("wrapper = %q, want %q", wrapper, tc.wantWrapper)
			}
		})
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatal(err)
	}
}
