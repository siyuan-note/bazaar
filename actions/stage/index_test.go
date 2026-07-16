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
	"testing"

	"github.com/siyuan-note/bazaar/actions/util"
)

func TestSameCommitPackageZipChanged(t *testing.T) {
	old := &util.StageRepo{PackageZipAssetID: 42}

	tests := []struct {
		name    string
		old     *util.StageRepo
		assetID int64
		want    bool
	}{
		{
			name:    "无旧条目",
			old:     nil,
			assetID: 99,
		},
		{
			name:    "旧条目无 asset id",
			old:     &util.StageRepo{},
			assetID: 99,
		},
		{
			name:    "asset id 未变化",
			old:     old,
			assetID: 42,
		},
		{
			name:    "asset id 变化",
			old:     old,
			assetID: 99,
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sameCommitPackageZipChanged(tt.old, tt.assetID); got != tt.want {
				t.Fatalf("sameCommitPackageZipChanged() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseHashFromStageURL(t *testing.T) {
	tests := []struct {
		name     string
		stageURL string
		want     string
	}{
		{
			name:     "标准格式",
			stageURL: "owner/repo@abc123def",
			want:     "abc123def",
		},
		{
			name:     "无 @ 分隔符",
			stageURL: "owner/repo",
			want:     "",
		},
		{
			name:     "@ 位于末尾",
			stageURL: "owner/repo@",
			want:     "",
		},
		{
			name:     "空字符串",
			stageURL: "",
			want:     "",
		},
		{
			name:     "仅 @",
			stageURL: "@",
			want:     "",
		},
		{
			name:     "@ 位于开头",
			stageURL: "@hashonly",
			want:     "hashonly",
		},
		{
			name:     "多个 @ 取第一个之后",
			stageURL: "owner/repo@hash@extra",
			want:     "hash@extra",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseHashFromStageURL(tt.stageURL); got != tt.want {
				t.Fatalf("parseHashFromStageURL(%q) = %q, want %q", tt.stageURL, got, tt.want)
			}
		})
	}
}
