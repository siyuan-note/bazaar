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
