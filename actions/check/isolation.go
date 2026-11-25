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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/google/go-github/v89/github"
	"github.com/siyuan-note/bazaar/actions/util"
	"github.com/siyuan-note/bazaar/rules"
)

const maxReportSize = 1024 * 1024
const maxInputSize = 2 * 1024 * 1024

var commitID = regexp.MustCompile("^[a-f0-9]{40}$")
var prSnapshots *struct{ base, head string }

// Only materialize bounded regular blobs at fixed paths. Never check out a fork
// working tree, honor its symlinks, or run its build tools.
func preparePRSnapshots(ctx context.Context) (func(), error) {
	head, base := os.Getenv("PR_HEAD_SHA"), os.Getenv("PR_BASE_SHA")
	if !commitID.MatchString(head) || !commitID.MatchString(base) {
		return nil, fmt.Errorf("invalid PR commit identity")
	}
	dir, err := os.MkdirTemp("", "bazaar-pr-data-")
	if err != nil {
		return nil, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	names := []string{util.DeprecatedRegistryRelPath}
	for _, pt := range rules.AllPackageTypes() {
		names = append(names, pt.ReposListFile())
	}
	for side, sha := range map[string]string{"head": head, "base": base} {
		target := filepath.Join(dir, side)
		if err := os.Mkdir(target, 0700); err != nil {
			cleanup()
			return nil, err
		}
		for _, name := range names {
			if err := copyRegularBlob(ctx, BAZAAR_HEAD_PATH, sha, name, target); err != nil {
				cleanup()
				return nil, err
			}
		}
	}
	PR_HEAD_PATH, PR_BASE_PATH = filepath.Join(dir, "head"), filepath.Join(dir, "base")
	prSnapshots = &struct{ base, head string }{base, head}
	return cleanup, nil
}

func copyRegularBlob(ctx context.Context, repo, sha, name, target string) error {
	git := func(args ...string) ([]byte, error) {
		return exec.CommandContext(ctx, "git", append([]string{"-C", repo}, args...)...).Output()
	}
	entry, err := git("ls-tree", "-z", sha, "--", name)
	if err != nil {
		return fmt.Errorf("list PR data %s: %w", name, err)
	}
	if len(entry) == 0 {
		return nil // Existing checker handles missing list entries/files.
	}
	meta, path, ok := strings.Cut(strings.TrimSuffix(string(entry), "\x00"), "\t")
	fields := strings.Fields(meta)
	if !ok || path != name || len(fields) != 3 || fields[0] != "100644" || fields[1] != "blob" || !commitID.MatchString(fields[2]) {
		return fmt.Errorf("PR data %s must be a regular, non-executable file", name)
	}
	sizeText, err := git("cat-file", "-s", fields[2])
	if err != nil {
		return fmt.Errorf("size PR data %s: %w", name, err)
	}
	size, err := strconv.Atoi(strings.TrimSpace(string(sizeText)))
	if err != nil || size < 0 || size > maxInputSize {
		return fmt.Errorf("PR data %s exceeds the size limit or is invalid", name)
	}
	data, err := git("cat-file", "blob", fields[2])
	if err != nil || len(data) != size {
		return fmt.Errorf("read PR data %s: unexpected blob contents", name)
	}
	return os.WriteFile(filepath.Join(target, name), data, 0600)
}

type checkReport struct {
	Number int
	Head   string
	Title  string
	Types  Set
	Result *CheckResult
}

func writeReport(result *CheckResult, types Set, title string) error {
	_, _, n, ok := prIdentity()
	if !ok {
		return fmt.Errorf("missing PR identity")
	}
	report := checkReport{n, prSnapshots.head, title, types, result}
	data, err := json.Marshal(report)
	if err != nil {
		return err
	}
	if len(data) > maxReportSize {
		return fmt.Errorf("check report too large")
	}
	return os.WriteFile(CHECK_RESULT_OUTPUT, data, 0600)
}

func readReport(r io.Reader, number int, head string) (*checkReport, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxReportSize+1))
	if err != nil || len(data) > maxReportSize {
		return nil, fmt.Errorf("cannot read bounded check report")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var p checkReport
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("invalid check report")
	}
	if dec.Decode(new(any)) != io.EOF || number < 1 || p.Number != number || !commitID.MatchString(head) || p.Head != head || p.Result == nil ||
		len(p.Title) > 256 || strings.ContainsFunc(p.Title, unicode.IsControl) {
		return nil, fmt.Errorf("invalid check report scope")
	}
	allowed := make(Set)
	for _, pt := range rules.AllPackageTypes() {
		allowed[pt.String()] = struct{}{}
	}
	for label := range p.Types {
		if _, ok := allowed[label]; !ok {
			return nil, fmt.Errorf("invalid check report label type")
		}
	}
	p.Result.MetaJSON = "" // Only the publisher may generate scheduling metadata.
	return &p, nil
}

func publishReport() error {
	owner, repo, number, ok := prIdentity()
	if !ok || PAT != "" || GITHUB_TOKEN == "" {
		return fmt.Errorf("publishing requires an Actions token, PR identity and no PAT")
	}
	f, err := os.Open(os.Getenv("CHECK_REPORT_INPUT"))
	if err != nil {
		return err
	}
	defer f.Close()
	report, err := readReport(f, number, os.Getenv("PR_HEAD_SHA"))
	if err != nil {
		return err
	}
	githubRepoClient, repoRateObs, err = util.NewGitHubClientWithRateObserver(GITHUB_TOKEN, REQUEST_TIMEOUT)
	if err != nil {
		return err
	}
	pr, _, err := githubRepoClient.PullRequests.Get(githubContext, owner, repo, number)
	if err != nil {
		return err
	}
	if !reportMatchesPR(report, pr) {
		logger.Infof("skip publication: PR no longer matches checked revision or is ineligible")
		return nil
	}
	publishResult(report.Result, report.Types, report.Title)
	if os.Getenv("REMOVE_CHECK_LABEL") == "true" {
		if _, err := githubRepoClient.Issues.RemoveLabelForIssue(githubContext, owner, repo, number, "Check"); err != nil {
			return fmt.Errorf("remove Check label: %w", err)
		}
	}
	return nil
}

func reportMatchesPR(report *checkReport, pr *github.PullRequest) bool {
	return pr != nil && pr.GetNumber() == report.Number && pr.GetState() == "open" &&
		pr.GetBase().GetRef() == "main" && pr.GetHead().GetSHA() == report.Head && !prHasLabel(pr, "ci-skip")
}
