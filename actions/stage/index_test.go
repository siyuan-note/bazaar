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

func TestPackageRootUploadFilesUsesDeclaredImagesAndMIME(t *testing.T) {
	icon := "brand.webp"
	preview := "cover.avif"
	pkg := &rules.Package{
		Readme:  rules.LocaleStrings{"default": "README.md", "zh-CN": "README_zh_CN.md"},
		Icon:    &icon,
		Preview: &preview,
	}
	files := packageRootUploadFiles(pkg, "plugin.json")
	want := map[string]string{
		"README.md":       "",
		"README_zh_CN.md": "",
		"plugin.json":     "",
		"brand.webp":      "image/webp",
		"cover.avif":      "image/avif",
	}
	if len(files) != len(want) {
		t.Fatalf("files = %#v, want %#v", files, want)
	}
	for name, contentType := range want {
		if files[name] != contentType {
			t.Fatalf("files[%q] = %q, want %q", name, files[name], contentType)
		}
	}
	if _, exists := files["icon.png"]; exists {
		t.Fatal("unselected legacy icon must not be uploaded")
	}
}

func TestPackageRootUploadFilesSkipsExplicitlyMissingImages(t *testing.T) {
	empty := ""
	files := packageRootUploadFiles(&rules.Package{Icon: &empty, Preview: &empty}, "theme.json")
	if len(files) != 2 || files["README.md"] != "" || files["theme.json"] != "" {
		t.Fatalf("unexpected files for package without images: %#v", files)
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

func TestCloneStageRepoWithRepoRef(t *testing.T) {
	original := &util.StageRepo{URL: "owner/repo@hash", RepoRef: "v1.0.0"}
	updated := cloneStageRepoWithRepoRef(original, "v1.0.1")
	if updated == original {
		t.Fatal("changed repoRef should return a clone")
	}
	if updated.RepoRef != "v1.0.1" || original.RepoRef != "v1.0.0" {
		t.Fatalf("unexpected refs: updated=%q original=%q", updated.RepoRef, original.RepoRef)
	}
	if got := cloneStageRepoWithRepoRef(original, "v1.0.0"); got != original {
		t.Fatal("unchanged repoRef should reuse the original entry")
	}
	if got := cloneStageRepoWithRepoRef(original, ""); got != original {
		t.Fatal("empty new repoRef should reuse the original entry")
	}
}


