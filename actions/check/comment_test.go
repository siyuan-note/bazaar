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
)

func TestIsActiveCheckResultComment(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{name: "new active", body: checkResultCommentMarker + "\n## check", want: true},
		{name: "legacy active", body: checkResultCommentTagLegacy + "\n## check", want: true},
		{name: "outdated", body: checkResultCommentMarkerOutdated + "\n## check", want: false},
		{name: "outdated wins over active text", body: checkResultCommentMarkerOutdated + "\n" + checkResultCommentMarker, want: false},
		{name: "unrelated", body: "hello", want: false},
		{name: "empty", body: "", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isActiveCheckResultComment(tt.body); got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMarkCheckResultCommentOutdated(t *testing.T) {
	t.Run("new marker", func(t *testing.T) {
		in := checkResultCommentMarker + "\nbody"
		got := markCheckResultCommentOutdated(in)
		if !strings.Contains(got, checkResultCommentMarkerOutdated) {
			t.Fatalf("missing outdated marker: %q", got)
		}
		if isActiveCheckResultComment(got) {
			t.Fatalf("should not stay active: %q", got)
		}
		if !strings.Contains(got, "body") {
			t.Fatal("body lost")
		}
	})
	t.Run("legacy marker", func(t *testing.T) {
		in := checkResultCommentTagLegacy + "\nbody"
		got := markCheckResultCommentOutdated(in)
		if isActiveCheckResultComment(got) {
			t.Fatalf("legacy should become outdated: %q", got)
		}
		if !strings.Contains(got, checkResultCommentMarkerOutdated) {
			t.Fatalf("want outdated marker: %q", got)
		}
	})
	t.Run("already outdated", func(t *testing.T) {
		in := checkResultCommentMarkerOutdated + "\nbody"
		if got := markCheckResultCommentOutdated(in); got != in {
			t.Fatalf("got %q", got)
		}
	})
}

func TestEnsureCheckResultCommentMarker(t *testing.T) {
	with := checkResultCommentMarker + "\n<!-- bazaar-check-meta\n{}\n-->\n## title"
	if got := ensureCheckResultCommentMarker(with); got != with {
		t.Fatalf("should keep existing marker\ngot: %q", got)
	}
	legacy := checkResultCommentTagLegacy + "\n## title"
	got := ensureCheckResultCommentMarker(legacy)
	if !strings.HasPrefix(strings.TrimSpace(got), checkResultCommentMarker) {
		t.Fatalf("want new marker prefix: %q", got)
	}
	if strings.Contains(got, checkResultCommentTagLegacy) {
		t.Fatalf("legacy tag should be stripped: %q", got)
	}
	if !strings.Contains(got, "## title") {
		t.Fatal("title lost")
	}
}

func TestPickCheckMetaPrefersActive(t *testing.T) {
	// 纯函数级：模拟 loadCheckMeta 的优先规则（active meta 优于任意 meta）
	type row struct {
		body string
	}
	comments := []row{
		{body: checkResultCommentMarkerOutdated + "\n" + checkMetaCommentStart + "\n" +
			`{"v":1,"checked_at":"2026-07-01T00:00:00Z","result_hash":"old","unchanged_streak":1,"next_due_at":"2026-07-01T01:00:00Z"}` + "\n-->"},
		{body: checkResultCommentMarker + "\n" + checkMetaCommentStart + "\n" +
			`{"v":1,"checked_at":"2026-07-20T00:00:00Z","result_hash":"new","unchanged_streak":0,"next_due_at":"2026-07-20T00:20:00Z"}` + "\n-->"},
	}
	var latestActive, latestAny *CheckMeta
	for _, c := range comments {
		meta, ok := parseCheckMetaFromComment(c.body)
		if !ok {
			t.Fatalf("parse failed: %s", c.body)
		}
		latestAny = meta
		if isActiveCheckResultComment(c.body) {
			latestActive = meta
		}
	}
	if latestActive == nil || latestActive.ResultHash != "new" {
		t.Fatalf("active meta = %+v", latestActive)
	}
	if latestAny == nil || latestAny.ResultHash != "new" {
		t.Fatalf("any meta = %+v", latestAny)
	}
	// 若只有归档评，应能回退到 old
	onlyOutdated := comments[:1]
	latestActive, latestAny = nil, nil
	for _, c := range onlyOutdated {
		meta, ok := parseCheckMetaFromComment(c.body)
		if !ok {
			t.Fatal("parse")
		}
		latestAny = meta
		if isActiveCheckResultComment(c.body) {
			latestActive = meta
		}
	}
	if latestActive != nil {
		t.Fatal("expected no active")
	}
	if latestAny.ResultHash != "old" {
		t.Fatalf("fallback = %+v", latestAny)
	}
}
