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
	"testing"

	"github.com/siyuan-note/bazaar/rules"
)

func TestConventionalPRTitle_Add(t *testing.T) {
	plans := []typeCheckPlan{
		{packageType: rules.TypePlugin, diff: repoDiff{New: []string{"alice/foo"}}},
		{packageType: rules.TypeTheme},
	}
	title, ok := conventionalPRTitle(plans)
	if !ok || title != "Add alice/foo" {
		t.Fatalf("got (%q, %v), want (Add alice/foo, true)", title, ok)
	}
}

func TestConventionalPRTitle_AddTheme(t *testing.T) {
	plans := []typeCheckPlan{
		{packageType: rules.TypeTheme, diff: repoDiff{New: []string{"alice/dark"}}},
	}
	title, ok := conventionalPRTitle(plans)
	if !ok || title != "Add theme alice/dark" {
		t.Fatalf("got (%q, %v), want (Add theme alice/dark, true)", title, ok)
	}
}

func TestConventionalPRTitle_Delist(t *testing.T) {
	plans := []typeCheckPlan{
		{packageType: rules.TypeWidget, diff: repoDiff{Deleted: []string{"bob/bar"}}},
	}
	title, ok := conventionalPRTitle(plans)
	if !ok || title != "Delist widget bob/bar" {
		t.Fatalf("got (%q, %v), want (Delist widget bob/bar, true)", title, ok)
	}
}

func TestConventionalPRTitle_DelistMultiple(t *testing.T) {
	plans := []typeCheckPlan{
		{packageType: rules.TypePlugin, diff: repoDiff{Deleted: []string{"a/p1", "b/p2"}}},
		{packageType: rules.TypeIcon, diff: repoDiff{Deleted: []string{"c/i1"}}},
	}
	title, ok := conventionalPRTitle(plans)
	if !ok || title != "Delist 3 packages" {
		t.Fatalf("got (%q, %v), want (Delist 3 packages, true)", title, ok)
	}
}

func TestConventionalPRTitle_MaintainerChange(t *testing.T) {
	plans := []typeCheckPlan{
		{
			packageType: rules.TypeTheme,
			diff: repoDiff{
				New:           []string{"carol/theme"},
				Deleted:       []string{"dave/theme"},
				PreviousRepos: map[string]string{"carol/theme": "dave/theme"},
			},
		},
	}
	title, ok := conventionalPRTitle(plans)
	want := "Add theme carol/theme (maintainer change)"
	if !ok || title != want {
		t.Fatalf("got (%q, %v), want (%s, true)", title, ok, want)
	}
}

func TestConventionalPRTitle_MultipleRepos(t *testing.T) {
	plans := []typeCheckPlan{
		{packageType: rules.TypePlugin, diff: repoDiff{New: []string{"a/p1"}}},
		{packageType: rules.TypeIcon, diff: repoDiff{Deleted: []string{"b/i1"}}},
	}
	if title, ok := conventionalPRTitle(plans); ok {
		t.Fatalf("expected ok=false, got title=%q", title)
	}
}

func TestConventionalPRTitle_ParseError(t *testing.T) {
	plans := []typeCheckPlan{
		{packageType: rules.TypePlugin, diff: repoDiff{New: []string{"a/p1"}}},
		{packageType: rules.TypeTheme, parseError: "bad"},
	}
	if title, ok := conventionalPRTitle(plans); ok {
		t.Fatalf("expected ok=false on parseError, got title=%q", title)
	}
}

func TestConventionalPRTitle_AddPlusUnrelatedDelist(t *testing.T) {
	// 流程规则禁止「新增 + 无关下架」；标题同样不生成
	plans := []typeCheckPlan{
		{packageType: rules.TypePlugin, diff: repoDiff{New: []string{"a/p1"}, Deleted: []string{"b/old"}}},
	}
	if title, ok := conventionalPRTitle(plans); ok {
		t.Fatalf("expected ok=false, got title=%q", title)
	}
}

func TestConventionalDeprecationPRTitle_DeprecatePlugin(t *testing.T) {
	title, ok := conventionalDeprecationPRTitle(&DeprecationCheck{
		Changes: []DeprecationChange{{
			PackageType: rules.TypePlugin,
			OwnerRepo:   "old/plugin",
			Action:      deprecationActionAdd,
		}},
	})
	if !ok || title != "Deprecate old/plugin" {
		t.Fatalf("got (%q, %v), want (Deprecate old/plugin, true)", title, ok)
	}
}

func TestConventionalDeprecationPRTitle_DeprecateTheme(t *testing.T) {
	title, ok := conventionalDeprecationPRTitle(&DeprecationCheck{
		Changes: []DeprecationChange{{
			PackageType: rules.TypeTheme,
			OwnerRepo:   "old/theme",
			Action:      deprecationActionAdd,
		}},
	})
	if !ok || title != "Deprecate theme old/theme" {
		t.Fatalf("got (%q, %v), want (Deprecate theme old/theme, true)", title, ok)
	}
}

func TestConventionalDeprecationPRTitle_RestoreWidget(t *testing.T) {
	title, ok := conventionalDeprecationPRTitle(&DeprecationCheck{
		Changes: []DeprecationChange{{
			PackageType: rules.TypeWidget,
			OwnerRepo:   "old/widget",
			Action:      deprecationActionRemove,
		}},
	})
	if !ok || title != "Restore widget old/widget" {
		t.Fatalf("got (%q, %v), want (Restore widget old/widget, true)", title, ok)
	}
}

func TestConventionalDeprecationPRTitle_UpdateReason(t *testing.T) {
	title, ok := conventionalDeprecationPRTitle(&DeprecationCheck{
		Changes: []DeprecationChange{{
			PackageType: rules.TypePlugin,
			OwnerRepo:   "old/plugin",
			Action:      deprecationActionUpdate,
		}},
	})
	if !ok || title != "Update deprecation old/plugin" {
		t.Fatalf("got (%q, %v), want (Update deprecation old/plugin, true)", title, ok)
	}
}

func TestConventionalDeprecationPRTitle_UpdateTheme(t *testing.T) {
	title, ok := conventionalDeprecationPRTitle(&DeprecationCheck{
		Changes: []DeprecationChange{{
			PackageType: rules.TypeTheme,
			OwnerRepo:   "old/theme",
			Action:      deprecationActionUpdate,
		}},
	})
	if !ok || title != "Update deprecation theme old/theme" {
		t.Fatalf("got (%q, %v), want (Update deprecation theme old/theme, true)", title, ok)
	}
}

func TestConventionalDeprecationPRTitle_MultipleAdds(t *testing.T) {
	title, ok := conventionalDeprecationPRTitle(&DeprecationCheck{
		Changes: []DeprecationChange{
			{PackageType: rules.TypePlugin, OwnerRepo: "a/p1", Action: deprecationActionAdd},
			{PackageType: rules.TypeIcon, OwnerRepo: "b/i1", Action: deprecationActionAdd},
		},
	})
	if !ok || title != "Deprecate 2 packages" {
		t.Fatalf("got (%q, %v), want (Deprecate 2 packages, true)", title, ok)
	}
}

func TestConventionalDeprecationPRTitle_MultipleRemoves(t *testing.T) {
	title, ok := conventionalDeprecationPRTitle(&DeprecationCheck{
		Changes: []DeprecationChange{
			{PackageType: rules.TypePlugin, OwnerRepo: "a/p1", Action: deprecationActionRemove},
			{PackageType: rules.TypePlugin, OwnerRepo: "b/p2", Action: deprecationActionRemove},
		},
	})
	if !ok || title != "Restore 2 packages" {
		t.Fatalf("got (%q, %v), want (Restore 2 packages, true)", title, ok)
	}
}

func TestConventionalDeprecationPRTitle_MultipleUpdates(t *testing.T) {
	title, ok := conventionalDeprecationPRTitle(&DeprecationCheck{
		Changes: []DeprecationChange{
			{PackageType: rules.TypeTheme, OwnerRepo: "a/t1", Action: deprecationActionUpdate},
			{PackageType: rules.TypeTheme, OwnerRepo: "b/t2", Action: deprecationActionUpdate},
			{PackageType: rules.TypeTheme, OwnerRepo: "c/t3", Action: deprecationActionUpdate},
		},
	})
	if !ok || title != "Update 3 deprecations" {
		t.Fatalf("got (%q, %v), want (Update 3 deprecations, true)", title, ok)
	}
}

func TestConventionalDeprecationPRTitle_MixedActions(t *testing.T) {
	title, ok := conventionalDeprecationPRTitle(&DeprecationCheck{
		Changes: []DeprecationChange{
			{PackageType: rules.TypePlugin, OwnerRepo: "a/p1", Action: deprecationActionAdd},
			{PackageType: rules.TypePlugin, OwnerRepo: "b/p2", Action: deprecationActionRemove},
		},
	})
	if !ok || title != "Update deprecation registry" {
		t.Fatalf("got (%q, %v), want (Update deprecation registry, true)", title, ok)
	}
}

func TestConventionalDeprecationPRTitle_NoChange(t *testing.T) {
	if title, ok := conventionalDeprecationPRTitle(nil); ok {
		t.Fatalf("expected ok=false for nil, got title=%q", title)
	}
	if title, ok := conventionalDeprecationPRTitle(&DeprecationCheck{}); ok {
		t.Fatalf("expected ok=false for empty changes, got title=%q", title)
	}
}
