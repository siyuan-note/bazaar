// SiYuan community bazaar.
// Copyright (c) 2021-present, b3log.org
//
// Bazaar is licensed under Mulan PSL v2.
// You can use this software according to the terms and conditions of the Mulan PSL v2.
// You may obtain a copy of Mulan PSL v2 at:
//         http://license.coscl.org.cn/MulanPSL2
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR PURPOSE.
// See the Mulan PSL v2 for more details.

package rules

import (
	"fmt"
	"os"
	"path/filepath"
)

// ResolvePackageRoot 确定包根目录。
// 若 path 下恰好有一个子目录且根下没有文件，则将该子目录视为包根
// （兼容「zip 内多包一层文件夹」的常见打包方式）。
func ResolvePackageRoot(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("PackageRoot 不能为空")
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("无法访问目录: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("PackageRoot 不是目录: %s", path)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return "", fmt.Errorf("读取目录失败: %w", err)
	}

	var dirs []string
	hasFile := false
	for _, e := range entries {
		name := e.Name()
		if name == ".DS_Store" || name == "__MACOSX" {
			continue
		}
		if e.IsDir() {
			dirs = append(dirs, name)
			continue
		}
		hasFile = true
	}

	if !hasFile && len(dirs) == 1 {
		return filepath.Join(path, dirs[0]), nil
	}
	if len(dirs) > 1 && !hasFile {
		return "", fmt.Errorf("解压根目录下存在多个子目录且无文件，无法确定包根: %s", path)
	}
	return path, nil
}
