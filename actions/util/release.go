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
)

var (
	// ErrNoLatestRelease 仓库没有 Latest Release，或 GitHub API 读取失败。
	ErrNoLatestRelease = errors.New("no latest release")
	// ErrNoPackageZip Latest Release 中缺少名为 package.zip 的资源。
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

// FetchLatestRelease 获取 Latest Release，并校验 package.zip 与 tag → commit。
//
// 返回的 error 用 sentinel（ErrNoLatestRelease / ErrNoPackageZip / ErrReleaseTag）标识失败阶段。
//
// 若在 GetLatestRelease 之后失败（缺 package.zip 或 tag 无效），仍返回已取到的 Tag、URL 等字段：
// PR Check 的评论模板 check-result.md.tpl 在报错时也会渲染 Release 链接，便于作者点开对应 Release 补传 zip 或修 tag。
// Stage 在 err != nil 时不使用这些字段，可忽略。
func FetchLatestRelease(ctx context.Context, client *github.Client, owner, repo string) (LatestRelease, error) {
	var info LatestRelease
	if client == nil {
		return info, fmt.Errorf("%w: github client is nil", ErrNoLatestRelease)
	}
	if ctx == nil {
		ctx = context.Background()
	}

	// REF https://docs.github.com/en/rest/releases/releases#get-the-latest-release
	release, _, err := client.Repositories.GetLatestRelease(ctx, owner, repo)
	if err != nil {
		return info, fmt.Errorf("%w: %w", ErrNoLatestRelease, err)
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
		return info, ErrNoPackageZip
	}

	info.CommitSHA, err = resolveReleaseTagCommit(ctx, client, owner, repo, info.Tag)
	if err != nil {
		// package.zip 已找到；保留 Tag / URL / PackageZipAssetID 供 PR 评论展示链接。
		return info, fmt.Errorf("%w: %w", ErrReleaseTag, err)
	}
	return info, nil
}

func resolveReleaseTagCommit(ctx context.Context, client *github.Client, owner, repo, tagName string) (string, error) {
	if tagName == "" {
		return "", fmt.Errorf("tag name is empty")
	}

	// REF https://pkg.go.dev/github.com/google/go-github/v89/github#GitService.GetRef
	ref, _, err := client.Git.GetRef(ctx, owner, repo, "tags/"+tagName)
	if err != nil {
		return "", fmt.Errorf("get ref tags/%s: %w", tagName, err)
	}

	sha := ref.GetObject().GetSHA()
	if sha == "" {
		return "", fmt.Errorf("ref tags/%s has empty sha", tagName)
	}

	switch ref.GetObject().GetType() {
	case "commit":
		// 轻量 tag，object.sha 即为 commit
		return sha, nil
	case "tag":
		// REF https://pkg.go.dev/github.com/google/go-github/v89/github#GitService.GetTag
		tag, _, err := client.Git.GetTag(ctx, owner, repo, sha)
		if err != nil {
			return "", fmt.Errorf("get annotated tag %s (%s): %w", tagName, sha, err)
		}
		commitSHA := tag.GetObject().GetSHA()
		if commitSHA == "" {
			return "", fmt.Errorf("annotated tag %s (%s) has empty commit sha", tagName, sha)
		}
		return commitSHA, nil
	default:
		return "", fmt.Errorf("ref tags/%s has unknown type %q", tagName, ref.GetObject().GetType())
	}
}
