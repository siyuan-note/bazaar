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
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// packageHashLen 为 stage URL / OSS 键 owner/repo@hash 中 hash 的长度。
// 取 package.zip 内容 SHA-256 hex 的前缀。
const packageHashLen = 7

// NormalizeAssetDigest 将 GitHub asset digest（`sha256:<hex>` 或裸 hex）规范为 64 位小写 hex。
// 无法解析时返回空字符串。
func NormalizeAssetDigest(digest string) string {
	hexPart := strings.TrimSpace(strings.ToLower(digest))
	if after, ok := strings.CutPrefix(hexPart, "sha256:"); ok {
		hexPart = strings.TrimSpace(after)
	}
	if len(hexPart) != 64 {
		return ""
	}
	for i := 0; i < len(hexPart); i++ {
		c := hexPart[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return ""
		}
	}
	return hexPart
}

// SHA256Hex 计算数据的 SHA-256 并返回小写 hex。
func SHA256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// PackageHashFromSHA256 由 64 位 SHA-256 hex 生成集市包 hash（前 packageHashLen 位）。
func PackageHashFromSHA256(sha256Hex string) string {
	hexPart := strings.TrimSpace(strings.ToLower(sha256Hex))
	if len(hexPart) < packageHashLen {
		return ""
	}
	for i := 0; i < packageHashLen; i++ {
		c := hexPart[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return ""
		}
	}
	return hexPart[:packageHashLen]
}

// PackageHashFromDigest 由 GitHub asset digest 生成集市包 hash。
func PackageHashFromDigest(digest string) string {
	return PackageHashFromSHA256(NormalizeAssetDigest(digest))
}
