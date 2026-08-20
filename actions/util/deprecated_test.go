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
				Reason: rules.LocaleStrings{"zh-CN": "   "},
				Alternatives: []string{
					"old/plugin",
					"new/plugin",
					"new/plugin",
					"missing/plugin",
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
		rules.TypeWidget:   {},
	}
	issues := ValidateDeprecatedRegistry(registry, repos)
	joined := issuesText(issues)
	for _, want := range []string{"必须提供 `default`", "不能为空或仅含空白字符", "源包自身", "重复", "不在同类型列表"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("issues missing %q: %s", want, joined)
		}
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
	repos := []StageRepo{
		{
			URL: "old/plugin@aaaaaaa",
			Package: rules.Package{
				Name:             "old-package",
				Deprecated:       true,
				DeprecatedReason: rules.LocaleStrings{"default": "stale"},
				Alternatives:     []string{"stale-package"},
			},
		},
		{
			URL: "new/plugin@bbbbbbb",
			Package: rules.Package{
				Name:             "new-package",
				Deprecated:       true,
				DeprecatedReason: rules.LocaleStrings{"default": "stale"},
				Alternatives:     []string{"stale-package"},
			},
		},
	}
	registry := &DeprecatedRegistry{
		Plugins: map[string]DeprecatedEntry{
			"old/plugin": {
				Reason: rules.LocaleStrings{
					"default": "archived <script>",
					"zh-CN":   "停止维护",
				},
				Alternatives: []string{"old/plugin", " new/plugin ", "new/plugin", "new/plugin", "missing/plugin"},
			},
			"missing/source": {},
		},
	}
	warnings := ApplyDeprecatedRegistry(rules.TypePlugin, repos, registry)
	if len(warnings) == 0 {
		t.Fatal("expected defensive warnings")
	}
	oldPackage := repos[0].Package
	if !oldPackage.Deprecated {
		t.Fatal("source should be deprecated")
	}
	if got := oldPackage.DeprecatedReason["default"]; got != "archived &lt;script&gt;" {
		t.Fatalf("sanitized reason = %q", got)
	}
	if len(oldPackage.Alternatives) != 1 || oldPackage.Alternatives[0] != "new-package" {
		t.Fatalf("alternatives = %#v", oldPackage.Alternatives)
	}
	newPackage := repos[1].Package
	if newPackage.Deprecated || newPackage.DeprecatedReason != nil || newPackage.Alternatives != nil {
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
