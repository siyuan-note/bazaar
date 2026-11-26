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
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
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
//
// 五个包列表文件若不是普通的不可执行 100644 文件、或超过 2MiB，记入 listProblems 供调用方写成 FlowError，
// 不中止准备，以便仍能发布检查评论。deprecated.json 等其他受检文件仍遇错即失败。
func preparePRSnapshots(ctx context.Context) (cleanup func(), listProblems []listFileProblem, err error) {
	head, base := os.Getenv("PR_HEAD_SHA"), os.Getenv("PR_BASE_SHA")
	if !commitID.MatchString(head) || !commitID.MatchString(base) {
		return nil, nil, fmt.Errorf("invalid PR commit identity")
	}
	dir, err := os.MkdirTemp("", "bazaar-pr-data-")
	if err != nil {
		return nil, nil, err
	}
	cleanup = func() { _ = os.RemoveAll(dir) }
	names := []string{util.DeprecatedRegistryRelPath}
	for _, pt := range rules.AllPackageTypes() {
		names = append(names, pt.ReposListFile())
	}
	for _, side := range []struct{ name, sha string }{{"head", head}, {"base", base}} {
		target := filepath.Join(dir, side.name)
		if err := os.Mkdir(target, 0700); err != nil {
			cleanup()
			return nil, nil, err
		}
		for _, name := range names {
			if err := copyRegularBlob(ctx, BAZAAR_HEAD_PATH, side.sha, name, target); err != nil {
				var blobErr *prDataBlobError
				if isWhitelistedPath(name) && errors.As(err, &blobErr) {
					listProblems = append(listProblems, listFileProblem{
						Name: name,
						Side: side.name,
						Kind: blobErr.Kind,
						Mode: blobErr.Mode,
					})
					continue
				}
				cleanup()
				return nil, nil, err
			}
		}
	}
	sortListFileProblems(listProblems)
	PR_HEAD_PATH, PR_BASE_PATH = filepath.Join(dir, "head"), filepath.Join(dir, "base")
	prSnapshots = &struct{ base, head string }{base, head}
	return cleanup, listProblems, nil
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
		mode := ""
		if len(fields) > 0 {
			mode = fields[0]
		}
		return &prDataBlobError{Name: name, Kind: prDataBlobIrregular, Mode: mode}
	}
	sizeText, err := git("cat-file", "-s", fields[2])
	if err != nil {
		return fmt.Errorf("size PR data %s: %w", name, err)
	}
	size, err := strconv.Atoi(strings.TrimSpace(string(sizeText)))
	if err != nil || size < 0 {
		return fmt.Errorf("size PR data %s: invalid size %q", name, strings.TrimSpace(string(sizeText)))
	}
	if size > maxInputSize {
		return &prDataBlobError{Name: name, Kind: prDataBlobOversized}
	}
	data, err := git("cat-file", "blob", fields[2])
	if err != nil || len(data) != size {
		return fmt.Errorf("read PR data %s: unexpected blob contents", name)
	}
	return os.WriteFile(filepath.Join(target, name), data, 0600)
}

// prDataBlobKind 是受检 PR 文件未通过形态校验的原因。
type prDataBlobKind int

const (
	prDataBlobIrregular prDataBlobKind = iota
	prDataBlobOversized
)

// prDataBlobError 表示某个受检路径存在，但不是可安全物化的普通 100644 文件（或超过大小上限）。
type prDataBlobError struct {
	Name string
	Kind prDataBlobKind
	Mode string
}

func (e *prDataBlobError) Error() string {
	if e == nil {
		return "invalid PR data"
	}
	if e.Kind == prDataBlobOversized {
		return fmt.Sprintf("PR data %s exceeds the size limit or is invalid", e.Name)
	}
	return fmt.Sprintf("PR data %s must be a regular, non-executable file", e.Name)
}

// listFileProblem 记录某一侧（PR head / merge base）的包列表文件形态问题。
type listFileProblem struct {
	Name string
	Side string // head | base
	Kind prDataBlobKind
	Mode string
}

func sortListFileProblems(problems []listFileProblem) {
	sideOrder := map[string]int{"head": 0, "base": 1}
	sort.SliceStable(problems, func(i, j int) bool {
		if problems[i].Name != problems[j].Name {
			return problems[i].Name < problems[j].Name
		}
		oi, oj := sideOrder[problems[i].Side], sideOrder[problems[j].Side]
		if oi != oj {
			return oi < oj
		}
		return problems[i].Side < problems[j].Side
	})
}

func listFileProblemNames(problems []listFileProblem) []string {
	names := make([]string, 0, len(problems))
	seen := make(map[string]struct{}, len(problems))
	for _, p := range problems {
		if _, ok := seen[p.Name]; ok {
			continue
		}
		seen[p.Name] = struct{}{}
		names = append(names, p.Name)
	}
	return names
}

func listFileProblemSideZh(side string) string {
	if side == "base" {
		return "本 PR 的合并基点"
	}
	return "本 PR 的提交"
}

func listFileProblemSideEn(side string) string {
	if side == "base" {
		return "this PR's merge base"
	}
	return "this PR's commit"
}

func listFileProblemReasonZh(p listFileProblem) string {
	switch {
	case p.Kind == prDataBlobOversized:
		return "大小超过 2MiB"
	case p.Mode == "120000":
		return "是符号链接，而不是普通文件"
	case p.Mode == "100755":
		return "是可执行文件，而不是普通的不可执行文件"
	default:
		return "必须是普通的、不可执行的文件（Git 模式 100644）"
	}
}

func listFileProblemReasonEn(p listFileProblem) string {
	switch {
	case p.Kind == prDataBlobOversized:
		return "exceeds the 2MiB size limit"
	case p.Mode == "120000":
		return "is a symbolic link, not a regular file"
	case p.Mode == "100755":
		return "is an executable file, not a regular non-executable file"
	default:
		return "must be a regular, non-executable file (Git mode 100644)"
	}
}

// formatInvalidListFileFlowError 返回包列表文件形态不合法时的双语 FlowError 正文。
func formatInvalidListFileFlowError(problems []listFileProblem) string {
	if len(problems) == 0 {
		return ""
	}
	sorted := append([]listFileProblem(nil), problems...)
	sortListFileProblems(sorted)
	var zhItems, enItems strings.Builder
	for _, p := range sorted {
		fmt.Fprintf(&zhItems, "- `%s`（%s）：%s\n", p.Name, listFileProblemSideZh(p.Side), listFileProblemReasonZh(p))
		fmt.Fprintf(&enItems, "- `%s` (%s): %s\n", p.Name, listFileProblemSideEn(p.Side), listFileProblemReasonEn(p))
	}
	return "本 PR 的集市包列表文件必须是普通的、不可执行的文件（Git 模式 100644），且大小不超过 2MiB。当前无法检查以下文件：\n\n" +
		zhItems.String() +
		"\n请将上述列表恢复为普通文本文件后再提交。\n\n" +
		"The package list files in this PR must be regular, non-executable files (Git mode 100644) no larger than 2MiB. The following files cannot be checked:\n\n" +
		enItems.String() +
		"\nPlease restore the listed files to regular text files and push again.\n"
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
	return nil
}

func reportMatchesPR(report *checkReport, pr *github.PullRequest) bool {
	return pr != nil && pr.GetNumber() == report.Number && pr.GetState() == "open" &&
		pr.GetBase().GetRef() == "main" && pr.GetHead().GetSHA() == report.Head && !prHasLabel(pr, "ci-skip")
}
