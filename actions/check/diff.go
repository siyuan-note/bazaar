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

// formatOnePackageLimitError 生成「一次只能添加/更改一个包」的双语说明（含涉及仓库列表）。
// total 为添加或更换维护者的合计数量（须 > 1）。
func formatOnePackageLimitError(total int, plans []typeCheckPlan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "本 PR 添加或更换了 %d 个集市包，但一次只能添加或更换 1 个（下架不限数量）。请将每个包拆成独立的 Pull Request 后再提交。\n\n", total)
	fmt.Fprintf(&b, "This PR adds or changes %d bazaar packages, but only 1 package may be added or have its maintainer changed per PR (delistings are unlimited). Please split each package into its own Pull Request.\n\n", total)

	b.WriteString("涉及的仓库 / Involved repos:\n")
	for _, plan := range plans {
		if len(plan.diff.New) == 0 {
			continue
		}
		fmt.Fprintf(&b, "- `%s`: %s\n", plan.packageType.ReposListFile(), strings.Join(plan.diff.New, ", "))
	}
	return b.String()
}
