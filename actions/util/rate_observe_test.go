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
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-github/v89/github"
)

func TestRateHeaderObserverCoreHeaders(t *testing.T) {
	obs := &RateHeaderObserver{}
	rt := obs.wrapTransport(roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		resp := httptest.NewRecorder()
		resp.Header().Set(github.HeaderRateLimit, "5000")
		resp.Header().Set(github.HeaderRateRemaining, "4990")
		resp.Header().Set(github.HeaderRateUsed, "10")
		resp.Header().Set(github.HeaderRateResource, "core")
		return resp.Result(), nil
	}))

	req := httptest.NewRequest(http.MethodGet, "https://api.github.com/repos/o/r/releases/latest", nil)
	if _, err := rt.RoundTrip(req); err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	req2 := httptest.NewRequest(http.MethodGet, "https://api.github.com/repos/o/r/git/ref/tags/v1", nil)
	// 第二次 remaining 更低、used 更高
	rt = obs.wrapTransport(roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		resp := httptest.NewRecorder()
		resp.Header().Set(github.HeaderRateLimit, "5000")
		resp.Header().Set(github.HeaderRateRemaining, "4988")
		resp.Header().Set(github.HeaderRateUsed, "12")
		resp.Header().Set(github.HeaderRateResource, "core")
		return resp.Result(), nil
	}))
	if _, err := rt.RoundTrip(req2); err != nil {
		t.Fatalf("RoundTrip2: %v", err)
	}

	snap := obs.Snapshot()
	if !snap.HasData {
		t.Fatal("expected HasData")
	}
	if snap.Samples != 2 {
		t.Fatalf("Samples=%d, want 2", snap.Samples)
	}
	// 首次响应 Remaining=4990 → 开始前剩余 4991；结束为最后一次 4988
	if snap.StartRemaining != 4991 || snap.FirstRemaining != 4990 || snap.LastRemaining != 4988 || snap.MinRemaining != 4988 {
		t.Fatalf("remaining start/first/last/min = %d/%d/%d/%d", snap.StartRemaining, snap.FirstRemaining, snap.LastRemaining, snap.MinRemaining)
	}
}

func TestRateHeaderObserverIgnoresRateLimitPathAndSearch(t *testing.T) {
	obs := &RateHeaderObserver{}
	rt := obs.wrapTransport(roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		resp := httptest.NewRecorder()
		if req.URL.Path == "/rate_limit" || req.URL.Path == "rate_limit" {
			resp.Header().Set(github.HeaderRateLimit, "5000")
			resp.Header().Set(github.HeaderRateRemaining, "5000")
			resp.Header().Set(github.HeaderRateUsed, "0")
			resp.Header().Set(github.HeaderRateResource, "core")
		} else {
			resp.Header().Set(github.HeaderRateLimit, "30")
			resp.Header().Set(github.HeaderRateRemaining, "29")
			resp.Header().Set(github.HeaderRateUsed, "1")
			resp.Header().Set(github.HeaderRateResource, "search")
		}
		return resp.Result(), nil
	}))

	if _, err := rt.RoundTrip(httptest.NewRequest(http.MethodGet, "https://api.github.com/rate_limit", nil)); err != nil {
		t.Fatalf("rate_limit: %v", err)
	}
	if _, err := rt.RoundTrip(httptest.NewRequest(http.MethodGet, "https://api.github.com/search/users?q=a", nil)); err != nil {
		t.Fatalf("search: %v", err)
	}

	snap := obs.Snapshot()
	if snap.HasData {
		t.Fatal("rate_limit / search must not seed observer")
	}
	if snap.Samples != 0 {
		t.Fatalf("Samples=%d, want 0", snap.Samples)
	}

	// 随后真实 core 响应才锚定起点（即使 rate_limit 曾报 5000）
	rt = obs.wrapTransport(roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		resp := httptest.NewRecorder()
		resp.Header().Set(github.HeaderRateLimit, "5000")
		resp.Header().Set(github.HeaderRateRemaining, "3747")
		resp.Header().Set(github.HeaderRateUsed, "1253")
		resp.Header().Set(github.HeaderRateResource, "core")
		return resp.Result(), nil
	}))
	if _, err := rt.RoundTrip(httptest.NewRequest(http.MethodGet, "https://api.github.com/user", nil)); err != nil {
		t.Fatalf("user: %v", err)
	}
	snap = obs.Snapshot()
	if !snap.HasData || snap.Samples != 1 {
		t.Fatalf("HasData=%v Samples=%d", snap.HasData, snap.Samples)
	}
	if snap.StartRemaining != 3748 || snap.LastRemaining != 3747 {
		t.Fatalf("StartRemaining=%d LastRemaining=%d", snap.StartRemaining, snap.LastRemaining)
	}
}

func TestFormatRateHeaderObservation(t *testing.T) {
	if msg := FormatRateHeaderObservation("PAT", nil); msg != "GitHub API (PAT core via headers) no rate-limit headers observed" {
		t.Fatalf("empty: %q", msg)
	}
	obs := &RateHeaderObserver{}
	rt := obs.wrapTransport(roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		resp := httptest.NewRecorder()
		resp.Header().Set(github.HeaderRateLimit, "5000")
		resp.Header().Set(github.HeaderRateRemaining, "4990")
		resp.Header().Set(github.HeaderRateResource, "core")
		return resp.Result(), nil
	}))
	if _, err := rt.RoundTrip(httptest.NewRequest(http.MethodGet, "https://api.github.com/user", nil)); err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	msg := FormatRateHeaderObservation("PAT", obs)
	want := "GitHub API (PAT core via headers) samples=1 remaining 4991→4990 (min 4990) / 5000"
	if msg != want {
		t.Fatalf("got %q, want %q", msg, want)
	}
}

func TestSeedRateHeaderBaselineEmptyOwnerRepo(t *testing.T) {
	client, err := github.NewClient()
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	_, err = SeedRateHeaderBaseline(t.Context(), client, "", "bazaar")
	if err == nil || !strings.Contains(err.Error(), "empty owner/repo") {
		t.Fatalf("got %v, want empty owner/repo error", err)
	}
	_, err = SeedRateHeaderBaseline(t.Context(), client, "siyuan-note", "")
	if err == nil || !strings.Contains(err.Error(), "empty owner/repo") {
		t.Fatalf("got %v, want empty owner/repo error", err)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
