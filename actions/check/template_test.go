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

	"github.com/siyuan-note/bazaar/actions/util"
	"github.com/siyuan-note/bazaar/rules"
)

func TestCheckResultTemplate(t *testing.T) {
	tmpl, err := parseCheckResultTemplate()
	if err != nil {
		t.Fatal(err)
	}

	sample := CheckResult{
		Plugins: []PackageCheck{
			{
				RepoInfo: RepoInfo{Path: "siyuan-note/plugin-sample", Home: "https://github.com/siyuan-note/plugin-sample"},
				Release: util.LatestRelease{
					Tag: "v0.0.1", URL: "https://github.com/siyuan-note/plugin-sample/releases/tag/v0.0.1",
					PackageZipAssetID: 1,
				},
			},
			{
				RepoInfo: RepoInfo{Path: "example/broken-plugin", Home: "https://github.com/example/broken-plugin"},
				Issues: []rules.Issue{{
					MessageZh: "缺少 icon.png", MessageEn: "missing icon.png",
				}},
			},
			{
				RepoInfo: RepoInfo{Path: "example/no-release", Home: "https://github.com/example/no-release"},
				Issues: []rules.Issue{{
					MessageZh: "无 Latest Release", MessageEn: "no Latest Release",
				}},
			},
			{
				RepoInfo: RepoInfo{Path: "example/no-package-zip", Home: "https://github.com/example/no-package-zip"},
				Issues: []rules.Issue{{
					MessageZh: "无 package.zip", MessageEn: "no package.zip",
				}},
			},
		},
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, sample); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"无 Latest Release",
		"无 package.zip",
		"缺少 icon.png",
		"Check passed.",
		"Latest Release: [v0.0.1](https://github.com/siyuan-note/plugin-sample/releases/tag/v0.0.1)",
		"检测到以下问题，请在修复之后重新打包 `package.zip` 发布新的 Release，并将 Release 标记为 Latest。",
		"We found the following issues. Please fix them, rebuild `package.zip`, publish a new Release, and mark that Release as Latest.",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q\n%s", want, out)
		}
	}
	introIdx := strings.Index(out, "检测到以下问题")
	issueIdx := strings.Index(out, "[01]")
	if introIdx < 0 || issueIdx < 0 || introIdx > issueIdx {
		t.Fatalf("issue intro should appear before [01]\n%s", out)
	}
	passIdx := strings.Index(out, "Check passed.")
	releaseIdx := strings.Index(out, "Latest Release: [v0.0.1]")
	if passIdx < 0 || releaseIdx < 0 || releaseIdx > passIdx {
		t.Fatalf("Latest Release should appear before Check passed.\n%s", out)
	}
	if strings.Contains(out, "Release that must exist") {
		t.Fatal("old checkbox wording should be gone")
	}
}

func TestCheckResultTemplate_FlowError(t *testing.T) {
	tmpl, err := parseCheckResultTemplate()
	if err != nil {
		t.Fatal(err)
	}

	sample := CheckResult{
		FlowError: formatOnePackageLimitError(2, []typeCheckPlan{
			{packageType: rules.TypePlugin, diff: repoDiff{New: []string{"a/p1", "b/p2"}}},
		}),
		PluginsDeleted: []string{"x/old"},
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, sample); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"流程规则未通过",
		"Flow rule failed",
		"添加或更换了 2 个集市包",
		"`plugins.txt`: a/p1, b/p2",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q\n%s", want, out)
		}
	}
	if strings.Contains(out, "集市包列表无实际变更") {
		t.Fatalf("empty-change message should not appear when FlowError is set\n%s", out)
	}
	if strings.Contains(out, "Check passed.") {
		t.Fatalf("package checks should be skipped when FlowError is set\n%s", out)
	}
	// 一次一包失败时不展示移除列表（FlowError 文案里的「移除不限」除外）
	if strings.Contains(out, "### 移除") || strings.Contains(out, "x/old") {
		t.Fatalf("deleted repos should be hidden when FlowError is set\n%s", out)
	}
}

func TestCheckResultTemplate_RemoveBeforeAdd(t *testing.T) {
	tmpl, err := parseCheckResultTemplate()
	if err != nil {
		t.Fatal(err)
	}

	sample := CheckResult{
		Plugins: []PackageCheck{{
			RepoInfo: RepoInfo{Path: "alice/new-plugin", Home: "https://github.com/alice/new-plugin"},
		}},
		PluginsDeleted: []string{"old/plugin-x"},
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, sample); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "### 新增插件仓库 / Add Plugin Repo") {
		t.Fatalf("add heading missing\n%s", out)
	}
	if strings.Contains(out, "新增 `") || strings.Contains(out, "个插件仓库") {
		t.Fatalf("add heading should not include count\n%s", out)
	}
	rmIdx := strings.Index(out, "### 移除插件仓库")
	addIdx := strings.Index(out, "### 新增插件仓库")
	if rmIdx < 0 || addIdx < 0 || rmIdx > addIdx {
		t.Fatalf("remove section should appear before add section\n%s", out)
	}
}
