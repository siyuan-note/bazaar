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
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/google/go-github/v89/github"
)

func TestFetchLatestReleaseRetriesTransient404(t *testing.T) {
	oldWait := latestRelease404RetryWait
	oldMax := latestRelease404MaxAttempts
	latestRelease404RetryWait = 0
	latestRelease404MaxAttempts = 3
	t.Cleanup(func() {
		latestRelease404RetryWait = oldWait
		latestRelease404MaxAttempts = oldMax
	})

	var latestHits atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/repos/o/r/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		n := latestHits.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"Not Found"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":           99,
			"tag_name":     "v1.0.0",
			"html_url":     "https://github.com/o/r/releases/tag/v1.0.0",
			"published_at": "2024-01-02T03:04:05Z",
			"assets": []map[string]any{
				{"id": 42, "name": "package.zip", "updated_at": "2024-01-02T04:00:00Z"},
			},
		})
	})
	mux.HandleFunc("/api/v3/repos/o/r/git/ref/tags/v1.0.0", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ref": "refs/tags/v1.0.0",
			"object": map[string]any{
				"type": "commit",
				"sha":  "abc123def456",
			},
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client, err := github.NewClient(github.WithEnterpriseURLs(srv.URL, srv.URL))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	info, err := FetchLatestRelease(context.Background(), client, "o", "r")
	if err != nil {
		t.Fatalf("FetchLatestRelease: %v", err)
	}
	if got := latestHits.Load(); got != 3 {
		t.Fatalf("latest release hits = %d, want 3", got)
	}
	if info.ID != 99 || info.Tag != "v1.0.0" || info.PackageZipAssetID != 42 || info.CommitSHA != "abc123def456" {
		t.Fatalf("unexpected release info: %+v", info)
	}
	if info.PackageZipUpdatedAt != "2024-01-02T04:00:00Z" {
		t.Fatalf("PackageZipUpdatedAt = %q", info.PackageZipUpdatedAt)
	}
}

func TestProbeLatestRelease(t *testing.T) {
	oldWait := latestRelease404RetryWait
	oldMax := latestRelease404MaxAttempts
	latestRelease404RetryWait = 0
	latestRelease404MaxAttempts = 1
	t.Cleanup(func() {
		latestRelease404RetryWait = oldWait
		latestRelease404MaxAttempts = oldMax
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/repos/o/r/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":           7,
			"tag_name":     "v2",
			"html_url":     "https://github.com/o/r/releases/tag/v2",
			"published_at": "2024-02-01T00:00:00Z",
			"assets": []map[string]any{
				{"id": 8, "name": "package.zip", "updated_at": "2024-02-01T01:00:00Z"},
			},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client, err := github.NewClient(github.WithEnterpriseURLs(srv.URL, srv.URL))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	info, err := ProbeLatestRelease(context.Background(), client, "o", "r")
	if err != nil {
		t.Fatalf("ProbeLatestRelease: %v", err)
	}
	if info.ID != 7 || info.Tag != "v2" || info.PackageZipAssetID != 8 || info.CommitSHA != "" {
		t.Fatalf("unexpected probe info: %+v", info)
	}
}

func TestFetchLatestReleasePersistent404(t *testing.T) {
	oldWait := latestRelease404RetryWait
	oldMax := latestRelease404MaxAttempts
	latestRelease404RetryWait = 0
	latestRelease404MaxAttempts = 3
	t.Cleanup(func() {
		latestRelease404RetryWait = oldWait
		latestRelease404MaxAttempts = oldMax
	})

	var latestHits atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/repos/o/r/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		latestHits.Add(1)
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Not Found"}`))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client, err := github.NewClient(github.WithEnterpriseURLs(srv.URL, srv.URL))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	_, err = FetchLatestRelease(context.Background(), client, "o", "r")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrNoLatestRelease) {
		t.Fatalf("errors.Is(ErrNoLatestRelease) = false, err=%v", err)
	}
	if !IsGitHubNotFound(err) {
		t.Fatalf("want wrapped 404, err=%v", err)
	}
	if got := latestHits.Load(); got != 3 {
		t.Fatalf("latest release hits = %d, want 3", got)
	}
}
