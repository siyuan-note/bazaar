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

import (
	"encoding/json"
	"testing"
)

func TestClearEmptyFunding(t *testing.T) {
	t.Run("nil package", func(t *testing.T) {
		ClearEmptyFunding(nil)
	})

	t.Run("empty funding object becomes nil", func(t *testing.T) {
		pkg := &Package{Funding: &Funding{}}
		ClearEmptyFunding(pkg)
		if pkg.Funding != nil {
			t.Fatalf("expected nil funding, got %#v", pkg.Funding)
		}
	})

	t.Run("empty custom slice becomes nil", func(t *testing.T) {
		pkg := &Package{Funding: &Funding{Custom: []string{}}}
		ClearEmptyFunding(pkg)
		if pkg.Funding != nil {
			t.Fatalf("expected nil funding, got %#v", pkg.Funding)
		}
	})

	t.Run("keeps non-empty funding", func(t *testing.T) {
		pkg := &Package{Funding: &Funding{GitHub: "b3log"}}
		ClearEmptyFunding(pkg)
		if pkg.Funding == nil || pkg.Funding.GitHub != "b3log" {
			t.Fatalf("expected funding kept, got %#v", pkg.Funding)
		}
	})
}

func TestClearEmptyFundingOmitsEmptyFundingJSON(t *testing.T) {
	pkg := &Package{
		Name:    "demo",
		Author:  "a",
		URL:     "https://github.com/a/b",
		Version: "0.0.1",
		Funding: &Funding{},
	}
	ClearEmptyFunding(pkg)
	data, err := json.Marshal(pkg)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"name":"demo","author":"a","url":"https://github.com/a/b","version":"0.0.1"}` {
		t.Fatalf("unexpected json: %s", data)
	}
}

func TestClearRedundantLocales(t *testing.T) {
	t.Run("nil package", func(t *testing.T) {
		ClearRedundantLocales(nil)
	})

	t.Run("removes locales identical to default", func(t *testing.T) {
		pkg := &Package{
			DisplayName: LocaleStrings{
				"default": "Demo",
				"zh-CN":   "Demo",
				"en":      "Demo EN",
			},
			Description: LocaleStrings{
				"default": "desc",
				"zh-CN":   "desc",
			},
			Readme: LocaleStrings{
				"default": "README.md",
				"zh-CN":   "README.md",
				"en":      "README_en_US.md",
			},
		}
		ClearRedundantLocales(pkg)
		if _, ok := pkg.DisplayName["zh-CN"]; ok {
			t.Fatalf("expected zh-CN displayName removed, got %#v", pkg.DisplayName)
		}
		if pkg.DisplayName["en"] != "Demo EN" {
			t.Fatalf("expected distinct en displayName kept, got %#v", pkg.DisplayName)
		}
		if _, ok := pkg.Description["zh-CN"]; ok {
			t.Fatalf("expected zh-CN description removed, got %#v", pkg.Description)
		}
		if _, ok := pkg.Readme["zh-CN"]; ok {
			t.Fatalf("expected zh-CN readme removed, got %#v", pkg.Readme)
		}
		if pkg.Readme["en"] != "README_en_US.md" {
			t.Fatalf("expected distinct en readme kept, got %#v", pkg.Readme)
		}
	})

	t.Run("keeps all when no default", func(t *testing.T) {
		pkg := &Package{
			DisplayName: LocaleStrings{"zh-CN": "演示"},
		}
		ClearRedundantLocales(pkg)
		if pkg.DisplayName["zh-CN"] != "演示" {
			t.Fatalf("expected locale kept without default, got %#v", pkg.DisplayName)
		}
	})
}

func TestPackageForPublicIndex(t *testing.T) {
	src := Package{
		Name: "demo",
		DisplayName: LocaleStrings{
			"default": "Demo",
			"zh-CN":   "Demo",
			"en":      "Demo EN",
		},
	}
	out := PackageForPublicIndex(src)
	if _, ok := out.DisplayName["zh-CN"]; ok {
		t.Fatalf("expected zh-CN removed in public copy, got %#v", out.DisplayName)
	}
	if _, ok := src.DisplayName["zh-CN"]; !ok {
		t.Fatalf("expected source displayName unchanged, got %#v", src.DisplayName)
	}
	if out.DisplayName["en"] != "Demo EN" {
		t.Fatalf("expected distinct en kept, got %#v", out.DisplayName)
	}
}
