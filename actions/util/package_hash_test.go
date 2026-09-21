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
	"strings"
	"testing"
)

func TestNormalizeAssetDigestAndPackageHash(t *testing.T) {
	const full = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	tests := []struct {
		name   string
		digest string
		want   string
		hash   string
	}{
		{name: "空", digest: "", want: "", hash: ""},
		{name: "带前缀", digest: "sha256:" + full, want: full, hash: "0123456"},
		{name: "大写", digest: "SHA256:" + strings.ToUpper(full), want: full, hash: "0123456"},
		{name: "无前缀", digest: full, want: full, hash: "0123456"},
		{name: "过短", digest: "sha256:abc", want: "", hash: ""},
		{name: "非法字符", digest: "sha256:" + strings.Replace(full, "0", "g", 1), want: "", hash: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeAssetDigest(tt.digest)
			if got != tt.want {
				t.Fatalf("NormalizeAssetDigest(%q) = %q, want %q", tt.digest, got, tt.want)
			}
			if h := PackageHashFromDigest(tt.digest); h != tt.hash {
				t.Fatalf("PackageHashFromDigest(%q) = %q, want %q", tt.digest, h, tt.hash)
			}
		})
	}

	data := []byte("hello")
	sum := SHA256Hex(data)
	if len(sum) != 64 {
		t.Fatalf("SHA256Hex len=%d", len(sum))
	}
	if PackageHashFromSHA256(sum) == "" {
		t.Fatal("PackageHashFromSHA256 empty")
	}
}
