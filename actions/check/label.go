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
	"net/http"
	"slices"

	"github.com/google/go-github/v89/github"
	"github.com/siyuan-note/bazaar/rules"
)

// packageTypeLabelColors 类型标签默认颜色（仓库尚无该标签时创建用）。
var packageTypeLabelColors = map[string]string{
	"plugin":   "0E8A16",
	"theme":    "1D76DB",
	"icon":     "FBCA04",
	"template": "D93F0B",
	"widget":   "B60205",
}

// packageTypeLabelSet 全部集市包类型标签名。
func packageTypeLabelSet() Set {
	set := make(Set, len(rules.AllPackageTypes()))
	for _, t := range rules.AllPackageTypes() {
		set[t.String()] = struct{}{}
	}
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

// buildPRLabelsAfterTypeSync 保留非类型标签，再按固定顺序接上 expected 类型标签。
func buildPRLabelsAfterTypeSync(current []string, expected Set) []string {
	typeLabels := packageTypeLabelSet()
	next := make([]string, 0, len(current)+len(expected))
	for _, name := range current {
		if _, isType := typeLabels[name]; !isType {
			next = append(next, name)
		}
	}
	return append(next, setKeysInTypeOrder(expected)...)
}

func sameLabelNames(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	as := slices.Clone(a)
	bs := slices.Clone(b)
	slices.Sort(as)
	slices.Sort(bs)
	return slices.Equal(as, bs)
}

// syncPRTypeLabels 按本 PR 实际涉及的 *.txt 同步类型标签：缺则补、多则删；不改动非类型标签。
// 用 Replace 一次写回，尽量减少 API 调用；失败只记日志，不中断检查。
func syncPRTypeLabels(plans []typeCheckPlan) {
	owner, repo, prNumber, ok := prIdentity()
	if !ok {
		logger.Infof("skip PR type labels sync: PR_NUMBER or GITHUB_REPOSITORY not set / invalid")
		return
	}

	expected := typeLabelSyncPlan(plans)
	current, err := listPRLabelNames(owner, repo, prNumber)
	if err != nil {
		logger.Errorf("list PR #%d labels failed: %s", prNumber, err)
		return
	}

	next := buildPRLabelsAfterTypeSync(current, expected)
	if sameLabelNames(current, next) {
		logger.Infof("PR #%d type labels already match expected %v", prNumber, setKeysInTypeOrder(expected))
		return
	}

	if err := replacePRLabels(owner, repo, prNumber, next); err != nil {
		// 多半是仓库尚无某类型标签：补齐 expected 后重试一次
		if ensureErr := ensureRepoLabels(owner, repo, expected); ensureErr != nil {
			logger.Errorf("ensure type labels failed: %s (replace err: %s)", ensureErr, err)
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
	_, _, err := githubClient.Issues.ReplaceLabelsForIssue(githubContext, owner, repo, prNumber, names)
	return err
}

func listPRLabelNames(owner, repo string, prNumber int) ([]string, error) {
	opts := &github.ListOptions{PerPage: 100}
	var names []string
	for {
		labels, resp, err := githubClient.Issues.ListLabelsByIssue(githubContext, owner, repo, prNumber, opts)
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

// ensureRepoLabels 确保仓库存在 names 中的类型标签；已存在则跳过。
func ensureRepoLabels(owner, repo string, names Set) error {
	for _, name := range setKeysInTypeOrder(names) {
		if err := ensureRepoLabel(owner, repo, name); err != nil {
			return err
		}
	}
	return nil
}

// ensureRepoLabel 确保仓库存在该类型标签；不存在则按默认颜色创建。
func ensureRepoLabel(owner, repo, name string) error {
	_, resp, err := githubClient.Issues.GetLabel(githubContext, owner, repo, name)
	if err == nil {
		return nil
	}
	if resp == nil || resp.StatusCode != http.StatusNotFound {
		return err
	}
	_, _, err = githubClient.Issues.CreateLabel(githubContext, owner, repo, &github.Label{
		Name:        new(name),
		Color:       new(packageTypeLabelColors[name]),
		Description: new(fmt.Sprintf("Bazaar package type: %s", name)),
	})
	if err != nil {
		// 并发创建时可能已存在
		if _, _, getErr := githubClient.Issues.GetLabel(githubContext, owner, repo, name); getErr == nil {
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
