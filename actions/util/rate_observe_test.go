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
	if snap.FirstRemaining != 4990 || snap.LastRemaining != 4988 || snap.MinRemaining != 4988 {
		t.Fatalf("remaining first/last/min = %d/%d/%d", snap.FirstRemaining, snap.LastRemaining, snap.MinRemaining)
	}
	if snap.UsedDelta() != 2 {
		t.Fatalf("UsedDelta=%d, want 2", snap.UsedDelta())
	}
}

func TestRateHeaderObserverSkipsRateLimitPathAndSearch(t *testing.T) {
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
	if !snap.HasData {
		t.Fatal("rate_limit response should seed HasData")
	}
	if snap.Samples != 0 {
		t.Fatalf("Samples=%d, want 0 (rate_limit excluded, search ignored)", snap.Samples)
	}
	if snap.FirstRemaining != 5000 {
		t.Fatalf("FirstRemaining=%d, want 5000", snap.FirstRemaining)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
