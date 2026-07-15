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

	"github.com/siyuan-note/bazaar/check"
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
				Release: ReleaseInfo{
					Tag: "v0.0.1", URL: "https://github.com/siyuan-note/plugin-sample/releases/tag/v0.0.1",
					PackageZipAssetID: 1,
				},
			},
			{
				RepoInfo: RepoInfo{Path: "example/broken-plugin", Home: "https://github.com/example/broken-plugin"},
				Issues: []check.Issue{{
					Rule: "files/required", MessageZh: "缺少 icon.png", MessageEn: "missing icon.png",
				}},
			},
			{
				RepoInfo: RepoInfo{Path: "example/no-release", Home: "https://github.com/example/no-release"},
				Issues: []check.Issue{{
					Rule: "release/latest", MessageZh: "无 Latest Release", MessageEn: "no Latest Release",
				}},
			},
			{
				RepoInfo: RepoInfo{Path: "example/no-package-zip", Home: "https://github.com/example/no-package-zip"},
				Issues: []check.Issue{{
					Rule: "release/package_zip", MessageZh: "无 package.zip", MessageEn: "no package.zip",
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
		"01 [release/latest]",
		"01 [release/package_zip]",
		"01 [files/required]",
		"Check passed.",
		"Latest Release: [v0.0.1](https://github.com/siyuan-note/plugin-sample/releases/tag/v0.0.1)",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q\n%s", want, out)
		}
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
