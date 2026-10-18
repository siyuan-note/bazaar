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
	"context"
	"net/url"
	"strings"
	"testing"

	"github.com/siyuan-note/bazaar/actions/util"
	"github.com/siyuan-note/bazaar/rules"
)

func TestCheckResultTemplate(t *testing.T) {
	tmpl, err := parseCheckResultTemplate(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	sample := CheckResult{
		MetaJSON: `{"v":1,"checked_at":"2026-07-20T14:00:00Z","result_hash":"deadbeef","unchanged_streak":0,"next_due_at":"2026-07-20T14:20:00Z"}`,
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
		"<!-- bazaar-check-result -->",
		"<!-- bazaar-check-meta",
		`"result_hash":"deadbeef"`,
		"无 Latest Release",
		"无 package.zip",
		"缺少 icon.png",
		"Check passed.",
		"Latest Release: [v0.0.1](https://github.com/siyuan-note/plugin-sample/releases/tag/v0.0.1)",
		"检测到以下问题，请在修复之后重新打包 `package.zip` 发布新的 Release，并将 Release 标记为 Latest。",
		"We found the following issues. Please fix them, rebuild `package.zip`, publish a new Release, and mark that Release as Latest.",
		// 安装入口只在检查通过且取到 Release 的仓库下给出，一行小字号，带上仓库路径、tag 与语种
		"<sub>[在思源中试用本次 Release](https://siyuan-note.github.io/bazaar/install/?repo=siyuan-note/plugin-sample&tag=v0.0.1&lang=zh-CN)",
		"[Try this Release in SiYuan](https://siyuan-note.github.io/bazaar/install/?repo=siyuan-note/plugin-sample&tag=v0.0.1&lang=en)</sub>",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q\n%s", want, out)
		}
	}
	// 评论里不出现裸协议链接（GitHub 会剔掉），且入口只给通过的那一个仓库
	if strings.Contains(out, "siyuan://") {
		t.Fatalf("check-result comment should not carry a raw siyuan:// link\n%s", out)
	}
	if got := strings.Count(out, "siyuan-note.github.io/bazaar/install/"); got != 2 {
		t.Fatalf("install entry should appear twice (zh + en), only for the passing repo, got %d\n%s", got, out)
	}
	if got := strings.Count(out, "<sub>"); got != 1 {
		t.Fatalf("install entry should stay on one line, got %d sub blocks\n%s", got, out)
	}
	if strings.Contains(out, "@") {
		t.Fatalf("check-result comment should not @ anyone\n%s", out)
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
	tmpl, err := parseCheckResultTemplate(context.Background())
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
	// 流程失败时不展示下架列表
	if strings.Contains(out, "### 下架") || strings.Contains(out, "x/old") {
		t.Fatalf("deleted repos should be hidden when FlowError is set\n%s", out)
	}
}

func TestCheckResultTemplate_DeprecationRegistry(t *testing.T) {
	tmpl, err := parseCheckResultTemplate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	sample := CheckResult{Deprecation: &DeprecationCheck{
		Types: []rules.PackageType{rules.TypePlugin},
		Changes: []DeprecationChange{{
			PackageType: rules.TypePlugin,
			OwnerRepo:   "old/plugin",
			Action:      deprecationActionAdd,
		}},
	}}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, sample); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	normalized := strings.ReplaceAll(out, "\r\n", "\n")
	for _, want := range []string{
		"弃用注册表 / Deprecation Registry",
		"### 弃用注册表 / Deprecation Registry\n\n- [`old/plugin`](https://github.com/old/plugin)（plugins）：新增 / Add",
		"自动校验通过",
		"maintainers still need to review",
	} {
		if !strings.Contains(normalized, want) {
			t.Fatalf("output missing %q\n%s", want, out)
		}
	}
	if strings.Contains(out, "无实际变更") {
		t.Fatalf("no-change message must not appear for a deprecation change\n%s", out)
	}
}

func TestCheckResultTemplate_DeprecationRegistryIssues(t *testing.T) {
	tmpl, err := parseCheckResultTemplate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	sample := CheckResult{Deprecation: &DeprecationCheck{
		Issues: []rules.Issue{{MessageZh: "替代包无效", MessageEn: "Invalid alternative"}},
	}}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, sample); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"替代包无效", "Invalid alternative", "[01]"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q\n%s", want, out)
		}
	}
	if strings.Contains(out, "自动校验通过") {
		t.Fatalf("pass message must not appear when issues exist\n%s", out)
	}
}

func TestCheckResultTemplate_BlacklistFlowError(t *testing.T) {
	tmpl, err := parseCheckResultTemplate(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// 与 main 一致：黑名单命中时不填充包检查 / 仅可能残留删除列表（模板在 FlowError 下隐藏下架）
	sample := CheckResult{
		FlowError:      formatBlacklistFlowError(),
		PluginsDeleted: []string{"x/old"},
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, sample); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"流程规则未通过",
		"本 PR 修改了不允许直接改动的文件",
		"This PR modifies files that must not be changed directly",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q\n%s", want, out)
		}
	}
	if strings.Contains(out, "集市包列表无实际变更") {
		t.Fatalf("empty-change message should not appear when FlowError is set\n%s", out)
	}
	if strings.Contains(out, "Check passed.") || strings.Contains(out, "### 新增") {
		t.Fatalf("package checks should be skipped when blacklist FlowError is set\n%s", out)
	}
	if strings.Contains(out, "### 下架") || strings.Contains(out, "x/old") {
		t.Fatalf("deleted repos should be hidden when FlowError is set\n%s", out)
	}
}

func TestCheckResultTemplate_MaintainerChangeNotice(t *testing.T) {
	sha, err := gitRevParseHEAD(context.Background(), BAZAAR_HEAD_PATH)
	if err != nil {
		t.Fatal(err)
	}
	tmpl, err := parseCheckResultTemplate(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	sample := CheckResult{
		Themes: []PackageCheck{{
			RepoInfo:          RepoInfo{Path: "bob/theme", Home: "https://github.com/bob/theme"},
			MaintainerChanged: true,
			Release: util.LatestRelease{
				Tag: "v1.0.0", URL: "https://github.com/bob/theme/releases/tag/v1.0.0",
				PackageZipAssetID: 1,
			},
		}},
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, sample); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"(更换维护者 / Change Maintainer)",
		"检测到更换维护者",
		"blob/" + sha + "/README.zh-CN.md#更换维护者",
		"This PR changes the package maintainer",
		"blob/" + sha + "/README.md#changing-maintainers",
		"Latest Release: [v1.0.0]",
		"Check passed.",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q\n%s", want, out)
		}
	}
	if strings.Contains(out, "blob/main/") {
		t.Fatalf("doc links should use commit hash, not main\n%s", out)
	}
	noticeIdx := strings.Index(out, "检测到更换维护者")
	releaseIdx := strings.Index(out, "Latest Release: [v1.0.0]")
	if noticeIdx < 0 || releaseIdx < 0 || noticeIdx > releaseIdx {
		t.Fatalf("maintainer-change notice should appear before Latest Release\n%s", out)
	}
}

func TestBazaarDocURL(t *testing.T) {
	const sha = "0123456789abcdef0123456789abcdef01234567"
	got := bazaarDocURL(sha, "README.md", "changing-maintainers")
	want := "https://github.com/siyuan-note/bazaar/blob/" + sha + "/README.md#changing-maintainers"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestInstallPageURL(t *testing.T) {
	base := installPageBaseURL
	if got, want := installPageURL("a/b", "", ""), base+"?repo=a/b"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got, want := installPageURL("a/b", "v1.0.0", "zh-CN"), base+"?repo=a/b&tag=v1.0.0&lang=zh-CN"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	// 页面用 URLSearchParams 读取参数；查询串里的裸 `+` 会被解析成空格，必须转义
	got := installPageURL("a/b", "v1.0.0+build", "en")
	if want := base + "?repo=a/b&tag=v1.0.0%2Bbuild&lang=en"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if uri, err := url.Parse(got); err != nil {
		t.Fatalf("install page url should parse: %v", err)
	} else {
		if tag := uri.Query().Get("tag"); tag != "v1.0.0+build" {
			t.Fatalf("tag should survive the round trip, got %q", tag)
		}
		if lang := uri.Query().Get("lang"); lang != "en" {
			t.Fatalf("lang should survive the round trip, got %q", lang)
		}
	}
}

func TestCheckResultTemplate_DelistBeforeAdd(t *testing.T) {
	tmpl, err := parseCheckResultTemplate(context.Background())
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
	rmIdx := strings.Index(out, "### 下架插件仓库")
	addIdx := strings.Index(out, "### 新增插件仓库")
	if rmIdx < 0 || addIdx < 0 || rmIdx > addIdx {
		t.Fatalf("delist section should appear before add section\n%s", out)
	}
}
