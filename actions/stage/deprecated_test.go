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
	"reflect"
	"testing"

	"github.com/siyuan-note/bazaar/actions/util"
	"github.com/siyuan-note/bazaar/rules"
)

// TestApplyDeprecatedRegistryCrossType 验证统一后处理：跨类型替代包能解析到目标包名并写入 choices，
// 但不进 alternatives；未被 staged 的类型保持原样。
func TestApplyDeprecatedRegistryCrossType(t *testing.T) {
	root := t.TempDir()
	writeStageRepoFixtures(t, root, map[rules.PackageType][]util.StageRepo{
		rules.TypePlugin: {{URL: "old/plugin@aaaaaaa", Package: rules.Package{Name: "old-package"}}},
		rules.TypeWidget: {{URL: "widget/one@bbbbbbb", Package: rules.Package{Name: "widget-package"}}},
	})

	registry := &util.DeprecatedRegistry{
		Plugins: map[string]util.DeprecatedEntry{
			"old/plugin": {Alternatives: []string{"widget/one"}},
		},
	}
	if err := applyDeprecatedRegistry(root, []rules.PackageType{rules.TypePlugin}, registry); err != nil {
		t.Fatal(err)
	}

	staged, err := util.ReadStageFile(filepath.Join(root, "stage", rules.TypePlugin.StageJSONFile()))
	if err != nil {
		t.Fatal(err)
	}
	if len(staged.Repos) != 1 || !staged.Repos[0].Package.Deprecated {
		t.Fatalf("unexpected staged repos: %#v", staged.Repos)
	}
	// 纯跨类型目标不进 alternatives：旧客户端按源包类型打开，写进去只会查到「未找到集市包」。
	if len(staged.Repos[0].Package.Alternatives) != 0 {
		t.Fatalf("alternatives = %#v, want empty", staged.Repos[0].Package.Alternatives)
	}
	want := []rules.PackageChoice{{PackageType: "widgets", Name: "widget-package"}}
	if !reflect.DeepEqual(staged.Repos[0].Package.Choices, want) {
		t.Fatalf("choices = %#v, want %#v", staged.Repos[0].Package.Choices, want)
	}

	// 本轮未 staged 的类型不得被改写（只有本项目主动重写的类型才回写）。
	untouched, err := util.ReadStageFile(filepath.Join(root, "stage", rules.TypeWidget.StageJSONFile()))
	if err != nil {
		t.Fatal(err)
	}
	if untouched.Repos[0].Package.Deprecated || untouched.Repos[0].Package.Choices != nil {
		t.Fatalf("unstaged type must not be rewritten: %#v", untouched.Repos[0].Package)
	}
}

// writeStageRepoFixtures 按类型写出 stage JSON 夹具；未列出的类型不建文件（ReadStageFile 视为空）。
func writeStageRepoFixtures(t *testing.T, root string, reposByType map[rules.PackageType][]util.StageRepo) {
	t.Helper()
	stageDir := filepath.Join(root, "stage")
	if err := os.MkdirAll(stageDir, 0755); err != nil {
		t.Fatal(err)
	}
	for packageType, repos := range reposByType {
		data, err := marshalSortedIndentedJSON(util.StageFile{Repos: repos})
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(stageDir, packageType.StageJSONFile()), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
}
