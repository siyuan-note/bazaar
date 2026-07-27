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

	"github.com/siyuan-note/bazaar/rules"
)

func TestResolveMaintainerChangeLegacy(t *testing.T) {
	dir := t.TempDir()
	stageDir := filepath.Join(dir, "stage")
	if err := os.MkdirAll(stageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	payload := `{
  "repos": [
    {
      "url": "alice/transfer@abc123",
      "updated": "2025-01-01T00:00:00Z",
      "stars": 1,
      "openIssues": 0,
      "size": 10,
      "installSize": 20,
      "package": {
        "name": "transfer-pkg",
        "version": "1.0.0"
      }
    }
  ]
}`
	if err := os.WriteFile(filepath.Join(stageDir, "plugins.json"), []byte(payload), 0o644); err != nil {
		t.Fatal(err)
	}

	prev := BAZAAR_HEAD_PATH
	BAZAAR_HEAD_PATH = dir
	t.Cleanup(func() { BAZAAR_HEAD_PATH = prev })

	t.Run("从 stage 取旧 name/version", func(t *testing.T) {
		name, ver, issues := resolveMaintainerChangeLegacy(rules.TypePlugin, "bob/transfer", "alice/transfer")
		if len(issues) != 0 {
			t.Fatalf("issues = %v, want none", issues)
		}
		if name != "transfer-pkg" {
			t.Fatalf("oldName = %q, want transfer-pkg", name)
		}
		if ver != "1.0.0" {
			t.Fatalf("oldVersion = %q, want 1.0.0", ver)
		}
	})

	t.Run("缺少旧路径", func(t *testing.T) {
		name, ver, issues := resolveMaintainerChangeLegacy(rules.TypePlugin, "bob/transfer", "")
		if name != "" || ver != "" || len(issues) != 1 {
			t.Fatalf("name=%q ver=%q issues=%v, want empty and 1 issue", name, ver, issues)
		}
	})

	t.Run("stage 无旧条目", func(t *testing.T) {
		name, ver, issues := resolveMaintainerChangeLegacy(rules.TypePlugin, "bob/missing", "alice/missing")
		if name != "" || ver != "" || len(issues) != 1 {
			t.Fatalf("name=%q ver=%q issues=%v, want empty and 1 issue", name, ver, issues)
		}
	})
}
