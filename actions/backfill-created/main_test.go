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
	"time"
)

func TestClassifyStageURL(t *testing.T) {
	tests := []struct {
		value      string
		want       string
		wantHashed bool
		ok         bool
	}{
		// 2021-08 .. 2026-07：40 位 git commit sha
		{value: "whyt-byte/Blue_Topaz@915029d8e825002285b4ce10205141dfb6623a88", want: "whyt-byte/Blue_Topaz", wantHashed: true, ok: true},
		// 2026-08 起：package.zip sha256 前 7 位
		{value: "siyuan-note/icon-sample@b0e22b9", want: "siyuan-note/icon-sample", wantHashed: true, ok: true},
		// 2022-11 前后：无 hash
		{value: "Hi-Windom/winsay", want: "Hi-Windom/winsay", ok: true},
		// package.url：https 链接与空串
		{value: "https://github.com/owner/repo", ok: false},
		{value: "", ok: false},
		// 不含 owner/repo 形态
		{value: "owner", ok: false},
	}
	for _, tt := range tests {
		got, hashed, ok := classifyStageURL(tt.value)
		if ok != tt.ok || hashed != tt.wantHashed || got != tt.want {
			t.Fatalf("classifyStageURL(%q) = (%q, %v, %v), want (%q, %v, %v)",
				tt.value, got, hashed, ok, tt.want, tt.wantHashed, tt.ok)
		}
	}
}

func TestClassifyHunk(t *testing.T) {
	tests := []struct {
		name string
		hunk []string
		want []string
	}{
		{
			name: "带 hash 的条目 url 直接采信",
			hunk: []string{`      "repoRef": "v1.0.0",`, `      "url": "alice/repo@abc1234",`},
			want: []string{"alice/repo"},
		},
		{
			name: "新建条目时接受无 hash 的 url",
			hunk: []string{`    {`, `      "url": "Hi-Windom/winsay",`, `      "updated": "2022-11-14T08:38:28Z",`},
			want: []string{"Hi-Windom/winsay"},
		},
		{
			// 2026-03-02 缩进修正前：顶层 url 缩进 4 格，package.url 缩进 2 格
			name: "旧缩进（4 格）的条目 url 同样可识别",
			hunk: []string{`  {`, `    "url": "royc01/notion-theme@dffaec7",`, `    "updated": "2021-08-26T16:37:39Z"`},
			want: []string{"royc01/notion-theme"},
		},
		{
			// 只改 package.url（清单里自己声明成裸 owner/repo）时 hunk 内没有 updated
			name: "无 hash 且同 hunk 无 updated 视为 package.url",
			hunk: []string{`        "url": "jeasionr-ui/openlist",`},
			want: nil,
		},
		{
			name: "package.url 的 https 链接始终忽略",
			hunk: []string{`        "url": "https://github.com/alice/repo",`, `      "updated": "2025-01-01T00:00:00Z",`},
			want: nil,
		},
		{
			// 空串 package.url 也出现在历史里
			name: "空 url 忽略",
			hunk: []string{`        "url": "",`, `      "updated": "2025-01-01T00:00:00Z",`},
			want: nil,
		},
		{
			// package 对象整块新增时 hunk 内有 package.url，但没有 updated
			name: "package 对象新增不产生条目",
			hunk: []string{`      "package": {`, `  "url": "https://github.com/alice/repo",`, `  "version": "1.0.0"`},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyHunk(tt.hunk)
			if len(got) != len(tt.want) {
				t.Fatalf("classifyHunk() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("classifyHunk() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

// 同一仓库反复挪位（updated 变化导致整体移除再新增）时，取最小时间；带时区偏移的提交时间统一转 UTC。
func TestScanHistory(t *testing.T) {
	const stream = "\x012023-05-05T12:20:32+08:00\n" + `
diff --git a/stage/plugins.json b/stage/plugins.json
index 300b7a3c4..9cd24c1e4 100644
--- a/stage/plugins.json
+++ b/stage/plugins.json
@@ -2 +2,9 @@
-  "repos": null
+  "repos": [
+    {
+      "url": "siyuan-note/plugin-sample@b7563549",
+      "updated": "2023-05-05T04:18:59Z"
+    }
+  ]
` + "\x012024-01-02T03:04:05Z\n" + `
diff --git a/stage/plugins.json b/stage/plugins.json
index 9cd24c1e4..a205201d4 100644
--- a/stage/plugins.json
+++ b/stage/plugins.json
@@ -1,6 +1,10 @@
+    {
+      "url": "alice/new@def",
+      "updated": "2024-01-02T03:00:00Z"
+    },
+    {
+      "url": "siyuan-note/plugin-sample@c0ffee01",
+      "updated": "2024-01-02T03:00:00Z"
+    },
+    {
+      "url": "bare/entry",
+      "updated": "2024-01-02T03:00:00Z"
+    },
@@ -40 +44 @@
+        "url": "package/url",
+++ not a url line
` + "\x012022-12-31T00:00:00Z\n" + `
diff --git a/stage/plugins.json b/stage/plugins.json
--- a/stage/plugins.json
+++ b/stage/plugins.json
@@ -1,1 +1,1 @@
+      "url": "bob/late@h",
`

	got := map[string]time.Time{}
	err := scanHistory(strings.NewReader(stream), func(ownerRepo string, at time.Time) {
		if prev, ok := got[ownerRepo]; !ok || at.Before(prev) {
			got[ownerRepo] = at
		}
	})
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		// 首见 2023-05-05T12:20:32+08:00 → UTC 04:20:32；后续 2024 年挪位不得覆盖
		"siyuan-note/plugin-sample": "2023-05-05T04:20:32Z",
		"alice/new":                 "2024-01-02T03:04:05Z",
		"bare/entry":                "2024-01-02T03:04:05Z",
		"bob/late":                  "2022-12-31T00:00:00Z",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d repos, want %d: %#v", len(got), len(want), got)
	}
	for repo, wantAt := range want {
		if at, ok := got[repo]; !ok {
			t.Fatalf("missing %q in %#v", repo, got)
		} else if v := at.Format(time.RFC3339); v != wantAt {
			t.Fatalf("%s = %s, want %s", repo, v, wantAt)
		}
	}
	if _, ok := got["package/url"]; ok {
		t.Fatal("bare package.url outside an entry-creation hunk must be ignored")
	}
}

func TestEarliestRFC3339(t *testing.T) {
	derived := time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		existing string
		want     string
	}{
		{name: "空值取历史", existing: "", want: "2024-05-01T00:00:00Z"},
		{name: "已有更早值保持", existing: "2023-01-01T00:00:00Z", want: "2023-01-01T00:00:00Z"},
		{name: "已有更晚值被修正", existing: "2026-01-01T00:00:00Z", want: "2024-05-01T00:00:00Z"},
		{name: "非法值取历史", existing: "not-a-time", want: "2024-05-01T00:00:00Z"},
		{
			name:     "带时区偏移的旧值归一化为 UTC",
			existing: "2023-01-01T08:00:00+08:00",
			want:     "2023-01-01T00:00:00Z",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := earliestRFC3339(tt.existing, derived); got != tt.want {
				t.Fatalf("earliestRFC3339(%q) = %q, want %q", tt.existing, got, tt.want)
			}
		})
	}
}
