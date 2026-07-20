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
	"strings"
)

// repoDiff 单类型列表相对 PR base / bazaar head 过滤后的增删结果。
type repoDiff struct {
	New           []string          // 本 PR 新增或更换维护者后的 owner/repo
	Deleted       []string          // 本 PR 下架的 owner/repo
	PreviousRepos map[string]string // 换维护者：新 owner/repo → 已删除的旧 owner/repo（键为 New 的子集）
}

// computeRepoDiff 按 base/head diff 并过滤：
// 新增 = head 有而 base 无且不在 bazaar head（多为解决冲突时从 bazaar head 合并来的）；
// 删除 = base 有而 head 无且仍在 bazaar head（确属本 PR 删除）。
func computeRepoDiff(
	headPaths []string,
	basePaths []string,
	baseSet Set,
	headSet Set,
	bazaarHeadSet Set,
	baseNameToOwner map[string]string,
) repoDiff {
	newRepos := make([]string, 0)
	for _, path := range headPaths {
		_, inBase := baseSet[path]
		_, inBazaarHead := bazaarHeadSet[path]
		if !inBase && !inBazaarHead {
			newRepos = append(newRepos, path)
		}
	}
	deletedRepos := make([]string, 0)
	for _, path := range basePaths {
		_, inHead := headSet[path]
		_, inBazaarHead := bazaarHeadSet[path]
		if !inHead && inBazaarHead {
			deletedRepos = append(deletedRepos, path)
		}
	}

	// 更换维护者：同 GitHub 仓库名、不同 owner，且旧 owner/repo 已从列表删除。
	// 若旧路径仍在列表，不算换维护者（按纯新包处理，包名唯一性会拦住重名）。
	deletedSet := make(Set, len(deletedRepos))
	for _, path := range deletedRepos {
		deletedSet[path] = struct{}{}
	}
	previousRepos := make(map[string]string)
	for _, path := range newRepos {
		// newRepos 中既有纯新增，也可能含更换维护者后的新 owner/repo
		newOwner, name, ok := strings.Cut(path, "/")
		if !ok {
			continue
		}
		oldOwner, oldExists := baseNameToOwner[name]
		if !oldExists {
			// base 中不存在该 name，是新增，不是更换维护者
			continue
		}
		if oldOwner == newOwner {
			continue
		}
		oldPath := oldOwner + "/" + name
		if _, removed := deletedSet[oldPath]; !removed {
			// 旧路径未删除：与已有仓库 GitHub 名冲突，不当作换维护者
			continue
		}
		previousRepos[path] = oldPath
	}

	return repoDiff{
		New:           newRepos,
		Deleted:       deletedRepos,
		PreviousRepos: previousRepos,
	}
}

// validatePRListChangeFlow 校验列表变更形态。
// 仅允许：
// 1. 纯新增：跨类型合计恰好新增 1 个，且无下架；
// 2. 更换维护者：跨类型合计恰好新增 1 个，且仅删除与其同类型、同 GitHub 仓库名的旧 owner/repo（无额外下架）；
// 3. 纯下架：无新增，删除一个或多个。
// 违反时返回双语 FlowError；通过返回空串。parseError 的类型跳过统计（由 ParseError 另行展示）。
func validatePRListChangeFlow(plans []typeCheckPlan) string {
	totalNew := 0
	var pureDeleted []string
	for _, plan := range plans {
		if plan.parseError != "" {
			continue
		}
		totalNew += len(plan.diff.New)
		for _, path := range plan.diff.Deleted {
			if isPreviousRepo(plan.diff, path) {
				continue
			}
			pureDeleted = append(pureDeleted, path)
		}
	}
	if totalNew > 1 {
		return formatOnePackageLimitError(totalNew, plans)
	}
	if totalNew == 1 && len(pureDeleted) > 0 {
		return formatMixedAddDelistError(plans, pureDeleted)
	}
	return ""
}

// formatOnePackageLimitError 生成「一次只能添加/更改一个包」的双语说明（含涉及仓库列表）。
// total 为添加或更换维护者的合计数量（须 > 1）。
func formatOnePackageLimitError(total int, plans []typeCheckPlan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "本 PR 添加或更换了 %d 个集市包，但每个 Pull Request 最多只能添加或更换 1 个包。请将每个包拆成独立的 Pull Request 后再提交。\n\n", total)
	fmt.Fprintf(&b, "This PR adds or changes %d bazaar packages, but each Pull Request may add or change at most 1 package. Please split each package into its own Pull Request.\n\n", total)
	b.WriteString(prFlowShapeRulesHint())
	b.WriteString("涉及的仓库 / Involved repos:\n")
	appendNewReposList(&b, plans)
	return b.String()
}

// formatMixedAddDelistError 生成「新增/换维护者与无关下架混用」的双语说明。
func formatMixedAddDelistError(plans []typeCheckPlan, pureDeleted []string) string {
	var b strings.Builder
	b.WriteString("本 PR 在添加或更换集市包的同时，还下架了其他包。添加/更换与无关下架不能放在同一个 Pull Request 中。\n\n")
	b.WriteString("This PR both adds or changes a bazaar package and delists unrelated package(s). Adding/changing and unrelated delistings cannot be combined in the same Pull Request.\n\n")
	b.WriteString(prFlowShapeRulesHint())
	b.WriteString("涉及的新增或更换 / Involved add or change:\n")
	appendNewReposList(&b, plans)
	b.WriteString("涉及的无关下架 / Involved unrelated delistings:\n")
	for _, path := range pureDeleted {
		fmt.Fprintf(&b, "- `%s`\n", path)
	}
	return b.String()
}

func prFlowShapeRulesHint() string {
	return "每个 Pull Request 仅允许以下之一：\n" +
		"1. 仅添加 1 个新包；\n" +
		"2. 更换维护者（添加 1 个新 `owner/repo`，并删除同类型、同 GitHub 仓库名的旧 `owner/repo`）；\n" +
		"3. 仅下架一个或多个包。\n\n" +
		"Each Pull Request may only be one of:\n" +
		"1. Add exactly 1 new package;\n" +
		"2. Change maintainer (add 1 new `owner/repo` and delete the old `owner/repo` with the same type and same GitHub repository name);\n" +
		"3. Delist one or more packages only.\n\n"
}

func appendNewReposList(b *strings.Builder, plans []typeCheckPlan) {
	for _, plan := range plans {
		if plan.parseError != "" || len(plan.diff.New) == 0 {
			continue
		}
		fmt.Fprintf(b, "- `%s`: %s\n", plan.packageType.ReposListFile(), strings.Join(plan.diff.New, ", "))
	}
}
