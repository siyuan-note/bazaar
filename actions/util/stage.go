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
	"os"
	"path/filepath"
	"strings"

	"github.com/88250/gulu"
	"github.com/siyuan-note/bazaar/rules"
)

// StageFile 对应 stage/*.json 顶层结构。
type StageFile struct {
	Repos []StageRepo `json:"repos"`
}

// StageRepo 对应 stage/*.json 中 repos 数组的单项。
type StageRepo struct {
	URL               string        `json:"url"`
	Updated           string        `json:"updated"`
	Stars             int           `json:"stars"`
	OpenIssues        int           `json:"openIssues"`
	Size              int64         `json:"size"`
	InstallSize       int64         `json:"installSize"`
	PackageZipAssetID int64         `json:"packageZipAssetId,omitempty"`
	Package           rules.Package `json:"package"`
}

// StageIndexFile 供 OSS 集市索引发布的 stage JSON，不含 stage 流程内部字段。
type StageIndexFile struct {
	Repos []StageIndexRepo `json:"repos"`
}

// StageIndexRepo 对应发布后集市索引 repos 数组的单项。
type StageIndexRepo struct {
	URL         string        `json:"url"`
	Updated     string        `json:"updated"`
	Stars       int           `json:"stars"`
	OpenIssues  int           `json:"openIssues"`
	Size        int64         `json:"size"`
	InstallSize int64         `json:"installSize"`
	Package     rules.Package `json:"package"`
}

// ForPublicIndex 转为供客户端使用的集市索引，去掉 stage 流程内部字段，
// 并剔除与 default 值相同的冗余多语言键（对 locale map 做拷贝，不修改原 stage 条目）。
func (f StageFile) ForPublicIndex() StageIndexFile {
	out := StageIndexFile{Repos: make([]StageIndexRepo, len(f.Repos))}
	for i, repo := range f.Repos {
		out.Repos[i] = StageIndexRepo{
			URL:         repo.URL,
			Updated:     repo.Updated,
			Stars:       repo.Stars,
			OpenIssues:  repo.OpenIssues,
			Size:        repo.Size,
			InstallSize: repo.InstallSize,
			Package:     rules.PackageForPublicIndex(repo.Package),
		}
	}
	return out
}

// ReadStageFile 读取并解析 stage JSON 文件。文件不存在时返回空 StageFile（不报错）。
func ReadStageFile(filePath string) (*StageFile, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &StageFile{Repos: nil}, nil
		}
		return nil, err
	}

	var stageFile StageFile
	if err = gulu.JSON.UnmarshalJSON(data, &stageFile); err != nil {
		return nil, err
	}
	return &stageFile, nil
}

// OwnerRepoFromStageURL 从 stage 条目的 URL（格式 owner/repo@hash）中解析 owner/repo。
func OwnerRepoFromStageURL(stageURL string) (ownerRepo string, ok bool) {
	ownerRepo, _, ok = strings.Cut(stageURL, "@")
	if !ok || ownerRepo == "" {
		return "", false
	}
	return ownerRepo, true
}

// FindStageRepo 从 bazaarHead/stage/<type>.json 中按 owner/repo 查找条目。
// 文件不存在或未找到时返回 (nil, nil)；读取/解析失败时返回错误。
func FindStageRepo(bazaarHead string, packageType rules.PackageType, ownerRepo string) (*StageRepo, error) {
	if ownerRepo == "" {
		return nil, nil
	}
	filePath := filepath.Join(bazaarHead, "stage", packageType.StageJSONFile())
	stageFile, err := ReadStageFile(filePath)
	if err != nil {
		return nil, err
	}
	for i := range stageFile.Repos {
		key, ok := OwnerRepoFromStageURL(stageFile.Repos[i].URL)
		if ok && key == ownerRepo {
			return &stageFile.Repos[i], nil
		}
	}
	return nil, nil
}
