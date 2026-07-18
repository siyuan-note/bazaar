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

	"github.com/siyuan-note/bazaar/rules"
)

func TestTypeLabelSyncPlan_AddPlugin(t *testing.T) {
	plans := []typeCheckPlan{
		{packageType: rules.TypePlugin, diff: repoDiff{New: []string{"alice/foo"}}},
		{packageType: rules.TypeTheme},
	}
	expected := typeLabelSyncPlan(plans)
	if _, ok := expected["plugin"]; !ok || len(expected) != 1 {
		t.Fatalf("expected=%v, want {plugin}", expected)
	}
}

func TestTypeLabelSyncPlan_MultipleTypes(t *testing.T) {
	plans := []typeCheckPlan{
		{packageType: rules.TypePlugin, diff: repoDiff{Deleted: []string{"a/p1", "b/p2"}}},
		{packageType: rules.TypeIcon, diff: repoDiff{Deleted: []string{"c/i1"}}},
		{packageType: rules.TypeTheme},
	}
	expected := typeLabelSyncPlan(plans)
	if len(expected) != 2 {
		t.Fatalf("expected=%v, want plugin+icon", expected)
	}
	if _, ok := expected["plugin"]; !ok {
		t.Fatalf("missing plugin in %v", expected)
	}
	if _, ok := expected["icon"]; !ok {
		t.Fatalf("missing icon in %v", expected)
	}
}

func TestTypeLabelSyncPlan_ParseErrorLabeled(t *testing.T) {
	plans := []typeCheckPlan{
		{packageType: rules.TypePlugin, diff: repoDiff{New: []string{"a/p1"}}},
		{packageType: rules.TypeTheme, parseError: "bad"},
	}
	expected := typeLabelSyncPlan(plans)
	if len(expected) != 2 {
		t.Fatalf("expected=%v, want plugin+theme", expected)
	}
	if _, ok := expected["plugin"]; !ok {
		t.Fatalf("missing plugin in %v", expected)
	}
	if _, ok := expected["theme"]; !ok {
		t.Fatalf("missing theme in %v", expected)
	}
}

func TestTypeLabelSyncPlan_NoChange(t *testing.T) {
	plans := []typeCheckPlan{
		{packageType: rules.TypePlugin},
		{packageType: rules.TypeTheme},
	}
	expected := typeLabelSyncPlan(plans)
	if len(expected) != 0 {
		t.Fatalf("expected=%v, want empty", expected)
	}
}

func TestTypeLabelSyncPlan_MaintainerChange(t *testing.T) {
	plans := []typeCheckPlan{
		{
			packageType: rules.TypeWidget,
			diff: repoDiff{
				New:               []string{"carol/w"},
				Deleted:           []string{"dave/w"},
				MaintainerChanged: Set{"carol/w": {}},
			},
		},
	}
	expected := typeLabelSyncPlan(plans)
	if _, ok := expected["widget"]; !ok || len(expected) != 1 {
		t.Fatalf("expected=%v, want {widget}", expected)
	}
}

func TestPackageTypeLabelSet(t *testing.T) {
	set := packageTypeLabelSet()
	for _, name := range []string{"plugin", "theme", "icon", "template", "widget"} {
		if _, ok := set[name]; !ok {
			t.Fatalf("missing %q in %v", name, set)
		}
	}
	if len(set) != 5 {
		t.Fatalf("len=%d, want 5", len(set))
	}
}

func TestBuildPRLabelsAfterTypeSync(t *testing.T) {
	current := []string{"Check", "plugin", "bug", "theme"}
	expected := Set{"icon": {}, "plugin": {}}
	got := buildPRLabelsAfterTypeSync(current, expected)
	want := []string{"Check", "bug", "plugin", "icon"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestSameLabelNames(t *testing.T) {
	if !sameLabelNames([]string{"a", "b"}, []string{"b", "a"}) {
		t.Fatal("order should not matter")
	}
	if sameLabelNames([]string{"a"}, []string{"a", "b"}) {
		t.Fatal("different sets should not match")
	}
}
