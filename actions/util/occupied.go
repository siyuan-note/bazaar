// SiYuan community bazaar.
// Copyright (c) 2021-present, b3log.org
//
// Bazaar is licensed under Mulan PSL v2.
// You can use this software according to the terms and conditions of the Mulan PSL v2.
// You may obtain a copy of Mulan PSL v2 at:
//         http://license.coscl.org.cn/MulanPSL2
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR PURPOSE.
// See the Mulan PSL v2 for more details.

package util

import (
	"fmt"
	"path/filepath"
	"strings"
)

// LoadOccupiedNames 从 bazaarHead 下的 stage/*.json 收集已占用 package.name（键为小写）。
func LoadOccupiedNames(bazaarHead string) (map[string]struct{}, error) {
	occupied := make(map[string]struct{})
	jsonFiles := []string{"plugins.json", "themes.json", "icons.json", "templates.json", "widgets.json"}
	for _, jsonFile := range jsonFiles {
		filePath := filepath.Join(bazaarHead, "stage", jsonFile)
		names, err := parseNamesFromStageJSON(filePath)
		if err != nil {
			return nil, fmt.Errorf("load stage repos [%s]: %w", filePath, err)
		}
		for name := range names {
			occupied[name] = struct{}{}
		}
	}
	return occupied, nil
}

// parseNamesFromStageJSON 从单个 stage JSON 中解析 package.name，键为转小写后的形式供唯一性比较使用。
// 文件不存在时返回空集合（不报错），便于本地缺 stage 文件时仍可运行。
func parseNamesFromStageJSON(filePath string) (map[string]struct{}, error) {
	stageFile, err := ReadStageFile(filePath)
	if err != nil {
		return nil, err
	}

	nameSet := make(map[string]struct{})
	for _, repo := range stageFile.Repos {
		name := repo.Package.Name
		if name == "" {
			continue
		}
		nameSet[strings.ToLower(name)] = struct{}{}
	}
	return nameSet, nil
}
