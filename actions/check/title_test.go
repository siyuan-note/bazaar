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
	// 一次一包允许「1 个新增 + 任意下架」，但标题要求仅涉及一个仓库
	plans := []typeCheckPlan{
		{packageType: rules.TypePlugin, diff: repoDiff{New: []string{"a/p1"}, Deleted: []string{"b/old"}}},
	}
	if title, ok := conventionalPRTitle(plans); ok {
		t.Fatalf("expected ok=false, got title=%q", title)
	}
}
