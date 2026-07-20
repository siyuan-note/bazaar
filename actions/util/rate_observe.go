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
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v89/github"
)

// RateHeaderSnapshot 汇总本客户端实际 API 响应头中的 X-RateLimit-*（仅 core）。
// 官方建议优先用响应头而非单独轮询 GET /rate_limit。
type RateHeaderSnapshot struct {
	Samples        int // 带 core 限流头的响应次数（不含 GET /rate_limit）
	Limit          int
	FirstRemaining int
	LastRemaining  int
	MinRemaining   int
	FirstUsed      int
	LastUsed       int
	MaxUsed        int
	HasData        bool
}

// UsedDelta 为本轮观测窗口内 X-RateLimit-Used 的增量；窗口重置导致 Used 回落时返回 -1。
func (s RateHeaderSnapshot) UsedDelta() int {
	if !s.HasData {
		return 0
	}
	d := s.MaxUsed - s.FirstUsed
	if d < 0 {
		return -1
	}
	return d
}

// RateHeaderObserver 通过 HTTP Transport 采集 GitHub REST 响应头中的 rate limit。
type RateHeaderObserver struct {
	mu sync.Mutex

	samples        int
	limit          int
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

	skipSample := req != nil && isGitHubRateLimitPath(req.URL.Path)

	o.mu.Lock()
	defer o.mu.Unlock()
	if !skipSample {
		o.samples++
	}
	if !o.hasData {
		o.hasData = true
		o.limit = limit
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
		FirstRemaining: o.firstRemaining,
		LastRemaining:  o.lastRemaining,
		MinRemaining:   o.minRemaining,
		FirstUsed:      o.firstUsed,
		LastUsed:       o.lastUsed,
		MaxUsed:        o.maxUsed,
		HasData:        o.hasData,
	}
}
