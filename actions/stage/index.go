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
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/88250/gulu"
	"github.com/siyuan-note/bazaar/actions/util"
	"github.com/siyuan-note/bazaar/rules"
)

// parseHashFromStageURL 从 stage 条目的 URL（格式 owner/repo@hash）中解析出 hash 部分，若无 @ 或 @ 后为空则返回空字符串
func parseHashFromStageURL(stageURL string) string {
	idx := strings.Index(stageURL, "@")
	if idx < 0 || idx >= len(stageURL)-1 {
		return ""
	}
	return stageURL[idx+1:]
}

// getOldPackageFields 从已 stage 的条目中解析出 package 的 name、version
func getOldPackageFields(oldRepo *util.StageRepo) (name, version string) {
	if oldRepo == nil {
		return "", ""
	}
	return oldRepo.Package.Name, oldRepo.Package.Version
}

// indexPackage 索引包，返回的 pkg 为解析后的清单元数据。
// oldStageURL 为当前已 stage 的该仓库 URL（格式 owner/repo@hash），若与 Latest Release 的 hash 一致则跳过下载并返回 skipped=true。
// oldStageRepo 用于清单校验时与旧 name/version 对比，可为 nil（如新仓库）。
// allowThemeJS 仅主题为 themes 时可能为 true（theme.js 白名单内仓库）；其他类型恒为 false。
// occupiedNames 为已占用 package.name 集合，供 rules.Check 做跨类型唯一性检查。
// heavySem 限制同时进行「下载 + 上传 OSS」的仓库数。
func indexPackage(
	ownerRepo string,
	packageType rules.PackageType,
	oldStageURL string,
	oldStageRepo *util.StageRepo,
	allowThemeJS bool,
	occupiedNames map[string]struct{},
	heavySem chan struct{},
) (ok, skipped bool, hash, published string, size, installSize int64, pkg *rules.Package) {
	releaseInfo, releaseOk := getRepoLatestRelease(ownerRepo)
	if !releaseOk {
		logger.Errorf("get [%s] latest release failed", ownerRepo)
		return
	}
	hash = releaseInfo.CommitSHA
	published = releaseInfo.Published
	packageZipAssetID := releaseInfo.PackageZipAssetID

	// Latest Release 的 hash 与已 stage 的 hash 一致则跳过，不下载、不更新，沿用旧条目
	if oldStageURL != "" {
		oldHash := parseHashFromStageURL(oldStageURL)
		if oldHash != "" && hash == oldHash {
			logger.Infof("skip repo [%s], hash unchanged [%s]", ownerRepo, hash)
			skipped = true
			return
		}
	}

	owner, name, cutOk := strings.Cut(ownerRepo, "/")
	if !cutOk {
		logger.Errorf("download/unzip [%s] failed: invalid owner/repo", ownerRepo)
		return
	}

	heavySem <- struct{}{}
	defer func() { <-heavySem }()

	tmpUnzipPath, data, cleanup, err := util.DownloadAndUnzipPackageZip(githubContext, githubClient, owner, name, packageZipAssetID)
	if err != nil {
		logger.Errorf("download/unzip [%s] asset %d failed: %s", ownerRepo, packageZipAssetID, err)
		return
	}
	defer cleanup()

	// 记录 zip 体积，用于 stage 条目的 size 字段
	size = int64(len(data))
	installSize = size

	oldName, oldVersion := getOldPackageFields(oldStageRepo)
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
			logger.Errorf("check [%s] failed: %s", ownerRepo, issue.MessageEn)
		}
		return
	}

	packageRoot := result.PackageRoot
	if packageRoot == "" {
		packageRoot = tmpUnzipPath
	}

	// 计算解压后目录体积，用于 stage 条目的 installSize 字段
	installSize, err = util.SizeOfDirectory(packageRoot)
	if nil != err {
		logger.Errorf("stat package [%s] size failed: %s", ownerRepo, err)
		return
	}

	// 从解压目录读取清单，以便根据 readme 字段收集要上传的文件
	pkg = getPackage(packageRoot, packageType)
	if nil == pkg {
		logger.Errorf("get package [%s] failed", ownerRepo)
		return
	}

	// 校验通过后再上传 package.zip，避免无效包写入 OSS
	key := "package/" + ownerRepo + "@" + hash
	if err := util.UploadOSS(key, "application/zip", data); nil != err {
		logger.Errorf("upload package [%s] failed: %s", ownerRepo, err)
		return
	}

	// 收集需要上传的 README 文件列表（根据包配置中的 readme 字段）
	readmeFiles := make(map[string]bool)
	if nil != pkg.Readme {
		for _, readmePath := range pkg.Readme {
			readmePath = strings.TrimSpace(readmePath) // 跟思源内核逻辑一致，TrimSpace
			if readmePath == "" {
				continue
			}
			readmeFiles["/"+readmePath] = true
		}
	}
	// 仅 README.md 始终加入上传列表（若包内存在则上传）
	readmeFiles["/README.md"] = true

	// 从解压目录读取 README、preview、icon、清单 JSON 并并发上传到 OSS；任一份上传失败则整包视为失败
	var anyUploadFailed int32
	wg := &sync.WaitGroup{}
	wg.Add(3 + len(readmeFiles))
	for readmeFile := range readmeFiles {
		go indexPackageFile(ownerRepo, hash, packageRoot, readmeFile, 0, 0, wg, &anyUploadFailed)
	}
	go indexPackageFile(ownerRepo, hash, packageRoot, "/preview.png", 0, 0, wg, &anyUploadFailed)
	go indexPackageFile(ownerRepo, hash, packageRoot, "/icon.png", 0, 0, wg, &anyUploadFailed)
	go indexPackageFile(ownerRepo, hash, packageRoot, "/"+packageType.ManifestFile(), size, installSize, wg, &anyUploadFailed)
	wg.Wait()
	if atomic.LoadInt32(&anyUploadFailed) != 0 {
		return
	}
	ok = true
	return
}

// getPackage 从解压后的包根目录 unzipRoot 读取该类型的清单 JSON（如 plugin.json），解析为 Package。
func getPackage(unzipRoot string, packageType rules.PackageType) *rules.Package {
	jsonPath := filepath.Join(unzipRoot, packageType.ManifestFile())
	pkg, err := rules.ReadPackage(jsonPath)
	if err != nil {
		logger.Errorf("read package [%s] failed: %s", jsonPath, err)
		return nil
	}
	rules.SanitizePackage(pkg)
	return pkg
}

// indexPackageFile 从解压后的包根目录 unzipRoot 读取 filePath 对应文件（大小写敏感），上传到 OSS；可选文件不存在时仅记录并跳过，其它失败时设置 anyFail。filePath 为相对包根的路径，如 /README.md、/icon.png。
func indexPackageFile(ownerRepo, hash, unzipRoot, filePath string, size, installSize int64, wg *sync.WaitGroup, anyFail *int32) {
	defer wg.Done()
	relPath := strings.TrimPrefix(filepath.ToSlash(filePath), "/")
	localPath := filepath.Join(unzipRoot, filepath.FromSlash(relPath))
	data, err := os.ReadFile(localPath)
	if err != nil {
		// 可选文件（如部分 README、preview.png）可能不存在，仅记录并跳过，不导致整包失败
		if os.IsNotExist(err) {
			logger.Errorf("file not found in package, skip upload [%s]", relPath)
			return
		}
		logger.Errorf("read [%s] failed: %s", localPath, err)
		atomic.StoreInt32(anyFail, 1)
		return
	}

	// 规范化为 /path 形式用于 OSS key
	normPath := "/" + filepath.ToSlash(relPath)

	var contentType string
	if strings.HasSuffix(normPath, ".md") {
		contentType = "text/markdown"
	} else if strings.HasSuffix(normPath, ".png") {
		contentType = "image/png"
	} else if strings.HasSuffix(normPath, ".json") {
		contentType = "application/json"
		// 注入 size/installSize 到清单 JSON
		meta := map[string]any{}
		if err := gulu.JSON.UnmarshalJSON(data, &meta); err != nil {
			logger.Errorf("unmarshal manifest [%s] failed: %s", localPath, err)
			atomic.StoreInt32(anyFail, 1)
			return
		}
		meta["size"] = size
		meta["installSize"] = installSize
		data, err = gulu.JSON.MarshalIndentJSON(meta, "", "  ")
		if err != nil {
			logger.Errorf("marshal package [%s] meta json failed: %s", localPath, err)
			atomic.StoreInt32(anyFail, 1)
			return
		}
	}

	key := "package/" + ownerRepo + "@" + hash + normPath
	if err := util.UploadOSS(key, contentType, data); err != nil {
		logger.Errorf("upload package file [%s] failed: %s", key, err)
		atomic.StoreInt32(anyFail, 1)
		return
	}
}

func repoStats(ownerRepo string) (stars, openIssues int, ok bool) {
	owner, name, cutOk := strings.Cut(ownerRepo, "/")
	if !cutOk {
		logger.Errorf("get [%s] failed: invalid owner/repo", ownerRepo)
		return
	}
	ctx, cancel := context.WithTimeout(githubContext, REQUEST_TIMEOUT)
	defer cancel()
	repo, _, err := githubClient.Repositories.Get(ctx, owner, name)
	if err != nil {
		logger.Errorf("get [%s] failed: %s", ownerRepo, err)
		return
	}
	stars = repo.GetStargazersCount()
	openIssues = repo.GetOpenIssuesCount()
	ok = true
	return
}

// getRepoLatestRelease 获取仓库最新发布的版本
func getRepoLatestRelease(ownerRepo string) (util.LatestRelease, bool) {
	owner, name, cutOk := strings.Cut(ownerRepo, "/")
	if !cutOk {
		logger.Errorf("get [%s] latest release failed: invalid owner/repo", ownerRepo)
		return util.LatestRelease{}, false
	}
	ctx, cancel := context.WithTimeout(githubContext, REQUEST_TIMEOUT)
	defer cancel()

	releaseInfo, err := util.FetchLatestRelease(ctx, githubClient, owner, name)
	if err != nil {
		logger.Errorf("get release [%s] failed: %s", ownerRepo, err)
		return util.LatestRelease{}, false
	}
	return releaseInfo, true
}
