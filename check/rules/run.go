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
//   - 缺少本步依赖的数据（如 Manifest == nil）：跳过本步
type step func(*Context)

// Run 按 pipeline 固定顺序调用所有步骤。
//
// 设计要点：
//   - 流水线本身不 break；始终依次调用每个 step。
//   - OwnerRepo / PackageRoot 失败时调用 Halt()，后续 step 自跳过。
//   - 包根有效之后，文件类与清单字段类问题尽量累计，方便一次评论里列全。
//   - 清单读失败只记 Issue、不 Halt，以便此前已发现的缺文件等问题仍保留；字段检查因 Manifest == nil 而跳过。
func Run(c *Context) {
	if c == nil {
		return
	}
	for _, s := range pipeline {
		s(c)
	}
}

// pipeline 为检查步骤的固定顺序。新增规则时：先在 rules 里实现，再按依赖插入本切片。
var pipeline = []step{
	stepOwnerRepo,     // 解析 OwnerRepo 得到 Owner/Repo，失败则 Halt
	stepPackageRoot,   // 解析解压目录得到包根 Root，失败则 Halt
	stepRequiredFiles, // 检查展示文件、清单文件及类型运行时必要文件，可累计报错
	stepPathNames,     // 递归检查路径与文件名规范
	stepThemeJS,       // 仅主题：非白名单不得包含 theme.js
	stepLoadManifest,  // 读取清单 JSON，失败只记 Issue、不 Halt
	stepManifest,      // 校验清单字段，依赖上一步的 Manifest
}

// stepOwnerRepo 校验并拆分 OwnerRepo，写入 c.Owner / c.Repo。
// 无效时 Halt，避免后续用空的 owner/repo 去做 url、name 比对。
func stepOwnerRepo(c *Context) {
	if c.Halted() {
		return
	}
	if strings.TrimSpace(c.OwnerRepo) == "" {
		c.Add(issue("input/owner_repo",
			"内部错误：未提供待检查的 GitHub 仓库（owner/repo）。这通常是集市检查流程配置问题，请联系维护者重试。",
			"Internal error: OwnerRepo (owner/repo) was not provided. This is usually a bazaar checker configuration issue; contact a maintainer.",
		))
		c.Halt()
		return
	}
	owner, repo, ok := splitOwnerRepo(c.OwnerRepo)
	if !ok {
		c.Add(issue("input/owner_repo",
			fmt.Sprintf("仓库标识 %q 格式不正确，应为 owner/repo（中间一个斜杠，两侧无空格），例如 siyuan-note/plugin-sample。", c.OwnerRepo),
			fmt.Sprintf("Repository id %q is invalid; expected owner/repo with no spaces (e.g. siyuan-note/plugin-sample).", c.OwnerRepo),
		))
		c.Halt()
		return
	}
	c.Owner = owner
	c.Repo = repo
}

// splitOwnerRepo 要求恰好一段 "/"，且两侧无首尾空白（空白视为格式错误，不自动 Trim 后接受）。
func splitOwnerRepo(ownerRepo string) (owner, repo string, ok bool) {
	parts := strings.Split(ownerRepo, "/")
	if len(parts) != 2 {
		return "", "", false
	}
	owner = strings.TrimSpace(parts[0])
	repo = strings.TrimSpace(parts[1])
	if owner == "" || repo == "" {
		return "", "", false
	}
	if owner != parts[0] || repo != parts[1] {
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
		c.Add(issue("root/invalid",
			fmt.Sprintf("无法从解压结果确定 package.zip 的包根目录（%v）。请确认 zip 内文件直接在根目录，或仅多一层包装文件夹；不要在根下并排放多个无关目录。重新打包后再更新 Release。", err),
			fmt.Sprintf("Cannot determine the package root from the extracted package.zip (%v). Put files at the zip root, or use a single wrapping folder—not multiple top-level directories. Rebuild and update the Release.", err),
		))
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
	c.Add(RequiredFiles(c.Root, c.Type, c.Mode)...)
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

// stepLoadManifest 读取类型对应的清单 JSON。
// 失败时不 Halt：前面的缺文件等问题应保留；字段步靠 Manifest == nil 跳过。
func stepLoadManifest(c *Context) {
	if c.Halted() {
		return
	}
	c.ManifestPath = filepath.Join(c.Root, c.Type.ManifestFile())
	manifest, err := ReadManifest(c.ManifestPath)
	if err != nil {
		c.Add(issue("manifest/read",
			fmt.Sprintf("无法读取或解析清单文件 %s：%v。请确认 package.zip 包根下存在该文件、JSON 语法正确，且文件名大小写完全一致。", c.Type.ManifestFile(), err),
			fmt.Sprintf("Cannot read or parse manifest %s: %v. Ensure the file exists at the package root with valid JSON and an exact case-sensitive filename.", c.Type.ManifestFile(), err),
		))
		return
	}
	c.Manifest = manifest
}

// stepManifest 校验清单字段（name/url/version/readme 等，含 OccupiedNames 唯一性）。
func stepManifest(c *Context) {
	if c.Halted() || c.Manifest == nil {
		return
	}
	c.Add(Manifest(c.Manifest, ManifestInput{
		PackageRoot:   c.Root,
		Owner:         c.Owner,
		Repo:          c.Repo,
		Type:          c.Type,
		Mode:          c.Mode,
		OldName:       c.OldName,
		OldVersion:    c.OldVersion,
		OccupiedNames: c.OccupiedNames,
	})...)
}
