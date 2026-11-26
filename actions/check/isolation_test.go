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
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-github/v89/github"
	"github.com/siyuan-note/bazaar/actions/util"
	"github.com/siyuan-note/bazaar/rules"
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
	var symlinkErr *prDataBlobError
	if err := copyRegularBlob(ctx, repo, commit(), name, dst); err == nil || !errors.As(err, &symlinkErr) || symlinkErr.Kind != prDataBlobIrregular || symlinkErr.Mode != "120000" {
		t.Fatalf("symbolic links must not be materialized: %v", err)
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("test-owner/test-plugin\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(file, 0755); err != nil {
		t.Fatal(err)
	}
	var irregular *prDataBlobError
	if err := copyRegularBlob(ctx, repo, commit(), name, dst); err == nil || !errors.As(err, &irregular) || irregular.Kind != prDataBlobIrregular || irregular.Mode != "100755" {
		t.Fatalf("executable blobs must not be materialized: %v", err)
	}
	if err := os.Chmod(file, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(strings.Repeat("x", maxInputSize+1)), 0644); err != nil {
		t.Fatal(err)
	}
	var oversized *prDataBlobError
	if err := copyRegularBlob(ctx, repo, commit(), name, dst); err == nil || !errors.As(err, &oversized) || oversized.Kind != prDataBlobOversized {
		t.Fatalf("oversized blobs must not be materialized: %v", err)
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

func TestFormatInvalidListFileFlowError(t *testing.T) {
	got := formatInvalidListFileFlowError([]listFileProblem{
		{Name: "themes.txt", Side: "head", Kind: prDataBlobOversized},
		{Name: "plugins.txt", Side: "head", Kind: prDataBlobIrregular, Mode: "120000"},
		{Name: "plugins.txt", Side: "base", Kind: prDataBlobIrregular, Mode: "100755"},
	})
	for _, want := range []string{
		"本 PR 的集市包列表文件必须是普通的、不可执行的文件（Git 模式 100644）",
		"`plugins.txt`（本 PR 的提交）：是符号链接，而不是普通文件",
		"`plugins.txt`（本 PR 的合并基点）：是可执行文件，而不是普通的不可执行文件",
		"`themes.txt`（本 PR 的提交）：大小超过 2MiB",
		"The package list files in this PR must be regular, non-executable files (Git mode 100644)",
		"`plugins.txt` (this PR's commit): is a symbolic link, not a regular file",
		"`plugins.txt` (this PR's merge base): is an executable file, not a regular non-executable file",
		"`themes.txt` (this PR's commit): exceeds the 2MiB size limit",
		"Please restore the listed files to regular text files and push again.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	pluginsAt := strings.Index(got, "`plugins.txt`（本 PR 的提交）")
	themesAt := strings.Index(got, "`themes.txt`（本 PR 的提交）")
	if pluginsAt < 0 || themesAt < 0 || pluginsAt > themesAt {
		t.Fatalf("problems should be sorted by file name then side:\n%s", got)
	}
}

func TestPreparePRSnapshots_InvalidListFileIsProblemNotFatal(t *testing.T) {
	ctx := context.Background()
	repo := t.TempDir()
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
	writeChecked := func() {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repo, util.DeprecatedRegistryRelPath), []byte("{}\n"), 0644); err != nil {
			t.Fatal(err)
		}
		for _, pt := range rules.AllPackageTypes() {
			if err := os.WriteFile(filepath.Join(repo, pt.ReposListFile()), []byte("test-owner/test-pkg\n"), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	writeChecked()
	git("add", ".")
	git("-c", "user.name=test", "-c", "user.email=test@example.invalid", "commit", "-m", "base")
	base := git("rev-parse", "HEAD")

	plugin := filepath.Join(repo, "plugins.txt")
	if err := os.Remove(plugin); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("fixture", plugin); err != nil {
		t.Fatal(err)
	}
	git("add", "-A", "plugins.txt")
	git("-c", "user.name=test", "-c", "user.email=test@example.invalid", "commit", "-m", "symlink list")
	head := git("rev-parse", "HEAD")

	origHead, origBase, origSnapshots, origBazaar := PR_HEAD_PATH, PR_BASE_PATH, prSnapshots, BAZAAR_HEAD_PATH
	t.Cleanup(func() {
		PR_HEAD_PATH, PR_BASE_PATH = origHead, origBase
		prSnapshots = origSnapshots
		BAZAAR_HEAD_PATH = origBazaar
	})
	t.Setenv("PR_HEAD_SHA", head)
	t.Setenv("PR_BASE_SHA", base)
	BAZAAR_HEAD_PATH = repo

	cleanup, problems, err := preparePRSnapshots(ctx)
	if err != nil {
		t.Fatalf("invalid list file should not abort snapshot preparation: %v", err)
	}
	t.Cleanup(cleanup)
	if len(problems) != 1 || problems[0].Name != "plugins.txt" || problems[0].Side != "head" || problems[0].Kind != prDataBlobIrregular || problems[0].Mode != "120000" {
		t.Fatalf("problems=%v", problems)
	}
	if _, err := os.Stat(filepath.Join(PR_HEAD_PATH, "plugins.txt")); !os.IsNotExist(err) {
		t.Fatalf("invalid list file must not be materialized: %v", err)
	}
	if _, err := os.Stat(filepath.Join(PR_HEAD_PATH, "themes.txt")); err != nil {
		t.Fatalf("valid list files should still be materialized: %v", err)
	}
	if _, err := os.Stat(filepath.Join(PR_BASE_PATH, "plugins.txt")); err != nil {
		t.Fatalf("valid base list file should still be materialized: %v", err)
	}
}

func TestPreparePRSnapshots_InvalidDeprecatedStillFatal(t *testing.T) {
	ctx := context.Background()
	repo := t.TempDir()
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
	if err := os.WriteFile(filepath.Join(repo, util.DeprecatedRegistryRelPath), []byte("{}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, pt := range rules.AllPackageTypes() {
		if err := os.WriteFile(filepath.Join(repo, pt.ReposListFile()), []byte("test-owner/test-pkg\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	git("add", ".")
	git("-c", "user.name=test", "-c", "user.email=test@example.invalid", "commit", "-m", "base")
	base := git("rev-parse", "HEAD")

	deprecated := filepath.Join(repo, util.DeprecatedRegistryRelPath)
	if err := os.Remove(deprecated); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("fixture", deprecated); err != nil {
		t.Fatal(err)
	}
	git("add", "-A", util.DeprecatedRegistryRelPath)
	git("-c", "user.name=test", "-c", "user.email=test@example.invalid", "commit", "-m", "symlink deprecated")
	head := git("rev-parse", "HEAD")

	origHead, origBase, origSnapshots, origBazaar := PR_HEAD_PATH, PR_BASE_PATH, prSnapshots, BAZAAR_HEAD_PATH
	t.Cleanup(func() {
		PR_HEAD_PATH, PR_BASE_PATH = origHead, origBase
		prSnapshots = origSnapshots
		BAZAAR_HEAD_PATH = origBazaar
	})
	t.Setenv("PR_HEAD_SHA", head)
	t.Setenv("PR_BASE_SHA", base)
	BAZAAR_HEAD_PATH = repo

	cleanup, problems, err := preparePRSnapshots(ctx)
	if cleanup != nil {
		t.Cleanup(cleanup)
	}
	if err == nil || len(problems) != 0 {
		t.Fatalf("invalid deprecated.json must still abort: err=%v problems=%v", err, problems)
	}
}
