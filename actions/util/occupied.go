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
	"os"
	"path/filepath"
	"strings"

	"github.com/88250/gulu"
)

// LoadOccupiedNames 从 bazaarHead 下的 stage/*.json 收集已占用 package.name（键为小写），供 check.Check 的 OccupiedNames 使用。
func LoadOccupiedNames(bazaarHead string) (map[string]struct{}, error) {
	occupied := make(map[string]struct{})
	jsonFiles := []string{"icons.json", "plugins.json", "templates.json", "themes.json", "widgets.json"}
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
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]struct{}), nil
		}
		return nil, err
	}

	var m map[string]any
	if err = gulu.JSON.UnmarshalJSON(data, &m); err != nil {
		return nil, err
	}

	repos, _ := m["repos"].([]any)
	if repos == nil {
		return make(map[string]struct{}), nil
	}

	nameSet := make(map[string]struct{})
	for _, r := range repos {
		repoMap, ok := r.(map[string]any)
		if !ok {
			continue
		}
		pkg, ok := repoMap["package"].(map[string]any)
		if !ok {
			continue
		}
		name, ok := pkg["name"].(string)
		if !ok || name == "" {
			continue
		}
		nameSet[strings.ToLower(name)] = struct{}{}
	}
	return nameSet, nil
}
