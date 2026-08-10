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
	"fmt"
	"maps"
	"net/http"

	"github.com/google/go-github/v89/github"
	"github.com/siyuan-note/bazaar/rules"
)

const (
	labelCIFailed = "ci-failed"
	labelCIPassed = "ci-passed"
)

// labelColors 仓库尚无该标签时创建用的默认颜色。
// 类型色避开 ci-failed 红、ci-passed 蓝；passed 用蓝而非绿，便于红绿色弱与 ci-failed 区分。
var labelColors = map[string]string{
	"plugin":      "6F42C1", // 紫
	"theme":       "1B7C83", // 青
	"icon":        "8B6914", // 深金（浅黄会触发黑字）
	"template":    "8B5A1F", // 棕
	"widget":      "B83280", // 深粉（浅粉会触发黑字）
	labelCIFailed: "B60205",
	labelCIPassed: "0969DA",
}

// managedLabelSet 本流程托管的标签：类型 + CI 状态（互斥）。
func managedLabelSet() Set {
	set := make(Set, len(rules.AllPackageTypes())+2)
	for _, t := range rules.AllPackageTypes() {
		set[t.String()] = struct{}{}
	}
	set[labelCIFailed] = struct{}{}
	set[labelCIPassed] = struct{}{}
	return set
}

// typeLabelSyncPlan 根据 diff / 解析结果计算应有的类型标签。
// 有增删，或该类型 TXT 解析失败（便于一眼看到问题类型）时纳入 expected。
func typeLabelSyncPlan(plans []typeCheckPlan) Set {
	expected := make(Set)
	for _, plan := range plans {
		name := plan.packageType.String()
		if plan.parseError != "" || len(plan.diff.New) > 0 || len(plan.diff.Deleted) > 0 {
			expected[name] = struct{}{}
		}
	}
	return expected
}

// checkResultCIPassed 判断本轮 PR Check 是否通过硬门槛。
// ParseError / FlowError / 任一包 Issues 均视为失败；无列表增删（无实际变更）视为失败；纯下架且无错误视为通过。
func checkResultCIPassed(r *CheckResult) bool {
	if r == nil {
		return false
	}
	if r.ParseError != "" || r.FlowError != "" {
		return false
	}
	hasActivity := false
	for _, list := range [][]PackageCheck{r.Plugins, r.Themes, r.Icons, r.Templates, r.Widgets} {
		if len(list) > 0 {
			hasActivity = true
		}
		for _, pc := range list {
			if len(pc.Issues) > 0 {
				return false
			}
		}
	}
	for _, deleted := range [][]string{r.PluginsDeleted, r.ThemesDeleted, r.IconsDeleted, r.TemplatesDeleted, r.WidgetsDeleted} {
		if len(deleted) > 0 {
			return true
		}
	}
	return hasActivity
}

// isNoActualChange 是否为「无实际变更」：无解析/流程错误，且无任何增删与包检查结果。
// 与 check-result 模板中「无实际变更」展示条件一致；有 Issues 的包检查结果不算无变更。
func isNoActualChange(r *CheckResult) bool {
	if r == nil {
		return true
	}
	if r.ParseError != "" || r.FlowError != "" {
		return false
	}
	for _, list := range [][]PackageCheck{r.Plugins, r.Themes, r.Icons, r.Templates, r.Widgets} {
		if len(list) > 0 {
			return false
		}
	}
	for _, deleted := range [][]string{r.PluginsDeleted, r.ThemesDeleted, r.IconsDeleted, r.TemplatesDeleted, r.WidgetsDeleted} {
		if len(deleted) > 0 {
			return false
		}
	}
	return true
}

// ciStatusLabel 根据检查结果返回应挂的 CI 标签（ci-failed / ci-passed 互斥）。
func ciStatusLabel(passed bool) string {
	if passed {
		return labelCIPassed
	}
	return labelCIFailed
}

// buildPRLabelsAfterSync 保留非托管标签，再接上 expected 类型标签与唯一 CI 状态标签。
// 托管范围：类型标签 + ci-failed / ci-passed；其它标签（如 Check、ci-skip）原样保留。
func buildPRLabelsAfterSync(current []string, expectedTypes Set, ciPassed bool) []string {
	managed := managedLabelSet()
	next := make([]string, 0, len(current)+len(expectedTypes)+1)
	for _, name := range current {
		if _, ok := managed[name]; ok {
			continue
		}
		next = append(next, name)
	}
	next = append(next, setKeysInTypeOrder(expectedTypes)...)
	next = append(next, ciStatusLabel(ciPassed))
	return next
}

func sameLabelNames(a, b []string) bool {
	as := make(Set, len(a))
	for _, n := range a {
		as[n] = struct{}{}
	}
	bs := make(Set, len(b))
	for _, n := range b {
		bs[n] = struct{}{}
	}
	return maps.Equal(as, bs)
}

// syncPRLabels 同步类型标签与 CI 状态标签：缺则补、多则删；不改动其它标签。
// 须在检查结果产出后调用，以便按 ParseError / FlowError / Issues 打 ci-failed 或 ci-passed。
// 用 Replace 一次写回，尽量减少 API 调用；失败只记日志，不中断检查。
func syncPRLabels(plans []typeCheckPlan, checkResult *CheckResult) {
	owner, repo, prNumber, ok := prIdentity()
	if !ok {
		logger.Infof("skip PR labels sync: PR_NUMBER or GITHUB_REPOSITORY not set / invalid")
		return
	}

	expectedTypes := typeLabelSyncPlan(plans)
	ciPassed := checkResultCIPassed(checkResult)
	current, err := listPRLabelNames(owner, repo, prNumber)
	if err != nil {
		logger.Errorf("list PR #%d labels failed: %s", prNumber, err)
		return
	}

	next := buildPRLabelsAfterSync(current, expectedTypes, ciPassed)
	if sameLabelNames(current, next) {
		logger.Infof("PR #%d labels already match: types=%v ci=%s", prNumber, setKeysInTypeOrder(expectedTypes), ciStatusLabel(ciPassed))
		return
	}

	managed := managedLabelSet()
	toEnsure := make(Set)
	for _, name := range next {
		if _, ok := managed[name]; ok {
			toEnsure[name] = struct{}{}
		}
	}

	if err := replacePRLabels(owner, repo, prNumber, next); err != nil {
		// 多半是仓库尚无某标签：补齐后重试一次
		if ensureErr := ensureRepoLabels(owner, repo, toEnsure); ensureErr != nil {
			logger.Errorf("ensure labels failed: %s (replace err: %s)", ensureErr, err)
			return
		}
		if err := replacePRLabels(owner, repo, prNumber, next); err != nil {
			logger.Errorf("replace PR #%d labels failed: %s", prNumber, err)
			return
		}
	}
	logger.Infof("synced PR #%d labels: %v -> %v", prNumber, current, next)
}

func replacePRLabels(owner, repo string, prNumber int, names []string) error {
	_, _, err := githubRepoClient.Issues.ReplaceLabelsForIssue(githubContext, owner, repo, prNumber, names)
	return err
}

func listPRLabelNames(owner, repo string, prNumber int) ([]string, error) {
	opts := &github.ListOptions{PerPage: 100}
	var names []string
	for {
		labels, resp, err := githubRepoClient.Issues.ListLabelsByIssue(githubContext, owner, repo, prNumber, opts)
		if err != nil {
			return nil, err
		}
		for _, l := range labels {
			names = append(names, l.GetName())
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return names, nil
}

// ensureRepoLabels 确保仓库存在 names 中的标签；已存在则跳过。
func ensureRepoLabels(owner, repo string, names Set) error {
	for name := range names {
		if err := ensureRepoLabel(owner, repo, name); err != nil {
			return err
		}
	}
	return nil
}

// labelColor 返回创建仓库标签时使用的默认颜色。
func labelColor(name string) string {
	if c, ok := labelColors[name]; ok {
		return c
	}
	return "CCCCCC"
}

// labelDescription 返回创建仓库标签时使用的说明。
func labelDescription(name string) string {
	switch name {
	case labelCIFailed:
		return "PR Check / Pkg Check failed"
	case labelCIPassed:
		return "PR Check / Pkg Check passed"
	default:
		return fmt.Sprintf("Bazaar package type: %s", name)
	}
}

// ensureRepoLabel 确保仓库存在该标签；不存在则按默认颜色与说明创建。
func ensureRepoLabel(owner, repo, name string) error {
	_, resp, err := githubRepoClient.Issues.GetLabel(githubContext, owner, repo, name)
	if err == nil {
		return nil
	}
	if resp == nil || resp.StatusCode != http.StatusNotFound {
		return err
	}
	color := labelColor(name)
	desc := labelDescription(name)
	_, _, err = githubRepoClient.Issues.CreateLabel(githubContext, owner, repo, &github.Label{
		Name:        new(name),
		Color:       new(color),
		Description: new(desc),
	})
	if err != nil {
		// 并发创建时可能已存在
		if _, _, getErr := githubRepoClient.Issues.GetLabel(githubContext, owner, repo, name); getErr == nil {
			return nil
		}
		return err
	}
	logger.Infof("created repo label %q", name)
	return nil
}

// setKeysInTypeOrder 按集市包类型固定顺序输出 s 中的键（plugin → theme → …）。
func setKeysInTypeOrder(s Set) []string {
	keys := make([]string, 0, len(s))
	for _, t := range rules.AllPackageTypes() {
		name := t.String()
		if _, ok := s[name]; ok {
			keys = append(keys, name)
		}
	}
	return keys
}
