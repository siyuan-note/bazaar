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
	"bytes"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/google/go-github/v89/github"
)

func TestStaleCloseReason(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	age := 30 * 24 * time.Hour

	mkPR := func(created time.Time, draft bool, labels ...string) *github.PullRequest {
		pr := &github.PullRequest{
			Draft:     github.Ptr(draft),
			CreatedAt: &github.Timestamp{Time: created},
		}
		for _, name := range labels {
			n := name
			pr.Labels = append(pr.Labels, &github.Label{Name: &n})
		}
		return pr
	}

	tests := []struct {
		name string
		pr   *github.PullRequest
		want string
	}{
		{
			name: "ci-failed and old enough",
			pr:   mkPR(now.Add(-31*24*time.Hour), false, labelCIFailed),
			want: staleCloseReasonCIFailed,
		},
		{
			name: "ci-failed exactly at age boundary",
			pr:   mkPR(now.Add(-age), false, labelCIFailed),
			want: staleCloseReasonCIFailed,
		},
		{
			name: "ci-failed but too young",
			pr:   mkPR(now.Add(-29*24*time.Hour), false, labelCIFailed),
			want: "",
		},
		{
			name: "no matching labels",
			pr:   mkPR(now.Add(-40*24*time.Hour), false),
			want: "",
		},
		{
			name: "ci-passed alone not closed",
			pr:   mkPR(now.Add(-40*24*time.Hour), false, labelCIPassed),
			want: "",
		},
		{
			name: "manual-review alone not closed",
			pr:   mkPR(now.Add(-40*24*time.Hour), false, labelManualReview),
			want: "",
		},
		{
			name: "ci-passed and manual-review old enough",
			pr:   mkPR(now.Add(-31*24*time.Hour), false, labelCIPassed, labelManualReview),
			want: staleCloseReasonManualReview,
		},
		{
			name: "ci-passed and manual-review too young",
			pr:   mkPR(now.Add(-29*24*time.Hour), false, labelCIPassed, labelManualReview),
			want: "",
		},
		{
			name: "ci-skip blocks ci-failed close",
			pr:   mkPR(now.Add(-40*24*time.Hour), false, labelCIFailed, "ci-skip"),
			want: "",
		},
		{
			name: "ci-skip blocks manual-review close",
			pr:   mkPR(now.Add(-40*24*time.Hour), false, labelCIPassed, labelManualReview, "ci-skip"),
			want: "",
		},
		{
			name: "draft blocked",
			pr:   mkPR(now.Add(-40*24*time.Hour), true, labelCIFailed),
			want: "",
		},
		{
			name: "nil pr",
			pr:   nil,
			want: "",
		},
		{
			name: "zero created_at",
			pr:   &github.PullRequest{Draft: github.Ptr(false), Labels: []*github.Label{{Name: github.Ptr(labelCIFailed)}}},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := staleCloseReason(tt.pr, now, age); got != tt.want {
				t.Fatalf("staleCloseReason() = %q, want %q", got, tt.want)
			}
		})
	}

	if staleCloseReason(mkPR(now.Add(-40*24*time.Hour), false, labelCIFailed), now, 0) != "" {
		t.Fatal("age <= 0 should not close")
	}
}

func TestCloseStaleTemplate(t *testing.T) {
	tmpl, err := template.New("close-stale.md.tpl").Parse(closeStaleTemplateText)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("ci-failed", func(t *testing.T) {
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, closeStaleCommentData{Days: 30, Reason: staleCloseReasonCIFailed}); err != nil {
			t.Fatal(err)
		}
		out := buf.String()
		for _, want := range []string{
			"超过 30 天",
			"more than 30 days",
			"ci-failed",
			"打开一个新的拉取请求",
			"open a new pull request",
			"ci-skip",
			"<sub>",
		} {
			if !strings.Contains(out, want) {
				t.Fatalf("output missing %q\n%s", want, out)
			}
		}
		if strings.Contains(out, "manual-review") {
			t.Fatalf("ci-failed template should not mention manual-review:\n%s", out)
		}
		if strings.Contains(out, "@") {
			t.Fatalf("unexpected @ mention:\n%s", out)
		}
	})

	t.Run("manual-review", func(t *testing.T) {
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, closeStaleCommentData{Days: 30, Reason: staleCloseReasonManualReview}); err != nil {
			t.Fatal(err)
		}
		out := buf.String()
		for _, want := range []string{
			"超过 30 天",
			"more than 30 days",
			"ci-passed",
			"manual-review",
			"维护者的审核意见",
			"changes requested by maintainers",
			"打开一个新的拉取请求",
			"open a new pull request",
			"ci-skip",
		} {
			if !strings.Contains(out, want) {
				t.Fatalf("output missing %q\n%s", want, out)
			}
		}
		if strings.Contains(out, "`ci-failed`") {
			t.Fatalf("manual-review template should not use ci-failed reason wording:\n%s", out)
		}
		if strings.Contains(out, "@") {
			t.Fatalf("unexpected @ mention:\n%s", out)
		}
	})
}
