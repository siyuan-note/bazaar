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
	"os"
	"strconv"
	"strings"

	"github.com/google/go-github/v89/github"
	"github.com/siyuan-note/bazaar/rules"
)

var (
	PR_NUMBER         = os.Getenv("PR_NUMBER")         // 当前 PR 编号（GitHub Actions）
	GITHUB_REPOSITORY = os.Getenv("GITHUB_REPOSITORY") // bazaar 仓库 owner/repo（GitHub Actions 自动注入）
)

// conventionalPRTitle 生成约定 PR 标题。
// 新增/换维护者：Add [type] owner/repo（插件省略类型；换维护者附 (maintainer change)）。
// 纯下架：仅 1 个用 Delist [type] owner/repo；多个用 Delist N packages。
// 调用方须已通过 validatePRListChangeFlow。有 parseError，或新增同时带有纯下架时返回 ok=false。
func conventionalPRTitle(plans []typeCheckPlan) (title string, ok bool) {
	var added string
	var addedType rules.PackageType
	var maintainerChanged bool
	var removed string
	var removedType rules.PackageType
	removedCount := 0
	for _, plan := range plans {
		if plan.parseError != "" {
			return "", false
		}
		if len(plan.diff.New) > 0 {
			added = plan.diff.New[0] // 一次一包后全局最多一个
			addedType = plan.packageType
			_, maintainerChanged = plan.diff.PreviousRepos[added]
		}
		for _, path := range plan.diff.Deleted {
			if isPreviousRepo(plan.diff, path) {
				continue
			}
			removedCount++
			removed = path
			removedType = plan.packageType
		}
	}
	switch {
	case added != "" && removedCount == 0:
		title = formatActionTitle("Add", addedType, added)
		if maintainerChanged {
			title += " (maintainer change)"
		}
		return title, true
	case added == "" && removedCount == 1:
		return formatActionTitle("Delist", removedType, removed), true
	case added == "" && removedCount > 1:
		return fmt.Sprintf("Delist %d packages", removedCount), true
	default:
		return "", false
	}
}

// conventionalDeprecationPRTitle 生成弃用注册表 PR 的约定标题。
// 单一动作且仅 1 个包：Deprecate / Restore / Update deprecation [type] owner/repo（插件省略类型）。
// 单一动作且多个包：Deprecate / Restore N packages，或 Update N deprecations。
// 混合动作：Update deprecation registry。无有效变更时 ok=false。
func conventionalDeprecationPRTitle(check *DeprecationCheck) (title string, ok bool) {
	if check == nil || len(check.Changes) == 0 {
		return "", false
	}
	action := check.Changes[0].Action
	for _, change := range check.Changes[1:] {
		if change.Action != action {
			return "Update deprecation registry", true
		}
	}
	if len(check.Changes) == 1 {
		change := check.Changes[0]
		verb := deprecationTitleAction(change.Action)
		if verb == "" {
			return "", false
		}
		return formatActionTitle(verb, change.PackageType, change.OwnerRepo), true
	}
	switch action {
	case deprecationActionAdd:
		return fmt.Sprintf("Deprecate %d packages", len(check.Changes)), true
	case deprecationActionRemove:
		return fmt.Sprintf("Restore %d packages", len(check.Changes)), true
	case deprecationActionUpdate:
		return fmt.Sprintf("Update %d deprecations", len(check.Changes)), true
	default:
		return "", false
	}
}

func deprecationTitleAction(action string) string {
	switch action {
	case deprecationActionAdd:
		return "Deprecate"
	case deprecationActionRemove:
		return "Restore"
	case deprecationActionUpdate:
		return "Update deprecation"
	default:
		return ""
	}
}

// formatActionTitle 生成 Add / Delist / Deprecate 等标题；非插件类型在动作词后插入类型名。
func formatActionTitle(action string, typ rules.PackageType, ownerRepo string) string {
	if typ != rules.TypePlugin {
		return action + " " + typ.String() + " " + ownerRepo
	}
	return action + " " + ownerRepo
}

// isPreviousRepo 判断 deleted 是否为换维护者时被替换的旧 owner/repo（不单独计入下架）。
func isPreviousRepo(d repoDiff, deleted string) bool {
	for _, previous := range d.PreviousRepos {
		if previous == deleted {
			return true
		}
	}
	return false
}

// prIdentity 解析当前 PR 的仓库与编号；缺环境变量或非法时 ok=false。
func prIdentity() (owner, repo string, prNumber int, ok bool) {
	if PR_NUMBER == "" || GITHUB_REPOSITORY == "" {
		return "", "", 0, false
	}
	n, err := strconv.Atoi(PR_NUMBER)
	if err != nil || n < 1 {
		logger.Errorf("invalid PR_NUMBER %q", PR_NUMBER)
		return "", "", 0, false
	}
	owner, repo, cutOK := strings.Cut(GITHUB_REPOSITORY, "/")
	if !cutOK || owner == "" || repo == "" {
		logger.Errorf("invalid GITHUB_REPOSITORY %q", GITHUB_REPOSITORY)
		return "", "", 0, false
	}
	return owner, repo, n, true
}

// prIsMergedOrClosed 查询 PR 是否已合并或关闭。
// 无法解析身份或 API 失败时返回 false（不跳过副作用，避免误吞 open PR 的真·无变更）。
func prIsMergedOrClosed() bool {
	owner, repo, prNumber, ok := prIdentity()
	if !ok {
		return false
	}
	pr, _, err := githubRepoClient.PullRequests.Get(githubContext, owner, repo, prNumber)
	if err != nil {
		logger.Errorf("get PR #%d state failed: %s", prNumber, err)
		return false
	}
	return pr.GetMerged() || pr.GetState() == "closed"
}

// maybeUpdatePRTitle 将 PR 标题改为约定格式；缺环境变量或已是目标标题时跳过，失败只记日志不中断检查。
func maybeUpdatePRTitle(title string) {
	if title == "" {
		return
	}
	owner, repo, prNumber, ok := prIdentity()
	if !ok {
		logger.Infof("skip PR title update: PR_NUMBER or GITHUB_REPOSITORY not set / invalid")
		return
	}

	current, _, err := githubRepoClient.PullRequests.Get(githubContext, owner, repo, prNumber)
	if err != nil {
		logger.Errorf("get PR #%d title failed: %s", prNumber, err)
		return
	}
	if current.GetTitle() == title {
		logger.Infof("PR #%d title already %q, skip update", prNumber, title)
		return
	}

	_, _, err = githubRepoClient.PullRequests.Edit(githubContext, owner, repo, prNumber, &github.PullRequest{
		Title: new(title),
	})
	if err != nil {
		logger.Errorf("update PR #%d title to %q failed: %s", prNumber, title, err)
		return
	}
	logger.Infof("updated PR #%d title: %q -> %q", prNumber, current.GetTitle(), title)
}
