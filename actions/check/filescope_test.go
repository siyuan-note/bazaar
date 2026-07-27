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
	"slices"
	"strings"
	"testing"

	"github.com/siyuan-note/bazaar/actions/util"
)

func TestNormalizeRepoRelPath(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", ""},
		{"  plugins.txt  ", "plugins.txt"},
		{`stage\plugins.json`, "stage/plugins.json"},
		{"./plugins.txt", "plugins.txt"},
	}
	for _, tt := range tests {
		if got := normalizeRepoRelPath(tt.in); got != tt.want {
			t.Errorf("normalizeRepoRelPath(%q)=%q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestIsWhitelistedPath(t *testing.T) {
	for _, p := range []string{"plugins.txt", "themes.txt", "icons.txt", "templates.txt", "widgets.txt"} {
		if !isWhitelistedPath(p) {
			t.Errorf("%q should be whitelisted", p)
		}
	}
	for _, p := range []string{
		"stage/plugins.json",
		util.ThemeJsAllowlistRelPath,
		"README.md",
		"actions/check/main.go",
		"plugins.json",
	} {
		if isWhitelistedPath(p) {
			t.Errorf("%q should not be whitelisted", p)
		}
	}
}

func TestIsBlacklistedPath(t *testing.T) {
	for _, p := range []string{
		"stage/plugins.json",
		"stage/themes.json",
		"stage",
		"stage/README.md",
		util.ThemeJsAllowlistRelPath,
	} {
		if !isBlacklistedPath(p) {
			t.Errorf("%q should be blacklisted", p)
		}
	}
	for _, p := range []string{"plugins.txt", "README.md", "config/other.txt"} {
		if isBlacklistedPath(p) {
			t.Errorf("%q should not be blacklisted", p)
		}
	}
}

func TestClassifyPRFiles(t *testing.T) {
	black, white := classifyPRFiles([]string{
		"stage/plugins.json",
		`stage\themes.json`,
		"plugins.txt",
		"README.md",
		util.ThemeJsAllowlistRelPath,
		"./themes.txt",
	})
	wantBlack := []string{"stage/plugins.json", "stage/themes.json", util.ThemeJsAllowlistRelPath}
	wantWhite := []string{"plugins.txt", "themes.txt"}
	if !slices.Equal(black, wantBlack) {
		t.Fatalf("black=%v, want %v", black, wantBlack)
	}
	if !slices.Equal(white, wantWhite) {
		t.Fatalf("white=%v, want %v", white, wantWhite)
	}
}

func TestFormatBlacklistFlowError(t *testing.T) {
	got := formatBlacklistFlowError()
	for _, want := range []string{
		"本 PR 修改了不允许直接改动的文件",
		"plugins.txt",
		"This PR modifies files that must not be changed directly",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Disallowed files") || strings.Contains(got, "不允许修改的文件") {
		t.Fatalf("should not list disallowed files:\n%s", got)
	}
}
