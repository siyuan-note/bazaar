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
	"strings"
	"testing"

	"github.com/siyuan-note/bazaar/rules"
)

func TestComputeRepoDiff_AddRemoveAndMaintainerChange(t *testing.T) {
	basePaths := []string{"alice/old", "alice/keep", "alice/transfer"}
	headPaths := []string{"alice/keep", "bob/transfer", "carol/new"}
	baseSet := Set{"alice/old": {}, "alice/keep": {}, "alice/transfer": {}}
	headSet := Set{"alice/keep": {}, "bob/transfer": {}, "carol/new": {}}
	bazaarHead := Set{"alice/old": {}, "alice/keep": {}, "alice/transfer": {}}
	baseNameToOwner := map[string]string{
		"old":      "alice",
		"keep":     "alice",
		"transfer": "alice",
	}

	d := computeRepoDiff(headPaths, basePaths, baseSet, headSet, bazaarHead, baseNameToOwner)

	if len(d.New) != 2 {
		t.Fatalf("New = %v, want 2 entries", d.New)
	}
	wantNew := map[string]bool{"bob/transfer": true, "carol/new": true}
	for _, p := range d.New {
		if !wantNew[p] {
			t.Fatalf("unexpected New entry %q in %v", p, d.New)
		}
	}
	// 换维护者时旧 owner/repo 会计入 Deleted；纯移除亦然
	wantDeleted := map[string]bool{"alice/old": true, "alice/transfer": true}
	if len(d.Deleted) != 2 {
		t.Fatalf("Deleted = %v, want 2 entries", d.Deleted)
	}
	for _, p := range d.Deleted {
		if !wantDeleted[p] {
			t.Fatalf("unexpected Deleted entry %q in %v", p, d.Deleted)
		}
	}
	if _, ok := d.MaintainerChanged["bob/transfer"]; !ok {
		t.Fatalf("MaintainerChanged missing bob/transfer: %v", d.MaintainerChanged)
	}
	if _, ok := d.MaintainerChanged["carol/new"]; ok {
		t.Fatalf("carol/new should not be maintainer change: %v", d.MaintainerChanged)
	}
}

func TestComputeRepoDiff_FilterAlreadyOnBazaarHead(t *testing.T) {
	// 解决冲突时从 bazaar head 合并进来的仓库，不应算作本 PR 新增
	headPaths := []string{"other/merged"}
	basePaths := []string{}
	baseSet := Set{}
	headSet := Set{"other/merged": {}}
	bazaarHead := Set{"other/merged": {}}

	d := computeRepoDiff(headPaths, basePaths, baseSet, headSet, bazaarHead, nil)
	if len(d.New) != 0 {
		t.Fatalf("New = %v, want empty", d.New)
	}
}

func TestFormatOnePackageLimitError(t *testing.T) {
	plans := []typeCheckPlan{
		{packageType: rules.TypePlugin, diff: repoDiff{New: []string{"a/p1", "b/p2"}}},
		{packageType: rules.TypeTheme, diff: repoDiff{New: []string{"c/t1"}}},
		{packageType: rules.TypeIcon},
	}
	out := formatOnePackageLimitError(3, plans)
	for _, want := range []string{
		"添加或更换了 3 个集市包",
		"adds or changes 3 bazaar packages",
		"`plugins.txt`: a/p1, b/p2",
		"`themes.txt`: c/t1",
		"拆成独立的 Pull Request",
		"split each package into its own Pull Request",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q\n%s", want, out)
		}
	}
}
