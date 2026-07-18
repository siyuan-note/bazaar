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
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/siyuan-note/bazaar/actions/util"
	"github.com/siyuan-note/bazaar/rules"
	"golang.org/x/sync/errgroup"
)

// errInvalidOwnerRepo 防御性错误：列表解析已保证 owner/repo，正常流程不应出现。
// 仅打日志，不写入 Stage 失败汇总 Issue。
var errInvalidOwnerRepo = errors.New("invalid owner/repo")

// indexOldStageByRepoName 按 GitHub 仓库名（owner/repo 的 repo 段）索引旧 stage 条目，供换维护者交接。
// 若同名出现多次，后者覆盖前者（正常 stage 中同类型不应并存同名仓库）。
func indexOldStageByRepoName(oldStageData map[string]*util.StageRepo) map[string]*util.StageRepo {
	byName := make(map[string]*util.StageRepo, len(oldStageData))
	for ownerRepo, repo := range oldStageData {
		_, repoName, ok := strings.Cut(ownerRepo, "/")
		if !ok || repoName == "" {
			continue
		}
		byName[repoName] = repo
	}
	return byName
}

// resolveStageCheckLegacy 决定 rules.Check 使用的旧 name/version，以及失败时可否保留的同路径旧条目。
// - 同路径命中：返回 exactOld，并带 OldName+OldVersion（常规更新）
// - 换维护者（同 GitHub 仓库名，且旧 owner/repo 已不在列表）：同样带 OldName+OldVersion（视同更新须升版），exactOld 为 nil（不得写回旧 URL）
func resolveStageCheckLegacy(
	ownerRepo string,
	oldStageData map[string]*util.StageRepo,
	oldByRepoName map[string]*util.StageRepo,
	listed Set,
) (exactOld *util.StageRepo, oldName, oldVersion string) {
	if o, ok := oldStageData[ownerRepo]; ok {
		return o, o.Package.Name, o.Package.Version
	}
	_, repoName, ok := strings.Cut(ownerRepo, "/")
	if !ok || repoName == "" {
		return nil, "", ""
	}
	legacy := oldByRepoName[repoName]
	if legacy == nil {
		return nil, "", ""
	}
	legacyKey, ok := util.OwnerRepoFromStageURL(legacy.URL)
	if !ok || legacyKey == "" {
		return nil, "", ""
	}
	if _, stillListed := listed[legacyKey]; stillListed {
		// 旧路径仍在列表：不是换维护者，按纯新包处理（唯一性校验会拦住重名）
		return nil, "", ""
	}
	return nil, legacy.Package.Name, legacy.Package.Version
}

// indexPackage 下载、校验并上传包，返回的 pkg 为解析后的清单元数据。
// hash、packageZipAssetID 来自 Latest Release，由调用方在跳过判断后传入。
// oldName/oldVersion 供清单校验（含换维护者：视同更新，须 version 更高）。
// allowThemeJS 仅主题为 themes 时可能为 true（theme.js 白名单内仓库）；其他类型恒为 false。
// occupiedNames 为已占用 package.name 集合，供 rules.Check 做跨类型唯一性检查。
// 失败时 issues 供固定 Issue 评论汇总（与 PR Check 同一套 MessageZh/MessageEn）。
func indexPackage(
	ownerRepo string,
	packageType rules.PackageType,
	hash string,
	packageZipAssetID int64,
	oldName, oldVersion string,
	allowThemeJS bool,
	occupiedNames map[string]struct{},
) (ok bool, size, installSize int64, pkg *rules.Package, issues []rules.Issue) {
	repoURL := util.GitHubRepoURL(ownerRepo)
	owner, name, cutOk := strings.Cut(ownerRepo, "/")
	if !cutOk {
		// 列表入口已校验；此处仅兜底打日志，不汇总到 Issue。
		logger.Errorf("download/unzip [%s] failed: invalid owner/repo", ownerRepo)
		return
	}

	tmpUnzipPath, data, cleanup, err := util.DownloadAndUnzipPackageZip(githubContext, githubClient, owner, name, packageZipAssetID)
	if err != nil {
		logger.Errorf("download/unzip [%s] asset %d failed: %s", repoURL, packageZipAssetID, err)
		issues = stageIssueFromErr(err)
		return
	}
	defer cleanup()

	// 记录 zip 体积
	size = int64(len(data))

	result := rules.Check(rules.Input{
		PackageRoot:   tmpUnzipPath,
		OwnerRepo:     ownerRepo,
		Type:          packageType,
		ZipData:       data,
		OldName:       oldName,
		OldVersion:    oldVersion,
		OccupiedNames: occupiedNames,
		AllowThemeJS:  allowThemeJS,
	})
	if !result.OK {
		for _, issue := range result.Issues {
			logger.Errorf("check [%s] failed: %s", repoURL, issue.MessageEn)
		}
		issues = result.Issues
		return
	}

	packageRoot := result.PackageRoot

	// 计算解压后目录体积，用于 stage 条目的 installSize 字段
	installSize, err = sizeOfDirectory(packageRoot)
	if err != nil {
		logger.Errorf("stat package [%s] size failed: %s", repoURL, err)
		issues = stageInternalIssue(
			"校验通过后统计解压目录体积失败。这通常是集市 Stage 流程内部问题，请联系维护者。",
			"Failed to measure the unzipped package size after checks passed. This is usually an internal Stage issue — please contact a maintainer.",
		)
		return
	}

	// 从解压目录读取清单，以便根据 readme 字段收集要上传的文件
	pkg = getPackage(packageRoot, packageType)
	if pkg == nil {
		logger.Errorf("get package [%s] failed", repoURL)
		issues = stageInternalIssue(
			"校验通过后重新读取清单失败。这通常是集市 Stage 流程内部问题，请联系维护者。",
			"Failed to re-read the package manifest after checks passed. This is usually an internal Stage issue — please contact a maintainer.",
		)
		return
	}

	// 校验通过后再上传 package.zip，避免无效包写入 OSS
	key := "package/" + ownerRepo + "@" + hash
	if err := util.UploadOSS(githubContext, key, data); err != nil {
		logger.Errorf("upload package [%s] failed: %s", repoURL, err)
		issues = stageInternalIssue(
			"上传 `package.zip` 到对象存储失败。这通常是集市 Stage 流程或存储配置问题，请联系维护者。",
			"Failed to upload `package.zip` to object storage. This is usually a Stage pipeline or storage config issue — please contact a maintainer.",
		)
		return
	}

	// 收集需要上传到 OSS 的包根目录文件
	uploadFiles := Set{
		"README.md":                struct{}{}, // 始终加入，思源将其作为最后回退
		"preview.png":              struct{}{},
		"icon.png":                 struct{}{},
		packageType.ManifestFile(): struct{}{},
	}
	if pkg.Readme != nil {
		for _, readmePath := range pkg.Readme {
			readmePath = strings.TrimSpace(readmePath) // 跟思源内核逻辑一致，TrimSpace
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
		logger.Errorf("upload package [%s] root files failed: %s", repoURL, err)
		issues = stageInternalIssue(
			"上传包根目录文件到对象存储失败。这通常是集市 Stage 流程或存储配置问题，请联系维护者。",
			"Failed to upload package root files to object storage. This is usually a Stage pipeline or storage config issue — please contact a maintainer.",
		)
		return
	}
	ok = true
	return
}

// getRepoLatestRelease 获取仓库最新发布的版本；失败时 error 为双语 LocalizedError（可转 Issue）。
func getRepoLatestRelease(ownerRepo string) (util.LatestRelease, error) {
	repoURL := util.GitHubRepoURL(ownerRepo)
	owner, name, cutOk := strings.Cut(ownerRepo, "/")
	if !cutOk {
		// 列表入口已校验；此处仅兜底打日志，不汇总到 Issue。
		logger.Errorf("get [%s] latest release failed: invalid owner/repo", ownerRepo)
		return util.LatestRelease{}, errInvalidOwnerRepo
	}
	ctx, cancel := context.WithTimeout(githubContext, REQUEST_TIMEOUT)
	defer cancel()

	releaseInfo, err := util.FetchLatestRelease(ctx, githubClient, owner, name)
	if err != nil {
		logger.Errorf("get release [%s] failed: %s", repoURL, err)
		return releaseInfo, err
	}
	return releaseInfo, nil
}

// sameCommitPackageZipChanged 判断 Latest Release 仍指向同一 commit，但 package.zip 是否已被替换。
func sameCommitPackageZipChanged(old *util.StageRepo, assetID int64) bool {
	if old == nil || old.PackageZipAssetID == 0 || assetID == 0 {
		return false
	}
	return old.PackageZipAssetID != assetID
}

// parseHashFromStageURL 从 stage 条目的 URL（格式 owner/repo@hash）中解析出 hash 部分，若无 @ 或 @ 后为空则返回空字符串
func parseHashFromStageURL(stageURL string) string {
	_, hash, ok := strings.Cut(stageURL, "@")
	if !ok || hash == "" {
		return ""
	}
	return hash
}

// sizeOfDirectory 计算目录大小。
// 跟思源内核实现一致，参考 kernel\util\file.go
func sizeOfDirectory(path string) (size int64, err error) {
	err = filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		info, err := d.Info()
		if err != nil {
			logger.Errorf("size of dir [%s] failed: %s", path, err)
			return err
		}

		if !info.IsDir() {
			size += info.Size()
		} else {
			size += 4096
		}
		return nil
	})
	if err != nil {
		logger.Errorf("size of dir [%s] failed: %s", path, err)
	}
	return
}

// getPackage 从解压后的包根目录 unzipRoot 读取该类型的清单 JSON（如 plugin.json），解析为 Package。
func getPackage(unzipRoot string, packageType rules.PackageType) *rules.Package {
	jsonPath := filepath.Join(unzipRoot, packageType.ManifestFile())
	_, pkg, err := rules.ReadPackage(jsonPath)
	if err != nil {
		logger.Errorf("read package [%s] failed: %s", jsonPath, err)
		return nil
	}
	rules.SanitizePackage(pkg)
	return pkg
}

// uploadPackageRootFile 从包根目录 unzipRoot 读取 fileName 对应文件（大小写敏感）并上传到 OSS；文件不存在时仅记录并跳过。
func uploadPackageRootFile(ctx context.Context, ownerRepo, hash, unzipRoot, fileName string) error {
	repoURL := util.GitHubRepoURL(ownerRepo)
	localPath := filepath.Join(unzipRoot, fileName)
	data, err := os.ReadFile(localPath)
	if err != nil {
		// 可选文件（如部分 README、preview.png）可能不存在，仅记录并跳过，不导致整包失败
		if os.IsNotExist(err) {
			logger.Errorf("file not found in package [%s], skip upload [%s]", repoURL, fileName)
			return nil
		}
		logger.Errorf("read package [%s] file [%s] failed: %s", repoURL, fileName, err)
		return err
	}

	// 阻塞结束后检查是否已取消
	if err := ctx.Err(); err != nil {
		return err
	}

	key := "package/" + ownerRepo + "@" + hash + "/" + fileName
	if err := util.UploadOSS(ctx, key, data); err != nil {
		logger.Errorf("upload package [%s] file [%s] failed: %s", repoURL, fileName, err)
		return err
	}
	return nil
}
