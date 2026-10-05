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
	"bytes"
	"strings"
	"testing"

	"github.com/siyuan-note/bazaar/rules"
)

// TestWriteReportSections 校验报告各节的归属关系。
// 归档状态可被作者取消，因此「已登记弃用但仓库未归档」必须单独成节：
// 若只判断「已归档」，取消归档后的陈旧弃用条目会落进所有分支的缝隙而不被报告。
func TestWriteReportSections(t *testing.T) {
	states := []repoState{
		// 已弃用 + 仓库正常（未归档）→ 新节，需复核条目是否陈旧
		{listedRepo: listedRepo{PackageType: rules.TypePlugin, OwnerRepo: "a/active", Deprecated: true},
			Exists: true, NameWithOwner: "a/active", PushedAt: "2026-10-01T02:03:04Z"},
		// 已弃用 + 已归档 → 保留在「已归档且已登记弃用」
		{listedRepo: listedRepo{PackageType: rules.TypeTheme, OwnerRepo: "b/archived", Deprecated: true},
			Exists: true, NameWithOwner: "b/archived", IsArchived: true, ArchivedAt: "2025-01-02T03:04:05Z"},
		// 未弃用 + 已归档 → 「已归档但未登记弃用」
		{listedRepo: listedRepo{PackageType: rules.TypeWidget, OwnerRepo: "c/regressed", Deprecated: false},
			Exists: true, NameWithOwner: "c/regressed", IsArchived: true, ArchivedAt: "2024-01-02T03:04:05Z"},
		// 已弃用 + 未归档 + 已改名 → 新节需同时给出新名
		{listedRepo: listedRepo{PackageType: rules.TypeIcon, OwnerRepo: "d/oldname", Deprecated: true},
			Exists: true, NameWithOwner: "e/newname", PushedAt: "2026-09-30T11:22:33Z"},
		// 未弃用 + 已禁用 + 未归档 → 「已禁用但未登记弃用」
		{listedRepo: listedRepo{PackageType: rules.TypeTemplate, OwnerRepo: "f/disabled", Deprecated: false},
			Exists: true, NameWithOwner: "f/disabled", IsDisabled: true},
	}

	var buf bytes.Buffer
	writeReport(&buf, states)
	report := buf.String()
	sections := sectionsOf(report)

	unarchived := sections["已登记弃用但仓库未归档（2）"]
	archivedDeprecated := sections["已归档且已登记弃用（1）"]

	checks := []struct {
		name string
		got  bool
	}{
		{"汇总行：未归档的弃用条目计 2", strings.Contains(report, "- 已登记弃用但仓库未归档：2")},
		{"汇总行：已归档合计 2（未登记弃用 1 / 已登记弃用 1）",
			strings.Contains(report, "- 已归档：2（未登记弃用 1 / 已登记弃用 1）")},
		{"汇总行：已禁用且未登记弃用计 1", strings.Contains(report, "- 已禁用且未登记弃用：1")},
		{"新节存在且计数为 2", unarchived != ""},
		{"新节含未归档的弃用条目 a/active", strings.Contains(unarchived, "`a/active`")},
		{"新节含已改名条目 d/oldname 并给出新名 e/newname",
			strings.Contains(unarchived, "`d/oldname`") && strings.Contains(unarchived, "`e/newname`")},
		{"新节显示最后推送时间（用于判断是否已恢复维护）", strings.Contains(unarchived, "2026-10-01 02:03")},
		{"新节不含已归档条目 b/archived", !strings.Contains(unarchived, "b/archived")},
		{"新节不含未弃用的禁用条目 f/disabled", !strings.Contains(unarchived, "f/disabled")},
		{"已归档且已登记弃用节仍含 b/archived", strings.Contains(archivedDeprecated, "`b/archived`")},
		{"已归档但未登记弃用节仍含 c/regressed",
			strings.Contains(sections["已归档但未登记弃用（1）"], "`c/regressed`")},
		{"已禁用但未登记弃用节仍含 f/disabled",
			strings.Contains(sections["已禁用但未登记弃用（1）"], "`f/disabled`")},
	}
	for _, check := range checks {
		if !check.got {
			t.Errorf("FAIL %s", check.name)
		}
	}

	// 已弃用 + 已禁用 + 未归档：应进入新节并标为禁用，且不重复计入「已禁用但未登记弃用」。
	var disabled bytes.Buffer
	writeReport(&disabled, []repoState{{
		listedRepo: listedRepo{PackageType: rules.TypeIcon, OwnerRepo: "g/disabled-deprecated", Deprecated: true},
		Exists:     true, NameWithOwner: "g/disabled-deprecated", IsDisabled: true,
	}})
	disabledReport := disabled.String()
	if !strings.Contains(disabledReport, "已被禁用 / Disabled") {
		t.Errorf("FAIL 被禁用的未归档弃用条目应标为「已被禁用 / Disabled」")
	}
	if !strings.Contains(disabledReport, "## 已禁用但未登记弃用（0）") {
		t.Errorf("FAIL 已弃用的禁用条目不应重复进入「已禁用但未登记弃用」节")
	}

	// 未归档的活跃仓库不能因「无归档时间」被误标为禁用。
	if strings.Contains(unarchived, "已被禁用") {
		t.Errorf("FAIL 未归档的活跃仓库被误标为已禁用")
	}
}

// sectionsOf 把报告按 `## ` 标题切成 标题 -> 正文，便于逐节断言。
func sectionsOf(report string) map[string]string {
	sections := map[string]string{}
	for _, part := range strings.Split(report, "\n## ") {
		title, body, found := strings.Cut(part, "\n")
		if !found {
			continue
		}
		sections[strings.TrimSpace(title)] = body
	}
	return sections
}
