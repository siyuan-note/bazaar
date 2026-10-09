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
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// syncPassthroughHiddenDirName 唯一不被数据同步忽略的点开头目录名。
// dejavu 需要穿透它读取 `syncignore`，因此 `data/.siyuan` 参与同步。
const syncPassthroughHiddenDirName = ".siyuan"

// 内核数据同步还有一条按目录名匹配、不限深度的忽略规则，对象是文件系统探测目录
// `filesys_status_check`（`kernel/util/runtime.go` 的真实创建位置是 `data/.siyuan/filesys_status_check`）。
// 按目录名匹配会让包内任意深度的同名目录被整棵排除，但概率极低，而且内核正在收窄为按实际路径判定，
// 届时这条规则会自行失效，因此集市不检查：
// https://github.com/siyuan-note/siyuan/issues/20353

// syncIgnoredLegacyStorageFiles 内核为兼容旧目录结构按路径后缀排除的存储文件。
// 判定见 `kernel/model/sync_ignore.go` 的 `syncPathFilter`，因此包内任意深度的
// `data/storage/local.json` 等路径都不参与同步。
var syncIgnoredLegacyStorageFiles = []string{"local.json", "recent-doc.json", "ref-used.json"}

// syncIgnoredKind 包内条目命中数据同步忽略规则的原因。
type syncIgnoredKind int

const (
	syncIgnoredDotEntry      syncIgnoredKind = iota // 点开头的文件或目录
	syncIgnoredTempFile                             // `*.tmp` 临时文件
	syncIgnoredLegacyStorage                        // 命中旧 `data/storage/` 后缀的存储文件
)

// syncIgnoredKinds 固定输出顺序，便于测试与评论阅读。
var syncIgnoredKinds = []syncIgnoredKind{
	syncIgnoredDotEntry, syncIgnoredTempFile, syncIgnoredLegacyStorage,
}

// SyncIgnoredEntries 检查包内条目是否命中思源数据同步的忽略规则。
// installRelPath 是包安装目录相对 `data` 的路径（例如 `/plugins/demo`），
// 用于复现内核按完整相对路径判定的规则；其余规则与安装位置无关。
//
// 命中的条目安装后不会随数据同步到达其他设备，表现为「界面或插件行为在不同设备上不一致」，
// 且没有日志和界面提示，因此在此拦截。判定只覆盖当前所有已发布版本都会忽略的规则：
// dejavu 的文件系统不变量（点开头条目、`*.tmp`）与内核按完整路径判定的旧存储路径后缀。
// 未纳入的两类规则：
// 用户指南的笔记本目录依赖会随内核版本变化的内部 ID 列表，复刻收益极低；
// 文件系统探测目录名 `filesys_status_check` 按名匹配但概率极低，且内核正在收窄判定。
func SyncIgnoredEntries(root, installRelPath string) []Issue {
	if root == "" {
		return nil
	}

	byKind := map[syncIgnoredKind][]string{}
	add := func(kind syncIgnoredKind, rel string) {
		byKind[kind] = append(byKind[kind], rel)
	}

	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || p == root {
			// 遍历错误由 PathNames 报告，此处不重复；只关注忽略规则命中。
			return nil
		}
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		name := d.Name()

		if d.IsDir() {
			if strings.HasPrefix(name, ".") && name != syncPassthroughHiddenDirName {
				add(syncIgnoredDotEntry, rel)
				// 忽略规则整棵剪枝，这里同样不再深入，顺带收敛条目数量。
				return fs.SkipDir
			}
			return nil
		}

		if isSyncIgnoredLegacyStorage(installRelPath, rel) {
			add(syncIgnoredLegacyStorage, rel)
			return nil
		}
		// 目录只按点前缀判定，`*.tmp` 只作用于文件，与 dejavu 的 IgnorePath 一致。
		if strings.HasPrefix(name, ".") {
			add(syncIgnoredDotEntry, rel)
			return nil
		}
		if strings.HasSuffix(name, ".tmp") {
			add(syncIgnoredTempFile, rel)
			return nil
		}
		return nil
	})

	var issues []Issue
	for _, kind := range syncIgnoredKinds {
		entries := byKind[kind]
		if len(entries) == 0 {
			continue
		}
		slices.Sort(entries)
		issues = append(issues, syncIgnoredIssue(kind, entries))
	}
	return issues
}

// isSyncIgnoredLegacyStorage 复现内核按完整相对路径后缀判定旧存储文件的方式。
// 由于判定对象是拼接安装目录后的完整路径，包名本身也可能凑出该后缀：
// 名为 `data` 的包若包含 `storage/local.json`，完整路径同样以 `data/storage/local.json` 结尾。
func isSyncIgnoredLegacyStorage(installRelPath, rel string) bool {
	full := path.Join(installRelPath, rel)
	for _, name := range syncIgnoredLegacyStorageFiles {
		if strings.HasSuffix(full, "data/storage/"+name) {
			return true
		}
	}
	return false
}

// syncIgnoredIssue 按命中的原因生成双语问题，示例最多列出 issueExampleLimit 条。
func syncIgnoredIssue(kind syncIgnoredKind, entries []string) Issue {
	examplesZh, examplesEn := formatExampleList(entries)
	count := len(entries)
	switch kind {
	case syncIgnoredDotEntry:
		return issue(
			fmt.Sprintf("`package.zip` 内有 %d 个以 `.` 开头的条目，例如 %s。思源的数据同步会忽略点开头的文件和目录（`data/.siyuan` 目录除外），这些条目在其它设备上不会存在，界面和插件行为会不一致。请从 `package.zip` 中删除本地开发残留（如 `.git`、`.github`、`.vscode`、`.gitignore`、`.DS_Store`），或把需要随包分发的文件改成不以 `.` 开头的名字。", count, examplesZh),
			fmt.Sprintf("`package.zip` has %d entries whose names start with `.`, e.g. %s. SiYuan's data sync ignores dot-prefixed files and folders (except the `data/.siyuan` folder), so these entries won't exist on other devices and the UI or plugin behavior will diverge. Please remove local development leftovers (such as `.git`, `.github`, `.vscode`, `.gitignore`, `.DS_Store`) from `package.zip`, or rename files you do want to ship so they don't start with `.`.", count, examplesEn),
		)
	case syncIgnoredTempFile:
		return issue(
			fmt.Sprintf("`package.zip` 内有 %d 个 `.tmp` 文件，例如 %s。思源的数据同步会忽略 `*.tmp`，这些文件在其它设备上不会存在。请从 `package.zip` 中删除临时文件。", count, examplesZh),
			fmt.Sprintf("`package.zip` has %d files ending in `.tmp`, e.g. %s. SiYuan's data sync ignores `*.tmp`, so they won't exist on other devices. Please remove these temporary files from `package.zip`.", count, examplesEn),
		)
	default:
		return issue(
			fmt.Sprintf("`package.zip` 内有 %d 个文件的路径以 `data/storage/local.json`、`data/storage/recent-doc.json` 或 `data/storage/ref-used.json` 结尾，例如 %s。思源的数据同步为兼容旧目录结构按后缀排除这些路径，它们在其它设备上不会存在。请避免在包内使用 `data/storage/` 这一目录结构。", count, examplesZh),
			fmt.Sprintf("`package.zip` has %d files whose paths end with `data/storage/local.json`, `data/storage/recent-doc.json` or `data/storage/ref-used.json`, e.g. %s. SiYuan's data sync excludes those paths by suffix for backward compatibility, so they won't exist on other devices. Please avoid using the `data/storage/` layout inside the package.", count, examplesEn),
		)
	}
}
