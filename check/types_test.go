// SiYuan community bazaar.
// Copyright (c) 2021-present, b3log.org
//
// Bazaar is licensed under Mulan PSL v2.
// You can use this software according to the terms and conditions of the Mulan PSL v2.
// You may obtain a copy of Mulan PSL v2 at:
//         http://license.coscl.org.cn/MulanPSL2
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR PURPOSE.
// See the Mulan PSL v2 for more details.

package check

import "testing"

func TestParsePackageType(t *testing.T) {
	cases := []struct {
		in   string
		want PackageType
		ok   bool
	}{
		{"plugin", TypePlugin, true},
		{"plugins", TypePlugin, true},
		{"theme", TypeTheme, true},
		{"themes", TypeTheme, true},
		{"icon", TypeIcon, true},
		{"icons", TypeIcon, true},
		{"template", TypeTemplate, true},
		{"templates", TypeTemplate, true},
		{"widget", TypeWidget, true},
		{"widgets", TypeWidget, true},
		{"unknown", 0, false},
	}

	for _, tc := range cases {
		got, ok := ParsePackageType(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Errorf("ParsePackageType(%q) = (%v, %v), want (%v, %v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestPackageTypeNames(t *testing.T) {
	for _, typ := range AllPackageTypes() {
		if _, ok := ParsePackageType(typ.String()); !ok {
			t.Errorf("ParsePackageType(String()) failed for %v", typ)
		}
		if _, ok := ParsePackageType(typ.Plural()); !ok {
			t.Errorf("ParsePackageType(Plural()) failed for %v", typ)
		}
		if typ.ManifestFile() != typ.String()+".json" {
			t.Errorf("ManifestFile() mismatch for %v", typ)
		}
		if typ.ReposListFile() != typ.Plural()+".txt" {
			t.Errorf("ReposListFile() mismatch for %v", typ)
		}
		if typ.StageJSONFile() != typ.Plural()+".json" {
			t.Errorf("StageJSONFile() mismatch for %v", typ)
		}
	}
}

func TestStageOrderPackageTypes(t *testing.T) {
	order := StageOrderPackageTypes()
	if len(order) != len(packageTypeMetas) {
		t.Fatalf("StageOrderPackageTypes len = %d, want %d", len(order), len(packageTypeMetas))
	}
	seen := make(map[PackageType]struct{}, len(order))
	for _, typ := range order {
		if !typ.valid() {
			t.Errorf("invalid type in StageOrderPackageTypes: %v", typ)
		}
		if _, dup := seen[typ]; dup {
			t.Errorf("duplicate type in StageOrderPackageTypes: %v", typ)
		}
		seen[typ] = struct{}{}
	}
}

func TestCheckOrderPackageTypes(t *testing.T) {
	order := CheckOrderPackageTypes()
	if len(order) != len(packageTypeMetas) {
		t.Fatalf("CheckOrderPackageTypes len = %d, want %d", len(order), len(packageTypeMetas))
	}
	seen := make(map[PackageType]struct{}, len(order))
	for _, typ := range order {
		if !typ.valid() {
			t.Errorf("invalid type in CheckOrderPackageTypes: %v", typ)
		}
		if _, dup := seen[typ]; dup {
			t.Errorf("duplicate type in CheckOrderPackageTypes: %v", typ)
		}
		seen[typ] = struct{}{}
	}
}
