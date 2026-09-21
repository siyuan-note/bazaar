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
	"testing"
)

func TestParseCSVList(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{name: "empty", raw: "", want: nil},
		{name: "spaces only", raw: "  , , ", want: nil},
		{name: "single", raw: "TCOTC", want: []string{"TCOTC"}},
		{name: "trim and split", raw: " alice, bob ,carol ", want: []string{"alice", "bob", "carol"}},
		{name: "dedupe case", raw: "Alice,alice,BOB,bob", want: []string{"Alice", "BOB"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCSVList(tt.raw)
			if !slices.Equal(got, tt.want) {
				t.Fatalf("parseCSVList(%q)=%v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestFilterOutLogin(t *testing.T) {
	got := filterOutLogin([]string{"Alice", "bob"}, "alice")
	want := []string{"bob"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got := filterOutLogin([]string{"bob"}, ""); !slices.Equal(got, []string{"bob"}) {
		t.Fatalf("empty exclude should keep list, got %v", got)
	}
}

func TestFilterOutSet(t *testing.T) {
	already := map[string]struct{}{"alice": {}}
	got := filterOutSet([]string{"Alice", "bob"}, already)
	want := []string{"bob"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
