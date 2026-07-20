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
				New:           []string{"carol/w"},
				Deleted:       []string{"dave/w"},
				PreviousRepos: map[string]string{"carol/w": "dave/w"},
			},
		},
	}
	expected := typeLabelSyncPlan(plans)
	if _, ok := expected["widget"]; !ok || len(expected) != 1 {
		t.Fatalf("expected=%v, want {widget}", expected)
	}
}

func TestManagedLabelSet(t *testing.T) {
	set := managedLabelSet()
	for _, name := range []string{"plugin", "theme", "icon", "template", "widget", labelCIFailed, labelCIPassed} {
		if _, ok := set[name]; !ok {
			t.Fatalf("missing %q in %v", name, set)
		}
	}
	if len(set) != 7 {
		t.Fatalf("len=%d, want 7", len(set))
	}
}

func TestBuildPRLabelsAfterSync_Passed(t *testing.T) {
	current := []string{"Check", "plugin", "bug", "theme", "ci-failed"}
	expected := Set{"icon": {}, "plugin": {}}
	got := buildPRLabelsAfterSync(current, expected, true)
	want := []string{"Check", "bug", "plugin", "icon", "ci-passed"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestBuildPRLabelsAfterSync_Failed(t *testing.T) {
	current := []string{"ci-passed", "plugin", "waiting-author"}
	expected := Set{"plugin": {}}
	got := buildPRLabelsAfterSync(current, expected, false)
	want := []string{"waiting-author", "plugin", "ci-failed"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestCheckResultCIPassed(t *testing.T) {
	if checkResultCIPassed(&CheckResult{}) {
		t.Fatal("empty result (no list activity) should fail")
	}
	if checkResultCIPassed(&CheckResult{ParseError: "bad"}) {
		t.Fatal("ParseError should fail")
	}
	if checkResultCIPassed(&CheckResult{FlowError: "limit"}) {
		t.Fatal("FlowError should fail")
	}
	if checkResultCIPassed(&CheckResult{
		Plugins: []PackageCheck{{Issues: []rules.Issue{{MessageZh: "x"}}}},
	}) {
		t.Fatal("package Issues should fail")
	}
	if !checkResultCIPassed(&CheckResult{
		Plugins:        []PackageCheck{{RepoInfo: RepoInfo{Path: "a/b"}}},
		PluginsDeleted: []string{"c/d"},
	}) {
		t.Fatal("clean checks and deletions should pass")
	}
	if !checkResultCIPassed(&CheckResult{
		PluginsDeleted: []string{"c/d"},
	}) {
		t.Fatal("pure deletion should pass")
	}
	if checkResultCIPassed(nil) {
		t.Fatal("nil should fail")
	}
}

func TestIsNoActualChange(t *testing.T) {
	if !isNoActualChange(nil) {
		t.Fatal("nil should be no actual change")
	}
	if !isNoActualChange(&CheckResult{}) {
		t.Fatal("empty result should be no actual change")
	}
	if isNoActualChange(&CheckResult{ParseError: "bad"}) {
		t.Fatal("ParseError is not no actual change")
	}
	if isNoActualChange(&CheckResult{FlowError: "limit"}) {
		t.Fatal("FlowError is not no actual change")
	}
	if isNoActualChange(&CheckResult{
		Plugins: []PackageCheck{{Issues: []rules.Issue{{MessageZh: "x"}}}},
	}) {
		t.Fatal("package with Issues is not no actual change")
	}
	if isNoActualChange(&CheckResult{
		Plugins: []PackageCheck{{RepoInfo: RepoInfo{Path: "a/b"}}},
	}) {
		t.Fatal("added package is not no actual change")
	}
	if isNoActualChange(&CheckResult{PluginsDeleted: []string{"c/d"}}) {
		t.Fatal("deletion is not no actual change")
	}
}

func TestCIStatusLabel(t *testing.T) {
	if got := ciStatusLabel(true); got != labelCIPassed {
		t.Fatalf("got %q, want %q", got, labelCIPassed)
	}
	if got := ciStatusLabel(false); got != labelCIFailed {
		t.Fatalf("got %q, want %q", got, labelCIFailed)
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
