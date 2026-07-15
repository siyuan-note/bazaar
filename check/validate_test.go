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

func TestValidatePackageName(t *testing.T) {
	if err := validatePackageName("sample-plugin"); err != nil {
		t.Fatalf("valid name rejected: %v", err)
	}
	cases := []string{".hidden", " leading", "trailing.", "中文", "a<b", "CON"}
	for _, name := range cases {
		if err := validatePackageName(name); err == nil {
			t.Fatalf("expected reject for %q", name)
		}
	}
}

func TestValidatePlainStringForHTML(t *testing.T) {
	if err := validatePlainStringForHTML("demo"); err != nil {
		t.Fatal(err)
	}
	if err := validatePlainStringForHTML("a<script>"); err == nil {
		t.Fatal("expected reject HTML specials")
	}
}
