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
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ParseReposFromTxt 从 TXT 文件解析包列表：按行分割、过滤空行，校验每行为合法 owner/repo，返回 []string。
// 不做 TrimSpace，行首尾或 owner/repo 首尾含空格均视为解析错误。兼容多种换行符（\n、\r\n、\r）。
func ParseReposFromTxt(filePath string) (repos []string, err error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	repos = make([]string, 0)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if line == "" {
			continue
		}
		if line != strings.TrimSpace(line) {
			return nil, fmt.Errorf("line %d: leading or trailing space not allowed: %q", lineNum, line)
		}
		parts := strings.Split(line, "/")
		if len(parts) != 2 {
			return nil, fmt.Errorf("line %d: invalid format (expected owner/repo): %q", lineNum, line)
		}
		owner, repo := parts[0], parts[1]
		if owner == "" || repo == "" {
			return nil, fmt.Errorf("line %d: invalid format (owner and repo must be non-empty): %q", lineNum, line)
		}
		if owner != strings.TrimSpace(owner) || repo != strings.TrimSpace(repo) {
			return nil, fmt.Errorf("line %d: leading or trailing space in owner/repo not allowed: %q", lineNum, line)
		}
		repos = append(repos, owner+"/"+repo)
	}
	if err = scanner.Err(); err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	return repos, nil
}
