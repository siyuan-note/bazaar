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

func TestValidatePackageName(t *testing.T) {
	if errs := validatePackageName("sample-plugin"); len(errs) != 0 {
		t.Fatalf("valid name rejected: %v", errs)
	}
	cases := []string{".hidden", " leading", "trailing.", "中文", "a<b", "CON"}
	for _, name := range cases {
		if errs := validatePackageName(name); len(errs) == 0 {
			t.Fatalf("expected reject for %q", name)
		}
	}
}

func TestValidatePackageNameCollectsMultiple(t *testing.T) {
	errs := validatePackageName(".a<b")
	if len(errs) < 2 {
		t.Fatalf("expected multiple issues for %q, got %d: %v", ".a<b", len(errs), errs)
	}
}

func TestValidateManifestAuthor(t *testing.T) {
	if errs := validateManifestAuthor("demo", "demo"); len(errs) != 0 {
		t.Fatal(errs)
	}
	if errs := validateManifestAuthor("a<script>", "demo"); len(errs) == 0 {
		t.Fatal("expected reject HTML specials")
	}
	if errs := validateManifestAuthor("   ", "demo"); len(errs) == 0 {
		t.Fatal("expected reject whitespace only")
	}
}
