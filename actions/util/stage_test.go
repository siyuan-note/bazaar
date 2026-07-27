// SiYuan community bazaar.
// Copyright (c) 2021-present, b3log.org
//
// Bazaar is licensed under Mulan PSL v2.
// You can use this software according to the terms and conditions of the Mulan PSL v2.
// You may obtain a copy of Mulan PSL v2 at:
//         http://license.coscl.org.cn/MulanPSL2
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR PURPOSE.
// See the Mulan PSL v2 for more details.

package util

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/siyuan-note/bazaar/rules"
)

func TestFindStageRepo(t *testing.T) {
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

	got, err := FindStageRepo(dir, rules.TypePlugin, "alice/transfer")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Package.Name != "transfer-pkg" {
		t.Fatalf("FindStageRepo = %+v, want name transfer-pkg", got)
	}

	missing, err := FindStageRepo(dir, rules.TypePlugin, "bob/transfer")
	if err != nil {
		t.Fatal(err)
	}
	if missing != nil {
		t.Fatalf("FindStageRepo missing = %+v, want nil", missing)
	}
}

func TestStageFileForPublicIndex_stripsInternalFields(t *testing.T) {
	stageFile := StageFile{
		Repos: []StageRepo{
			{
				URL:               "owner/repo@abc123",
				Updated:           "2025-01-01T00:00:00Z",
				Stars:             10,
				OpenIssues:        1,
				Size:              100,
				InstallSize:       200,
				PackageZipAssetID: 424242,
				Package: rules.Package{
					Name:    "demo",
					Version: "1.0.0",
					DisplayName: rules.LocaleStrings{
						"default": "Demo",
						"zh-CN":   "Demo",
						"en":      "Demo EN",
					},
				},
			},
		},
	}

	public := stageFile.ForPublicIndex()
	data, err := json.Marshal(public)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if strings.Contains(got, "packageZipAssetId") {
		t.Fatalf("public index must not contain packageZipAssetId: %s", got)
	}
	if !strings.Contains(got, `"url":"owner/repo@abc123"`) {
		t.Fatalf("public index missing expected fields: %s", got)
	}
	if _, ok := public.Repos[0].Package.DisplayName["zh-CN"]; ok {
		t.Fatalf("public index must strip locale identical to default: %s", got)
	}
	if _, ok := stageFile.Repos[0].Package.DisplayName["zh-CN"]; !ok {
		t.Fatalf("ForPublicIndex must not mutate source stage entry locales")
	}
	if public.Repos[0].Package.DisplayName["en"] != "Demo EN" {
		t.Fatalf("public index should keep distinct locales: %s", got)
	}
}
