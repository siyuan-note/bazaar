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

	"github.com/siyuan-note/bazaar/actions/util"
	"github.com/siyuan-note/bazaar/rules"
)

func TestParseStageFailBodyMarker(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
		ok   bool
	}{
		{
			name: "标准 marker",
			body: "<!-- bazaar-stage-fail {\"repo\":\"foo/bar\"} -->\n### [foo/bar](https://github.com/foo/bar)\n",
			want: "foo/bar",
			ok:   true,
		},
		{
			name: "JSON 前后有空格",
			body: "<!-- bazaar-stage-fail  {\"repo\":\"foo/bar\"}  -->\n",
			want: "foo/bar",
			ok:   true,
		},
		{
			name: "无 marker",
			body: "random comment",
		},
		{
			name: "marker 不完整",
			body: "<!-- bazaar-stage-fail {\"repo\":\"foo/bar\"}",
		},
		{
			name: "非法 JSON",
			body: "<!-- bazaar-stage-fail {repo:foo/bar} -->",
		},
		{
			name: "缺少斜杠",
			body: "<!-- bazaar-stage-fail {\"repo\":\"foobar\"} -->",
		},
		{
			name: "旧格式不识别",
			body: "<!-- bazaar-stage-fail:foo/bar -->",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseStageFailBodyMarker(tt.body)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("parseStageFailBodyMarker() = (%q, %v), want (%q, %v)", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestFormatStageFailIssueBody(t *testing.T) {
	body, err := formatStageFailIssueBody(stageReport{
		OwnerRepo:   "owner/repo",
		PackageType: rules.TypePlugin,
		Kind:        stageReportFail,
		Release: util.LatestRelease{
			Tag: "v1.2.3",
			URL: "https://github.com/owner/repo/releases/tag/v1.2.3",
		},
		Hash: "abc123",
		Issues: []rules.Issue{{
			MessageZh: "缺少 icon.png",
			MessageEn: "missing icon.png",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	checks := []string{
		`<!-- bazaar-stage-fail {"repo":"owner/repo"} -->`,
		"[owner/repo](https://github.com/owner/repo)",
		"(`plugin`)",
		"[v1.2.3](https://github.com/owner/repo/releases/tag/v1.2.3)",
		"hash `abc123`",
		"[01]",
		"缺少 icon.png",
		"missing icon.png",
		"提升清单字段 `version`",
	}
	for _, want := range checks {
		if !strings.Contains(body, want) {
			t.Fatalf("formatStageFailIssueBody missing %q\nbody:\n%s", want, body)
		}
	}
	introIdx := strings.Index(body, "检测到以下问题")
	issueIdx := strings.Index(body, "[01]")
	if introIdx < 0 || issueIdx < 0 || introIdx > issueIdx {
		t.Fatalf("issue intro should appear before [01]\n%s", body)
	}
	sepIdx := strings.Index(body[introIdx:issueIdx], "---")
	if sepIdx < 0 {
		t.Fatalf("want --- between intro and [01]\n%s", body)
	}
}

func TestFormatStageIssueIndex(t *testing.T) {
	if got := formatStageIssueIndex(0, 1); got != "01" {
		t.Fatalf("got %q, want 01", got)
	}
	if got := formatStageIssueIndex(8, 12); got != "09" {
		t.Fatalf("got %q, want 09", got)
	}
}

func TestStageFailIssueTitle(t *testing.T) {
	if got := stageFailIssueTitle(rules.TypePlugin, "owner/repo"); got != "Plugin update failed: owner/repo" {
		t.Fatalf("got %q", got)
	}
	if got := stageFailIssueTitle(rules.TypeWidget, "a/b"); got != "Widget update failed: a/b" {
		t.Fatalf("got %q", got)
	}
}

func TestStageFailBodyMarkerRoundTrip(t *testing.T) {
	marker := stageFailBodyMarker("a/b")
	got, ok := parseStageFailBodyMarker(marker + "\nrest")
	if !ok || got != "a/b" {
		t.Fatalf("round-trip failed: got (%q, %v), marker=%q", got, ok, marker)
	}
}

func TestStageFailIssueContentEqual(t *testing.T) {
	base := "<!-- bazaar-stage-fail {\"repo\":\"a/b\"} -->\n### [a/b](https://github.com/a/b) (`plugin`)\n\n[01]\n\n缺 icon\n\nmissing icon\n\n---"
	withRunA := base + "\n\n工作流 / Workflow: https://github.com/siyuan-note/bazaar/actions/runs/1"
	withRunB := base + "\n\n工作流 / Workflow: https://github.com/siyuan-note/bazaar/actions/runs/2"
	changed := base + "\n\nextra"

	if !stageFailIssueContentEqual(withRunA, withRunB) {
		t.Fatal("want equal when only workflow URL differs")
	}
	if !stageFailIssueContentEqual(withRunA, base) {
		t.Fatal("want equal when one side has no workflow footer")
	}
	if !stageFailIssueContentEqual(base+"\n\n", base) {
		t.Fatal("want equal after trimming trailing newlines")
	}
	if stageFailIssueContentEqual(base, changed) {
		t.Fatal("want unequal when issue body differs")
	}
}
