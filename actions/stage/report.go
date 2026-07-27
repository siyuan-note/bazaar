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
	"slices"
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
	stageReportSkip stageReportKind = iota // hash 未变；若有 open stage-fail 则关闭
	stageReportPass                        // 本轮成功入库，关闭对应 Issue
	stageReportFail                        // 本轮失败，upsert Issue
)

// stageFailCloseReason 关闭 stage-fail Issue 的原因，用于评论说明。
type stageFailCloseReason int

const (
	stageFailClosePass stageFailCloseReason = iota // 本轮成功重新入库
	stageFailCloseSkip                             // 本轮校验通过且 hash 未变
	stageFailCloseDuplicate                        // 同仓重复 Issue
	stageFailCloseDelisted                         // 已不在任一 *s.txt（下架或换维护者旧仓）
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

// stageFailRepoOwner 取 owner/repo 左侧段；个人账号与组织账号同为 GitHub owner，可直接 @。
func stageFailRepoOwner(ownerRepo string) string {
	owner, _, ok := strings.Cut(ownerRepo, "/")
	if !ok {
		return ""
	}
	return owner
}

func parseStageFailTemplate() (*template.Template, error) {
	return template.New("stage-fail.md.tpl").Funcs(template.FuncMap{
		"issueIndex": formatStageIssueIndex,
		"repoURL":    util.GitHubRepoURL,
		"repoOwner":  stageFailRepoOwner,
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
const stageFailWorkflowFooterPrefix = "\n\n工作流日志 / Workflow log: "

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
		State:       "open",
		Labels:      []string{stageFailLabel},
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

// stageFailCloseComment 生成关闭 Issue 前的说明评论（中英双语）。
func stageFailCloseComment(reason stageFailCloseReason) string {
	var zh, en string
	switch reason {
	case stageFailClosePass:
		zh = "本轮 Stage 已成功重新索引该包，问题已修复，因此关闭本 Issue。"
		en = "This package was successfully re-indexed in this Stage run, so this issue is being closed as fixed."
	case stageFailCloseSkip:
		zh = "本轮 Stage 已能正常获取该包的 Latest Release，且入库内容未变化（hash 未变），视为已恢复，因此关闭本 Issue。"
		en = "This Stage run could fetch the package's Latest Release successfully, and the staged content is unchanged (hash unchanged), so this issue is being closed as recovered."
	case stageFailCloseDuplicate:
		zh = "关闭重复的 stage-fail Issue；请以同仓库的主 Issue 为准。"
		en = "Closing a duplicate stage-fail issue; please follow the primary issue for this repository."
	case stageFailCloseDelisted:
		zh = "该仓库已不在集市包列表中（已下架或更换维护者），因此关闭本 Issue。"
		en = "This repository is no longer in the bazaar package lists (delisted or maintainer changed), so this issue is being closed."
	default:
		zh = "本轮 Stage 已确认该包无需继续跟踪，因此关闭本 Issue。"
		en = "This Stage run confirmed the package no longer needs tracking, so this issue is being closed."
	}
	body := zh + "\n\n" + en
	if runURL := workflowRunURL(); runURL != "" {
		body += "\n\n工作流日志 / Workflow log: " + runURL
	}
	return body
}

// closeStageFailIssue 先评论关闭原因，再将 Issue 设为 closed。
func closeStageFailIssue(ctx context.Context, client *github.Client, owner, repo string, number int, reason stageFailCloseReason) error {
	comment := stageFailCloseComment(reason)
	_, _, err := client.Issues.CreateComment(ctx, owner, repo, number, &github.IssueComment{Body: &comment})
	if err != nil {
		return fmt.Errorf("comment before close: %w", err)
	}
	state := "closed"
	_, _, err = client.Issues.Edit(ctx, owner, repo, number, &github.IssueRequest{State: &state})
	if err != nil {
		return fmt.Errorf("edit state closed: %w", err)
	}
	return nil
}

// ownerRepoListedSet 将各类型当前 *s.txt 路径收成集合，供判断 stage-fail 是否对应已下架仓。
func ownerRepoListedSet(reposByType map[rules.PackageType][]string) Set {
	listed := make(Set)
	for _, repos := range reposByType {
		for _, ownerRepo := range repos {
			listed[ownerRepo] = struct{}{}
		}
	}
	return listed
}

// stageFailDelistedRepos 返回仍有 open Issue、但已不在当前包列表中的 owner/repo（稳定排序便于测试）。
func stageFailDelistedRepos(issuesByRepo map[string][]stageFailIssue, listed Set) []string {
	var out []string
	for ownerRepo := range issuesByRepo {
		if _, ok := listed[ownerRepo]; ok {
			continue
		}
		out = append(out, ownerRepo)
	}
	slices.Sort(out)
	return out
}

// syncStageFailReports 将本轮 pass/fail/skip 同步为按仓独立 Issue：失败 upsert（正文未变则跳过 Edit）；
// 成功入库或 hash 跳过（校验已通过）则先评论说明再关闭；
// 仍有 open Issue 但已不在任一 *s.txt 的仓（下架 / 换维护者旧仓）按 delisted 关闭。
// listed 须为当前完整包列表：不得用「本轮未出现在 reports」判断下架（增量跳过类型、限流中止时会误关）。
func syncStageFailReports(ctx context.Context, client *github.Client, reports []stageReport, listed Set) error {
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

	closeRepoIssues := func(ownerRepo string, reason stageFailCloseReason) error {
		for _, issue := range issuesByRepo[ownerRepo] {
			if err := closeStageFailIssue(ctx, client, owner, repo, issue.Number, reason); err != nil {
				if util.IsGitHubRateLimit(err) {
					return fmt.Errorf("close issue #%d for [%s]: GitHub API rate limited, stop syncing remaining issues: %w", issue.Number, ownerRepo, err)
				}
				return fmt.Errorf("close issue #%d for [%s]: %w", issue.Number, ownerRepo, err)
			}
			logger.Infof("closed stage-fail issue #%d for [%s]", issue.Number, ownerRepo)
		}
		delete(issuesByRepo, ownerRepo)
		return nil
	}

	var passCount, failCount, skipped, unchanged, delisted int
	for _, r := range reports {
		switch r.Kind {
		case stageReportSkip:
			skipped++
			if err := closeRepoIssues(r.OwnerRepo, stageFailCloseSkip); err != nil {
				return err
			}
		case stageReportPass:
			passCount++
			if err := closeRepoIssues(r.OwnerRepo, stageFailClosePass); err != nil {
				return err
			}
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
				if err := closeStageFailIssue(ctx, client, owner, repo, issue.Number, stageFailCloseDuplicate); err != nil {
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
	for _, ownerRepo := range stageFailDelistedRepos(issuesByRepo, listed) {
		if err := closeRepoIssues(ownerRepo, stageFailCloseDelisted); err != nil {
			return err
		}
		delisted++
	}
	logger.Infof("stage-fail issue sync done: fail=%d pass=%d skip=%d unchanged=%d delisted=%d", failCount, passCount, skipped, unchanged, delisted)
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
