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

func TestShouldCloseStaleCIFailed(t *testing.T) {
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
		want bool
	}{
		{
			name: "ci-failed and old enough",
			pr:   mkPR(now.Add(-31*24*time.Hour), false, labelCIFailed),
			want: true,
		},
		{
			name: "ci-failed exactly at age boundary",
			pr:   mkPR(now.Add(-age), false, labelCIFailed),
			want: true,
		},
		{
			name: "ci-failed but too young",
			pr:   mkPR(now.Add(-29*24*time.Hour), false, labelCIFailed),
			want: false,
		},
		{
			name: "no ci-failed label",
			pr:   mkPR(now.Add(-40*24*time.Hour), false),
			want: false,
		},
		{
			name: "ci-skip blocks close",
			pr:   mkPR(now.Add(-40*24*time.Hour), false, labelCIFailed, "ci-skip"),
			want: false,
		},
		{
			name: "draft blocked",
			pr:   mkPR(now.Add(-40*24*time.Hour), true, labelCIFailed),
			want: false,
		},
		{
			name: "nil pr",
			pr:   nil,
			want: false,
		},
		{
			name: "zero created_at",
			pr:   &github.PullRequest{Draft: github.Ptr(false), Labels: []*github.Label{{Name: github.Ptr(labelCIFailed)}}},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldCloseStaleCIFailed(tt.pr, now, age); got != tt.want {
				t.Fatalf("shouldCloseStaleCIFailed() = %v, want %v", got, tt.want)
			}
		})
	}

	if shouldCloseStaleCIFailed(mkPR(now.Add(-40*24*time.Hour), false, labelCIFailed), now, 0) {
		t.Fatal("age <= 0 should not close")
	}
}

func TestCloseStaleTemplate(t *testing.T) {
	tmpl, err := template.New("close-stale.md.tpl").Parse(closeStaleTemplateText)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("with author", func(t *testing.T) {
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, closeStaleCommentData{PRAuthor: "demo-author", Days: 30}); err != nil {
			t.Fatal(err)
		}
		out := buf.String()
		for _, want := range []string{
			"@demo-author",
			"超过 30 天",
			"more than 30 days",
			"ci-failed",
			"打开一个新的拉取请求",
			"open a new pull request",
			"ci-skip",
			"<small>",
		} {
			if !strings.Contains(out, want) {
				t.Fatalf("output missing %q\n%s", want, out)
			}
		}
	})

	t.Run("without author", func(t *testing.T) {
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, closeStaleCommentData{Days: 30}); err != nil {
			t.Fatal(err)
		}
		out := buf.String()
		if strings.Contains(out, "@") {
			t.Fatalf("unexpected @ mention:\n%s", out)
		}
		if !strings.Contains(out, "超过 30 天") {
			t.Fatalf("output missing days text:\n%s", out)
		}
	})
}
