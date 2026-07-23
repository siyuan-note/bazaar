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
	"errors"
	"net/http"
	"time"

	"github.com/google/go-github/v89/github"
)

// NewGitHubClient 创建带 Token、超时与 User-Agent 的 GitHub API 客户端。
func NewGitHubClient(token string, timeout time.Duration) (*github.Client, error) {
	return github.NewClient(
		github.WithAuthToken(token),
		github.WithTimeout(timeout),
		github.WithUserAgent(UserAgent),
	)
}

// IsGitHubRateLimit 判断 err 是否为 GitHub REST API 主限流或次级（滥用）限流。
// 可穿透 LocalizedError / fmt %w 等包装链。
func IsGitHubRateLimit(err error) bool {
	if err == nil {
		return false
	}
	if _, ok := errors.AsType[*github.RateLimitError](err); ok {
		return true
	}
	_, ok := errors.AsType[*github.AbuseRateLimitError](err)
	return ok
}

// IsGitHubNotFound 判断 err 是否为 GitHub REST API 的 404 Not Found。
// 可穿透 LocalizedError / fmt %w 等包装链。
func IsGitHubNotFound(err error) bool {
	if err == nil {
		return false
	}
	resp, ok := errors.AsType[*github.ErrorResponse](err)
	if !ok || resp.Response == nil {
		return false
	}
	return resp.Response.StatusCode == http.StatusNotFound
}

// GitHubRepoURL 由 owner/repo 拼出仓库主页地址
func GitHubRepoURL(ownerRepo string) string {
	return "https://github.com/" + ownerRepo
}
