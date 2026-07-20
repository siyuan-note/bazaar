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
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/siyuan-note/bazaar/actions/util"
	"github.com/siyuan-note/bazaar/rules"
)

func TestParseCheckMetaFromComment(t *testing.T) {
	body := `<!-- thollander/actions-comment-pull-request "check-result" -->
<!-- bazaar-check-meta
{"v":1,"checked_at":"2026-07-20T14:00:00Z","result_hash":"abcd","unchanged_streak":2,"next_due_at":"2026-07-20T15:20:00Z","fp":{"repo":"a/b","release_id":1,"tag":"v1","zip_id":2,"zip_updated_at":"2026-07-20T13:00:00Z"}}
-->
## 拉取请求自动化检查`

	meta, ok := parseCheckMetaFromComment(body)
	if !ok {
		t.Fatal("expected meta")
	}
	if meta.V != 1 || meta.ResultHash != "abcd" || meta.UnchangedStreak != 2 {
		t.Fatalf("unexpected meta: %+v", meta)
	}
	if meta.FP == nil || meta.FP.Repo != "a/b" || meta.FP.ReleaseID != 1 || meta.FP.ZipID != 2 {
		t.Fatalf("unexpected fp: %+v", meta.FP)
	}
}

func TestBackoffInterval(t *testing.T) {
	if got := backoffInterval(0); got != 20*time.Minute {
		t.Fatalf("streak0 = %v", got)
	}
	if got := backoffInterval(3); got != 80*time.Minute {
		t.Fatalf("streak3 = %v", got)
	}
	if got := backoffInterval(100); got != 8*time.Hour {
		t.Fatalf("streak100 = %v", got)
	}
}

func TestComputeResultHashIgnoresIssueOrder(t *testing.T) {
	a := &CheckResult{
		Widgets: []PackageCheck{{
			RepoInfo: RepoInfo{Path: "o/w"},
			Release:  util.LatestRelease{ID: 1, Tag: "1.0.0", PackageZipAssetID: 2},
			Issues: []rules.Issue{
				{MessageZh: "字段 backends", MessageEn: "field backends"},
				{MessageZh: "字段 frontends", MessageEn: "field frontends"},
			},
		}},
	}
	b := &CheckResult{
		Widgets: []PackageCheck{{
			RepoInfo: RepoInfo{Path: "o/w"},
			Release:  util.LatestRelease{ID: 1, Tag: "1.0.0", PackageZipAssetID: 2},
			Issues: []rules.Issue{
				{MessageZh: "字段 frontends", MessageEn: "field frontends"},
				{MessageZh: "字段 backends", MessageEn: "field backends"},
			},
		}},
	}
	if ha, hb := computeResultHash(a), computeResultHash(b); ha != hb {
		t.Fatalf("hash should ignore issue order: %s vs %s", ha, hb)
	}
}

func TestBuildNextCheckMetaStreak(t *testing.T) {
	now := time.Date(2026, 7, 20, 14, 0, 0, 0, time.UTC)
	result := &CheckResult{
		Plugins: []PackageCheck{{
			RepoInfo: RepoInfo{Path: "o/r"},
			Release:  util.LatestRelease{ID: 9, Tag: "v1", PackageZipAssetID: 8},
			Issues:   []rules.Issue{{MessageZh: "x", MessageEn: "y"}},
		}},
	}
	hash := computeResultHash(result)
	prev := &CheckMeta{
		ResultHash:      hash,
		UnchangedStreak: 1,
		FP:              &CheckFingerprint{Repo: "o/r", ReleaseID: 9, Tag: "v1", ZipID: 8},
	}
	next := buildNextCheckMeta(prev, result, now)
	if next.UnchangedStreak != 2 {
		t.Fatalf("streak = %d, want 2", next.UnchangedStreak)
	}
	if next.NextDueAt != now.Add(60*time.Minute).Format(time.RFC3339) {
		t.Fatalf("next_due = %s", next.NextDueAt)
	}
	if next.FP == nil || next.FP.Repo != "o/r" {
		t.Fatalf("fp = %+v", next.FP)
	}

	changed := &CheckResult{
		Plugins: []PackageCheck{{
			RepoInfo: RepoInfo{Path: "o/r"},
			Issues:   []rules.Issue{{MessageZh: "other", MessageEn: "other"}},
		}},
	}
	reset := buildNextCheckMeta(prev, changed, now)
	if reset.UnchangedStreak != 0 {
		t.Fatalf("reset streak = %d", reset.UnchangedStreak)
	}
}

func TestShouldScheduleRecheck(t *testing.T) {
	now := time.Date(2026, 7, 20, 14, 0, 0, 0, time.UTC)
	meta := &CheckMeta{
		CheckedAt:  now.Add(-30 * time.Minute).Format(time.RFC3339),
		NextDueAt:  now.Add(30 * time.Minute).Format(time.RFC3339),
		FP:         &CheckFingerprint{Repo: "o/r", ReleaseID: 1, Tag: "v1", ZipID: 2},
		ResultHash: "h",
	}
	same := &CheckFingerprint{Repo: "o/r", ReleaseID: 1, Tag: "v1", ZipID: 2}
	if reason, ok := shouldScheduleRecheck(meta, same, nil, now, false); ok {
		t.Fatalf("expected skip, got %s", reason)
	}
	changed := &CheckFingerprint{Repo: "o/r", ReleaseID: 1, Tag: "v2", ZipID: 3}
	if reason, ok := shouldScheduleRecheck(meta, changed, nil, now, false); !ok || reason != "fp-changed" {
		t.Fatalf("fp-changed: %s %v", reason, ok)
	}
	if reason, ok := shouldScheduleRecheck(nil, nil, nil, now, false); !ok || reason != "no-meta" {
		t.Fatalf("no-meta: %s %v", reason, ok)
	}
	if reason, ok := shouldScheduleRecheck(meta, same, errors.New("boom"), now, false); !ok || reason != "probe-error" {
		t.Fatalf("probe-error: %s %v", reason, ok)
	}
	dueMeta := *meta
	dueMeta.NextDueAt = now.Add(-time.Minute).Format(time.RFC3339)
	if reason, ok := shouldScheduleRecheck(&dueMeta, same, nil, now, false); !ok || reason != "backoff-due" {
		t.Fatalf("backoff-due: %s %v", reason, ok)
	}
	oldMeta := *meta
	oldMeta.CheckedAt = now.Add(-25 * time.Hour).Format(time.RFC3339)
	oldMeta.NextDueAt = now.Add(time.Hour).Format(time.RFC3339)
	if reason, ok := shouldScheduleRecheck(&oldMeta, same, nil, now, false); !ok || reason != "max-age" {
		t.Fatalf("max-age: %s %v", reason, ok)
	}
}

func TestCmpSelectCandidate(t *testing.T) {
	now := time.Now()
	a := selectCandidate{fpChanged: true, streak: 5, checkedAt: now, entry: selectMatrixEntry{Number: 2}}
	b := selectCandidate{fpChanged: false, streak: 0, checkedAt: now.Add(-time.Hour), entry: selectMatrixEntry{Number: 1}}
	if cmpSelectCandidate(a, b) >= 0 {
		t.Fatal("fpChanged should come first")
	}
	c := selectCandidate{streak: 1, checkedAt: now.Add(-2 * time.Hour), entry: selectMatrixEntry{Number: 3}}
	d := selectCandidate{streak: 2, checkedAt: now.Add(-3 * time.Hour), entry: selectMatrixEntry{Number: 4}}
	if cmpSelectCandidate(c, d) >= 0 {
		t.Fatal("lower streak should come first")
	}
}

func TestMarshalCheckMetaRoundTrip(t *testing.T) {
	m := &CheckMeta{
		V: 1, CheckedAt: "2026-07-20T14:00:00Z", ResultHash: "ab",
		UnchangedStreak: 0, NextDueAt: "2026-07-20T14:20:00Z",
		FP: &CheckFingerprint{Repo: "o/r", ReleaseID: 1, Tag: "v1", ZipID: 2},
	}
	raw, err := marshalCheckMetaJSON(m)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, "\n") {
		t.Fatalf("want compact JSON, got %q", raw)
	}
	body := checkMetaCommentStart + "\n" + raw + "\n-->"
	got, ok := parseCheckMetaFromComment(body)
	if !ok || got.ResultHash != "ab" || got.FP.Repo != "o/r" {
		t.Fatalf("roundtrip: ok=%v meta=%+v", ok, got)
	}
}
