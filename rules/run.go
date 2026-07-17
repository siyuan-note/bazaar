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
	"path/filepath"
	"strings"
)

// step 为流水线中的一步。步骤内部自行判断是否可执行：
//   - 已 Halt：直接 return，不再追加 Issue（保证「开头走不通时只留下开头的错误」）
//   - 缺少本步依赖的数据：跳过本步
type step func(*Context)

// Run 按 pipeline 固定顺序调用所有步骤。
//
// 设计要点：
//   - 流水线本身不 break；始终依次调用每个 step。
//   - OwnerRepo / PackageRoot 失败时调用 Halt()，后续 step 自跳过。
//   - 包根有效之后，文件类与清单字段类问题尽量累计，方便一次评论里列全。
//   - 清单读失败只记 Issue、不 Halt，以便此前已发现的缺文件等问题仍保留；字段检查随读失败一并跳过。
func Run(c *Context) {
	if c == nil {
		return
	}
	for _, s := range pipeline {
		s(c)
	}
}

// pipeline 为检查步骤的固定顺序。新增规则时：先在本包实现，再按依赖插入本切片。
var pipeline = []step{
	stepOwnerRepo,     // 校验 OwnerRepo 格式，失败则 Halt
	stepPackageRoot,   // 解析解压目录得到包根 Root，失败则 Halt
	stepRequiredFiles, // 检查展示文件、清单文件及类型运行时必要文件，可累计报错
	stepPathNames,     // 递归检查路径与文件名规范
	stepThemeJS,       // 仅主题：非白名单不得包含 theme.js
	stepManifest,      // 读取并校验清单字段；读失败只记 Issue、不 Halt
}

// stepOwnerRepo 校验 OwnerRepo 格式（owner/repo）。
// 无效时 Halt，避免后续用无效标识去做 url、name 比对。
func stepOwnerRepo(c *Context) {
	if c.Halted() {
		return
	}
	if strings.TrimSpace(c.OwnerRepo) == "" {
		c.Add(issue(
			"内部错误：未提供待检查的 GitHub 仓库（`owner/repo`）。这通常是集市检查流程配置问题，请联系维护者。",
			"Internal error: OwnerRepo (`owner/repo`) was not provided. This is usually a bazaar checker configuration issue; contact a maintainer.",
		))
		c.Halt()
		return
	}
	if _, _, ok := splitOwnerRepo(c.OwnerRepo); !ok {
		c.Add(issue(
			fmt.Sprintf("仓库标识 `%s` 格式不正确，应为 `owner/repo`（中间一个斜杠，整行无空格），例如 `siyuan-note/plugin-sample`。请修正集市列表文件 `%s` 中对应行后重新提交。", c.OwnerRepo, c.Type.ReposListFile()),
			fmt.Sprintf("Repository id `%s` is invalid; expected `owner/repo` with a single slash and no spaces on the whole line (e.g. `siyuan-note/plugin-sample`). Fix the corresponding line in bazaar list file `%s` and push again.", c.OwnerRepo, c.Type.ReposListFile()),
		))
		c.Halt()
		return
	}
}

// splitOwnerRepo 要求恰好一段 "/"，且两侧无首尾空白（空白视为格式错误，不自动 Trim 后接受）。
func splitOwnerRepo(ownerRepo string) (owner, repo string, ok bool) {
	owner, repo, ok = strings.Cut(ownerRepo, "/")
	if !ok || owner == "" || repo == "" || strings.Contains(repo, "/") ||
		strings.TrimSpace(owner) != owner ||
		strings.TrimSpace(repo) != repo {
		return "", "", false
	}
	return owner, repo, true
}

// stepPackageRoot 解析解压目录得到真实包根（可能剥掉单独一层包装目录），写入 c.Root。
func stepPackageRoot(c *Context) {
	if c.Halted() {
		return
	}
	root, err := ResolvePackageRoot(c.PackageRoot)
	if err != nil {
		c.Add(IssueFromErr(err))
		c.Halt()
		return
	}
	c.Root = root
}

// stepRequiredFiles 检查展示文件、清单文件及类型运行时必要文件。
func stepRequiredFiles(c *Context) {
	if c.Halted() {
		return
	}
	c.Add(RequiredFiles(c.Root, c.Type)...)
}

// stepPathNames 递归检查包内路径命名（首尾空格、Windows 保留名等）。
func stepPathNames(c *Context) {
	if c.Halted() {
		return
	}
	c.Add(PathNames(c.Root)...)
}

// stepThemeJS 仅对主题检查 theme.js 是否允许出现。
func stepThemeJS(c *Context) {
	if c.Halted() || c.Type != TypeTheme {
		return
	}
	c.Add(ThemeJS(c.Root, c.AllowThemeJS)...)
}

// stepManifest 读取类型对应的清单 JSON，并校验字段（name/url/version/readme 等，含 OccupiedNames 唯一性）。
// 读失败时不 Halt：前面的缺文件等问题应保留；此时也不做字段检查。
func stepManifest(c *Context) {
	if c.Halted() {
		return
	}
	manifestPath := filepath.Join(c.Root, c.Type.ManifestFile())
	manifest, err := ReadManifest(manifestPath)
	if err != nil {
		c.Add(IssueFromErr(err))
		return
	}
	c.Package = packageFromMap(manifest)

	owner, repo, ok := splitOwnerRepo(c.OwnerRepo)
	if !ok {
		// stepOwnerRepo 已通过时不应发生；仍记一条内部错误，避免静默跳过字段校验。
		c.Add(issue(
			fmt.Sprintf("内部错误：仓库标识 `%s` 无法解析为 `owner/repo`。这通常是集市检查流程内部状态不一致，请联系维护者。", c.OwnerRepo),
			fmt.Sprintf("Internal error: repository id `%s` could not be parsed as `owner/repo`. This usually indicates inconsistent checker state; contact a maintainer.", c.OwnerRepo),
		))
		return
	}
	c.Add(Manifest(manifest, ManifestInput{
		PackageRoot:   c.Root,
		Owner:         owner,
		Repo:          repo,
		Type:          c.Type,
		OldName:       c.OldName,
		OldVersion:    c.OldVersion,
		OccupiedNames: c.OccupiedNames,
	})...)
}
