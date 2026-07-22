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
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/go-github/v89/github"
	"github.com/siyuan-note/bazaar/rules"
)

func TestIsGitHubRateLimit(t *testing.T) {
	rateErr := &github.RateLimitError{
		Message:  "API rate limit of 5000 still exceeded",
		Response: &http.Response{StatusCode: http.StatusForbidden, Request: &http.Request{Method: http.MethodGet}},
	}
	abuseErr := &github.AbuseRateLimitError{
		Message:  "You have exceeded a secondary rate limit",
		Response: &http.Response{StatusCode: http.StatusForbidden, Request: &http.Request{Method: http.MethodPost}},
	}

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "普通错误", err: errors.New("boom"), want: false},
		{name: "RateLimitError", err: rateErr, want: true},
		{name: "AbuseRateLimitError", err: abuseErr, want: true},
		{
			name: "LocalizedError 包装 RateLimitError",
			err: rules.LocalizedErr(
				"无法获取 Latest Release",
				"Couldn't fetch the Latest Release",
				fmt.Errorf("%w: %w", ErrNoLatestRelease, rateErr),
			),
			want: true,
		},
		{
			name: "LocalizedError 包装 AbuseRateLimitError",
			err: rules.LocalizedErr(
				"无法获取 Latest Release",
				"Couldn't fetch the Latest Release",
				fmt.Errorf("%w: %w", ErrNoLatestRelease, abuseErr),
			),
			want: true,
		},
		{
			name: "普通 API 错误不误判",
			err: rules.LocalizedErr(
				"无法获取 Latest Release",
				"Couldn't fetch the Latest Release",
				fmt.Errorf("%w: %w", ErrNoLatestRelease, errors.New("404 Not Found")),
			),
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsGitHubRateLimit(tt.err); got != tt.want {
				t.Fatalf("IsGitHubRateLimit() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsGitHubNotFound(t *testing.T) {
	notFound := &github.ErrorResponse{
		Response: &http.Response{StatusCode: http.StatusNotFound, Request: &http.Request{Method: http.MethodGet}},
		Message:  "Not Found",
	}
	forbidden := &github.ErrorResponse{
		Response: &http.Response{StatusCode: http.StatusForbidden, Request: &http.Request{Method: http.MethodGet}},
		Message:  "Forbidden",
	}

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "普通错误", err: errors.New("boom"), want: false},
		{name: "ErrorResponse 404", err: notFound, want: true},
		{name: "ErrorResponse 403", err: forbidden, want: false},
		{
			name: "LocalizedError 包装 404",
			err: rules.LocalizedErr(
				"无法获取 Latest Release",
				"Couldn't fetch the Latest Release",
				fmt.Errorf("%w: %w", ErrNoLatestRelease, notFound),
			),
			want: true,
		},
		{
			name: "纯字符串 404 不误判",
			err:  errors.New("404 Not Found"),
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsGitHubNotFound(tt.err); got != tt.want {
				t.Fatalf("IsGitHubNotFound() = %v, want %v", got, tt.want)
			}
		})
	}
}
