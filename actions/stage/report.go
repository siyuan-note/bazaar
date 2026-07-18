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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/google/go-github/v89/github"
	"github.com/siyuan-note/bazaar/actions/util"
	"github.com/siyuan-note/bazaar/rules"
)

// 固定汇总 Issue：https://github.com/siyuan-note/bazaar/issues/1921
const stageFailIssue = 1921

// 评论身份标记：<!-- bazaar-stage-fail {"repo":"owner/repo"} -->
const stageFailCommentMarkerPrefix = "<!-- bazaar-stage-fail "

// stageFailMarker 为 HTML 注释中的单行 JSON，便于 upsert/delete 时稳定解析。
type stageFailMarker struct {
	Repo string `json:"repo"`
}

var (
	GITHUB_REPOSITORY = os.Getenv("GITHUB_REPOSITORY") // bazaar 仓库 owner/repo（GitHub Actions 自动注入）
	GITHUB_SERVER_URL = os.Getenv("GITHUB_SERVER_URL") // GitHub 实例 URL，如 https://github.com
	GITHUB_RUN_ID     = os.Getenv("GITHUB_RUN_ID")     // 当前工作流 run id，如 12345678901
)

type stageReportKind int

const (
	stageReportSkip stageReportKind = iota // hash 未变等，不改动 Issue 评论
	stageReportPass                        // 本轮成功入库，删除对应评论
	stageReportFail                        // 本轮失败，upsert 评论
)

// stageReport 单仓本轮 Stage 结果，供失败汇总同步到固定 Issue。
type stageReport struct {
	OwnerRepo   string
	PackageType rules.PackageType
	Kind        stageReportKind
	Release     util.LatestRelease
	Hash        string
	Issues      []rules.Issue
	KeptOld     bool
}

type stageReportCollector struct {
	mu      sync.Mutex
	reports []stageReport
}

func (c *stageReportCollector) add(r stageReport) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reports = append(c.reports, r)
}

func (c *stageReportCollector) snapshot() []stageReport {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]stageReport, len(c.reports))
	copy(out, c.reports)
	return out
}

func bazaarOwnerRepo() (owner, repo string, ok bool) {
	owner, repo, cutOK := strings.Cut(GITHUB_REPOSITORY, "/")
	if !cutOK || owner == "" || repo == "" {
		return "", "", false
	}
	return owner, repo, true
}

func stageFailCommentMarker(ownerRepo string) string {
	payload, err := json.Marshal(stageFailMarker{Repo: ownerRepo})
	if err != nil {
		// Marshal 基本类型失败不应发生；退回空 repo 便于调用方察觉
		payload = []byte(`{"repo":""}`)
	}
	return stageFailCommentMarkerPrefix + string(payload) + " -->"
}

func parseStageFailCommentMarker(body string) (ownerRepo string, ok bool) {
	_, after, cutOK := strings.Cut(body, stageFailCommentMarkerPrefix)
	if !cutOK {
		return "", false
	}
	raw, _, cutOK := strings.Cut(after, " -->")
	if !cutOK {
		return "", false
	}
	var m stageFailMarker
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &m); err != nil {
		return "", false
	}
	if m.Repo == "" || !strings.Contains(m.Repo, "/") {
		return "", false
	}
	return m.Repo, true
}

func formatStageIssueIndex(i, total int) string {
	if total < 1 {
		total = 1
	}
	width := max(len(strconv.Itoa(total)), 2)
	return fmt.Sprintf("%0*d", width, i+1)
}

func workflowRunURL() string {
	server := strings.TrimRight(GITHUB_SERVER_URL, "/")
	if server == "" || GITHUB_REPOSITORY == "" || GITHUB_RUN_ID == "" {
		return ""
	}
	return server + "/" + GITHUB_REPOSITORY + "/actions/runs/" + GITHUB_RUN_ID
}

// formatStageFailComment 生成固定 Issue 下单仓失败评论正文（含 marker，便于 upsert/delete）。
func formatStageFailComment(r stageReport) string {
	var b strings.Builder
	b.WriteString(stageFailCommentMarker(r.OwnerRepo))
	b.WriteByte('\n')
	b.WriteString("### [")
	b.WriteString(r.OwnerRepo)
	b.WriteString("](")
	b.WriteString(util.GitHubRepoURL(r.OwnerRepo))
	b.WriteString(") (`")
	b.WriteString(r.PackageType.String())
	b.WriteString("`)\n")

	if r.Release.URL != "" {
		b.WriteString("\n最新 Release / Latest Release: [")
		b.WriteString(r.Release.Tag)
		b.WriteString("](")
		b.WriteString(r.Release.URL)
		b.WriteString(")")
		if r.Hash != "" {
			b.WriteString(" · hash `")
			b.WriteString(r.Hash)
			b.WriteString("`")
		}
		b.WriteByte('\n')
	} else if r.Hash != "" {
		b.WriteString("\nhash `")
		b.WriteString(r.Hash)
		b.WriteString("`\n")
	}

	if r.KeptOld {
		b.WriteString("\n本轮未更新入库，已沿用旧 stage 条目。\n")
		b.WriteString("This run did not update the staged package; the previous stage entry was kept.\n")
	}

	if len(r.Issues) > 0 {
		b.WriteString("\n检测到以下问题，请在修复之后提升清单字段 `version`，重新打包 `package.zip` 并发布新的 GitHub Release（标记为 Latest）。\n\n")
		b.WriteString("We found the following issues. Please fix them, bump the manifest `version`, rebuild `package.zip`, and publish a new GitHub Release marked as Latest.\n")
	}

	for i, issue := range r.Issues {
		b.WriteByte('\n')
		b.WriteString("[")
		b.WriteString(formatStageIssueIndex(i, len(r.Issues)))
		b.WriteString("]\n\n")
		b.WriteString(issue.MessageZh)
		b.WriteString("\n\n")
		b.WriteString(issue.MessageEn)
		b.WriteString("\n\n---\n")
	}

	if runURL := workflowRunURL(); runURL != "" {
		b.WriteString("\n工作流 / Workflow: ")
		b.WriteString(runURL)
		b.WriteByte('\n')
	}
	return b.String()
}

type stageFailComment struct {
	ID        int64
	OwnerRepo string
}

func listStageFailComments(ctx context.Context, client *github.Client, owner, repo string, issueNumber int) ([]stageFailComment, error) {
	var out []stageFailComment
	opts := &github.IssueListCommentsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	for {
		comments, resp, err := client.Issues.ListComments(ctx, owner, repo, issueNumber, opts)
		if err != nil {
			return nil, err
		}
		for _, c := range comments {
			ownerRepo, ok := parseStageFailCommentMarker(c.GetBody())
			if !ok {
				continue
			}
			out = append(out, stageFailComment{ID: c.GetID(), OwnerRepo: ownerRepo})
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return out, nil
}

// syncStageFailReports 将本轮 pass/fail 同步到固定 Issue：失败 upsert 一仓一条评论，成功则删除。
// skip 不改动已有评论（例如 hash 未变未复检）。
func syncStageFailReports(ctx context.Context, client *github.Client, reports []stageReport) error {
	owner, repo, ok := bazaarOwnerRepo()
	if !ok {
		logger.Errorf("skip stage-fail comment sync: GITHUB_REPOSITORY not set / invalid")
		return nil
	}
	existing, err := listStageFailComments(ctx, client, owner, repo, stageFailIssue)
	if err != nil {
		return fmt.Errorf("list comments on %s/%s#%d: %w", owner, repo, stageFailIssue, err)
	}
	commentsByRepo := make(map[string][]stageFailComment, len(existing))
	for _, c := range existing {
		commentsByRepo[c.OwnerRepo] = append(commentsByRepo[c.OwnerRepo], c)
	}

	var passCount, failCount, skipped int
	for _, r := range reports {
		switch r.Kind {
		case stageReportSkip:
			skipped++
			continue
		case stageReportPass:
			passCount++
			for _, c := range commentsByRepo[r.OwnerRepo] {
				if _, err := client.Issues.DeleteComment(ctx, owner, repo, c.ID); err != nil {
					return fmt.Errorf("delete comment %d for [%s]: %w", c.ID, r.OwnerRepo, err)
				}
				logger.Infof("deleted stage-fail comment for [%s]", r.OwnerRepo)
			}
			delete(commentsByRepo, r.OwnerRepo)
		case stageReportFail:
			failCount++
			if len(r.Issues) == 0 {
				r.Issues = []rules.Issue{{
					MessageZh: "Stage 失败，但未收集到具体问题说明。请查看工作流日志或联系维护者。",
					MessageEn: "Stage failed, but no detailed issue was collected. Please check the workflow logs or contact a maintainer.",
				}}
			}
			body := formatStageFailComment(r)
			comments := commentsByRepo[r.OwnerRepo]
			if len(comments) == 0 {
				_, _, err := client.Issues.CreateComment(ctx, owner, repo, stageFailIssue, &github.IssueComment{Body: new(body)})
				if err != nil {
					return fmt.Errorf("create comment for [%s]: %w", r.OwnerRepo, err)
				}
				logger.Infof("created stage-fail comment for [%s]", r.OwnerRepo)
				continue
			}
			_, _, err := client.Issues.EditComment(ctx, owner, repo, comments[0].ID, &github.IssueComment{Body: new(body)})
			if err != nil {
				return fmt.Errorf("edit comment %d for [%s]: %w", comments[0].ID, r.OwnerRepo, err)
			}
			logger.Infof("updated stage-fail comment for [%s]", r.OwnerRepo)
			for _, c := range comments[1:] {
				if _, err := client.Issues.DeleteComment(ctx, owner, repo, c.ID); err != nil {
					return fmt.Errorf("delete duplicate comment %d for [%s]: %w", c.ID, r.OwnerRepo, err)
				}
			}
			delete(commentsByRepo, r.OwnerRepo)
		}
	}
	logger.Infof("stage-fail comment sync done: fail=%d pass=%d skip=%d issue=#%d", failCount, passCount, skipped, stageFailIssue)
	return nil
}

func stageIssueFromErr(err error) []rules.Issue {
	if err == nil {
		return nil
	}
	return []rules.Issue{rules.IssueFromErr(err)}
}

func stageInternalIssue(zh, en string) []rules.Issue {
	return []rules.Issue{{MessageZh: zh, MessageEn: en}}
}
