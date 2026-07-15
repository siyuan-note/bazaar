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
	"path/filepath"
	"strings"
	"unicode"

	"github.com/siyuan-note/bazaar/rules"
)

// ThemeJsAllowlistRelPath 为仓库根目录下「允许包含 theme.js」的主题列表相对路径。
const ThemeJsAllowlistRelPath = "config/themes-theme-js-allowlist.txt"

// ParseReposFromTxt 从 TXT 文件解析包列表：按行分割、过滤空行，校验每行为合法 owner/repo，返回 []string。
// 不做 TrimSpace，行首尾或 owner/repo 内含空格均视为解析错误。兼容多种换行符（\n、\r\n、\r）。
func ParseReposFromTxt(filePath string) (repos []string, err error) {
	fileLabel := filepath.Base(filePath)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, rules.LocalizedErr(
			fmt.Sprintf("无法读取 %s：%v。请确认该文件存在于 PR 变更中且路径正确。", fileLabel, err),
			fmt.Sprintf("Cannot read %s: %v. Ensure the file exists in the PR changes with the correct path.", fileLabel, err),
			err,
		)
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
			return nil, rules.LocalizedErr(
				fmt.Sprintf("%s 第 %d 行首尾不能含空格：%q。请删除首尾空格后重新提交。", fileLabel, lineNum, line),
				fmt.Sprintf("%s line %d must not have leading or trailing spaces: %q. Remove the spaces and push again.", fileLabel, lineNum, line),
				nil,
			)
		}
		parts := strings.Split(line, "/")
		if len(parts) != 2 {
			return nil, rules.LocalizedErr(
				fmt.Sprintf("%s 第 %d 行格式无效（应为 owner/repo）：%q。请改成例如 siyuan-note/plugin-sample 后重新提交。", fileLabel, lineNum, line),
				fmt.Sprintf("%s line %d has invalid format (expected owner/repo): %q. Use a form like siyuan-note/plugin-sample and push again.", fileLabel, lineNum, line),
				nil,
			)
		}
		owner, repo := parts[0], parts[1]
		if owner == "" || repo == "" {
			return nil, rules.LocalizedErr(
				fmt.Sprintf("%s 第 %d 行格式无效（owner 与 repo 均不能为空）：%q。请改成 owner/repo 格式后重新提交。", fileLabel, lineNum, line),
				fmt.Sprintf("%s line %d is invalid (owner and repo must be non-empty): %q. Use owner/repo format and push again.", fileLabel, lineNum, line),
				nil,
			)
		}
		if strings.IndexFunc(owner, unicode.IsSpace) >= 0 || strings.IndexFunc(repo, unicode.IsSpace) >= 0 {
			return nil, rules.LocalizedErr(
				fmt.Sprintf("%s 第 %d 行 owner/repo 不能含空格：%q。请去掉空格后重新提交。", fileLabel, lineNum, line),
				fmt.Sprintf("%s line %d owner/repo must not contain spaces: %q. Remove the spaces and push again.", fileLabel, lineNum, line),
				nil,
			)
		}
		repos = append(repos, owner+"/"+repo)
	}
	if err = scanner.Err(); err != nil {
		return nil, rules.LocalizedErr(
			fmt.Sprintf("读取 %s 失败：%v。请确认文件编码与换行正常后重新提交。", fileLabel, err),
			fmt.Sprintf("Failed to read %s: %v. Ensure the file encoding and line endings are valid, then push again.", fileLabel, err),
			err,
		)
	}
	return repos, nil
}
