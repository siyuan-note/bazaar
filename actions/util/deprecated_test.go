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
	"reflect"
	"strings"
	"testing"

	"github.com/siyuan-note/bazaar/rules"
)

const validDeprecatedRegistryJSON = `{
  "plugins": {
    "old/plugin": {
      "reason": {
        "default": "No longer maintained",
        "zh-CN": "不再维护"
      },
      "alternatives": ["new/plugin"]
    }
  },
  "themes": {},
  "icons": {},
  "templates": {},
  "widgets": {}
}`

func TestParseDeprecatedRegistry(t *testing.T) {
	registry, err := ParseDeprecatedRegistry("deprecated.json", []byte(validDeprecatedRegistryJSON))
	if err != nil {
		t.Fatal(err)
	}
	entry := registry.Plugins["old/plugin"]
	if entry.Reason["zh-CN"] != "不再维护" || len(entry.Alternatives) != 1 || entry.Alternatives[0] != "new/plugin" {
		t.Fatalf("unexpected entry: %#v", entry)
	}
}

func TestParseDeprecatedRegistryRejectsInvalidStructure(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{
			name: "duplicate key",
			data: `{"plugins": {}, "plugins": {}, "themes": {}, "icons": {}, "templates": {}, "widgets": {}}`,
			want: "重复键",
		},
		{
			name: "unknown root field",
			data: `{"plugins": {}, "themes": {}, "icons": {}, "templates": {}, "widgets": {}, "extra": {}}`,
			want: "unknown field",
		},
		{
			name: "missing type",
			data: `{"plugins": {}, "themes": {}, "icons": {}, "templates": {}}`,
			want: "缺少必填类型字段",
		},
		{
			name: "null type",
			data: `{"plugins": null, "themes": {}, "icons": {}, "templates": {}, "widgets": {}}`,
			want: "不能为 `null`",
		},
		{
			name: "unknown entry field",
			data: `{"plugins": {"old/plugin": {"note": "x"}}, "themes": {}, "icons": {}, "templates": {}, "widgets": {}}`,
			want: "unknown field",
		},
		{
			name: "null reason",
			data: `{"plugins": {"old/plugin": {"reason": null}}, "themes": {}, "icons": {}, "templates": {}, "widgets": {}}`,
			want: "不能为 `null`",
		},
		{
			name: "invalid utf8",
			data: string([]byte{'{', 0xff, '}'}),
			want: "UTF-8",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseDeprecatedRegistry("deprecated.json", []byte(test.data))
			if err == nil || !strings.Contains(err.Error()+localizedZH(err), test.want) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestValidateDeprecatedRegistry(t *testing.T) {
	registry := &DeprecatedRegistry{
		Plugins: map[string]DeprecatedEntry{
			"old/plugin": {
				Reason: rules.LocaleStrings{"": "已停止维护", "zh-CN": "   "},
				Alternatives: []string{
					"old/plugin",
					"new/plugin",
					"new/plugin",
					"missing/plugin",
					"widget/one",
				},
			},
		},
		Themes:    map[string]DeprecatedEntry{},
		Icons:     map[string]DeprecatedEntry{},
		Templates: map[string]DeprecatedEntry{},
		Widgets:   map[string]DeprecatedEntry{},
	}
	repos := map[rules.PackageType][]string{
		rules.TypePlugin:   {"old/plugin", "new/plugin"},
		rules.TypeTheme:    {},
		rules.TypeIcon:     {},
		rules.TypeTemplate: {},
		rules.TypeWidget:   {"widget/one"},
	}
	issues := ValidateDeprecatedRegistry(registry, repos)
	joined := issuesText(issues)
	for _, want := range []string{"必须提供 `default`", "不能为空或仅含空白字符", "名称为空或仅含空白字符的语言键", "源包自身", "重复", "不在任何集市包列表"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("issues missing %q: %s", want, joined)
		}
	}
	// 跨类型替代包（widget/one）只在一个类型列表中出现，不得报任何问题。
	if strings.Contains(joined, "alternatives[4]") {
		t.Fatalf("cross-type alternative must be accepted: %s", joined)
	}
}

// 替代包可以跨类型，但必须恰属于一个类型列表，否则无法确定包类型。
func TestValidateDeprecatedAlternativesCrossType(t *testing.T) {
	registry := &DeprecatedRegistry{
		Plugins: map[string]DeprecatedEntry{
			"old/plugin": {Alternatives: []string{"widget/one", "dup/two"}},
		},
		Themes:    map[string]DeprecatedEntry{},
		Icons:     map[string]DeprecatedEntry{},
		Templates: map[string]DeprecatedEntry{},
		Widgets:   map[string]DeprecatedEntry{},
	}
	repos := map[rules.PackageType][]string{
		rules.TypePlugin:   {"old/plugin"},
		rules.TypeTheme:    {},
		rules.TypeIcon:     {"dup/two"},
		rules.TypeTemplate: {},
		rules.TypeWidget:   {"widget/one", "dup/two"},
	}
	joined := issuesText(ValidateDeprecatedRegistry(registry, repos))
	if strings.Contains(joined, "alternatives[0]") {
		t.Fatalf("unambiguous cross-type alternative must be accepted: %s", joined)
	}
	if !strings.Contains(joined, "alternatives[1]") || !strings.Contains(joined, "无法确定其包类型") {
		t.Fatalf("ambiguous alternative must be rejected: %s", joined)
	}
}

func TestChangedDeprecatedTypes(t *testing.T) {
	before, err := ParseDeprecatedRegistry("deprecated.json", []byte(validDeprecatedRegistryJSON))
	if err != nil {
		t.Fatal(err)
	}
	after, err := ParseDeprecatedRegistry("deprecated.json", []byte(strings.Replace(validDeprecatedRegistryJSON, `"themes": {}`, `"themes": {"old/theme": {}}`, 1)))
	if err != nil {
		t.Fatal(err)
	}
	changed := ChangedDeprecatedTypes(before, after)
	if len(changed) != 1 || changed[0] != rules.TypeTheme {
		t.Fatalf("changed = %v, want theme", changed)
	}
}

func TestApplyDeprecatedRegistry(t *testing.T) {
	pluginRepos := []StageRepo{
		{
			URL: "old/plugin@aaaaaaa",
			Package: rules.Package{
				Name:             "old-package",
				Deprecated:       true,
				DeprecatedReason: rules.LocaleStrings{"default": "stale"},
				Alternatives:     []string{"stale-package"},
				Choices:          []rules.PackageChoice{{PackageType: "plugins", Name: "stale-package"}},
			},
		},
		{
			URL: "new/plugin@bbbbbbb",
			Package: rules.Package{
				Name:             "new-package",
				Deprecated:       true,
				DeprecatedReason: rules.LocaleStrings{"default": "stale"},
				Alternatives:     []string{"stale-package"},
				Choices:          []rules.PackageChoice{{PackageType: "plugins", Name: "stale-package"}},
			},
		},
	}
	widgetRepos := []StageRepo{
		{URL: "widget/one@ccccccc", Package: rules.Package{Name: "widget-package"}},
	}
	registry := &DeprecatedRegistry{
		Plugins: map[string]DeprecatedEntry{
			"old/plugin": {
				Reason: rules.LocaleStrings{
					"default": "archived <script>",
					"zh-CN":   "停止维护",
				},
				Alternatives: []string{"old/plugin", " new/plugin ", "new/plugin", "new/plugin", "missing/plugin", "widget/one"},
			},
			"missing/source": {},
		},
	}
	targets := NewDeprecatedTargets(map[rules.PackageType][]StageRepo{
		rules.TypePlugin: pluginRepos,
		rules.TypeWidget: widgetRepos,
	})
	warnings := ApplyDeprecatedRegistry(rules.TypePlugin, pluginRepos, registry, targets)
	if len(warnings) == 0 {
		t.Fatal("expected defensive warnings")
	}
	oldPackage := pluginRepos[0].Package
	if !oldPackage.Deprecated {
		t.Fatal("source should be deprecated")
	}
	if got := oldPackage.DeprecatedReason["default"]; got != "archived &lt;script&gt;" {
		t.Fatalf("sanitized reason = %q", got)
	}
	// alternatives 只收同类型目标：旧客户端按源包类型打开替代包，跨类型会查不到。
	if len(oldPackage.Alternatives) != 1 || oldPackage.Alternatives[0] != "new-package" {
		t.Fatalf("alternatives = %#v", oldPackage.Alternatives)
	}
	wantChoices := []rules.PackageChoice{
		{PackageType: "plugins", Name: "new-package"},
		{PackageType: "widgets", Name: "widget-package"},
	}
	if !reflect.DeepEqual(oldPackage.Choices, wantChoices) {
		t.Fatalf("choices = %#v, want %#v", oldPackage.Choices, wantChoices)
	}
	newPackage := pluginRepos[1].Package
	if newPackage.Deprecated || newPackage.DeprecatedReason != nil || newPackage.Alternatives != nil || newPackage.Choices != nil {
		t.Fatalf("unregistered package retained stale generated fields: %#v", newPackage)
	}
}

func localizedZH(err error) string {
	zh, _, _ := rules.AsLocalized(err)
	return zh
}

func issuesText(issues []rules.Issue) string {
	var builder strings.Builder
	for _, issue := range issues {
		builder.WriteString(issue.MessageZh)
		builder.WriteByte('\n')
	}
	return builder.String()
}
