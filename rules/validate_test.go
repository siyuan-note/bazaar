// SiYuan community bazaar.
// Copyright (c) 2021-present, b3log.org
//
// Bazaar is licensed under Mulan PSL v2.
// You can use this software according to the terms and conditions of the Mulan PSL v2.
// You may obtain a copy of Mulan PSL v2 at:
//         http://license.coscl.org.cn/MulanPSL2
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR PURPOSE.
// See the Mulan PSL v2 for more details.

package rules

import "testing"

func TestCheckNameValid(t *testing.T) {
	issues := checkName(map[string]any{"name": "sample-plugin"}, ManifestInput{
		Repo: "sample-plugin",
		Type: TypePlugin,
	})
	if len(issues) != 0 {
		t.Fatalf("valid name rejected: %v", issues)
	}
}

func TestCheckNameRejectsInvalid(t *testing.T) {
	cases := []string{".hidden", " leading", "trailing.", "中文", "a<b", "CON"}
	for _, name := range cases {
		issues := checkName(map[string]any{"name": name}, ManifestInput{
			Repo: name,
			Type: TypePlugin,
		})
		if len(issues) == 0 {
			t.Fatalf("expected reject for %q", name)
		}
	}
}

func TestCheckNameCollectsMultiple(t *testing.T) {
	issues := checkName(map[string]any{"name": ".a<b"}, ManifestInput{
		Repo: ".a<b",
		Type: TypePlugin,
	})
	if len(issues) < 2 {
		t.Fatalf("expected multiple issues for %q, got %d: %v", ".a<b", len(issues), issues)
	}
}

func TestCheckAuthor(t *testing.T) {
	if issues := checkAuthor(map[string]any{"author": "demo"}, "demo"); len(issues) != 0 {
		t.Fatal(issues)
	}
	if issues := checkAuthor(map[string]any{"author": "a<script>"}, "demo"); len(issues) != 0 {
		t.Fatalf("HTML specials in author should be allowed (sanitized at index write): %v", issues)
	}
	if issues := checkAuthor(map[string]any{"author": "   "}, "demo"); len(issues) == 0 {
		t.Fatal("expected reject whitespace only")
	}
}
