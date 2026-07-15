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

func TestPackageTypeNames(t *testing.T) {
	for _, typ := range AllPackageTypes() {
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

func TestAllPackageTypesOrder(t *testing.T) {
	want := []PackageType{TypePlugin, TypeTheme, TypeIcon, TypeTemplate, TypeWidget}
	got := AllPackageTypes()
	if len(got) != len(want) {
		t.Fatalf("AllPackageTypes len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("AllPackageTypes()[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestPackageTypeZeroIsInvalid(t *testing.T) {
	var zero PackageType
	if zero.valid() {
		t.Fatal("zero PackageType should be invalid")
	}
}
