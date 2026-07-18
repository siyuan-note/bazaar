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
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"text/template"

	"github.com/google/go-github/v89/github"
	"github.com/siyuan-note/bazaar/actions/util"
	"github.com/siyuan-note/bazaar/rules"
)

//go:embed stage-fail.md.tpl
var stageFailTemplateText string

var stageFailTemplate = template.Must(parseStageFailTemplate())

// 固定汇总 Issue：https://github.com/siyuan-note/bazaar/issues/1923
const stageFailIssue = 1923

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

func parseStageFailTemplate() (*template.Template, error) {
	return template.New("stage-fail.md.tpl").Funcs(template.FuncMap{
		"issueIndex": formatStageIssueIndex,
		"repoURL":    util.GitHubRepoURL,
	}).Parse(stageFailTemplateText)
}

// stageFailCommentView 为 stage-fail.md.tpl 的渲染数据。
type stageFailCommentView struct {
	Marker         string
	OwnerRepo      string
	PackageType    string
	Release        util.LatestRelease
	Hash           string
	KeptOld        bool
	Issues         []rules.Issue
	WorkflowRunURL string
}

// formatStageFailComment 生成固定 Issue 下单仓失败评论正文（含 marker，便于 upsert/delete）。
func formatStageFailComment(r stageReport) (string, error) {
	var buf bytes.Buffer
	err := stageFailTemplate.Execute(&buf, stageFailCommentView{
		Marker:         stageFailCommentMarker(r.OwnerRepo),
		OwnerRepo:      r.OwnerRepo,
		PackageType:    r.PackageType.String(),
		Release:        r.Release,
		Hash:           r.Hash,
		KeptOld:        r.KeptOld,
		Issues:         r.Issues,
		WorkflowRunURL: workflowRunURL(),
	})
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

type stageFailComment struct {
	ID        int64
	OwnerRepo string
	Body      string
}

// 评论末尾工作流链接每次 run 都会变，比对正文时忽略，避免无意义 Edit。
const stageFailWorkflowFooterPrefix = "\n\n工作流 / Workflow: "

func stageFailCommentComparableBody(body string) string {
	body = strings.TrimRight(body, "\n")
	before, _, found := strings.Cut(body, stageFailWorkflowFooterPrefix)
	if found {
		return before
	}
	return body
}

func stageFailCommentContentEqual(a, b string) bool {
	return stageFailCommentComparableBody(a) == stageFailCommentComparableBody(b)
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
			body := c.GetBody()
			ownerRepo, ok := parseStageFailCommentMarker(body)
			if !ok {
				continue
			}
			out = append(out, stageFailComment{ID: c.GetID(), OwnerRepo: ownerRepo, Body: body})
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return out, nil
}

// syncStageFailReports 将本轮 pass/fail 同步到固定 Issue：失败 upsert 一仓一条评论，成功则删除。
// skip 不改动已有评论（例如 hash 未变未复检）；失败且正文（忽略工作流链接）未变则跳过 Edit。
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

	var passCount, failCount, skipped, unchanged int
	for _, r := range reports {
		switch r.Kind {
		case stageReportSkip:
			skipped++
			continue
		case stageReportPass:
			passCount++
			for _, c := range commentsByRepo[r.OwnerRepo] {
				if _, err := client.Issues.DeleteComment(ctx, owner, repo, c.ID); err != nil {
					if util.IsGitHubRateLimit(err) {
						return fmt.Errorf("delete comment %d for [%s]: GitHub API rate limited, stop syncing remaining comments: %w", c.ID, r.OwnerRepo, err)
					}
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
			body, err := formatStageFailComment(r)
			if err != nil {
				return fmt.Errorf("format comment for [%s]: %w", r.OwnerRepo, err)
			}
			comments := commentsByRepo[r.OwnerRepo]
			if len(comments) == 0 {
				_, _, err := client.Issues.CreateComment(ctx, owner, repo, stageFailIssue, &github.IssueComment{Body: new(body)})
				if err != nil {
					if util.IsGitHubRateLimit(err) {
						return fmt.Errorf("create comment for [%s]: GitHub API rate limited, stop syncing remaining comments: %w", r.OwnerRepo, err)
					}
					return fmt.Errorf("create comment for [%s]: %w", r.OwnerRepo, err)
				}
				logger.Infof("created stage-fail comment for [%s]", r.OwnerRepo)
				continue
			}
			if stageFailCommentContentEqual(comments[0].Body, body) {
				unchanged++
				logger.Infof("stage-fail comment unchanged for [%s], skip edit", r.OwnerRepo)
			} else {
				_, _, err = client.Issues.EditComment(ctx, owner, repo, comments[0].ID, &github.IssueComment{Body: new(body)})
				if err != nil {
					if util.IsGitHubRateLimit(err) {
						return fmt.Errorf("edit comment %d for [%s]: GitHub API rate limited, stop syncing remaining comments: %w", comments[0].ID, r.OwnerRepo, err)
					}
					return fmt.Errorf("edit comment %d for [%s]: %w", comments[0].ID, r.OwnerRepo, err)
				}
				logger.Infof("updated stage-fail comment for [%s]", r.OwnerRepo)
			}
			for _, c := range comments[1:] {
				if _, err := client.Issues.DeleteComment(ctx, owner, repo, c.ID); err != nil {
					if util.IsGitHubRateLimit(err) {
						return fmt.Errorf("delete duplicate comment %d for [%s]: GitHub API rate limited, stop syncing remaining comments: %w", c.ID, r.OwnerRepo, err)
					}
					return fmt.Errorf("delete duplicate comment %d for [%s]: %w", c.ID, r.OwnerRepo, err)
				}
			}
			delete(commentsByRepo, r.OwnerRepo)
		}
	}
	logger.Infof("stage-fail comment sync done: fail=%d pass=%d skip=%d unchanged=%d issue=#%d", failCount, passCount, skipped, unchanged, stageFailIssue)
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
