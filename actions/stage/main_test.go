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

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var stageJSONFiles = []string{
	"icons.json",
	"plugins.json",
	"templates.json",
	"themes.json",
	"widgets.json",
}

// bazaarRoot 从当前工作目录向上查找包含 stage/*.json 的仓库根目录。
func bazaarRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		probe := filepath.Join(dir, "stage", "plugins.json")
		if _, err := os.Stat(probe); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("bazaar root not found")
		}
		dir = parent
	}
}

// normalizeJSONText 统一换行符，便于与 sortJSONKeys 输出（LF）对比。
// 仓库中的 stage 文件在 Windows 检出后可能带 CRLF。
func normalizeJSONText(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

func TestSortJSONKeys_stageFiles(t *testing.T) {
	root := bazaarRoot(t)
	for _, name := range stageJSONFiles {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(root, "stage", name)
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}

			got, err := sortJSONKeys(original)
			if err != nil {
				t.Fatal(err)
			}

			want := normalizeJSONText(string(original))
			gotStr := string(got)
			if gotStr != want {
				t.Fatalf("sortJSONKeys output differs from stage file\nlength: got %d, want %d", len(gotStr), len(want))
			}
		})
	}
}

func TestSortJSONKeys_idempotent(t *testing.T) {
	root := bazaarRoot(t)
	path := filepath.Join(root, "stage", "icons.json")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	first, err := sortJSONKeys(original)
	if err != nil {
		t.Fatal(err)
	}
	second, err := sortJSONKeys(first)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("sortJSONKeys is not idempotent")
	}
}
