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

// Input 对单个已解压集市包的检查输入。
// PackageRoot 可以是包根目录，也可以是解压后的临时目录（若其下仅有一个子目录，将自动视为包根）。
type Input struct {
	PackageRoot string
	OwnerRepo   string // owner/repo
	Type        PackageType

	// ZipData 为原始 package.zip 字节；非空时校验 zip 内路径分隔符等依赖原始条目名的规则。
	// 为空则跳过这类步骤（例如仅用解压目录做单测）。
	ZipData []byte

	OldName    string // 已上架时的 package.name；空表示首发（或 Stage 无旧数据）
	OldVersion string // 已上架时的 version；非空时要求新 version 更高

	// OccupiedNames 已上架集市包的 name 集合（键建议为小写；查询时会再 ToLower）。
	// 仅首发（OldName 为空）时用于跨类型唯一性检查；nil 或空则跳过。
	OccupiedNames map[string]struct{}

	// AllowThemeJS 为 true 时，主题允许存在 theme.js。
	// 白名单由调用方读取 config/themes-theme-js-allowlist.txt 决定（存量豁免）；
	// false 表示禁止（含未在白名单中的新主题）。REF https://github.com/siyuan-note/bazaar/issues/1821
	AllowThemeJS bool
}

// Result 检查结果。
type Result struct {
	OK          bool
	PackageRoot string
	Issues      []Issue
	Package     Package
}

// Check 对单个集市包（已解压目录）执行检查。不下载、不上传、不访问网络。
// 具体规则与短路逻辑在 Context 流水线中（见 run.go）。
func Check(in Input) *Result {
	if !in.Type.valid() {
		return &Result{
			Issues: []Issue{issue(
				"内部错误：未提供有效的集市包类型。这通常是集市检查流程配置问题，请联系维护者。",
				"Internal error: no valid package type was provided. This usually means a bazaar checker config problem — please contact a maintainer.",
			)},
		}
	}

	c := &Context{
		PackageRoot:   in.PackageRoot,
		OwnerRepo:     in.OwnerRepo,
		Type:          in.Type,
		ZipData:       in.ZipData,
		OldName:       in.OldName,
		OldVersion:    in.OldVersion,
		OccupiedNames: in.OccupiedNames,
		AllowThemeJS:  in.AllowThemeJS,
	}
	Run(c)

	return &Result{
		OK:          c.OK(),
		PackageRoot: c.Root,
		Issues:      c.Issues,
		Package:     c.Package,
	}
}
