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
	"text/template"
	"time"

	"github.com/google/go-github/v89/github"
)

const (
	staleCIFailedAge = 30 * 24 * time.Hour

	labelManualReview = "manual-review"

	staleCloseReasonCIFailed     = "ci-failed"
	staleCloseReasonManualReview = "manual-review"
)

//go:embed close-stale.md.tpl
var closeStaleTemplateText string

// closeStaleCommentData 关闭超龄 PR 时的评论模板数据。
type closeStaleCommentData struct {
	Days   int
	Reason string // ci-failed | manual-review
}

// staleCloseReason 返回超龄自动关闭原因；空字符串表示不应关闭。
// 条件：非草稿、无 ci-skip、created_at 距今 ≥ age，且满足其一：
//   - 有 ci-failed
//   - 同时有 ci-passed 与 manual-review（维护者已要求修改，作者未跟进）
func staleCloseReason(pr *github.PullRequest, now time.Time, age time.Duration) string {
	if pr == nil || age <= 0 {
		return ""
	}
	if pr.GetDraft() {
		return ""
	}
	if prHasLabel(pr, "ci-skip") {
		return ""
	}
	created := pr.GetCreatedAt().Time
	if created.IsZero() {
		return ""
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	if now.Before(created.UTC().Add(age)) {
		return ""
	}
	if prHasLabel(pr, labelCIFailed) {
		return staleCloseReasonCIFailed
	}
	if prHasLabel(pr, labelCIPassed) && prHasLabel(pr, labelManualReview) {
		return staleCloseReasonManualReview
	}
	return ""
}

// closeStalePRs 关闭超龄 PR（先评论再关），返回仍应参与后续处理的 PR。
// 单 PR 失败只记日志，不中断 select。
func closeStalePRs(ctx context.Context, client *github.Client, owner, repo string, prs []*github.PullRequest, now time.Time, age time.Duration) []*github.PullRequest {
	if len(prs) == 0 {
		return prs
	}
	tmpl, err := template.New("close-stale.md.tpl").Parse(closeStaleTemplateText)
	if err != nil {
		logger.Errorf("parse close-stale template failed: %s", err)
		return prs
	}
	days := int(age / (24 * time.Hour))
	if days < 1 {
		days = 1
	}

	kept := make([]*github.PullRequest, 0, len(prs))
	closed := 0
	for _, pr := range prs {
		reason := staleCloseReason(pr, now, age)
		if reason == "" {
			kept = append(kept, pr)
			continue
		}
		n := pr.GetNumber()
		if err := closeOneStalePR(ctx, client, owner, repo, pr, tmpl, days, reason); err != nil {
			logger.Errorf("close stale PR #%d (%s) failed: %s", n, reason, err)
			kept = append(kept, pr) // 关闭失败则仍参与后续 select
			continue
		}
		logger.Infof("closed stale PR #%d reason=%s (created_at=%s)", n, reason, pr.GetCreatedAt().UTC().Format(time.RFC3339))
		closed++
	}
	if closed > 0 {
		logger.Infof("closed %d stale PR(s); remaining for select: %d", closed, len(kept))
	}
	return kept
}

func closeOneStalePR(ctx context.Context, client *github.Client, owner, repo string, pr *github.PullRequest, tmpl *template.Template, days int, reason string) error {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, closeStaleCommentData{Days: days, Reason: reason}); err != nil {
		return err
	}
	body := buf.String()
	n := pr.GetNumber()
	if _, _, err := client.Issues.CreateComment(ctx, owner, repo, n, &github.IssueComment{Body: &body}); err != nil {
		return err
	}
	state := "closed"
	if _, _, err := client.Issues.Edit(ctx, owner, repo, n, &github.IssueRequest{State: &state}); err != nil {
		return err
	}
	return nil
}
