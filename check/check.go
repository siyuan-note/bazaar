// SiYuan community bazaar.
// Copyright (c) 2021-present, b3log.org
//
// Bazaar is licensed under Mulan PSL v2.
// You can use this software according to the terms and conditions of the Mulan PSL v2.
// You may obtain a copy of Mulan PSL v2 at:
//         http://license.coscl.org.cn/MulanPSL2
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR PURPOSE.
// See the Mulan PSL v2 for more details.

package check

import "github.com/siyuan-note/bazaar/check/rules"

// 对外仍使用 check.Mode / PackageType / Issue，实现位于 rules 子包。
type (
	Mode        = rules.Mode
	PackageType = rules.PackageType
	Issue       = rules.Issue
)

const (
	ModeStage = rules.ModeStage
	ModePR    = rules.ModePR

	TypePlugin   = rules.TypePlugin
	TypeTheme    = rules.TypeTheme
	TypeIcon     = rules.TypeIcon
	TypeTemplate = rules.TypeTemplate
	TypeWidget   = rules.TypeWidget
)

// Input 对单个已解压集市包的检查输入。
// PackageRoot 可以是包根目录，也可以是解压后的临时目录（若其下仅有一个子目录，将自动视为包根）。
type Input struct {
	PackageRoot string
	OwnerRepo   string // owner/repo
	Type        PackageType
	Mode        Mode

	// OldName 已上架时的 package.name；空表示首发（或 Stage 无旧数据）。
	OldName string
	// OldVersion 已上架时的 version；非空时要求新 version 更高。
	OldVersion string

	// OccupiedNames 已上架集市包的 name 集合（键建议为小写；查询时会再 ToLower）。
	// 仅首发（OldName 为空）时用于跨类型唯一性检查；nil 或空则跳过。
	OccupiedNames map[string]struct{}

	// AllowThemeJS 为 true 时，主题允许存在 theme.js（白名单由调用方决定）。
	AllowThemeJS bool
}

// Result 检查结果。
type Result struct {
	OK           bool           `json:"ok"`
	PackageRoot  string         `json:"packageRoot,omitempty"`
	Issues       []Issue        `json:"issues"`
	Manifest     map[string]any `json:"manifest,omitempty"`
	ManifestPath string         `json:"manifestPath,omitempty"`
}

// Check 对单个集市包（已解压目录）执行检查。不下载、不上传、不访问网络。
// 具体规则与短路逻辑均在 rules 包的 Context 流水线中。
func Check(in Input) *Result {
	c := &rules.Context{
		PackageRoot:   in.PackageRoot,
		OwnerRepo:     in.OwnerRepo,
		Type:          in.Type,
		Mode:          in.Mode,
		OldName:       in.OldName,
		OldVersion:    in.OldVersion,
		OccupiedNames: in.OccupiedNames,
		AllowThemeJS:  in.AllowThemeJS,
	}
	rules.Run(c)

	return &Result{
		OK:           c.OK(),
		PackageRoot:  c.Root,
		Issues:       c.Issues,
		Manifest:     c.Manifest,
		ManifestPath: c.ManifestPath,
	}
}

// ParsePackageType 解析类型字符串（plugins/plugin 等均可）。
func ParsePackageType(s string) (PackageType, bool) {
	return rules.ParsePackageType(s)
}

// SanitizeDisplayStrings 对清单中 displayName / description 做 HTML 转义（供 Stage 写索引前调用）。
func SanitizeDisplayStrings(m map[string]any) {
	rules.SanitizeDisplayStrings(m)
}
