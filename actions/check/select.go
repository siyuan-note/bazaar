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
	"os"
	"os/signal"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/google/go-github/v89/github"
	"github.com/siyuan-note/bazaar/actions/util"
)

const (
	selectPRLimitDefault  = 100
	checkResultCommentTag = `thollander/actions-comment-pull-request "check-result"`
)

// selectMatrixEntry 写入 GitHub Actions matrix 的单条 PR。
type selectMatrixEntry struct {
	Number   int    `json:"number"`
	HeadSHA  string `json:"head_sha"`
	BaseSHA  string `json:"base_sha"`
	HeadRepo string `json:"head_repo"`
}

type selectCandidate struct {
	entry     selectMatrixEntry
	reason    string
	fpChanged bool
	streak    int
	checkedAt time.Time
}

// runSelect 定时 / 手动复检：筛选待完整检查的 ci-failed PR，写入 GITHUB_OUTPUT matrix。
func runSelect() {
	logger.Infof("PR Check select started")

	var stop context.CancelFunc
	githubContext, stop = signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	repoToken := GITHUB_TOKEN
	if repoToken == "" {
		repoToken = PAT
	}
	var err error
	githubRepoClient, err = util.NewGitHubClient(repoToken, REQUEST_TIMEOUT)
	if err != nil {
		logger.Fatalf("create github repo client failed: %s", err)
	}
	pat := PAT
	if pat == "" {
		pat = repoToken
	}
	githubClient, err = util.NewGitHubClient(pat, REQUEST_TIMEOUT)
	if err != nil {
		logger.Fatalf("create github client failed: %s", err)
	}

	owner, repo, ok := splitOwnerRepo(GITHUB_REPOSITORY)
	if !ok {
		logger.Fatalf("invalid GITHUB_REPOSITORY %q", GITHUB_REPOSITORY)
	}

	forceAll := envTruthy("SELECT_FORCE_ALL")
	limit := envIntDefault("SELECT_LIMIT", selectPRLimitDefault)
	now := time.Now().UTC()

	prs, err := listOpenCIFailedPRs(githubContext, githubRepoClient, owner, repo)
	if err != nil {
		logger.Fatalf("list ci-failed PRs failed: %s", err)
	}
	logger.Infof("open ci-failed PRs: %d (force=%v limit=%d)", len(prs), forceAll, limit)

	candidates := make([]selectCandidate, 0, len(prs))
	for _, pr := range prs {
		c, include := evaluateSelectPR(githubContext, owner, repo, pr, now, forceAll)
		if !include {
			logger.Infof("skip PR #%d (%s)", pr.GetNumber(), c.reason)
			continue
		}
		logger.Infof("include PR #%d (%s)", pr.GetNumber(), c.reason)
		candidates = append(candidates, c)
	}

	slices.SortFunc(candidates, cmpSelectCandidate)
	if len(candidates) > limit {
		logger.Infof("truncate selected PRs %d -> %d", len(candidates), limit)
		candidates = candidates[:limit]
	}

	include := make([]selectMatrixEntry, 0, len(candidates))
	for _, c := range candidates {
		include = append(include, c.entry)
	}
	writeSelectMatrixOutput(include)
	logger.Infof("PR Check select completed: %d PRs", len(include))
}

func envTruthy(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func envIntDefault(key string, defaultVal int) int {
	if s := os.Getenv(key); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return n
		}
	}
	return defaultVal
}

func splitOwnerRepo(s string) (owner, repo string, ok bool) {
	owner, repo, ok = strings.Cut(s, "/")
	if !ok || owner == "" || repo == "" {
		return "", "", false
	}
	return owner, repo, true
}

func listOpenCIFailedPRs(ctx context.Context, client *github.Client, owner, repo string) ([]*github.PullRequest, error) {
	var out []*github.PullRequest
	opts := &github.PullRequestListOptions{
		State:       "open",
		Sort:        "updated",
		Direction:   "desc",
		ListOptions: github.ListOptions{PerPage: 100},
	}
	for {
		prs, resp, err := client.PullRequests.List(ctx, owner, repo, opts)
		if err != nil {
			return nil, err
		}
		for _, pr := range prs {
			if pr.GetDraft() {
				continue
			}
			if prHasLabel(pr, "ci-skip") {
				continue
			}
			if !prHasLabel(pr, labelCIFailed) {
				continue
			}
			out = append(out, pr)
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return out, nil
}

func prHasLabel(pr *github.PullRequest, name string) bool {
	for _, l := range pr.Labels {
		if l.GetName() == name {
			return true
		}
	}
	return false
}

func evaluateSelectPR(ctx context.Context, owner, repo string, pr *github.PullRequest, now time.Time, force bool) (selectCandidate, bool) {
	entry, ok := matrixEntryFromPR(pr)
	c := selectCandidate{entry: entry, reason: "invalid-pr"}
	if !ok {
		return c, false
	}

	meta, _ := loadCheckMetaFromPRComments(ctx, githubRepoClient, owner, repo, pr.GetNumber())
	if meta != nil {
		c.streak = meta.UnchangedStreak
		if t, err := time.Parse(time.RFC3339, meta.CheckedAt); err == nil {
			c.checkedAt = t
		}
	}

	var currentFP *CheckFingerprint
	var probeErr error
	if meta != nil && meta.FP != nil && meta.FP.Repo != "" {
		fpOwner, fpRepo, cutOK := strings.Cut(meta.FP.Repo, "/")
		if cutOK {
			rel, err := util.ProbeLatestRelease(ctx, githubClient, fpOwner, fpRepo)
			if err != nil {
				probeErr = err
				logger.Warnf("probe release [%s] for PR #%d failed: %s", meta.FP.Repo, pr.GetNumber(), err)
			} else {
				currentFP = fingerprintFromRelease(meta.FP.Repo, rel)
				c.fpChanged = !fingerprintsEqual(currentFP, meta.FP)
			}
		}
	}

	reason, include := shouldScheduleRecheck(meta, currentFP, probeErr, now, force)
	c.reason = reason
	return c, include
}

func matrixEntryFromPR(pr *github.PullRequest) (selectMatrixEntry, bool) {
	if pr == nil {
		return selectMatrixEntry{}, false
	}
	head := pr.GetHead()
	base := pr.GetBase()
	if head == nil || base == nil {
		return selectMatrixEntry{}, false
	}
	headRepo := ""
	if r := head.GetRepo(); r != nil {
		headRepo = r.GetFullName()
	}
	if headRepo == "" || head.GetSHA() == "" || base.GetSHA() == "" {
		return selectMatrixEntry{}, false
	}
	return selectMatrixEntry{
		Number:   pr.GetNumber(),
		HeadSHA:  head.GetSHA(),
		BaseSHA:  base.GetSHA(),
		HeadRepo: headRepo,
	}, true
}

// loadCheckMetaFromPRComments 从 PR 评论中读取最新的 bazaar-check-meta（优先含 meta 的评论）。
func loadCheckMetaFromPRComments(ctx context.Context, client *github.Client, owner, repo string, prNumber int) (*CheckMeta, bool) {
	if client == nil {
		return nil, false
	}
	opts := &github.IssueListCommentsOptions{ListOptions: github.ListOptions{PerPage: 100}}
	var latest *CheckMeta
	var fallbackBody string
	for {
		comments, resp, err := client.Issues.ListComments(ctx, owner, repo, prNumber, opts)
		if err != nil {
			logger.Warnf("list comments for PR #%d failed: %s", prNumber, err)
			return nil, false
		}
		// ListComments 默认按 created 升序：同页内后者更新，跨页继续覆盖
		for _, c := range comments {
			body := c.GetBody()
			if meta, ok := parseCheckMetaFromComment(body); ok {
				latest = meta
				continue
			}
			if strings.Contains(body, checkResultCommentTag) {
				fallbackBody = body
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	if latest != nil {
		return latest, true
	}
	if fallbackBody != "" {
		return parseCheckMetaFromComment(fallbackBody)
	}
	return nil, false
}

func cmpSelectCandidate(a, b selectCandidate) int {
	// fp 已变优先
	if a.fpChanged != b.fpChanged {
		if a.fpChanged {
			return -1
		}
		return 1
	}
	// streak 小优先
	if a.streak != b.streak {
		if a.streak < b.streak {
			return -1
		}
		return 1
	}
	// 最久未检优先（零值排后面）
	aZero := a.checkedAt.IsZero()
	bZero := b.checkedAt.IsZero()
	if aZero != bZero {
		if aZero {
			return 1
		}
		return -1
	}
	if a.checkedAt.Before(b.checkedAt) {
		return -1
	}
	if a.checkedAt.After(b.checkedAt) {
		return 1
	}
	if a.entry.Number < b.entry.Number {
		return -1
	}
	if a.entry.Number > b.entry.Number {
		return 1
	}
	return 0
}

func writeSelectMatrixOutput(include []selectMatrixEntry) {
	if include == nil {
		include = []selectMatrixEntry{}
	}
	includeJSON, err := json.Marshal(include)
	if err != nil {
		logger.Fatalf("marshal matrix include failed: %s", err)
	}
	matrixObj := map[string]any{"include": include}
	matrixJSON, err := json.Marshal(matrixObj)
	if err != nil {
		logger.Fatalf("marshal matrix failed: %s", err)
	}
	any := len(include) > 0
	logger.Infof("PRs to check (any=%v): %s", any, string(includeJSON))
	appendGitHubOutput("matrix", string(matrixJSON))
	appendGitHubOutput("any", strconv.FormatBool(any))
}
