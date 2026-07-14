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
	"fmt"

	"github.com/siyuan-note/bazaar/check"
)

// isKeyInSet 判断字符串是否在集合中
func isKeyInSet(
	key string,
	set StringSet,
) (exist bool) {
	_, exist = set[key]

	return
}

// buildRepoHomeURL 构造仓库主页地址
func buildRepoHomeURL(
	repoOwner string,
	repoName string,
) string {
	return fmt.Sprintf("https://github.com/%s/%s", repoOwner, repoName)
}

// resourceTypeToPackageType 将流程层 ResourceType 映射为 check.PackageType
func resourceTypeToPackageType(resourceType ResourceType) (check.PackageType, bool) {
	switch resourceType {
	case icons:
		return check.ParsePackageType("icons")
	case plugins:
		return check.ParsePackageType("plugins")
	case templates:
		return check.ParsePackageType("templates")
	case themes:
		return check.ParsePackageType("themes")
	case widgets:
		return check.ParsePackageType("widgets")
	default:
		return 0, false
	}
}

// releaseIssues 将 Latest Release / package.zip 流程层失败转为 Issue（与 Pkg Check 同一评论格式）。
func releaseIssues(r *Release) []check.Issue {
	if r == nil {
		return []check.Issue{{
			Rule:      "release/latest",
			MessageZh: "无法检查 GitHub Latest Release（内部错误）。请重跑 PR Check；若仍失败请联系维护者。",
			MessageEn: "Internal error: Latest Release could not be checked. Re-run PR Check; contact a maintainer if it keeps failing.",
		}}
	}
	if !r.LatestRelease.Pass {
		return []check.Issue{{
			Rule:      "release/latest",
			MessageZh: "仓库没有可用的 Latest Release（或 API 读取失败）。请在 GitHub 上创建一个 Release，并确保该仓库对集市检查所用令牌可见，然后重跑 PR Check。",
			MessageEn: "No usable Latest Release was found (or the GitHub API call failed). Create a GitHub Release, ensure the repo is visible to the bazaar checker token, then re-run PR Check.",
		}}
	}
	if !r.LatestRelease.PackageZip.Pass || r.LatestRelease.PackageZip.URL == "" {
		return []check.Issue{{
			Rule:      "release/package_zip",
			MessageZh: "Latest Release 中缺少名为 package.zip 的资源文件。请把打包好的 package.zip 作为 Release Asset 上传（文件名必须完全是 package.zip），然后重跑 PR Check。",
			MessageEn: "The Latest Release has no asset named package.zip. Upload package.zip as a Release asset (exact filename package.zip), then re-run PR Check.",
		}}
	}
	if !r.Pass {
		return []check.Issue{{
			Rule:      "release/tag",
			MessageZh: "已找到 Latest Release 与 package.zip，但无法解析 Release 对应的 Git 标签/提交。请确认 tag 指向有效 commit 后重跑 PR Check。",
			MessageEn: "Latest Release and package.zip were found, but the release tag/commit could not be resolved. Ensure the tag points to a valid commit, then re-run PR Check.",
		}}
	}
	return nil
}
