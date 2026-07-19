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
	"net/http"
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

// Stage 失败 Issue 标签，便于按仓检索与同步。
const stageFailLabel = "stage-fail"

// Issue 正文身份标记：<!-- bazaar-stage-fail {"repo":"owner/repo"} -->
const stageFailMarkerPrefix = "<!-- bazaar-stage-fail "

// stageFailMarker 为 HTML 注释中的单行 JSON，便于 upsert/close 时稳定解析。
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
	stageReportSkip stageReportKind = iota // hash 未变等，不改动 Issue
	stageReportPass                        // 本轮成功入库，关闭对应 Issue
	stageReportFail                        // 本轮失败，upsert Issue
)

// stageReport 单仓本轮 Stage 结果，供失败同步到按仓独立 Issue。
type stageReport struct {
	OwnerRepo   string
	PackageType rules.PackageType
	Kind        stageReportKind
	Release     util.LatestRelease
	Hash        string
	Issues      []rules.Issue
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

func stageFailIssueTitle(packageType rules.PackageType, ownerRepo string) string {
	typ := packageType.String()
	if typ != "" {
		typ = strings.ToUpper(typ[:1]) + typ[1:]
	}
	return typ + " update failed: " + ownerRepo
}

func stageFailBodyMarker(ownerRepo string) string {
	payload, err := json.Marshal(stageFailMarker{Repo: ownerRepo})
	if err != nil {
		// Marshal 基本类型失败不应发生；退回空 repo 便于调用方察觉
		payload = []byte(`{"repo":""}`)
	}
	return stageFailMarkerPrefix + string(payload) + " -->"
}

func parseStageFailBodyMarker(body string) (ownerRepo string, ok bool) {
	_, after, cutOK := strings.Cut(body, stageFailMarkerPrefix)
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

// stageFailIssueView 为 stage-fail.md.tpl 的渲染数据。
type stageFailIssueView struct {
	Marker         string
	OwnerRepo      string
	PackageType    string
	Release        util.LatestRelease
	Hash           string
	Issues         []rules.Issue
	WorkflowRunURL string
}

// formatStageFailIssueBody 生成单仓失败 Issue 正文（含 marker，便于 upsert/close）。
func formatStageFailIssueBody(r stageReport) (string, error) {
	var buf bytes.Buffer
	err := stageFailTemplate.Execute(&buf, stageFailIssueView{
		Marker:         stageFailBodyMarker(r.OwnerRepo),
		OwnerRepo:      r.OwnerRepo,
		PackageType:    r.PackageType.String(),
		Release:        r.Release,
		Hash:           r.Hash,
		Issues:         r.Issues,
		WorkflowRunURL: workflowRunURL(),
	})
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

type stageFailIssue struct {
	Number    int
	OwnerRepo string
	Title     string
	Body      string
}

// 正文末尾工作流链接每次 run 都会变，比对正文时忽略，避免无意义 Edit。
const stageFailWorkflowFooterPrefix = "\n\n工作流 / Workflow: "

func stageFailIssueComparableBody(body string) string {
	body = strings.TrimRight(body, "\n")
	before, _, found := strings.Cut(body, stageFailWorkflowFooterPrefix)
	if found {
		return before
	}
	return body
}

func stageFailIssueContentEqual(a, b string) bool {
	return stageFailIssueComparableBody(a) == stageFailIssueComparableBody(b)
}

func ensureStageFailLabel(ctx context.Context, client *github.Client, owner, repo string) error {
	_, resp, err := client.Issues.GetLabel(ctx, owner, repo, stageFailLabel)
	if err == nil {
		return nil
	}
	if resp == nil || resp.StatusCode != http.StatusNotFound {
		return err
	}
	name := stageFailLabel
	color := "D73A4A"
	desc := "Stage indexing failed for a bazaar package repo"
	_, _, err = client.Issues.CreateLabel(ctx, owner, repo, &github.Label{
		Name:        &name,
		Color:       &color,
		Description: &desc,
	})
	if err != nil {
		// 并发创建时可能已存在
		if _, _, getErr := client.Issues.GetLabel(ctx, owner, repo, stageFailLabel); getErr == nil {
			return nil
		}
		return err
	}
	logger.Infof("created repo label %q", stageFailLabel)
	return nil
}

func listOpenStageFailIssues(ctx context.Context, client *github.Client, owner, repo string) ([]stageFailIssue, error) {
	var out []stageFailIssue
	opts := &github.IssueListByRepoOptions{
		State:  "open",
		Labels: []string{stageFailLabel},
		ListOptions: github.ListOptions{PerPage: 100},
	}
	for {
		issues, resp, err := client.Issues.ListByRepo(ctx, owner, repo, opts)
		if err != nil {
			return nil, err
		}
		for _, issue := range issues {
			if issue.IsPullRequest() {
				continue
			}
			body := issue.GetBody()
			ownerRepo, ok := parseStageFailBodyMarker(body)
			if !ok {
				continue
			}
			out = append(out, stageFailIssue{
				Number:    issue.GetNumber(),
				OwnerRepo: ownerRepo,
				Title:     issue.GetTitle(),
				Body:      body,
			})
		}
		if resp.NextPage == 0 {
			break
		}
		opts.ListOptions.Page = resp.NextPage
	}
	return out, nil
}

func closeStageFailIssue(ctx context.Context, client *github.Client, owner, repo string, number int) error {
	state := "closed"
	_, _, err := client.Issues.Edit(ctx, owner, repo, number, &github.IssueRequest{State: &state})
	return err
}

// syncStageFailReports 将本轮 pass/fail 同步为按仓独立 Issue：失败 upsert（正文未变则跳过 Edit），成功则关闭；skip 不改动。
func syncStageFailReports(ctx context.Context, client *github.Client, reports []stageReport) error {
	owner, repo, ok := bazaarOwnerRepo()
	if !ok {
		logger.Errorf("skip stage-fail issue sync: GITHUB_REPOSITORY not set / invalid")
		return nil
	}
	if err := ensureStageFailLabel(ctx, client, owner, repo); err != nil {
		return fmt.Errorf("ensure label %q: %w", stageFailLabel, err)
	}
	existing, err := listOpenStageFailIssues(ctx, client, owner, repo)
	if err != nil {
		return fmt.Errorf("list open %s issues on %s/%s: %w", stageFailLabel, owner, repo, err)
	}
	issuesByRepo := make(map[string][]stageFailIssue, len(existing))
	for _, issue := range existing {
		issuesByRepo[issue.OwnerRepo] = append(issuesByRepo[issue.OwnerRepo], issue)
	}

	var passCount, failCount, skipped, unchanged int
	for _, r := range reports {
		switch r.Kind {
		case stageReportSkip:
			skipped++
			continue
		case stageReportPass:
			passCount++
			for _, issue := range issuesByRepo[r.OwnerRepo] {
				if err := closeStageFailIssue(ctx, client, owner, repo, issue.Number); err != nil {
					if util.IsGitHubRateLimit(err) {
						return fmt.Errorf("close issue #%d for [%s]: GitHub API rate limited, stop syncing remaining issues: %w", issue.Number, r.OwnerRepo, err)
					}
					return fmt.Errorf("close issue #%d for [%s]: %w", issue.Number, r.OwnerRepo, err)
				}
				logger.Infof("closed stage-fail issue #%d for [%s]", issue.Number, r.OwnerRepo)
			}
			delete(issuesByRepo, r.OwnerRepo)
		case stageReportFail:
			failCount++
			if len(r.Issues) == 0 {
				r.Issues = []rules.Issue{{
					MessageZh: "Stage 失败，但未收集到具体问题说明。请查看工作流日志或联系维护者。",
					MessageEn: "Stage failed, but no detailed issue was collected. Please check the workflow logs or contact a maintainer.",
				}}
			}
			body, err := formatStageFailIssueBody(r)
			if err != nil {
				return fmt.Errorf("format issue body for [%s]: %w", r.OwnerRepo, err)
			}
			title := stageFailIssueTitle(r.PackageType, r.OwnerRepo)
			issues := issuesByRepo[r.OwnerRepo]
			if len(issues) == 0 {
				labels := []string{stageFailLabel}
				created, _, err := client.Issues.Create(ctx, owner, repo, &github.IssueRequest{
					Title:  &title,
					Body:   &body,
					Labels: &labels,
				})
				if err != nil {
					if util.IsGitHubRateLimit(err) {
						return fmt.Errorf("create issue for [%s]: GitHub API rate limited, stop syncing remaining issues: %w", r.OwnerRepo, err)
					}
					return fmt.Errorf("create issue for [%s]: %w", r.OwnerRepo, err)
				}
				logger.Infof("created stage-fail issue #%d for [%s]", created.GetNumber(), r.OwnerRepo)
				continue
			}
			primary := issues[0]
			needEdit := !stageFailIssueContentEqual(primary.Body, body) || primary.Title != title
			if !needEdit {
				unchanged++
				logger.Infof("stage-fail issue #%d unchanged for [%s], skip edit", primary.Number, r.OwnerRepo)
			} else {
				_, _, err = client.Issues.Edit(ctx, owner, repo, primary.Number, &github.IssueRequest{
					Title: &title,
					Body:  &body,
				})
				if err != nil {
					if util.IsGitHubRateLimit(err) {
						return fmt.Errorf("edit issue #%d for [%s]: GitHub API rate limited, stop syncing remaining issues: %w", primary.Number, r.OwnerRepo, err)
					}
					return fmt.Errorf("edit issue #%d for [%s]: %w", primary.Number, r.OwnerRepo, err)
				}
				logger.Infof("updated stage-fail issue #%d for [%s]", primary.Number, r.OwnerRepo)
			}
			for _, issue := range issues[1:] {
				if err := closeStageFailIssue(ctx, client, owner, repo, issue.Number); err != nil {
					if util.IsGitHubRateLimit(err) {
						return fmt.Errorf("close duplicate issue #%d for [%s]: GitHub API rate limited, stop syncing remaining issues: %w", issue.Number, r.OwnerRepo, err)
					}
					return fmt.Errorf("close duplicate issue #%d for [%s]: %w", issue.Number, r.OwnerRepo, err)
				}
				logger.Infof("closed duplicate stage-fail issue #%d for [%s]", issue.Number, r.OwnerRepo)
			}
			delete(issuesByRepo, r.OwnerRepo)
		}
	}
	logger.Infof("stage-fail issue sync done: fail=%d pass=%d skip=%d unchanged=%d", failCount, passCount, skipped, unchanged)
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
