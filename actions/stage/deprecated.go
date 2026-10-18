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
	"path/filepath"

	"github.com/siyuan-note/bazaar/actions/util"
	"github.com/siyuan-note/bazaar/rules"
)

// applyDeprecatedRegistry 在全部类型写盘后统一应用弃用注册表。
//
// 不再在 performStage 内按类型就地应用：替代包可以跨类型，解析包名需要读取目标类型的 Stage，
// 而 performStage 按 plugins → themes → icons → templates → widgets 顺序写盘，
// 就地应用时「插件 → 挂件」这类目标类型尚未重写的组合只能读到上一轮的 Stage 数据。
//
// 只回写本轮 stagedTypes 中的类型：未重写的类型其弃用字段与未变化的注册表一致，保持原样；
// 但解析表始终读全部五个 Stage 文件，跨类型目标才能定位到包名。
func applyDeprecatedRegistry(root string, stagedTypes []rules.PackageType, registry *util.DeprecatedRegistry) error {
	if registry == nil || len(stagedTypes) == 0 {
		return nil
	}

	reposByType := make(map[rules.PackageType][]util.StageRepo, len(rules.AllPackageTypes()))
	for _, packageType := range rules.AllPackageTypes() {
		stageFile, err := util.ReadStageFile(filepath.Join(root, "stage", packageType.StageJSONFile()))
		if err != nil {
			return fmt.Errorf("read stage [%s] for deprecation: %w", packageType.StageJSONFile(), err)
		}
		reposByType[packageType] = stageFile.Repos
	}
	targets := util.NewDeprecatedTargets(reposByType)

	for _, packageType := range stagedTypes {
		repos := reposByType[packageType]
		if repos == nil {
			continue
		}
		for _, warning := range util.ApplyDeprecatedRegistry(packageType, repos, registry, targets) {
			logger.Errorf("deprecation registry warning: %s", warning)
		}
		for i := range repos {
			// 与 performStage 写盘前一致：弃用原因是在这之后写入的，冗余 locale 与空 funding 需再清理一次。
			rules.ClearEmptyFunding(&repos[i].Package)
			rules.ClearRedundantLocales(&repos[i].Package)
		}
		data, err := marshalSortedIndentedJSON(util.StageFile{Repos: repos})
		if err != nil {
			return fmt.Errorf("marshal stage [%s]: %w", packageType.StageJSONFile(), err)
		}
		if err = os.WriteFile(filepath.Join(root, "stage", packageType.StageJSONFile()), data, 0644); err != nil {
			return fmt.Errorf("write stage [%s]: %w", packageType.StageJSONFile(), err)
		}
	}
	return nil
}
