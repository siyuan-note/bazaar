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
	_ "embed"
	"strings"
	"text/template"
)

//go:embed flow-error.md.tpl
var flowErrorTemplateText string

var flowErrorTemplate = template.Must(template.New("flow-error.md.tpl").Parse(flowErrorTemplateText))

// flowRepoGroup 流程错误模板中按列表文件分组的仓库。
type flowRepoGroup struct {
	ListFile string
	Repos     string
}

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
		return formatMixedAddDelistError(plans)
	}
	return ""
}

// formatOnePackageLimitError 生成「一次只能添加/更改一个包」的双语说明（含涉及仓库列表）。
// total 为添加或更换维护者的合计数量（须 > 1）。
func formatOnePackageLimitError(total int, plans []typeCheckPlan) string {
	return executeFlowError("onePackageLimit", struct {
		Total   int
		NewRepos []flowRepoGroup
	}{
		Total:   total,
		NewRepos: collectNewRepoGroups(plans),
	})
}

// formatMixedAddDelistError 生成「新增/换维护者与无关下架混用」的双语说明。
func formatMixedAddDelistError(plans []typeCheckPlan) string {
	return executeFlowError("mixedAddDelist", struct {
		NewRepos    []flowRepoGroup
		PureDeleted []flowRepoGroup
	}{
		NewRepos:    collectNewRepoGroups(plans),
		PureDeleted: collectPureDeletedGroups(plans),
	})
}

func collectNewRepoGroups(plans []typeCheckPlan) []flowRepoGroup {
	groups := make([]flowRepoGroup, 0)
	for _, plan := range plans {
		if plan.parseError != "" || len(plan.diff.New) == 0 {
			continue
		}
		groups = append(groups, flowRepoGroup{
			ListFile: plan.packageType.ReposListFile(),
			Repos:     strings.Join(plan.diff.New, ", "),
		})
	}
	return groups
}

func collectPureDeletedGroups(plans []typeCheckPlan) []flowRepoGroup {
	groups := make([]flowRepoGroup, 0)
	for _, plan := range plans {
		if plan.parseError != "" {
			continue
		}
		var paths []string
		for _, path := range plan.diff.Deleted {
			if isPreviousRepo(plan.diff, path) {
				continue
			}
			paths = append(paths, path)
		}
		if len(paths) == 0 {
			continue
		}
		groups = append(groups, flowRepoGroup{
			ListFile: plan.packageType.ReposListFile(),
			Repos:     strings.Join(paths, ", "),
		})
	}
	return groups
}

func executeFlowError(name string, data any) string {
	var b bytes.Buffer
	if err := flowErrorTemplate.ExecuteTemplate(&b, name, data); err != nil {
		panic("flow error template " + name + ": " + err.Error())
	}
	return b.String()
}
