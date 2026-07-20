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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/siyuan-note/bazaar/actions/util"
	"github.com/siyuan-note/bazaar/rules"
)

const (
	stageModeFull        = "full"
	stageModeIncremental = "incremental"
)

// packageStageJob 某一包类型本轮要执行的 staging。
// checkRepos == nil 表示对 repos 全量打 API；非 nil 时仅检查集合内路径（可为空，用于纯下架重建）。
type packageStageJob struct {
	repos      []string
	checkRepos Set
}

// stageJobs 本轮要跑的类型 → job；未出现的类型在增量模式下表示列表未变，跳过。
type stageJobs map[rules.PackageType]packageStageJob

func stageModeFromEnv() string {
	mode := strings.TrimSpace(strings.ToLower(os.Getenv("STAGE_MODE")))
	if mode == stageModeIncremental {
		return stageModeIncremental
	}
	return stageModeFull
}

func stageBeforeSHAFromEnv() string {
	return strings.TrimSpace(os.Getenv("STAGE_BEFORE_SHA"))
}

// isZeroGitSHA 判断是否为 GitHub 对「新 ref 首次 push」使用的全零占位 SHA。
func isZeroGitSHA(sha string) bool {
	if sha == "" {
		return true
	}
	for _, c := range sha {
		if c != '0' {
			return false
		}
	}
	return true
}

// shouldCheck 判断 owner/repo 是否需要打 GitHub API。
// checkRepos == nil 表示全查。
func shouldCheck(ownerRepo string, checkRepos Set) bool {
	if checkRepos == nil {
		return true
	}
	_, ok := checkRepos[ownerRepo]
	return ok
}

// reposAdded 返回 after 相对 before 新出现的路径（保序）。
func reposAdded(before, after []string) []string {
	beforeSet := make(Set, len(before))
	for _, r := range before {
		beforeSet[r] = struct{}{}
	}
	added := make([]string, 0)
	for _, r := range after {
		if _, ok := beforeSet[r]; !ok {
			added = append(added, r)
		}
	}
	return added
}

// reposSetEqual 判断两个列表作为集合是否相等（忽略顺序）。
func reposSetEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	setA := make(Set, len(a))
	for _, r := range a {
		setA[r] = struct{}{}
	}
	for _, r := range b {
		if _, ok := setA[r]; !ok {
			return false
		}
	}
	return true
}

func countCheckRepos(jobs stageJobs) int {
	var n int
	for _, job := range jobs {
		if job.checkRepos == nil {
			n += len(job.repos)
			continue
		}
		n += len(job.checkRepos)
	}
	return n
}

func buildFullStageJobs(reposByType map[rules.PackageType][]string) stageJobs {
	jobs := make(stageJobs, len(reposByType))
	for packageType, repos := range reposByType {
		jobs[packageType] = packageStageJob{repos: repos, checkRepos: nil}
	}
	return jobs
}

// resolveStageJobs 按 STAGE_MODE / STAGE_BEFORE_SHA 决定本轮 jobs。
// 增量失败（before 无效、git 不可读、解析失败）时回退全量。
func resolveStageJobs(ctx context.Context, repoRoot string, reposByType map[rules.PackageType][]string) (stageJobs, string) {
	mode := stageModeFromEnv()
	if mode != stageModeIncremental {
		logger.Infof("stage mode: full")
		return buildFullStageJobs(reposByType), stageModeFull
	}

	beforeSHA := stageBeforeSHAFromEnv()
	if isZeroGitSHA(beforeSHA) {
		logger.Errorf("stage mode incremental requested but STAGE_BEFORE_SHA empty/zero; falling back to full")
		return buildFullStageJobs(reposByType), stageModeFull
	}
	if !gitCommitReadable(ctx, repoRoot, beforeSHA) {
		logger.Errorf("stage mode incremental requested but before commit [%s] not readable (shallow clone?); falling back to full", beforeSHA)
		return buildFullStageJobs(reposByType), stageModeFull
	}

	jobs, err := buildIncrementalStageJobs(ctx, repoRoot, beforeSHA, reposByType)
	if err != nil {
		logger.Errorf("build incremental stage jobs failed: %s; falling back to full", err)
		return buildFullStageJobs(reposByType), stageModeFull
	}
	logger.Infof("stage mode: incremental (before=%s), types=%d, repos to check=%d", beforeSHA, len(jobs), countCheckRepos(jobs))
	return jobs, stageModeIncremental
}

func buildIncrementalStageJobs(ctx context.Context, repoRoot, beforeSHA string, reposByType map[rules.PackageType][]string) (stageJobs, error) {
	jobs := make(stageJobs)
	for _, packageType := range rules.AllPackageTypes() {
		listFile := packageType.ReposListFile()
		beforeData, err := gitShowFile(ctx, repoRoot, beforeSHA, listFile)
		if err != nil {
			return nil, fmt.Errorf("git show %s:%s: %w", beforeSHA, listFile, err)
		}
		beforeRepos, err := util.ParseReposFromBytes(listFile, beforeData)
		if err != nil {
			return nil, fmt.Errorf("parse before %s: %w", listFile, err)
		}
		afterRepos := reposByType[packageType]
		if reposSetEqual(beforeRepos, afterRepos) {
			continue
		}
		added := reposAdded(beforeRepos, afterRepos)
		check := make(Set, len(added))
		for _, r := range added {
			check[r] = struct{}{}
		}
		jobs[packageType] = packageStageJob{repos: afterRepos, checkRepos: check}
		logger.Infof("incremental [%s]: listed=%d added=%d", packageType.Plural(), len(afterRepos), len(added))
	}
	return jobs, nil
}

func gitCommitReadable(ctx context.Context, repoRoot, sha string) bool {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "cat-file", "-e", sha+"^{commit}")
	return cmd.Run() == nil
}

func gitShowFile(ctx context.Context, repoRoot, rev, relPath string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	// 统一用正斜杠路径，兼容 Windows 工作区
	relPath = filepath.ToSlash(relPath)
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "show", rev+":"+relPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("%s", msg)
	}
	return stdout.Bytes(), nil
}
