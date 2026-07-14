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

// Context 单次检查的共享状态：输入字段 + 运行中填充的根目录/清单 + 累计 Issues。
type Context struct {
	// 输入（由调用方填入）
	PackageRoot   string
	OwnerRepo     string
	Type          PackageType
	OldName       string
	OldVersion    string
	OccupiedNames map[string]struct{}
	AllowThemeJS  bool

	// 运行中填充
	Owner        string
	Repo         string
	Root         string
	Manifest     map[string]any
	ManifestPath string
	Issues       []Issue

	// halt 为 true 时，后续步骤应自跳过且不再追加问题（例如输入/包根无效）。
	halt bool
}

// Add 追加问题。
func (c *Context) Add(issues ...Issue) {
	if len(issues) == 0 {
		return
	}
	c.Issues = append(c.Issues, issues...)
}

// Halt 标记后续步骤应跳过（流水线仍会跑完，但各步自行 return）。
func (c *Context) Halt() {
	c.halt = true
}

// Halted 报告是否已中止后续实质检查。
func (c *Context) Halted() bool {
	return c.halt
}

// OK 表示尚未产生任何问题。
func (c *Context) OK() bool {
	return len(c.Issues) == 0
}
