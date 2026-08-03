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

const staleCIFailedAge = 30 * 24 * time.Hour

//go:embed close-stale.md.tpl
var closeStaleTemplateText string

// closeStaleCommentData 关闭超龄 ci-failed PR 时的评论模板数据。
type closeStaleCommentData struct {
	Days int
}

// shouldCloseStaleCIFailed 判断是否应因超龄 ci-failed 自动关闭。
// 条件：非草稿、有 ci-failed、无 ci-skip、created_at 距今 ≥ age。
func shouldCloseStaleCIFailed(pr *github.PullRequest, now time.Time, age time.Duration) bool {
	if pr == nil || age <= 0 {
		return false
	}
	if pr.GetDraft() {
		return false
	}
	if prHasLabel(pr, "ci-skip") {
		return false
	}
	if !prHasLabel(pr, labelCIFailed) {
		return false
	}
	created := pr.GetCreatedAt().Time
	if created.IsZero() {
		return false
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	return !now.Before(created.UTC().Add(age))
}

// closeStaleCIFailedPRs 关闭超龄 ci-failed PR（先评论再关），返回仍应参与 select 的 PR。
// 单 PR 失败只记日志，不中断 select。
func closeStaleCIFailedPRs(ctx context.Context, client *github.Client, owner, repo string, prs []*github.PullRequest, now time.Time, age time.Duration) []*github.PullRequest {
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
		if !shouldCloseStaleCIFailed(pr, now, age) {
			kept = append(kept, pr)
			continue
		}
		n := pr.GetNumber()
		if err := closeOneStaleCIFailedPR(ctx, client, owner, repo, pr, tmpl, days); err != nil {
			logger.Errorf("close stale ci-failed PR #%d failed: %s", n, err)
			kept = append(kept, pr) // 关闭失败则仍参与后续 select
			continue
		}
		logger.Infof("closed stale ci-failed PR #%d (created_at=%s)", n, pr.GetCreatedAt().UTC().Format(time.RFC3339))
		closed++
	}
	if closed > 0 {
		logger.Infof("closed %d stale ci-failed PR(s); remaining for select: %d", closed, len(kept))
	}
	return kept
}

func closeOneStaleCIFailedPR(ctx context.Context, client *github.Client, owner, repo string, pr *github.PullRequest, tmpl *template.Template, days int) error {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, closeStaleCommentData{Days: days}); err != nil {
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
