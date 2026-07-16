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
	"strings"
	"testing"

	"github.com/siyuan-note/bazaar/rules"
)

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
				},
			},
		},
	}

	data, err := json.Marshal(stageFile.ForPublicIndex())
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
}
