// SiYuan community bazaar.
// Copyright (c) 2021-present, b3log.org
//
// Bazaar is licensed under Mulan PSL v2.
// You can use this software according to the terms and conditions of the Mulan PSL v2.
// You may obtain a copy of Mulan PSL v2 at:
//         http://license.coscl.org.cn/MulanPSL2
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR PURPOSE.
// See the Mulan PSL v2 for more details.

package util

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v89/github"
)

// RateHeaderSnapshot 汇总本客户端实际 API 响应头中的 X-RateLimit-*（仅 core）。
// 官方建议优先用响应头而非单独轮询 GET /rate_limit；本观测器完全忽略 /rate_limit。
type RateHeaderSnapshot struct {
	Samples        int // 计入 core 配额的响应次数（本客户端实际 core 请求数）
	Limit          int
	StartRemaining int // 第一次计入 samples 的请求之前的剩余（由该响应 Remaining+1 还原）
	FirstRemaining int // 第一次计入 samples 的响应头 Remaining（该请求已扣减后）
	LastRemaining  int // 最后一次观测到的 Remaining（结束时剩余配额）
	MinRemaining   int
	FirstUsed      int
	LastUsed       int
	MaxUsed        int
	HasData        bool
}

// RateHeaderObserver 通过 HTTP Transport 采集 GitHub REST 响应头中的 rate limit。
type RateHeaderObserver struct {
	mu sync.Mutex

	samples        int
	limit          int
	startRemaining int
	firstRemaining int
	lastRemaining  int
	minRemaining   int
	firstUsed      int
	lastUsed       int
	maxUsed        int
	hasData        bool
}

// NewGitHubClientWithRateObserver 同 NewGitHubClient，并挂载响应头观测 Transport。
func NewGitHubClientWithRateObserver(token string, timeout time.Duration) (*github.Client, *RateHeaderObserver, error) {
	obs := &RateHeaderObserver{}
	client, err := github.NewClient(
		github.WithAuthToken(token),
		github.WithTransport(obs.wrapTransport(http.DefaultTransport)),
		github.WithTimeout(timeout),
		github.WithUserAgent(UserAgent),
	)
	if err != nil {
		return nil, nil, err
	}
	return client, obs, nil
}

// SeedRateHeaderBaseline 串行打一次计入 core 的 API（GET /user），用真实响应头锚定观测起点，并返回该响应的 Rate。
// 不要用 GET /rate_limit：其 remaining 可能与后续业务响应头不一致。
func SeedRateHeaderBaseline(ctx context.Context, client *github.Client) (github.Rate, error) {
	_, resp, err := client.Users.Get(ctx, "")
	if err != nil {
		return github.Rate{}, err
	}
	if resp == nil {
		return github.Rate{}, fmt.Errorf("users.get: empty response")
	}
	return resp.Rate, nil
}

func (o *RateHeaderObserver) wrapTransport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &rateObserveTransport{base: base, obs: o}
}

type rateObserveTransport struct {
	base http.RoundTripper
	obs  *RateHeaderObserver
}

func (t *rateObserveTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if resp != nil {
		t.obs.observe(req, resp.Header)
	}
	return resp, err
}

func (o *RateHeaderObserver) observe(req *http.Request, h http.Header) {
	// GET /rate_limit 不计入 core，且其实测 remaining 可能与业务响应头脱节，整段忽略。
	if req != nil && isGitHubRateLimitPath(req.URL.Path) {
		return
	}
	resource := h.Get(github.HeaderRateResource)
	if resource != "" && !strings.EqualFold(resource, "core") {
		return
	}
	remainingS := h.Get(github.HeaderRateRemaining)
	limitS := h.Get(github.HeaderRateLimit)
	if remainingS == "" || limitS == "" {
		return
	}
	remaining, err1 := strconv.Atoi(remainingS)
	limit, err2 := strconv.Atoi(limitS)
	if err1 != nil || err2 != nil {
		return
	}
	used := 0
	hasUsed := false
	if usedS := h.Get(github.HeaderRateUsed); usedS != "" {
		if u, err := strconv.Atoi(usedS); err == nil {
			used = u
			hasUsed = true
		}
	}

	o.mu.Lock()
	defer o.mu.Unlock()
	o.samples++
	if !o.hasData {
		o.hasData = true
		o.limit = limit
		// 响应头 Remaining 为该请求扣减后的值；+1 还原「本次观测窗口开始前」的剩余。
		o.startRemaining = remaining + 1
		o.firstRemaining = remaining
		o.lastRemaining = remaining
		o.minRemaining = remaining
		if hasUsed {
			o.firstUsed = used
			o.lastUsed = used
			o.maxUsed = used
		}
		return
	}
	o.limit = limit
	o.lastRemaining = remaining
	if remaining < o.minRemaining {
		o.minRemaining = remaining
	}
	if hasUsed {
		o.lastUsed = used
		if used > o.maxUsed {
			o.maxUsed = used
		}
	}
}

func isGitHubRateLimitPath(path string) bool {
	return path == "rate_limit" || strings.HasSuffix(path, "/rate_limit")
}

// FormatRateHeaderObservation 格式化当次 core 请求次数与开始/结束剩余配额日志行。
func FormatRateHeaderObservation(label string, obs *RateHeaderObserver) string {
	snap := obs.Snapshot()
	if !snap.HasData {
		return fmt.Sprintf("GitHub API (%s core via headers) no rate-limit headers observed", label)
	}
	return fmt.Sprintf("GitHub API (%s core via headers) samples=%d remaining %d→%d (min %d) / %d",
		label, snap.Samples, snap.StartRemaining, snap.LastRemaining, snap.MinRemaining, snap.Limit)
}

// Snapshot 返回当前观测快照。
func (o *RateHeaderObserver) Snapshot() RateHeaderSnapshot {
	if o == nil {
		return RateHeaderSnapshot{}
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	return RateHeaderSnapshot{
		Samples:        o.samples,
		Limit:          o.limit,
		StartRemaining: o.startRemaining,
		FirstRemaining: o.firstRemaining,
		LastRemaining:  o.lastRemaining,
		MinRemaining:   o.minRemaining,
		FirstUsed:      o.firstUsed,
		LastUsed:       o.lastUsed,
		MaxUsed:        o.maxUsed,
		HasData:        o.hasData,
	}
}
