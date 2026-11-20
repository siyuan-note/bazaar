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
	"bytes"
	"encoding/json"
)

// MarshalSortedIndentedJSON 序列化为键序稳定、带缩进的 JSON。
// v 可为 struct，或原始 JSON 字节（[]byte）；struct 会先经 json.Marshal / Unmarshal 转为 JSON 树（map[string]any），
// 再按字典序输出键。先输出紧凑 JSON，再用 json.Indent 缩进，避免 json.MarshalIndent 对嵌套 any 的缩进错乱。
// SetEscapeHTML(false) 避免将 & 等字符转为 \uXXXX。
//
// 这是 stage/*.json 唯一的写出格式：stage 与回填工具共用，避免两处各写一份导致格式漂移。
func MarshalSortedIndentedJSON(v any) ([]byte, error) {
	data, ok := v.([]byte)
	if !ok {
		var err error
		data, err = json.Marshal(v)
		if err != nil {
			return nil, err
		}
	}
	var tree any
	if err := json.Unmarshal(data, &tree); err != nil {
		return nil, err
	}
	var compact bytes.Buffer
	enc := json.NewEncoder(&compact)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(tree); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, bytes.TrimSpace(compact.Bytes()), "", "  "); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
