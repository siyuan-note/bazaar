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
	"path/filepath"
	"strings"
	"testing"
)

func TestHaltKeepsOnlyLeadingIssue(t *testing.T) {
	c := &Context{
		PackageRoot: filepath.Join("testdata", "plugin_ok"),
		OwnerRepo:   "not-a-repo",
		Type:        TypePlugin,
	}
	Run(c)
	if len(c.Issues) != 1 {
		t.Fatalf("expected exactly 1 issue after early halt, got %d: %v", len(c.Issues), c.Issues)
	}
	if !strings.Contains(c.Issues[0].MessageZh, "格式不正确") {
		t.Fatalf("expected owner_repo issue, got %s", c.Issues[0].MessageZh)
	}
	if !c.Halted() {
		t.Fatal("expected Halted")
	}
	if c.Root != "" || c.Package.Name != "" {
		t.Fatal("later steps should not have filled Root/Package")
	}
}

func TestAccumulateAfterRootOK(t *testing.T) {
	c := &Context{
		PackageRoot: filepath.Join("testdata", "plugin_ok"),
		OwnerRepo:   "demo/sample-plugin",
		Type:        TypePlugin,
		OccupiedNames: map[string]struct{}{
			"sample-plugin": {},
		},
	}
	Run(c)
	if c.Halted() {
		t.Fatal("should not halt when inputs are valid")
	}
	if c.OK() || !issuesContain(c.Issues, "已被其他集市包占用") {
		t.Fatalf("expected uniqueness issue among accumulated results, issues=%v", c.Issues)
	}
}
