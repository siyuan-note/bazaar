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
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-github/v89/github"
)

func TestCopyRegularBlob(t *testing.T) {
	ctx := context.Background()
	repo, dst := t.TempDir(), t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init")
	name := "plugins.txt"
	file := filepath.Join(repo, name)
	commit := func() string {
		git("add", name)
		git("-c", "user.name=test", "-c", "user.email=test@example.invalid", "commit", "-m", "fixture")
		return git("rev-parse", "HEAD")
	}
	if err := os.WriteFile(file, []byte("test-owner/test-plugin\n"), 0644); err != nil {
		t.Fatal(err)
	}
	normal := commit()
	if err := copyRegularBlob(ctx, repo, normal, name, dst); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dst, name))
	if err != nil || string(got) != "test-owner/test-plugin\n" {
		t.Fatalf("unexpected materialized blob: %q, %v", got, err)
	}
	if err := copyRegularBlob(ctx, repo, normal, "themes.txt", dst); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dst, "themes.txt")); !os.IsNotExist(err) {
		t.Fatalf("missing blob should stay missing: %v", err)
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("fixture", file); err != nil {
		t.Fatal(err)
	}
	if err := copyRegularBlob(ctx, repo, commit(), name, dst); err == nil {
		t.Fatal("symbolic links must not be materialized")
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(strings.Repeat("x", maxInputSize+1)), 0644); err != nil {
		t.Fatal(err)
	}
	if err := copyRegularBlob(ctx, repo, commit(), name, dst); err == nil {
		t.Fatal("oversized blobs must not be materialized")
	}
}

func TestReadReportScope(t *testing.T) {
	head := strings.Repeat("a", 40)
	p := checkReport{Number: 7, Head: head, Title: "Add sample/plugin", Types: Set{"plugin": {}}, Result: &CheckResult{MetaJSON: "not authoritative", PluginsDeleted: []string{"sample/plugin"}}}
	encode := func(r checkReport) string {
		t.Helper()
		b, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	valid := encode(p)
	r, err := readReport(strings.NewReader(valid), 7, head)
	if err != nil || r.Result.MetaJSON != "" || !checkResultCIPassed(r.Result) {
		t.Fatalf("valid report was not preserved: %v, %v", r, err)
	}
	p.Types = Set{"ci-passed": {}}
	for name, tc := range map[string]struct {
		body   string
		number int
		head   string
	}{
		"new revision":  {valid, 7, strings.Repeat("b", 40)},
		"other PR":      {valid, 8, head},
		"invalid label": {encode(p), 7, head},
		"extra JSON":    {valid + "{}", 7, head},
		"oversize":      {strings.Repeat(" ", maxReportSize) + valid, 7, head},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := readReport(strings.NewReader(tc.body), tc.number, tc.head); err == nil {
				t.Fatal("accepted invalid report")
			}
		})
	}
}

func TestReportMatchesPR(t *testing.T) {
	head := strings.Repeat("a", 40)
	report := &checkReport{Number: 7, Head: head}
	pr := &github.PullRequest{
		Number: new(7), State: new("open"),
		Base: &github.PullRequestBranch{Ref: new("main")},
		Head: &github.PullRequestBranch{SHA: new(head)},
	}
	if !reportMatchesPR(report, pr) {
		t.Fatal("current open PR should accept its report")
	}
	pr.Head.SHA = new(strings.Repeat("b", 40))
	if reportMatchesPR(report, pr) {
		t.Fatal("stale report must not publish")
	}
	pr.Head.SHA = new(head)
	pr.Labels = []*github.Label{{Name: new("ci-skip")}}
	if reportMatchesPR(report, pr) {
		t.Fatal("skipped PR must not publish")
	}
	pr.Labels = nil
	pr.State = new("closed")
	if reportMatchesPR(report, pr) {
		t.Fatal("closed PR must not publish")
	}
}
