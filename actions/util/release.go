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
	"time"

	"github.com/google/go-github/v89/github"
	"github.com/siyuan-note/bazaar/rules"
)

var (
	// ErrNoLatestRelease 仓库没有 Latest Release，或 GitHub API 读取失败。
	ErrNoLatestRelease = errors.New("no latest release")
	// ErrNoPackageZip Latest Release 中缺少名为 `package.zip` 的资源。
	ErrNoPackageZip = errors.New("package.zip not found in latest release")
	// ErrReleaseTag 无法将 Release tag 解析为有效 commit。
	ErrReleaseTag = errors.New("release tag could not be resolved")
)

// LatestRelease 仓库 Latest Release 的纯数据摘要。
type LatestRelease struct {
	Tag               string
	URL               string
	Published         string // RFC3339
	PackageZipAssetID int64
	CommitSHA         string
}

// FetchLatestRelease 获取 Latest Release，并校验 `package.zip` 与 tag → commit。
//
// 返回的 error 用 sentinel（ErrNoLatestRelease / ErrNoPackageZip / ErrReleaseTag）标识失败阶段。
//
// 若在 GetLatestRelease 之后失败（缺 `package.zip` 或 tag 无效），仍返回已取到的 Tag、URL 等字段：
// PR Check 的评论模板 check-result.md.tpl 在报错时也会渲染 Release 链接，便于作者点开对应 Release 补传 zip 或修 tag。
// Stage 在 err != nil 时不使用这些字段，可忽略。
func FetchLatestRelease(ctx context.Context, client *github.Client, owner, repo string) (LatestRelease, error) {
	var info LatestRelease
	if client == nil {
		return info, rules.LocalizedErr(
			"内部错误：无法获取 Latest Release，GitHub 客户端未初始化。这通常是集市检查流程配置问题，请联系维护者。",
			"Internal error: could not fetch Latest Release because the GitHub client is not initialized. This is usually a bazaar checker configuration issue; contact a maintainer.",
			ErrNoLatestRelease,
		)
	}
	if ctx == nil {
		ctx = context.Background()
	}

	// REF https://docs.github.com/en/rest/releases/releases#get-the-latest-release
	release, _, err := client.Repositories.GetLatestRelease(ctx, owner, repo)
	if err != nil {
		return info, rules.LocalizedErr(
			fmt.Sprintf("无法获取 Latest Release：%v。请在 GitHub 上创建 Release，并确保仓库已设为公开。", err),
			fmt.Sprintf("Could not fetch Latest Release: %v. Create a GitHub Release and ensure the repository is public.", err),
			fmt.Errorf("%w: %w", ErrNoLatestRelease, err),
		)
	}

	info.Tag = release.GetTagName()
	info.URL = release.GetHTMLURL()
	info.Published = release.GetPublishedAt().Format(time.RFC3339)

	for _, asset := range release.Assets {
		if asset.GetName() == "package.zip" {
			info.PackageZipAssetID = asset.GetID()
			break
		}
	}
	if info.PackageZipAssetID == 0 {
		// Release 已存在；保留 Tag / URL 供 PR 评论展示链接。
		return info, rules.LocalizedErr(
			"Latest Release 中缺少名为 `package.zip` 的资源文件。请把打包好的 `package.zip` 作为 Release Asset 上传（文件名必须是 `package.zip`）。",
			"The Latest Release has no asset named `package.zip`. Upload `package.zip` as a Release asset (the filename must be `package.zip`).",
			ErrNoPackageZip,
		)
	}

	info.CommitSHA, err = resolveReleaseTagCommit(ctx, client, owner, repo, info.Tag)
	if err != nil {
		// `package.zip` 已找到；保留 Tag / URL / PackageZipAssetID 供 PR 评论展示链接。
		zh, en := rules.LocalizedMessages(err)
		return info, rules.LocalizedErr(
			fmt.Sprintf("已找到 Latest Release 与 `package.zip`，但无法解析 Release 标签 `%s` 对应的提交：%s。请在 GitHub 上确认该 tag 指向有效 commit（可尝试删除并重新创建 tag）。", info.Tag, zh),
			fmt.Sprintf("Latest Release and `package.zip` were found, but release tag `%s` could not be resolved to a commit: %s. Ensure the tag points to a valid commit on GitHub (try recreating the tag).", info.Tag, en),
			fmt.Errorf("%w: %w", ErrReleaseTag, err),
		)
	}
	return info, nil
}

func resolveReleaseTagCommit(ctx context.Context, client *github.Client, owner, repo, tagName string) (string, error) {
	if tagName == "" {
		return "", rules.LocalizedErr(
			"Latest Release 未关联有效的标签名。请在 GitHub Release 上填写 tag 并指向有效 commit。",
			"The Latest Release has no valid tag name. Set a tag on the GitHub Release that points to a valid commit.",
			nil,
		)
	}

	// REF https://pkg.go.dev/github.com/google/go-github/v89/github#GitService.GetRef
	ref, _, err := client.Git.GetRef(ctx, owner, repo, "tags/"+tagName)
	if err != nil {
		return "", rules.LocalizedErr(
			fmt.Sprintf("无法在仓库中找到 Release 标签 `%s`：%v。请确认 tag 已推送到 GitHub 且拼写正确。", tagName, err),
			fmt.Sprintf("Could not find release tag `%s` in the repository: %v. Ensure the tag is pushed to GitHub and spelled correctly.", tagName, err),
			err,
		)
	}

	sha := ref.GetObject().GetSHA()
	if sha == "" {
		return "", rules.LocalizedErr(
			fmt.Sprintf("Release 标签 `%s` 没有关联有效的 Git 对象。请删除并重新创建该 tag，使其指向有效 commit。", tagName),
			fmt.Sprintf("Release tag `%s` has no associated Git object. Delete and recreate the tag so it points to a valid commit.", tagName),
			nil,
		)
	}

	switch ref.GetObject().GetType() {
	case "commit":
		// 轻量 tag，object.sha 即为 commit
		return sha, nil
	case "tag":
		// REF https://pkg.go.dev/github.com/google/go-github/v89/github#GitService.GetTag
		tag, _, err := client.Git.GetTag(ctx, owner, repo, sha)
		if err != nil {
			return "", rules.LocalizedErr(
				fmt.Sprintf("无法读取 Release 附注标签 `%s`：%v。请确认 tag 未损坏，必要时在 GitHub 上重新创建。", tagName, err),
				fmt.Sprintf("Could not read annotated release tag `%s`: %v. Ensure the tag is valid or recreate it on GitHub.", tagName, err),
				err,
			)
		}
		commitSHA := tag.GetObject().GetSHA()
		if commitSHA == "" {
			return "", rules.LocalizedErr(
				fmt.Sprintf("Release 附注标签 `%s` 未指向有效 commit。请重新创建 tag 并关联到正确的提交。", tagName),
				fmt.Sprintf("Annotated release tag `%s` does not point to a valid commit. Recreate the tag and link it to the correct commit.", tagName),
				nil,
			)
		}
		return commitSHA, nil
	default:
		return "", rules.LocalizedErr(
			fmt.Sprintf("Release 标签 `%s` 的类型不受支持（`%s`）。请改用指向 commit 的轻量 tag 或标准附注 tag。", tagName, ref.GetObject().GetType()),
			fmt.Sprintf("Release tag `%s` has unsupported type `%s`. Use a lightweight tag or a standard annotated tag that points to a commit.", tagName, ref.GetObject().GetType()),
			nil,
		)
	}
}
