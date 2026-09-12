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
	"os"
	"path/filepath"
	"testing"

	"github.com/siyuan-note/bazaar/actions/util"
	"github.com/siyuan-note/bazaar/rules"
)

const emptyDeprecationRegistry = `{"plugins":{},"themes":{},"icons":{},"templates":{},"widgets":{}}`

func TestCheckDeprecationRegistry(t *testing.T) {
	mainRoot := t.TempDir()
	prRoot := t.TempDir()
	writeDeprecationCheckFixture(t, mainRoot, emptyDeprecationRegistry)
	writeDeprecationCheckFixture(t, prRoot, `{
  "plugins": {
    "old/plugin": {
      "reason": {"default": "No longer maintained"},
      "alternatives": ["new/plugin"]
    }
  },
  "themes": {},
  "icons": {},
  "templates": {},
  "widgets": {}
}`)
	oldBazaarHead, oldPRHead := BAZAAR_HEAD_PATH, PR_HEAD_PATH
	BAZAAR_HEAD_PATH, PR_HEAD_PATH = mainRoot, prRoot
	t.Cleanup(func() {
		BAZAAR_HEAD_PATH, PR_HEAD_PATH = oldBazaarHead, oldPRHead
	})

	result := checkDeprecationRegistry()
	if result == nil {
		t.Fatal("expected deprecation check")
	}
	if len(result.Issues) != 0 {
		t.Fatalf("unexpected issues: %#v", result.Issues)
	}
	if len(result.Types) != 1 || result.Types[0] != rules.TypePlugin {
		t.Fatalf("types = %v, want plugin", result.Types)
	}
	if len(result.Changes) != 1 || result.Changes[0].OwnerRepo != "old/plugin" || result.Changes[0].Action != deprecationActionAdd {
		t.Fatalf("changes = %#v", result.Changes)
	}
}

func TestCheckDeprecationRegistryRejectsMissingAlternative(t *testing.T) {
	mainRoot := t.TempDir()
	prRoot := t.TempDir()
	writeDeprecationCheckFixture(t, mainRoot, emptyDeprecationRegistry)
	writeDeprecationCheckFixture(t, prRoot, `{
  "plugins": {"old/plugin": {"alternatives": ["missing/plugin"]}},
  "themes": {},
  "icons": {},
  "templates": {},
  "widgets": {}
}`)
	oldBazaarHead, oldPRHead := BAZAAR_HEAD_PATH, PR_HEAD_PATH
	BAZAAR_HEAD_PATH, PR_HEAD_PATH = mainRoot, prRoot
	t.Cleanup(func() {
		BAZAAR_HEAD_PATH, PR_HEAD_PATH = oldBazaarHead, oldPRHead
	})

	result := checkDeprecationRegistry()
	if result == nil || len(result.Issues) == 0 {
		t.Fatalf("expected validation issue, got %#v", result)
	}
}

func TestCheckDeprecationRegistryNoActualChange(t *testing.T) {
	mainRoot := t.TempDir()
	prRoot := t.TempDir()
	writeDeprecationCheckFixture(t, mainRoot, emptyDeprecationRegistry)
	writeDeprecationCheckFixture(t, prRoot, emptyDeprecationRegistry)
	oldBazaarHead, oldPRHead := BAZAAR_HEAD_PATH, PR_HEAD_PATH
	BAZAAR_HEAD_PATH, PR_HEAD_PATH = mainRoot, prRoot
	t.Cleanup(func() {
		BAZAAR_HEAD_PATH, PR_HEAD_PATH = oldBazaarHead, oldPRHead
	})

	if result := checkDeprecationRegistry(); result != nil {
		t.Fatalf("result = %#v, want nil", result)
	}
}

func writeDeprecationCheckFixture(t *testing.T, root, registry string) {
	t.Helper()
	for _, packageType := range rules.AllPackageTypes() {
		body := ""
		if packageType == rules.TypePlugin {
			body = "old/plugin\nnew/plugin\n"
		}
		if err := os.WriteFile(filepath.Join(root, packageType.ReposListFile()), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, util.DeprecatedRegistryRelPath), []byte(registry), 0o644); err != nil {
		t.Fatal(err)
	}
}
