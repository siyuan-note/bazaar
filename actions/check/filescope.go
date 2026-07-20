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
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/siyuan-note/bazaar/actions/util"
	"github.com/siyuan-note/bazaar/rules"
)

// normalizeRepoRelPath 将变更路径规范为仓库根相对、正斜杠形式，便于黑白名单匹配。
func normalizeRepoRelPath(p string) string {
	return strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(p)), "./")
}

// isWhitelistedPath 判断是否为社区可改的集市包列表文件（五个根目录 *.txt）。p 须已 normalize。
func isWhitelistedPath(p string) bool {
	for _, typ := range rules.AllPackageTypes() {
		if p == typ.ReposListFile() {
			return true
		}
	}
	return false
}

// isBlacklistedPath 判断是否为社区不可直接改动的路径（stage 索引、theme.js allowlist）。p 须已 normalize。
func isBlacklistedPath(p string) bool {
	return p == util.ThemeJsAllowlistRelPath || p == "stage" || strings.HasPrefix(p, "stage/")
}

// classifyPRFiles 将本 PR 变更路径分为黑名单与白名单（其余为灰区，忽略）；黑名单优先。
func classifyPRFiles(paths []string) (black, white []string) {
	for _, raw := range paths {
		p := normalizeRepoRelPath(raw)
		if p == "" {
			continue
		}
		switch {
		case isBlacklistedPath(p):
			black = append(black, p)
		case isWhitelistedPath(p):
			white = append(white, p)
		}
	}
	return black, white
}

// formatBlacklistFlowError 返回命中黑名单时的固定双语 FlowError 正文。
func formatBlacklistFlowError() string {
	return "本 PR 修改了不允许直接改动的文件。集市包列表更新请只编辑仓库根目录的五个列表文件（`plugins.txt` / `themes.txt` / `icons.txt` / `templates.txt` / `widgets.txt`），每行一个 `owner/repo`，然后推送到本 PR。\n\n" +
		"This PR modifies files that must not be changed directly. To update the bazaar package list, edit only the five list files at the repo root (`plugins.txt` / `themes.txt` / `icons.txt` / `templates.txt` / `widgets.txt`), one `owner/repo` per line, then push to this PR.\n"
}

// gitRevParseHEAD 返回 dir 工作区内 HEAD 的完整 SHA。
func gitRevParseHEAD(ctx context.Context, dir string) (string, error) {
	if dir == "" {
		dir = "."
	}
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD in %s: %w", dir, err)
	}
	sha := strings.TrimSpace(string(out))
	if sha == "" {
		return "", fmt.Errorf("git rev-parse HEAD in %s returned empty", dir)
	}
	return sha, nil
}

// listPRChangedFiles 列出本 PR（merge base → head）变更的文件路径。
// 在 bazaar head 工作区执行 diff：该检出含完整历史且已 fetch PR head，能同时解析两侧 SHA。
func listPRChangedFiles(ctx context.Context) ([]string, error) {
	baseSHA, err := gitRevParseHEAD(ctx, PR_BASE_PATH)
	if err != nil {
		return nil, fmt.Errorf("resolve PR base SHA: %w", err)
	}
	headSHA, err := gitRevParseHEAD(ctx, PR_HEAD_PATH)
	if err != nil {
		return nil, fmt.Errorf("resolve PR head SHA: %w", err)
	}
	bazaarDir := BAZAAR_HEAD_PATH
	if bazaarDir == "" {
		bazaarDir = "."
	}
	cmd := exec.CommandContext(ctx, "git", "-C", bazaarDir, "diff", "--name-only", baseSHA, headSHA)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff --name-only %s %s in %s: %w", baseSHA, headSHA, bazaarDir, err)
	}
	var paths []string
	for _, line := range strings.Split(string(out), "\n") {
		if p := normalizeRepoRelPath(line); p != "" {
			paths = append(paths, p)
		}
	}
	return paths, nil
}
