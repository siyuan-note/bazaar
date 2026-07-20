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
	"os"
	"strings"

	"github.com/google/go-github/v89/github"
)

var (
	BAZAAR_REVIEWERS = os.Getenv("BAZAAR_REVIEWERS") // 逗号分隔的 GitHub 用户名（仓库 Variables 注入）
)

// parseCSVList 解析逗号分隔名单：去空白、去空项、保序去重（大小写不敏感）。
func parseCSVList(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, p := range parts {
		item := strings.TrimSpace(p)
		if item == "" {
			continue
		}
		key := strings.ToLower(item)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

// filterOutLogin 从列表中去掉指定登录名（大小写不敏感）；exclude 为空则原样返回。
func filterOutLogin(logins []string, exclude string) []string {
	if exclude == "" || len(logins) == 0 {
		return logins
	}
	ex := strings.ToLower(exclude)
	out := make([]string, 0, len(logins))
	for _, login := range logins {
		if strings.ToLower(login) == ex {
			continue
		}
		out = append(out, login)
	}
	return out
}

// filterOutSet 去掉 already 中已有的项（大小写不敏感）。
func filterOutSet(items []string, already map[string]struct{}) []string {
	if len(items) == 0 || len(already) == 0 {
		return items
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if _, ok := already[strings.ToLower(item)]; ok {
			continue
		}
		out = append(out, item)
	}
	return out
}

// maybeRequestReviewers 在检查通过后请求审查者。
// 名单来自 BAZAAR_REVIEWERS；会跳过 PR 作者与已请求者。
// 缺环境变量、名单为空或 API 失败时只记日志，不中断检查。
func maybeRequestReviewers(checkResult *CheckResult) {
	if !checkResultCIPassed(checkResult) {
		return
	}
	users := parseCSVList(BAZAAR_REVIEWERS)
	if len(users) == 0 {
		logger.Infof("skip request reviewers: BAZAAR_REVIEWERS empty")
		return
	}

	owner, repo, prNumber, ok := prIdentity()
	if !ok {
		logger.Infof("skip request reviewers: PR_NUMBER or GITHUB_REPOSITORY not set / invalid")
		return
	}

	pr, _, err := githubRepoClient.PullRequests.Get(githubContext, owner, repo, prNumber)
	if err != nil {
		logger.Errorf("get PR #%d for reviewers failed: %s", prNumber, err)
		return
	}
	users = filterOutLogin(users, pr.GetUser().GetLogin())

	requested, _, err := githubRepoClient.PullRequests.ListReviewers(githubContext, owner, repo, prNumber)
	if err != nil {
		logger.Errorf("list PR #%d reviewers failed: %s", prNumber, err)
		return
	}
	alreadyUsers := make(map[string]struct{}, len(requested.Users))
	for _, u := range requested.Users {
		alreadyUsers[strings.ToLower(u.GetLogin())] = struct{}{}
	}
	users = filterOutSet(users, alreadyUsers)
	if len(users) == 0 {
		logger.Infof("PR #%d reviewers already requested, skip", prNumber)
		return
	}

	_, _, err = githubRepoClient.PullRequests.RequestReviewers(githubContext, owner, repo, prNumber, github.ReviewersRequest{
		Reviewers: users,
	})
	if err != nil {
		logger.Errorf("request reviewers on PR #%d failed: %s", prNumber, err)
		return
	}
	logger.Infof("requested reviewers on PR #%d: %v", prNumber, users)
}
