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
	"testing"

	"github.com/google/go-github/v89/github"
)

func newTestGitHubClient(t *testing.T, handler http.Handler) *github.Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client, err := github.NewClient(github.WithEnterpriseURLs(srv.URL, srv.URL))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
}

func TestCheckRepoPublic(t *testing.T) {
	t.Run("公开通过", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/api/v3/repos/o/r", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"private": false})
		})
		err := CheckRepoPublic(context.Background(), newTestGitHubClient(t, mux), "o", "r")
		if err != nil {
			t.Fatalf("CheckRepoPublic: %v", err)
		}
	})

	t.Run("私有失败", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/api/v3/repos/o/r", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"private": true})
		})
		err := CheckRepoPublic(context.Background(), newTestGitHubClient(t, mux), "o", "r")
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, ErrRepoNotPublic) {
			t.Fatalf("errors.Is(ErrRepoNotPublic) = false, err=%v", err)
		}
	})

	t.Run("404 失败", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/api/v3/repos/o/r", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"Not Found"}`))
		})
		err := CheckRepoPublic(context.Background(), newTestGitHubClient(t, mux), "o", "r")
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, ErrRepoNotPublic) {
			t.Fatalf("errors.Is(ErrRepoNotPublic) = false, err=%v", err)
		}
		if !IsGitHubNotFound(err) {
			t.Fatalf("want wrapped 404, err=%v", err)
		}
	})

	t.Run("client nil", func(t *testing.T) {
		err := CheckRepoPublic(context.Background(), nil, "o", "r")
		if !errors.Is(err, ErrRepoNotPublic) {
			t.Fatalf("errors.Is(ErrRepoNotPublic) = false, err=%v", err)
		}
	})
}

func TestCheckRepoLicenseFile(t *testing.T) {
	writeRootListing := func(w http.ResponseWriter, files ...struct {
		name string
		size int
	}) {
		w.Header().Set("Content-Type", "application/json")
		entries := make([]map[string]any, 0, len(files))
		for _, f := range files {
			entries = append(entries, map[string]any{
				"name": f.name,
				"path": f.name,
				"type": "file",
				"size": f.size,
			})
		}
		_ = json.NewEncoder(w).Encode(entries)
	}
	file := func(name string, size int) struct {
		name string
		size int
	} {
		return struct {
			name string
			size int
		}{name: name, size: size}
	}

	t.Run("有 LICENSE", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/api/v3/repos/o/r/contents/", func(w http.ResponseWriter, r *http.Request) {
			writeRootListing(w, file("README.md", 10), file("LICENSE", 1200), file("plugin.json", 50))
		})
		err := CheckRepoLicenseFile(context.Background(), newTestGitHubClient(t, mux), "o", "r")
		if err != nil {
			t.Fatalf("CheckRepoLicenseFile: %v", err)
		}
	})

	t.Run("有 LICENSE.txt", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/api/v3/repos/o/r/contents/", func(w http.ResponseWriter, r *http.Request) {
			writeRootListing(w, file("LICENSE.txt", 800))
		})
		err := CheckRepoLicenseFile(context.Background(), newTestGitHubClient(t, mux), "o", "r")
		if err != nil {
			t.Fatalf("CheckRepoLicenseFile: %v", err)
		}
	})

	t.Run("大小写不敏感", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/api/v3/repos/o/r/contents/", func(w http.ResponseWriter, r *http.Request) {
			writeRootListing(w, file("License", 100), file("readme.md", 20))
		})
		err := CheckRepoLicenseFile(context.Background(), newTestGitHubClient(t, mux), "o", "r")
		if err != nil {
			t.Fatalf("CheckRepoLicenseFile: %v", err)
		}
	})

	t.Run("空文件无正文", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/api/v3/repos/o/r/contents/", func(w http.ResponseWriter, r *http.Request) {
			writeRootListing(w, file("LICENSE", 0))
		})
		err := CheckRepoLicenseFile(context.Background(), newTestGitHubClient(t, mux), "o", "r")
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, ErrRepoEmptyLicense) {
			t.Fatalf("errors.Is(ErrRepoEmptyLicense) = false, err=%v", err)
		}
	})

	t.Run("仅 LICENSE.md 不算", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/api/v3/repos/o/r/contents/", func(w http.ResponseWriter, r *http.Request) {
			writeRootListing(w, file("LICENSE.md", 500), file("licence", 500))
		})
		err := CheckRepoLicenseFile(context.Background(), newTestGitHubClient(t, mux), "o", "r")
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, ErrRepoNoLicense) {
			t.Fatalf("errors.Is(ErrRepoNoLicense) = false, err=%v", err)
		}
	})

	t.Run("缺失", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/api/v3/repos/o/r/contents/", func(w http.ResponseWriter, r *http.Request) {
			writeRootListing(w, file("README.md", 10))
		})
		err := CheckRepoLicenseFile(context.Background(), newTestGitHubClient(t, mux), "o", "r")
		if !errors.Is(err, ErrRepoNoLicense) {
			t.Fatalf("errors.Is(ErrRepoNoLicense) = false, err=%v", err)
		}
	})

	t.Run("API 失败", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/api/v3/repos/o/r/contents/", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"boom"}`))
		})
		err := CheckRepoLicenseFile(context.Background(), newTestGitHubClient(t, mux), "o", "r")
		if !errors.Is(err, ErrRepoNoLicense) {
			t.Fatalf("errors.Is(ErrRepoNoLicense) = false, err=%v", err)
		}
	})
}
