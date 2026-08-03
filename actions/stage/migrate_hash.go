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
	"cmp"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/siyuan-note/bazaar/actions/util"
	"github.com/siyuan-note/bazaar/rules"
	"golang.org/x/sync/errgroup"
)

// TEMP: 一次性迁移开关。为 true 时 Stage 只从 OSS 收集现有 stage 条目的 package.zip，
// 回填 packageZipSha256、将 url @hash 改为内容 SHA-256 前 7 位，并上传到新 OSS 键；
// 不拉 Latest Release、不跑 rules.Check、不更新 stars、不同步 stage-fail。
// 迁移跑通并提交 stage JSON 后务必改回 false（或删掉本文件相关逻辑）。
const stageHashMigrateOnly = true

const ossPackageBaseURL = "https://oss.b3logfile.com/package/"

// runStageHashMigrate 按 *s.txt 列出的仓，仅处理已有 stage 条目：从 OSS 取当前 package.zip 做内容哈希迁移。
func runStageHashMigrate() {
	logger.Infof("TEMP stage hash migrate: collect package.zip from OSS only (no release fetch, no rules.Check)")

	reposByType, err := loadReposByPackageType()
	if err != nil {
		logger.Fatalf("parse repos list failed: %s", err)
	}

	for _, packageType := range rules.AllPackageTypes() {
		repos := reposByType[packageType]
		if err := migrateStageType(packageType, repos); err != nil {
			logger.Fatalf("migrate stage [%s] failed: %s", packageType.Plural(), err)
		}
	}
	logger.Infof("TEMP stage hash migrate completed")
}

func migrateStageType(packageType rules.PackageType, reposSlice []string) error {
	oldStageData, err := loadOldStageData(packageType)
	if err != nil {
		return err
	}
	logger.Infof("migrate stage [%s] (%d listed, %d staged)", packageType.Plural(), len(reposSlice), len(oldStageData))

	var stageReposMu sync.Mutex
	var stageRepos []*util.StageRepo
	var g errgroup.Group
	g.SetLimit(STAGE_POOL_SIZE)

	for _, ownerRepo := range reposSlice {
		g.Go(func() error {
			exactOld := oldStageData[ownerRepo]
			if exactOld == nil {
				logger.Infof("skip unstaged [%s] during hash migrate", ownerRepo)
				return nil
			}

			migrated, migrateErr := migrateStagedRepo(ownerRepo, packageType, exactOld)
			if migrateErr != nil {
				logger.Errorf("migrate [%s] failed: %s; keeping old stage entry", ownerRepo, migrateErr)
				stageReposMu.Lock()
				stageRepos = append(stageRepos, exactOld)
				stageReposMu.Unlock()
				return nil
			}
			stageReposMu.Lock()
			stageRepos = append(stageRepos, migrated)
			stageReposMu.Unlock()
			return nil
		})
	}
	_ = g.Wait()

	slices.SortStableFunc(stageRepos, func(a, b *util.StageRepo) int {
		return cmp.Compare(b.Updated, a.Updated)
	})

	staged := util.StageFile{Repos: make([]util.StageRepo, len(stageRepos))}
	for i, repo := range stageRepos {
		staged.Repos[i] = *repo
		rules.ClearEmptyFunding(&staged.Repos[i].Package)
		rules.ClearRedundantLocales(&staged.Repos[i].Package)
	}

	data, err := marshalSortedIndentedJSON(staged)
	if err != nil {
		return fmt.Errorf("marshal stage [%s]: %w", packageType.StageJSONFile(), err)
	}
	stageFilePath := filepath.Join(BAZAAR_ROOT_PATH, "stage", packageType.StageJSONFile())
	if err = os.WriteFile(stageFilePath, data, 0644); err != nil {
		return fmt.Errorf("write stage [%s]: %w", packageType.StageJSONFile(), err)
	}
	logger.Infof("finish migrate stage [%s] (%d repos)", packageType.Plural(), len(stageRepos))
	return nil
}

func migrateStagedRepo(ownerRepo string, packageType rules.PackageType, exactOld *util.StageRepo) (*util.StageRepo, error) {
	oldHash := parseHashFromStageURL(exactOld.URL)
	if exactOld.PackageZipSHA256 != "" {
		wantHash := util.PackageHashFromSHA256(exactOld.PackageZipSHA256)
		if wantHash != "" && wantHash == oldHash {
			logger.Infof("skip migrated [%s], package.zip sha256 already set [%s]", ownerRepo, wantHash)
			return exactOld, nil
		}
	}

	zipData, err := downloadStagedPackageZip(githubContext, exactOld.URL)
	if err != nil {
		return nil, err
	}
	sha256Hex := util.SHA256Hex(zipData)
	hash := util.PackageHashFromSHA256(sha256Hex)
	if hash == "" {
		return nil, fmt.Errorf("empty package hash from sha256")
	}

	kept := *exactOld
	kept.PackageZipSHA256 = sha256Hex

	if oldHash == hash {
		logger.Infof("backfill sha256 only [%s] (url hash already %s)", ownerRepo, hash)
		return &kept, nil
	}

	if err := uploadMigratedPackage(ownerRepo, packageType, hash, zipData, &kept.Package); err != nil {
		return nil, err
	}
	kept.URL = ownerRepo + "@" + hash
	logger.Infof("migrated [%s] %s -> %s", ownerRepo, oldHash, hash)
	return &kept, nil
}

func downloadStagedPackageZip(ctx context.Context, stageURL string) ([]byte, error) {
	if stageURL == "" {
		return nil, fmt.Errorf("empty stage url")
	}
	u := ossPackageBaseURL + stageURL
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", util.UserAgent)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: status %d", u, resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("GET %s: empty body", u)
	}
	return data, nil
}

// uploadMigratedPackage 上传 package.zip 与展示用根文件到新 hash 键；不解发 rules.Check。
func uploadMigratedPackage(ownerRepo string, packageType rules.PackageType, hash string, zipData []byte, pkg *rules.Package) error {
	key := "package/" + ownerRepo + "@" + hash
	if err := util.UploadOSS(githubContext, key, zipData); err != nil {
		return fmt.Errorf("upload package.zip: %w", err)
	}

	tmpUnzipPath, cleanup, err := util.UnzipPackageZipData(zipData)
	if err != nil {
		return err
	}
	defer cleanup()

	packageRoot, err := rules.ResolvePackageRoot(tmpUnzipPath)
	if err != nil {
		return err
	}

	uploadFiles := Set{
		"README.md":                {},
		"preview.png":              {},
		"icon.png":                 {},
		packageType.ManifestFile(): {},
	}
	if pkg != nil && pkg.Readme != nil {
		for _, readmePath := range pkg.Readme {
			readmePath = strings.TrimSpace(readmePath)
			if readmePath == "" {
				continue
			}
			uploadFiles[readmePath] = struct{}{}
		}
	}

	g, ctx := errgroup.WithContext(githubContext)
	for fileName := range uploadFiles {
		g.Go(func() error {
			return uploadPackageRootFile(ctx, ownerRepo, hash, packageRoot, fileName)
		})
	}
	if err := g.Wait(); err != nil {
		return fmt.Errorf("upload package root files: %w", err)
	}
	return nil
}
