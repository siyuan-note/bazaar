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

// ParseReposFromTxt 从 TXT 文件解析包列表：按行分割、跳过空行，校验每行为合法 owner/repo，返回 []string。
// 不做 TrimSpace，行首尾或 owner/repo 内含空格均视为解析错误。兼容多种换行符（\n、\r\n、\r）。
// 空行是否允许不在本函数把关：Stage / 弃用校验用宽松语义，PR Check 用 ParseReposFromTxtStrict。
func ParseReposFromTxt(filePath string) (repos []string, err error) {
	return parseReposFromFile(filePath, false)
}

// ParseReposFromTxtStrict 与 ParseReposFromTxt 相同，但额外要求空行只能出现在文件末尾且最多一行。
// 仅供 PR Check 校验本 PR 提交的列表文件：Stage 读的是 main 上的既有内容，不应因一处多余空行整轮失败。
func ParseReposFromTxtStrict(filePath string) (repos []string, err error) {
	return parseReposFromFile(filePath, true)
}

func parseReposFromFile(filePath string, strict bool) (repos []string, err error) {
	fileLabel := filepath.Base(filePath)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, rules.LocalizedErr(
			fmt.Sprintf("无法读取 `%s`：%v。请确认该文件存在于 PR 变更中且路径正确。", fileLabel, err),
			fmt.Sprintf("Couldn't read `%s`: %v. Please make sure the file exists in the PR changes and the path is correct.", fileLabel, err),
			err,
		)
	}
	return ParseReposFromBytesWithMode(fileLabel, data, strict)
}

// ParseReposFromBytes 从 TXT 内容解析包列表，规则与 ParseReposFromTxt 相同（跳过空行）。
// fileLabel 用于错误文案（通常为文件名，如 plugins.txt）。
func ParseReposFromBytes(fileLabel string, data []byte) (repos []string, err error) {
	return ParseReposFromBytesWithMode(fileLabel, data, false)
}

// ParseReposFromBytesStrict 从 TXT 内容解析包列表，额外要求空行只能出现在文件末尾且最多一行。
func ParseReposFromBytesStrict(fileLabel string, data []byte) (repos []string, err error) {
	return ParseReposFromBytesWithMode(fileLabel, data, true)
}

// ParseReposFromBytesWithMode 按 strict 决定是否把中间空行视为错误。
// strict 只用于 PR Check：它要提示作者改掉多余空行，而 Stage 读的是 main 上的既有内容。
func ParseReposFromBytesWithMode(fileLabel string, data []byte, strict bool) (repos []string, err error) {
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	lines := make([]string, 0)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err = scanner.Err(); err != nil {
		return nil, rules.LocalizedErr(
			fmt.Sprintf("读取 `%s` 失败：%v。请确认文件编码与换行正常后重新提交。", fileLabel, err),
			fmt.Sprintf("Failed to read `%s`: %v. Please make sure the file encoding and line endings are fine, then submit again.", fileLabel, err),
			err,
		)
	}

	repos = make([]string, 0, len(lines))
	for i, line := range lines {
		lineNum := i + 1
		// 空行：宽松模式跳过；严格模式只允许出现在文件末尾且最多一行（形如 "a\n\n"）
		if line == "" || strings.TrimSpace(line) == "" {
			if !strict {
				continue
			}
			if line == "" && i == len(lines)-1 {
				continue
			}
			return nil, emptyLineError(fileLabel, lineNum)
		}
		if line != strings.TrimSpace(line) {
			return nil, rules.LocalizedErr(
				fmt.Sprintf("`%s` 第 %d 行首尾不能含空格：`%s`。请删除首尾空格后重新提交。", fileLabel, lineNum, line),
				fmt.Sprintf("`%s` line %d can't have leading or trailing spaces: `%s`. Please remove the spaces and submit again.", fileLabel, lineNum, line),
				nil,
			)
		}
		parts := strings.Split(line, "/")
		if len(parts) != 2 {
			return nil, rules.LocalizedErr(
				fmt.Sprintf("`%s` 第 %d 行格式无效（应为 `owner/repo`）：`%s`。请改成例如 `siyuan-note/plugin-sample` 后重新提交。", fileLabel, lineNum, line),
				fmt.Sprintf("`%s` line %d has an invalid format (expected `owner/repo`): `%s`. Please change it to something like `siyuan-note/plugin-sample`, then submit again.", fileLabel, lineNum, line),
				nil,
			)
		}
		owner, repo := parts[0], parts[1]
		if owner == "" || repo == "" {
			return nil, rules.LocalizedErr(
				fmt.Sprintf("`%s` 第 %d 行格式无效（owner 与 repo 均不能为空）：`%s`。请改成 `owner/repo` 格式后重新提交。", fileLabel, lineNum, line),
				fmt.Sprintf("`%s` line %d is invalid (owner and repo both need to be non-empty): `%s`. Please use the `owner/repo` format, then submit again.", fileLabel, lineNum, line),
				nil,
			)
		}
		if strings.IndexFunc(owner, unicode.IsSpace) >= 0 || strings.IndexFunc(repo, unicode.IsSpace) >= 0 {
			return nil, rules.LocalizedErr(
				fmt.Sprintf("`%s` 第 %d 行 `owner/repo` 不能含空格：`%s`。请去掉空格后重新提交。", fileLabel, lineNum, line),
				fmt.Sprintf("`%s` line %d `owner/repo` can't contain spaces: `%s`. Please remove the spaces and submit again.", fileLabel, lineNum, line),
				nil,
			)
		}
		repos = append(repos, owner+"/"+repo)
	}
	return repos, nil
}

// emptyLineError 生成「列表文件中间出现空行」的双语错误。
func emptyLineError(fileLabel string, lineNum int) error {
	return rules.LocalizedErr(
		fmt.Sprintf("`%s` 第 %d 行不能为空行或只含空格。空行只允许出现在文件末尾，且最多一行；请删除该行后重新提交。", fileLabel, lineNum),
		fmt.Sprintf("`%s` line %d must not be empty or contain only spaces. A blank line is allowed only at the end of the file, at most one; please remove this line and submit again.", fileLabel, lineNum),
		nil,
	)
}
