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
// 纯移除：仅 1 个用 Remove [type] owner/repo；多个用 Remove N packages。
// 调用方须已通过一次一包（addedOrChanged ≤ 1）。有 parseError，或新增同时带有纯移除时返回 ok=false。
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
			_, maintainerChanged = plan.diff.MaintainerChanged[added]
		}
		for _, path := range plan.diff.Deleted {
			if isMaintainerChangeOldSide(plan.diff, path) {
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
		return formatActionTitle("Remove", removedType, removed), true
	case added == "" && removedCount > 1:
		return fmt.Sprintf("Remove %d packages", removedCount), true
	default:
		return "", false
	}
}

// formatActionTitle 生成 Add/Remove 标题；非插件类型在动作词后插入类型名。
func formatActionTitle(action string, typ rules.PackageType, ownerRepo string) string {
	if typ != rules.TypePlugin {
		return action + " " + typ.String() + " " + ownerRepo
	}
	return action + " " + ownerRepo
}

// isMaintainerChangeOldSide 判断 deleted 是否为换维护者时的旧 owner/repo（同 name，不单独计入移除）。
func isMaintainerChangeOldSide(d repoDiff, deleted string) bool {
	_, name, ok := strings.Cut(deleted, "/")
	if !ok {
		return false
	}
	for newPath := range d.MaintainerChanged {
		_, newName, ok := strings.Cut(newPath, "/")
		if ok && newName == name {
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

	current, _, err := githubClient.PullRequests.Get(githubContext, owner, repo, prNumber)
	if err != nil {
		logger.Errorf("get PR #%d title failed: %s", prNumber, err)
		return
	}
	if current.GetTitle() == title {
		logger.Infof("PR #%d title already %q, skip update", prNumber, title)
		return
	}

	_, _, err = githubClient.PullRequests.Edit(githubContext, owner, repo, prNumber, &github.PullRequest{
		Title: new(title),
	})
	if err != nil {
		logger.Errorf("update PR #%d title to %q failed: %s", prNumber, title, err)
		return
	}
	logger.Infof("updated PR #%d title: %q -> %q", prNumber, current.GetTitle(), title)
}
