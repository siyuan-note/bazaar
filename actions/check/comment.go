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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/go-github/v89/github"
)

// 检查结果评论隐藏标记：当前条用 active；结果变化归档后改为 outdated（折叠为过时）。
// Legacy thollander 标记仍视为「当前条」，便于存量 PR 迁移。
const (
	checkResultCommentMarker         = `<!-- bazaar-check-result -->`
	checkResultCommentMarkerOutdated = `<!-- bazaar-check-result-outdated -->`
	checkResultCommentTagLegacy      = `<!-- thollander/actions-comment-pull-request "check-result" -->`
)

// isActiveCheckResultComment 是否为「当前」检查评论（未归档）。
func isActiveCheckResultComment(body string) bool {
	if body == "" || strings.Contains(body, checkResultCommentMarkerOutdated) {
		return false
	}
	return strings.Contains(body, checkResultCommentMarker) ||
		strings.Contains(body, checkResultCommentTagLegacy)
}

// markCheckResultCommentOutdated 将正文中的当前标记换成归档标记。
func markCheckResultCommentOutdated(body string) string {
	if strings.Contains(body, checkResultCommentMarkerOutdated) {
		return body
	}
	if strings.Contains(body, checkResultCommentMarker) {
		return strings.Replace(body, checkResultCommentMarker, checkResultCommentMarkerOutdated, 1)
	}
	if strings.Contains(body, checkResultCommentTagLegacy) {
		return strings.Replace(body, checkResultCommentTagLegacy, checkResultCommentMarkerOutdated, 1)
	}
	// 无已知标记时仍加上归档标记，避免被当成当前评
	return checkResultCommentMarkerOutdated + "\n" + body
}

// ensureCheckResultCommentMarker 保证正文以当前检查标记开头（模板已含则原样返回）。
func ensureCheckResultCommentMarker(body string) string {
	trimmed := strings.TrimSpace(body)
	if strings.HasPrefix(trimmed, checkResultCommentMarker) {
		return body
	}
	// 去掉可能残留的旧标记行后再加当前标记
	lines := strings.Split(body, "\n")
	out := make([]string, 0, len(lines)+1)
	out = append(out, checkResultCommentMarker)
	for _, line := range lines {
		if strings.Contains(line, checkResultCommentTagLegacy) ||
			strings.TrimSpace(line) == checkResultCommentMarker ||
			strings.TrimSpace(line) == checkResultCommentMarkerOutdated {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// publishCheckResultComment 将检查结果发到 PR：
// resultChanged=false 且已有当前评 → 原地编辑；否则先归档（改标记 + 折叠为过时）再新建。
func publishCheckResultComment(ctx context.Context, client *github.Client, owner, repo string, prNumber int, body string, resultChanged bool) error {
	if client == nil {
		return fmt.Errorf("nil github client")
	}
	body = ensureCheckResultCommentMarker(body)
	active, err := listActiveCheckResultComments(ctx, client, owner, repo, prNumber)
	if err != nil {
		return err
	}

	if !resultChanged && len(active) > 0 {
		latest := active[len(active)-1]
		_, _, err := client.Issues.EditComment(ctx, owner, repo, latest.GetID(), &github.IssueComment{Body: &body})
		if err != nil {
			return fmt.Errorf("edit check-result comment %d: %w", latest.GetID(), err)
		}
		// 若因历史原因存在多条「当前」评，多余的归档掉，只留刚编辑的那条
		for _, c := range active[:len(active)-1] {
			if archiveErr := archiveCheckResultComment(ctx, client, owner, repo, c); archiveErr != nil {
				logger.Errorf("archive extra check-result comment %d on PR #%d: %s", c.GetID(), prNumber, archiveErr)
			}
		}
		logger.Infof("updated check-result comment %d on PR #%d", latest.GetID(), prNumber)
		return nil
	}

	for _, c := range active {
		if archiveErr := archiveCheckResultComment(ctx, client, owner, repo, c); archiveErr != nil {
			logger.Errorf("archive check-result comment %d on PR #%d: %s", c.GetID(), prNumber, archiveErr)
		}
	}
	created, _, err := client.Issues.CreateComment(ctx, owner, repo, prNumber, &github.IssueComment{Body: &body})
	if err != nil {
		return fmt.Errorf("create check-result comment: %w", err)
	}
	logger.Infof("created check-result comment %d on PR #%d (archived %d)", created.GetID(), prNumber, len(active))
	return nil
}

func archiveCheckResultComment(ctx context.Context, client *github.Client, owner, repo string, c *github.IssueComment) error {
	if c == nil {
		return nil
	}
	outdated := markCheckResultCommentOutdated(c.GetBody())
	if _, _, err := client.Issues.EditComment(ctx, owner, repo, c.GetID(), &github.IssueComment{Body: &outdated}); err != nil {
		return fmt.Errorf("retag comment %d: %w", c.GetID(), err)
	}
	nodeID := c.GetNodeID()
	if nodeID == "" {
		return fmt.Errorf("comment %d missing node_id for minimize", c.GetID())
	}
	if err := minimizeCommentOutdated(ctx, client, nodeID); err != nil {
		return fmt.Errorf("minimize comment %d: %w", c.GetID(), err)
	}
	return nil
}

// listActiveCheckResultComments 按 created 升序返回当前检查评论。
func listActiveCheckResultComments(ctx context.Context, client *github.Client, owner, repo string, prNumber int) ([]*github.IssueComment, error) {
	all, err := listIssueComments(ctx, client, owner, repo, prNumber)
	if err != nil {
		return nil, err
	}
	var active []*github.IssueComment
	for _, c := range all {
		if isActiveCheckResultComment(c.GetBody()) {
			active = append(active, c)
		}
	}
	return active, nil
}

func listIssueComments(ctx context.Context, client *github.Client, owner, repo string, prNumber int) ([]*github.IssueComment, error) {
	opts := &github.IssueListCommentsOptions{ListOptions: github.ListOptions{PerPage: 100}}
	var all []*github.IssueComment
	for {
		comments, resp, err := client.Issues.ListComments(ctx, owner, repo, prNumber, opts)
		if err != nil {
			return nil, err
		}
		all = append(all, comments...)
		if resp == nil || resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return all, nil
}

// minimizeCommentOutdated 通过 GraphQL 将评论折叠为 OUTDATED。
func minimizeCommentOutdated(ctx context.Context, client *github.Client, nodeID string) error {
	payload := map[string]any{
		"query": `mutation($id: ID!) {
  minimizeComment(input: {subjectId: $id, classifier: OUTDATED}) {
    minimizedComment { isMinimized }
  }
}`,
		"variables": map[string]string{"id": nodeID},
	}
	req, err := client.NewRequest(ctx, "POST", "graphql", payload)
	if err != nil {
		return err
	}
	var raw json.RawMessage
	if _, err := client.Do(req, &raw); err != nil {
		return err
	}
	var resp struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return err
	}
	if len(resp.Errors) > 0 {
		var b bytes.Buffer
		for i, e := range resp.Errors {
			if i > 0 {
				b.WriteString("; ")
			}
			b.WriteString(e.Message)
		}
		return fmt.Errorf("graphql: %s", b.String())
	}
	return nil
}
