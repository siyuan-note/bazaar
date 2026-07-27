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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/siyuan-note/bazaar/actions/util"
	"github.com/siyuan-note/bazaar/rules"
)

func TestIsZeroGitSHA(t *testing.T) {
	t.Parallel()
	cases := []struct {
		sha  string
		want bool
	}{
		{"", true},
		{"0000000000000000000000000000000000000000", true},
		{"000", true},
		{"abc", false},
		{"0a0", false},
	}
	for _, tc := range cases {
		if got := isZeroGitSHA(tc.sha); got != tc.want {
			t.Fatalf("isZeroGitSHA(%q)=%v, want %v", tc.sha, got, tc.want)
		}
	}
}

func TestShouldCheck(t *testing.T) {
	t.Parallel()
	if !shouldCheck("a/b", nil) {
		t.Fatal("nil checkRepos should check all")
	}
	set := Set{"a/b": {}}
	if !shouldCheck("a/b", set) {
		t.Fatal("want check")
	}
	if shouldCheck("c/d", set) {
		t.Fatal("want skip")
	}
	if shouldCheck("a/b", Set{}) {
		t.Fatal("empty set should check none")
	}
}

func TestReposAddedAndSetEqual(t *testing.T) {
	t.Parallel()
	before := []string{"a/keep", "a/old"}
	after := []string{"a/keep", "b/new"}
	added := reposAdded(before, after)
	if len(added) != 1 || added[0] != "b/new" {
		t.Fatalf("added=%v", added)
	}
	if reposSetEqual(before, after) {
		t.Fatal("want unequal")
	}
	if !reposSetEqual([]string{"a/x", "b/y"}, []string{"b/y", "a/x"}) {
		t.Fatal("want equal ignoring order")
	}
	if reposSetEqual([]string{"a/x"}, []string{"a/x", "b/y"}) {
		t.Fatal("want unequal lengths")
	}
}

func TestCountCheckRepos(t *testing.T) {
	t.Parallel()
	jobs := stageJobs{
		rules.TypePlugin: {repos: []string{"a/1", "a/2"}, checkRepos: nil},
		rules.TypeTheme:  {repos: []string{"b/1", "b/2", "b/3"}, checkRepos: Set{"b/2": {}}},
	}
	if n := countCheckRepos(jobs); n != 3 {
		t.Fatalf("count=%d, want 3", n)
	}
}

func TestResolveStageJobs_fullByDefault(t *testing.T) {
	t.Setenv("STAGE_MODE", "")
	t.Setenv("STAGE_BEFORE_SHA", "")
	reposByType := map[rules.PackageType][]string{
		rules.TypePlugin: {"a/p"},
	}
	jobs, mode := resolveStageJobs(context.Background(), ".", reposByType)
	if mode != stageModeFull {
		t.Fatalf("mode=%s", mode)
	}
	job, ok := jobs[rules.TypePlugin]
	if !ok || job.checkRepos != nil || len(job.repos) != 1 {
		t.Fatalf("job=%v ok=%v", job, ok)
	}
}

func TestResolveStageJobs_incrementalFallsBackOnZeroBefore(t *testing.T) {
	t.Setenv("STAGE_MODE", stageModeIncremental)
	t.Setenv("STAGE_BEFORE_SHA", "0000000000000000000000000000000000000000")
	reposByType := map[rules.PackageType][]string{
		rules.TypePlugin: {"a/p"},
	}
	jobs, mode := resolveStageJobs(context.Background(), ".", reposByType)
	if mode != stageModeFull {
		t.Fatalf("mode=%s, want full fallback", mode)
	}
	if jobs[rules.TypePlugin].checkRepos != nil {
		t.Fatal("full job should check all")
	}
}

func TestBuildIncrementalStageJobs(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@example.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init")
	run("config", "core.autocrlf", "false")

	writeList := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// 五种列表都要有，便于 buildIncremental 遍历
	for _, pt := range rules.AllPackageTypes() {
		writeList(pt.ReposListFile(), "keep/"+pt.String()+"\n")
	}
	run("add", ".")
	run("commit", "-m", "base")
	before := gitRevParse(t, root)

	writeList("plugins.txt", "keep/plugin\nnew/plugin\n")
	writeList("themes.txt", "") // 删光：集合变化
	run("add", ".")
	run("commit", "-m", "change")

	reposByType := map[rules.PackageType][]string{}
	for _, pt := range rules.AllPackageTypes() {
		repos, err := util.ParseReposFromTxt(filepath.Join(root, pt.ReposListFile()))
		if err != nil {
			t.Fatal(err)
		}
		reposByType[pt] = repos
	}

	jobs, err := buildIncrementalStageJobs(context.Background(), root, before, reposByType)
	if err != nil {
		t.Fatal(err)
	}
	pluginJob, ok := jobs[rules.TypePlugin]
	if !ok {
		t.Fatal("want plugins job")
	}
	if !shouldCheck("new/plugin", pluginJob.checkRepos) || shouldCheck("keep/plugin", pluginJob.checkRepos) {
		t.Fatalf("plugin checkRepos=%v", pluginJob.checkRepos)
	}
	themeJob, ok := jobs[rules.TypeTheme]
	if !ok {
		t.Fatal("want themes job for deletion-only change")
	}
	if len(themeJob.checkRepos) != 0 {
		t.Fatalf("delete-only themes should have empty check set, got %v", themeJob.checkRepos)
	}
	if _, ok := jobs[rules.TypeIcon]; ok {
		t.Fatal("unchanged icons should be skipped")
	}
}

func TestResolveStageJobs_incrementalUnreadableBeforeFallsBack(t *testing.T) {
	t.Setenv("STAGE_MODE", stageModeIncremental)
	t.Setenv("STAGE_BEFORE_SHA", "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	reposByType := map[rules.PackageType][]string{
		rules.TypePlugin: {"a/p"},
	}
	_, mode := resolveStageJobs(context.Background(), t.TempDir(), reposByType)
	if mode != stageModeFull {
		t.Fatalf("mode=%s, want full fallback", mode)
	}
}

func gitRevParse(t *testing.T, root string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", root, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}
