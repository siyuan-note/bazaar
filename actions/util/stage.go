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
	"strings"

	"github.com/88250/gulu"
)

// StageFile 对应 stage/*.json 顶层结构。
type StageFile struct {
	Repos []StageRepo `json:"repos"`
}

// StageRepo 对应 stage/*.json 中 repos 数组的单项。
type StageRepo struct {
	URL         string       `json:"url"`
	Updated     string       `json:"updated"`
	Stars       int          `json:"stars"`
	OpenIssues  int          `json:"openIssues"`
	Size        int64        `json:"size"`
	InstallSize int64        `json:"installSize"`
	Package     StagePackage `json:"package"`
}

// StagePackage 对应 stage 条目中的 package 字段，合并各资源类型 manifest 的公共与专有字段。
type StagePackage struct {
	Name              string        `json:"name"`
	Author            string        `json:"author"`
	URL               string        `json:"url"`
	Version           string        `json:"version"`
	MinAppVersion     string        `json:"minAppVersion"`
	DisplayName       LocaleStrings `json:"displayName"`
	Description       LocaleStrings `json:"description"`
	Readme            LocaleStrings `json:"readme"`
	Funding           *Funding      `json:"funding"`
	Keywords          []string      `json:"keywords"`
	Backends          []string      `json:"backends,omitempty"`
	Frontends         []string      `json:"frontends,omitempty"`
	DisabledInPublish bool          `json:"disabledInPublish,omitempty"`
	Modes             []string      `json:"modes,omitempty"`
}

// LocaleStrings 表示按 locale 键（如 default、zh_CN、en_US）组织的多语言字符串。
type LocaleStrings map[string]string

// Funding 表示 package manifest 中的资助信息。
type Funding struct {
	OpenCollective string   `json:"openCollective"`
	Patreon        string   `json:"patreon"`
	GitHub         string   `json:"github"`
	Custom         []string `json:"custom"`
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
	idx := strings.Index(stageURL, "@")
	if idx <= 0 {
		return "", false
	}
	return stageURL[:idx], true
}
