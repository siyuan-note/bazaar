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
	"path/filepath"
	"reflect"
	"slices"

	"github.com/siyuan-note/bazaar/actions/util"
	"github.com/siyuan-note/bazaar/rules"
)

const (
	deprecationActionAdd    = "add"
	deprecationActionUpdate = "update"
	deprecationActionRemove = "remove"
)

// checkDeprecationRegistry 对比当前 main 与 PR head，并校验 PR 产生的有效变更。
func checkDeprecationRegistry() *DeprecationCheck {
	mainRegistry, err := util.ReadDeprecatedRegistry(filepath.Join(BAZAAR_HEAD_PATH, util.DeprecatedRegistryRelPath))
	if err != nil {
		return &DeprecationCheck{Issues: []rules.Issue{rules.IssueFromErr(err)}}
	}
	prRegistry, err := util.ReadDeprecatedRegistry(filepath.Join(PR_HEAD_PATH, util.DeprecatedRegistryRelPath))
	if err != nil {
		return &DeprecationCheck{Issues: []rules.Issue{rules.IssueFromErr(err)}}
	}
	changedTypes := util.ChangedDeprecatedTypes(mainRegistry, prRegistry)
	if len(changedTypes) == 0 {
		return nil
	}

	result := &DeprecationCheck{
		Types:   changedTypes,
		Changes: collectDeprecationChanges(mainRegistry, prRegistry, changedTypes),
	}
	reposByType, err := util.LoadReposByPackageType(BAZAAR_HEAD_PATH)
	if err != nil {
		result.Issues = append(result.Issues, rules.IssueFromErr(err))
		return result
	}
	result.Issues = append(result.Issues, util.ValidateDeprecatedRegistryTypes(prRegistry, reposByType, changedTypes)...)
	return result
}

func collectDeprecationChanges(before, after *util.DeprecatedRegistry, packageTypes []rules.PackageType) []DeprecationChange {
	var changes []DeprecationChange
	for _, packageType := range packageTypes {
		beforeEntries := before.Entries(packageType)
		afterEntries := after.Entries(packageType)
		keys := make([]string, 0, len(beforeEntries)+len(afterEntries))
		seen := map[string]struct{}{}
		for key := range beforeEntries {
			seen[key] = struct{}{}
			keys = append(keys, key)
		}
		for key := range afterEntries {
			if _, exists := seen[key]; exists {
				continue
			}
			keys = append(keys, key)
		}
		slices.Sort(keys)
		for _, ownerRepo := range keys {
			beforeEntry, existedBefore := beforeEntries[ownerRepo]
			afterEntry, existsAfter := afterEntries[ownerRepo]
			action := ""
			switch {
			case !existedBefore && existsAfter:
				action = deprecationActionAdd
			case existedBefore && !existsAfter:
				action = deprecationActionRemove
			case !reflect.DeepEqual(beforeEntry, afterEntry):
				action = deprecationActionUpdate
			}
			if action != "" {
				changes = append(changes, DeprecationChange{
					PackageType: packageType,
					OwnerRepo:   ownerRepo,
					Action:      action,
				})
			}
		}
	}
	return changes
}

func deprecationActionLabel(action string) string {
	switch action {
	case deprecationActionAdd:
		return "新增 / Add"
	case deprecationActionUpdate:
		return "更新 / Update"
	case deprecationActionRemove:
		return "移除 / Remove"
	default:
		return fmt.Sprintf("未知 / Unknown (%s)", action)
	}
}

// ActionLabel 返回评论模板使用的双语动作名称。
func (c DeprecationChange) ActionLabel() string {
	return deprecationActionLabel(c.Action)
}
