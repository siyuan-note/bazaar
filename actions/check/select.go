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
	selectPRLimitDefault = 100
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

// runSelect 定时 / 手动复检：先关超龄 PR，再筛选待完整检查的非 ci-passed PR，写入 GITHUB_OUTPUT matrix。
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
	githubRepoClient, repoRateObs, err = util.NewGitHubClientWithRateObserver(repoToken, REQUEST_TIMEOUT)
	if err != nil {
		logger.Fatalf("create github repo client failed: %s", err)
	}
	pat := PAT
	if pat == "" {
		pat = repoToken
	}
	githubClient, patRateObs, err = util.NewGitHubClientWithRateObserver(pat, REQUEST_TIMEOUT)
	if err != nil {
		logger.Fatalf("create github client failed: %s", err)
	}
	seedRateHeaderBaselines()

	owner, repo, ok := splitOwnerRepo(GITHUB_REPOSITORY)
	if !ok {
		logger.Fatalf("invalid GITHUB_REPOSITORY %q", GITHUB_REPOSITORY)
	}

	forceAll := envTruthy("SELECT_FORCE_ALL")
	limit := envIntDefault("SELECT_LIMIT", selectPRLimitDefault)
	now := time.Now().UTC()

	prs, err := listOpenPRsForSelect(githubContext, githubRepoClient, owner, repo)
	if err != nil {
		logger.Fatalf("list PRs for select failed: %s", err)
	}
	logger.Infof("open PRs for select/stale-close: %d (force=%v limit=%d)", len(prs), forceAll, limit)

	// 复用同一次 list：先关超龄（ci-failed，或 ci-passed+manual-review），再对剩余非 ci-passed 做指纹 / 退避筛选
	prs = closeStalePRs(githubContext, githubRepoClient, owner, repo, prs, now, staleCIFailedAge)

	candidates := make([]selectCandidate, 0, len(prs))
	for _, pr := range prs {
		if prHasLabel(pr, labelCIPassed) {
			// 仅因超龄关闭而列入（ci-passed + manual-review）；未到龄则不参与复检
			logger.Infof("skip PR #%d (ci-passed; listed for stale-close only)", pr.GetNumber())
			continue
		}
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
	logger.Infof("%s", util.FormatRateHeaderObservation("PAT", patRateObs))
	logger.Infof("%s", util.FormatRateHeaderObservation("GITHUB_TOKEN", repoRateObs))
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

// listOpenPRsForSelect 列出开放中、非草稿、无 ci-skip，且需参与复检或超龄关闭的 PR：
//   - 未挂 ci-passed：参与复检与 ci-failed 超龄关闭（含尚无 CI 标签者）
//   - 同时挂 ci-passed 与 manual-review：仅参与超龄关闭（不复检）
func listOpenPRsForSelect(ctx context.Context, client *github.Client, owner, repo string) ([]*github.PullRequest, error) {
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
			if prHasLabel(pr, labelCIPassed) {
				if !prHasLabel(pr, labelManualReview) {
					continue
				}
				// ci-passed + manual-review：仅供超龄关闭
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

// loadCheckMetaFromPRComments 从 PR 评论中读取调度 meta。
// 优先取带「当前检查」标记的评论中最新一条；若无则回退到任意含 bazaar-check-meta 的评论。
func loadCheckMetaFromPRComments(ctx context.Context, client *github.Client, owner, repo string, prNumber int) (*CheckMeta, bool) {
	if client == nil {
		return nil, false
	}
	comments, err := listIssueComments(ctx, client, owner, repo, prNumber)
	if err != nil {
		logger.Warnf("list comments for PR #%d failed: %s", prNumber, err)
		return nil, false
	}
	var latestActive *CheckMeta
	var latestAny *CheckMeta
	var fallbackActiveBody, fallbackAnyBody string
	// ListComments 默认按 created 升序：同页内后者更新，跨页继续覆盖
	for _, c := range comments {
		body := c.GetBody()
		meta, ok := parseCheckMetaFromComment(body)
		active := isActiveCheckResultComment(body)
		if ok {
			latestAny = meta
			if active {
				latestActive = meta
			}
			continue
		}
		if active {
			fallbackActiveBody = body
		}
		if strings.Contains(body, checkResultCommentMarker) ||
			strings.Contains(body, checkResultCommentMarkerOutdated) ||
			strings.Contains(body, checkResultCommentTagLegacy) {
			fallbackAnyBody = body
		}
	}
	if latestActive != nil {
		return latestActive, true
	}
	if latestAny != nil {
		return latestAny, true
	}
	if fallbackActiveBody != "" {
		return parseCheckMetaFromComment(fallbackActiveBody)
	}
	if fallbackAnyBody != "" {
		return parseCheckMetaFromComment(fallbackAnyBody)
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
