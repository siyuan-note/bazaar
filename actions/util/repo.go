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
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/go-github/v89/github"
	"github.com/siyuan-note/bazaar/rules"
)

var (
	// ErrRepoNotPublic 仓库不存在、无法访问，或未设为公开。
	ErrRepoNotPublic = errors.New("repository is not public")
	// ErrRepoNoLicense 仓库根目录缺少 LICENSE / LICENSE.txt。
	ErrRepoNoLicense = errors.New("repository has no LICENSE or LICENSE.txt")
	// ErrRepoEmptyLicense 仓库根目录的 LICENSE / LICENSE.txt 存在但无正文（空文件）。
	ErrRepoEmptyLicense = errors.New("repository LICENSE file is empty")
)

// licenseFileNames 集市要求的许可证文件名（根目录，大小写不敏感）。
var licenseFileNames = []string{"LICENSE", "LICENSE.txt"}

// CheckRepoPublic 确认仓库可通过 GitHub API 访问且为公开仓库。
// 私有、不存在或无权访问时返回 LocalizedError（cause 为 ErrRepoNotPublic）。
func CheckRepoPublic(ctx context.Context, client *github.Client, owner, repo string) error {
	if client == nil {
		return rules.LocalizedErr(
			"内部错误：无法检查仓库是否公开，GitHub 客户端未初始化。这通常是集市检查流程配置问题，请联系维护者。",
			"Internal error: couldn't check whether the repository is public because the GitHub client isn't initialized. This usually means a bazaar checker config problem — please contact a maintainer.",
			ErrRepoNotPublic,
		)
	}
	if ctx == nil {
		ctx = context.Background()
	}

	// REF https://docs.github.com/en/rest/repos/repos#get-a-repository
	ghRepo, _, err := client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return rules.LocalizedErr(
			fmt.Sprintf("无法访问仓库 `%s/%s`：%v。请确认仓库存在且已设为公开（Public）。", owner, repo, err),
			fmt.Sprintf("Couldn't access repository `%s/%s`: %v. Please make sure the repository exists and is set to Public.", owner, repo, err),
			fmt.Errorf("%w: %w", ErrRepoNotPublic, err),
		)
	}
	if ghRepo.GetPrivate() {
		return rules.LocalizedErr(
			fmt.Sprintf("仓库 `%s/%s` 未设为公开。请将 GitHub 仓库设为 Public 后重试；集市检查无法审核私有仓库。", owner, repo),
			fmt.Sprintf("Repository `%s/%s` is not public. Please set the GitHub repository to Public and try again; bazaar checks cannot review private repositories.", owner, repo),
			ErrRepoNotPublic,
		)
	}
	return nil
}

// CheckRepoLicenseFile 确认仓库默认分支根目录存在 `LICENSE` 或 `LICENSE.txt`（大小写不敏感），且文件非空。
// 缺失时返回 LocalizedError（cause 为 ErrRepoNoLicense）；空文件为 ErrRepoEmptyLicense；API 失败时同样返回 LocalizedError。
func CheckRepoLicenseFile(ctx context.Context, client *github.Client, owner, repo string) error {
	if client == nil {
		return rules.LocalizedErr(
			"内部错误：无法检查许可证文件，GitHub 客户端未初始化。这通常是集市检查流程配置问题，请联系维护者。",
			"Internal error: couldn't check for a license file because the GitHub client isn't initialized. This usually means a bazaar checker config problem — please contact a maintainer.",
			ErrRepoNoLicense,
		)
	}
	if ctx == nil {
		ctx = context.Background()
	}

	// 列出默认分支根目录（含 size，不含文件内容）。
	// REF https://docs.github.com/en/rest/repos/contents#get-repository-content
	_, entries, _, err := client.Repositories.GetContents(ctx, owner, repo, "", nil)
	if err != nil {
		return rules.LocalizedErr(
			fmt.Sprintf("无法读取仓库 `%s/%s` 根目录以检查许可证文件：%v。请确认仓库可访问。", owner, repo, err),
			fmt.Sprintf("Couldn't list the root of repository `%s/%s` to check for a license file: %v. Please make sure the repository is accessible.", owner, repo, err),
			fmt.Errorf("%w: %w", ErrRepoNoLicense, err),
		)
	}

	for _, entry := range entries {
		if entry.GetType() != "file" {
			continue
		}
		name := entry.GetName()
		for _, want := range licenseFileNames {
			if !strings.EqualFold(name, want) {
				continue
			}
			// size == 0 表示空文件，无许可证正文
			if entry.GetSize() < 1 {
				return rules.LocalizedErr(
					fmt.Sprintf("仓库 `%s/%s` 根目录的许可证文件 `%s` 为空。请写入许可证正文。", owner, repo, name),
					fmt.Sprintf("The license file `%s` at the root of repository `%s/%s` is empty. Please add the license text.", name, owner, repo),
					ErrRepoEmptyLicense,
				)
			}
			return nil
		}
	}

	return rules.LocalizedErr(
		fmt.Sprintf("仓库 `%s/%s` 根目录缺少许可证文件 `LICENSE` 或 `LICENSE.txt`。请添加合适的许可证文件。", owner, repo),
		fmt.Sprintf("Repository `%s/%s` is missing a license file named `LICENSE` or `LICENSE.txt` at the repository root. Please add an appropriate license file.", owner, repo),
		ErrRepoNoLicense,
	)
}
