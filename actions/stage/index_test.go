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
	"testing"

	"github.com/siyuan-note/bazaar/actions/util"
	"github.com/siyuan-note/bazaar/rules"
)

func TestResolveStageCheckLegacy(t *testing.T) {
	alice := &util.StageRepo{
		URL: "alice/transfer@abc",
		Package: rules.Package{
			Name:    "transfer-pkg",
			Version: "1.2.3",
		},
	}
	samePath := &util.StageRepo{
		URL: "bob/keep@def",
		Package: rules.Package{
			Name:    "keep-pkg",
			Version: "0.1.0",
		},
	}
	oldStageData := map[string]*util.StageRepo{
		"alice/transfer": alice,
		"bob/keep":       samePath,
	}
	oldByRepoName := indexOldStageByRepoName(oldStageData)

	t.Run("同路径更新", func(t *testing.T) {
		listed := Set{"bob/keep": {}}
		exact, name, ver := resolveStageCheckLegacy("bob/keep", oldStageData, oldByRepoName, listed)
		if exact != samePath {
			t.Fatalf("exactOld = %v, want samePath", exact)
		}
		if name != "keep-pkg" || ver != "0.1.0" {
			t.Fatalf("oldName/version = %q/%q, want keep-pkg/0.1.0", name, ver)
		}
	})

	t.Run("换维护者继承 OldName 与 OldVersion", func(t *testing.T) {
		listed := Set{"bob/transfer": {}}
		exact, name, ver := resolveStageCheckLegacy("bob/transfer", oldStageData, oldByRepoName, listed)
		if exact != nil {
			t.Fatalf("exactOld = %v, want nil for maintainer change", exact)
		}
		if name != "transfer-pkg" {
			t.Fatalf("oldName = %q, want transfer-pkg", name)
		}
		if ver != "1.2.3" {
			t.Fatalf("oldVersion = %q, want 1.2.3", ver)
		}
	})

	t.Run("旧路径仍在列表则不按换维护者处理", func(t *testing.T) {
		listed := Set{"alice/transfer": {}, "bob/transfer": {}}
		exact, name, ver := resolveStageCheckLegacy("bob/transfer", oldStageData, oldByRepoName, listed)
		if exact != nil || name != "" || ver != "" {
			t.Fatalf("got exact=%v name=%q ver=%q, want all empty", exact, name, ver)
		}
	})

	t.Run("纯新包", func(t *testing.T) {
		listed := Set{"carol/new": {}}
		exact, name, ver := resolveStageCheckLegacy("carol/new", oldStageData, oldByRepoName, listed)
		if exact != nil || name != "" || ver != "" {
			t.Fatalf("got exact=%v name=%q ver=%q, want all empty", exact, name, ver)
		}
	})
}

func TestSameCommitPackageZipChanged(t *testing.T) {
	old := &util.StageRepo{PackageZipAssetID: 42}

	tests := []struct {
		name    string
		old     *util.StageRepo
		assetID int64
		want    bool
	}{
		{
			name:    "无旧条目",
			old:     nil,
			assetID: 99,
		},
		{
			name:    "旧条目无 asset id",
			old:     &util.StageRepo{},
			assetID: 99,
		},
		{
			name:    "asset id 未变化",
			old:     old,
			assetID: 42,
		},
		{
			name:    "asset id 变化",
			old:     old,
			assetID: 99,
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sameCommitPackageZipChanged(tt.old, tt.assetID); got != tt.want {
				t.Fatalf("sameCommitPackageZipChanged() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseHashFromStageURL(t *testing.T) {
	tests := []struct {
		name     string
		stageURL string
		want     string
	}{
		{
			name:     "标准格式",
			stageURL: "owner/repo@abc123def",
			want:     "abc123def",
		},
		{
			name:     "无 @ 分隔符",
			stageURL: "owner/repo",
			want:     "",
		},
		{
			name:     "@ 位于末尾",
			stageURL: "owner/repo@",
			want:     "",
		},
		{
			name:     "空字符串",
			stageURL: "",
			want:     "",
		},
		{
			name:     "仅 @",
			stageURL: "@",
			want:     "",
		},
		{
			name:     "@ 位于开头",
			stageURL: "@hashonly",
			want:     "hashonly",
		},
		{
			name:     "多个 @ 取第一个之后",
			stageURL: "owner/repo@hash@extra",
			want:     "hash@extra",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseHashFromStageURL(tt.stageURL); got != tt.want {
				t.Fatalf("parseHashFromStageURL(%q) = %q, want %q", tt.stageURL, got, tt.want)
			}
		})
	}
}

func TestBackfillUnprocessedStageRepos(t *testing.T) {
	processed := &util.StageRepo{URL: "a/one@hash1", Updated: "2026-01-01T00:00:00Z"}
	oldTwo := &util.StageRepo{URL: "b/two@hash2", Updated: "2026-01-02T00:00:00Z"}
	oldStageData := map[string]*util.StageRepo{
		"b/two":   oldTwo,
		"c/three": {URL: "c/three@hash3"},
	}
	repos := []string{"a/one", "b/two", "c/three", "d/four"}
	got := backfillUnprocessedStageRepos(repos, []*util.StageRepo{processed}, oldStageData)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3 (processed + b/two + c/three)", len(got))
	}
	if got[0] != processed || got[1] != oldTwo {
		t.Fatalf("unexpected order/content: %#v", got)
	}
	if got[2].URL != "c/three@hash3" {
		t.Fatalf("third = %q, want c/three@hash3", got[2].URL)
	}
}
