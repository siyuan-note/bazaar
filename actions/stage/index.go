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
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/siyuan-note/bazaar/actions/util"
	"github.com/siyuan-note/bazaar/rules"
	"golang.org/x/sync/errgroup"
)

// indexPackage 下载、校验并上传包，返回的 pkg 为解析后的清单元数据。
// hash、packageZipAssetID 来自 Latest Release，由调用方在跳过判断后传入。
// oldStageRepo 用于清单校验时与旧 name/version 对比，可为 nil（如新仓库）。
// allowThemeJS 仅主题为 themes 时可能为 true（theme.js 白名单内仓库）；其他类型恒为 false。
// occupiedNames 为已占用 package.name 集合，供 rules.Check 做跨类型唯一性检查。
func indexPackage(
	ownerRepo string,
	packageType rules.PackageType,
	hash string,
	packageZipAssetID int64,
	oldStageRepo *util.StageRepo,
	allowThemeJS bool,
	occupiedNames map[string]struct{},
) (ok bool, size, installSize int64, pkg *rules.Package) {
	repoURL := util.GitHubRepoURL(ownerRepo)
	owner, name, cutOk := strings.Cut(ownerRepo, "/")
	if !cutOk {
		logger.Errorf("download/unzip [%s] failed: invalid owner/repo", ownerRepo)
		return
	}

	tmpUnzipPath, data, cleanup, err := util.DownloadAndUnzipPackageZip(githubContext, githubClient, owner, name, packageZipAssetID)
	if err != nil {
		logger.Errorf("download/unzip [%s] asset %d failed: %s", repoURL, packageZipAssetID, err)
		return
	}
	defer cleanup()

	// 记录 zip 体积
	size = int64(len(data))

	var oldName, oldVersion string
	if oldStageRepo != nil {
		oldName, oldVersion = oldStageRepo.Package.Name, oldStageRepo.Package.Version
	}
	result := rules.Check(rules.Input{
		PackageRoot:   tmpUnzipPath,
		OwnerRepo:     ownerRepo,
		Type:          packageType,
		OldName:       oldName,
		OldVersion:    oldVersion,
		OccupiedNames: occupiedNames,
		AllowThemeJS:  allowThemeJS,
	})
	if !result.OK {
		for _, issue := range result.Issues {
			logger.Errorf("check [%s] failed: %s", repoURL, issue.MessageEn)
		}
		return
	}

	packageRoot := result.PackageRoot

	// 计算解压后目录体积，用于 stage 条目的 installSize 字段
	installSize, err = sizeOfDirectory(packageRoot)
	if err != nil {
		logger.Errorf("stat package [%s] size failed: %s", repoURL, err)
		return
	}

	// 从解压目录读取清单，以便根据 readme 字段收集要上传的文件
	pkg = getPackage(packageRoot, packageType)
	if pkg == nil {
		logger.Errorf("get package [%s] failed", repoURL)
		return
	}

	// 校验通过后再上传 package.zip，避免无效包写入 OSS
	key := "package/" + ownerRepo + "@" + hash
	if err := util.UploadOSS(githubContext, key, data); err != nil {
		logger.Errorf("upload package [%s] failed: %s", repoURL, err)
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
		return
	}
	ok = true
	return
}

// getRepoLatestRelease 获取仓库最新发布的版本
func getRepoLatestRelease(ownerRepo string) (util.LatestRelease, bool) {
	repoURL := util.GitHubRepoURL(ownerRepo)
	owner, name, cutOk := strings.Cut(ownerRepo, "/")
	if !cutOk {
		logger.Errorf("get [%s] latest release failed: invalid owner/repo", ownerRepo)
		return util.LatestRelease{}, false
	}
	ctx, cancel := context.WithTimeout(githubContext, REQUEST_TIMEOUT)
	defer cancel()

	releaseInfo, err := util.FetchLatestRelease(ctx, githubClient, owner, name)
	if err != nil {
		logger.Errorf("get release [%s] failed: %s", repoURL, err)
		return util.LatestRelease{}, false
	}
	return releaseInfo, true
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
