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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/88250/gulu"
	"github.com/google/go-github/v52/github"
	"github.com/panjf2000/ants/v2"
	"github.com/siyuan-note/bazaar/actions/util"
	"github.com/siyuan-note/bazaar/check"
)

/*
Diff 流程（以 plugins.txt 为例）：
1. 签出 bazaar head（主分支最新）：读 plugins.txt，得到 bazaar head 的 owner/repo 集合，用于过滤
2. 签出 PR head：读 plugins.txt（PR 当前状态）
3. 签出 PR base（merge base）：读 plugins.txt（与 GitHub "Files changed" 的基准一致）
4. 比较 base 与 head：候选新增 = head 有 base 无，候选删除 = base 有 head 无
5. 过滤候选新增：排除已在 bazaar head 中的仓库（可能是解决冲突时从 bazaar head 合并来的）
6. 过滤候选删除：排除在 bazaar head 中已不存在的仓库（可能是其他 PR 删除的）
7. OccupiedNames 使用 bazaar head 的 stage/*.json 中所有类型的 package.name 集合（跨类型；比较前统一转小写）
8. 对最终新增列表：Latest Release / package.zip → 下载解压 → check.Check(ModePR)，并在 Bot 回复中列出删除列表、更换维护者列表

Check 流程：
1. 获取仓库最新 release 与 package.zip
2. 下载并解压 package.zip
3. 调用 check.Check（ModePR；OccupiedNames / AllowThemeJS；PR 新仓 OldName/OldVersion 为空）
4. 通过后将 name 写入 OccupiedNames（同 PR 内唯一性）
5. 生成检查结果并输出文件（使用 go 模板）
6. 使用 thollander/actions-comment-pull-request 将检查结果输出到 PR 中
*/

var (
	BAZAAR_HEAD_PATH    = os.Getenv("BAZAAR_HEAD_PATH")    // bazaar 主分支最新代码目录（用于过滤与 OccupiedNames）
	PR_HEAD_PATH        = os.Getenv("PR_HEAD_PATH")        // 本 PR 当前提交的代码目录（PR head）
	PR_BASE_PATH        = os.Getenv("PR_BASE_PATH")        // 本 PR 的 merge base 代码目录（做 diff 的旧侧，与 GitHub "Files changed" 一致）
	GITHUB_TOKEN        = os.Getenv("PAT")                 // GitHub Token
	CHECK_RESULT_OUTPUT = os.Getenv("CHECK_RESULT_OUTPUT") // 检查结果输出文件路径

	REQUEST_TIMEOUT = 30 * time.Second // 请求超时时间

	logger        = gulu.Log.NewLogger(os.Stdout)
	githubContext = context.Background()
	githubClient  = github.NewTokenClient(githubContext, GITHUB_TOKEN)
)

func main() {
	logger.Infof("PR Check running...")

	// 获取检查结果模板文件（含 issueIndex：Issues 序号 %02d）
	checkResultTemplate, err := template.New("check-result.md.tpl").Funcs(template.FuncMap{
		"issueIndex": func(i int) string {
			return fmt.Sprintf("%02d", i+1)
		},
	}).ParseFiles(FILE_PATH_CHECK_RESULT_TEMPLATE)
	if nil != err {
		logger.Fatalf("load check result template file [%s] failed: %s", FILE_PATH_CHECK_RESULT_TEMPLATE, err)
		panic(err)
	}

	// 打开检查结果输出文件
	checkResultOutputFile, err := os.OpenFile(CHECK_RESULT_OUTPUT, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if nil != err {
		logger.Fatalf("open check result output file [%s] failed: %s", CHECK_RESULT_OUTPUT, err)
		panic(err)
	}
	defer checkResultOutputFile.Close()

	checkResult := &CheckResult{} // 检查结果

	githubClient.Client().Timeout = REQUEST_TIMEOUT // 设置请求超时时间

	// 加载 stage 全量已占用 name，供 check.Check 做跨类型唯一性检查
	occupiedNames, err := util.LoadOccupiedNames(BAZAAR_HEAD_PATH)
	if err != nil {
		logger.Fatalf("load occupied names failed: %s", err)
		panic(err)
	}

	var parseErrorMu sync.Mutex
	var occupiedNamesMu sync.Mutex // 跨类型共享，保证同 PR 内 OccupiedNames 唯一性
	wg := &sync.WaitGroup{}
	wg.Add(5)

	go checkRepos(icons, checkResult, occupiedNames, &occupiedNamesMu, &parseErrorMu, wg)
	go checkRepos(plugins, checkResult, occupiedNames, &occupiedNamesMu, &parseErrorMu, wg)
	go checkRepos(templates, checkResult, occupiedNames, &occupiedNamesMu, &parseErrorMu, wg)
	go checkRepos(themes, checkResult, occupiedNames, &occupiedNamesMu, &parseErrorMu, wg)
	go checkRepos(widgets, checkResult, occupiedNames, &occupiedNamesMu, &parseErrorMu, wg)

	wg.Wait() // 等待所有检查完成

	// 将检查结果写入文件
	checkResultTemplate.Execute(checkResultOutputFile, checkResult)

	logger.Infof("PR Check finished")
}

// parseReposFromRootTxt 从集市包列表 TXT（每行一个 owner/repo）解析出路径列表、路径集合和 name->owner 映射
func parseReposFromRootTxt(filePath string) (paths []string, pathSet StringSet, nameToOwner map[string]string, err error) {
	repos, err := util.ParseReposFromTxt(filePath)
	if err != nil {
		return
	}
	paths = make([]string, 0, len(repos))
	pathSet = make(StringSet, len(repos))
	nameToOwner = make(map[string]string, len(repos))
	for _, s := range repos {
		parts := strings.Split(s, "/")
		if len(parts) != 2 {
			err = fmt.Errorf("invalid repo path: %s", s)
			return
		}
		owner := parts[0]
		name := parts[1]
		paths = append(paths, s)
		pathSet[s] = nil
		nameToOwner[name] = owner
	}
	return
}

// checkRepos 检查集市资源仓库列表
func checkRepos(
	resourceType ResourceType,
	checkResult *CheckResult,
	occupiedNames map[string]struct{},
	occupiedNamesMu *sync.Mutex,
	parseErrorMu *sync.Mutex,
	waitGroup *sync.WaitGroup,
) {
	defer waitGroup.Done()

	var repoListTxtName string
	switch resourceType {
	case icons:
		repoListTxtName = "icons.txt"
	case plugins:
		repoListTxtName = "plugins.txt"
	case templates:
		repoListTxtName = "templates.txt"
	case themes:
		repoListTxtName = "themes.txt"
	case widgets:
		repoListTxtName = "widgets.txt"
	default:
		panic("checkRepos: invalid resource type")
	}
	bazaarHeadReposPath := filepath.Join(BAZAAR_HEAD_PATH, repoListTxtName)
	prBaseReposPath := filepath.Join(PR_BASE_PATH, repoListTxtName)
	prHeadReposPath := filepath.Join(PR_HEAD_PATH, repoListTxtName)
	logger.Infof("start repos check [%s]", prHeadReposPath)

	// 加载三个版本的集市包列表：bazaar head（用于过滤）、PR base、PR head（用于 diff）
	_, bazaarHeadRepoSet, _, err := parseReposFromRootTxt(bazaarHeadReposPath)
	if err != nil {
		parseErrorMu.Lock()
		checkResult.ParseError += fmt.Sprintf("[%s] bazaar head: %s\n", repoListTxtName, err)
		parseErrorMu.Unlock()
		logger.Warnf("load bazaar head repos [%s] failed: %s, skip this type", bazaarHeadReposPath, err)
		return
	}
	basePaths, baseSet, baseNameToOwner, err := parseReposFromRootTxt(prBaseReposPath)
	if err != nil {
		parseErrorMu.Lock()
		checkResult.ParseError += fmt.Sprintf("[%s] PR base: %s\n", repoListTxtName, err)
		parseErrorMu.Unlock()
		logger.Warnf("load PR base repos [%s] failed: %s, skip this type", prBaseReposPath, err)
		return
	}
	headPaths, headSet, _, err := parseReposFromRootTxt(prHeadReposPath)
	if err != nil {
		parseErrorMu.Lock()
		checkResult.ParseError += fmt.Sprintf("[%s] PR head: %s\n", repoListTxtName, err)
		parseErrorMu.Unlock()
		logger.Warnf("load PR head repos [%s] failed: %s, skip this type", prHeadReposPath, err)
		return
	}

	var themeJsAllowSet map[string]struct{}
	if resourceType == themes {
		ap := filepath.Join(PR_HEAD_PATH, util.ThemeJsAllowlistRelPath)
		paths, errAllow := util.ParseReposFromTxt(ap)
		if errAllow != nil {
			parseErrorMu.Lock()
			checkResult.ParseError += fmt.Sprintf("[%s] PR head: %v\n", util.ThemeJsAllowlistRelPath, errAllow)
			parseErrorMu.Unlock()
			logger.Warnf("load theme.js allowlist [%s] failed: %v, skip this type", ap, errAllow)
			return
		}
		themeJsAllowSet = make(map[string]struct{}, len(paths))
		for _, p := range paths {
			themeJsAllowSet[p] = struct{}{}
		}
	}

	// 按 base/head diff 并过滤：新增 = head 有而 base 无且不在 bazaar head（多为解决冲突时从 bazaar head 合并来的）；删除 = base 有而 head 无且仍在 bazaar head（确属本 PR 删除）
	newRepos := make([]string, 0)
	for _, path := range headPaths {
		if !isKeyInSet(path, baseSet) && !isKeyInSet(path, bazaarHeadRepoSet) {
			newRepos = append(newRepos, path)
		}
	}
	deletedRepos := make([]string, 0)
	for _, path := range basePaths {
		if !isKeyInSet(path, headSet) && isKeyInSet(path, bazaarHeadRepoSet) {
			deletedRepos = append(deletedRepos, path)
		}
	}

	// 将本 PR 的删除列表写入检查结果，供模板输出
	switch resourceType {
	case icons:
		checkResult.IconsDeleted = deletedRepos
	case plugins:
		checkResult.PluginsDeleted = deletedRepos
	case templates:
		checkResult.TemplatesDeleted = deletedRepos
	case themes:
		checkResult.ThemesDeleted = deletedRepos
	case widgets:
		checkResult.WidgetsDeleted = deletedRepos
	default:
		panic("checkRepos: invalid resource type")
	}

	// 更换维护者：在 PR base 与 PR head 中，repo name 相同但 owner 不同，则视为更换维护者
	maintainerChanged := make([]string, 0)
	for _, path := range newRepos { // newRepos 包含了更换维护者的仓库
		parts := strings.Split(path, "/")
		if len(parts) != 2 {
			continue
		}
		newOwner := parts[0]
		name := parts[1]
		oldOwner, oldExists := baseNameToOwner[name]
		if !oldExists {
			continue // base 中不存在该 name，是新增，不是更换维护者
		}
		if oldOwner != newOwner {
			// base 中有 oldOwner/name，head 中有 newOwner/name，是更换维护者
			maintainerChanged = append(maintainerChanged, path)
		}
	}

	// 新增与更换维护者合并为待检查列表，统一做 Release + Pkg Check（更换维护者按新集市包处理）
	maintainerChangedSet := make(StringSet, len(maintainerChanged))
	for _, path := range maintainerChanged {
		maintainerChangedSet[path] = nil
	}
	resultChannel := make(chan checkOutput, 4)
	waitGroupCheck := &sync.WaitGroup{}
	waitGroupResult := &sync.WaitGroup{}

	waitGroupResult.Add(1)
	// 收集检查结果时，根据是否在 maintainerChangedSet 中打上 MaintainerChanged 标记，供模板区分展示
	go func() {
		for result := range resultChannel {
			pkg := result.pkg
			if isKeyInSet(pkg.RepoInfo.Path, maintainerChangedSet) {
				pkg.MaintainerChanged = true
			}
			switch result.resourceType {
			case icons:
				checkResult.Icons = append(checkResult.Icons, pkg)
			case plugins:
				checkResult.Plugins = append(checkResult.Plugins, pkg)
			case templates:
				checkResult.Templates = append(checkResult.Templates, pkg)
			case themes:
				checkResult.Themes = append(checkResult.Themes, pkg)
			case widgets:
				checkResult.Widgets = append(checkResult.Widgets, pkg)
			default:
			}
		}
		waitGroupResult.Done()
	}()

	// 检查新增的集市包（包含更换维护者的集市包），限制并发数为 8
	p, _ := ants.NewPoolWithFunc(8, func(arg interface{}) {
		defer waitGroupCheck.Done()
		ownerRepo := arg.(string)
		checkRepo(ownerRepo, occupiedNames, resourceType, resultChannel, occupiedNamesMu, themeJsAllowSet)
	})
	defer p.Release()

	for _, ownerRepo := range newRepos {
		waitGroupCheck.Add(1)
		p.Invoke(ownerRepo)
	}
	waitGroupCheck.Wait()  // 等待检查完成
	close(resultChannel)   // 关闭检查结果输出通道
	waitGroupResult.Wait() // 等待检查结果处理完成
	logger.Infof("finish repos check [%s]", prHeadReposPath)
}

// checkRepo 检查集市资源仓库：Latest Release / package.zip → 下载解压 → check.Check(ModePR)
func checkRepo(
	ownerRepo string,
	occupiedNames map[string]struct{},
	resourceType ResourceType,
	resultChannel chan checkOutput,
	nameSetMutex *sync.Mutex,
	themeJsAllowSet map[string]struct{},
) {

	logger.Infof("start repo check [%s]", ownerRepo)

	repoMeta := strings.Split(ownerRepo, "/")
	repoOwner := repoMeta[0]
	repoName := repoMeta[1]
	repoInfo := RepoInfo{
		Path: ownerRepo,
		Home: buildRepoHomeURL(repoOwner, repoName),
	}
	releaseCheckResult := checkRepoLatestRelease(repoOwner, repoName)

	out := PackageCheck{
		RepoInfo: repoInfo,
		Release:  *releaseCheckResult,
		Issues:   releaseIssues(releaseCheckResult),
	}

	// Release / package.zip 未通过时只报告流程层 Issue，不调用 Pkg Check
	if len(out.Issues) > 0 {
		resultChannel <- checkOutput{resourceType: resourceType, pkg: out}
		logger.Infof("finish repo check [%s] (release issues)", ownerRepo)
		return
	}

	tmpUnzipPath, _, cleanup, err := util.DownloadAndUnzipPackageZip(releaseCheckResult.LatestRelease.PackageZip.URL)
	if err != nil {
		logger.Warnf("download/unzip [%s] failed: %s", ownerRepo, err)
		out.Issues = append(out.Issues, check.Issue{
			Rule:      "release/package_zip",
			MessageZh: fmt.Sprintf("下载或解压 package.zip 失败：%s。请确认 Latest Release 中的 package.zip 可访问且为合法 zip，然后重新发布或重跑 PR Check。", err),
			MessageEn: fmt.Sprintf("Failed to download or unzip package.zip: %s. Ensure package.zip in the Latest Release is reachable and a valid zip, then republish or re-run PR Check.", err),
		})
		resultChannel <- checkOutput{resourceType: resourceType, pkg: out}
		logger.Infof("finish repo check [%s] (download failed)", ownerRepo)
		return
	}
	defer cleanup()

	pkgType, typeOk := resourceTypeToPackageType(resourceType)
	if !typeOk {
		logger.Errorf("repo [%s] invalid resourceType: %d", ownerRepo, resourceType)
		resultChannel <- checkOutput{resourceType: resourceType, pkg: out}
		return
	}

	_, allowThemeJS := themeJsAllowSet[ownerRepo]

	// 持锁调用 Check 并在通过后登记 name，保证同 PR 内 OccupiedNames 唯一性
	nameSetMutex.Lock()
	result := check.Check(check.Input{
		PackageRoot:   tmpUnzipPath,
		OwnerRepo:     ownerRepo,
		Type:          pkgType,
		Mode:          check.ModePR,
		OldName:       "", // PR 新仓按首发处理
		OldVersion:    "",
		OccupiedNames: occupiedNames,
		AllowThemeJS:  allowThemeJS,
	})
	if result.OK {
		if name, ok := result.Manifest["name"].(string); ok && name != "" {
			occupiedNames[strings.ToLower(name)] = struct{}{}
		}
	}
	nameSetMutex.Unlock()

	out.Issues = append(out.Issues, result.Issues...)
	resultChannel <- checkOutput{resourceType: resourceType, pkg: out}
	logger.Infof("finish repo check [%s]", ownerRepo)
}

// checkRepoLatestRelease 检查最新发行信息
func checkRepoLatestRelease(
	repoOwner string,
	repoName string,
) (releaseCheckResult *Release) {
	releaseCheckResult = &Release{}

	// 获取 latest release
	githubRelease, _, err := githubClient.Repositories.GetLatestRelease(githubContext, repoOwner, repoName)
	if nil != err {
		logger.Warnf("get repo [%s/%s] latest release failed: %s", repoOwner, repoName, err)
		return
	}

	releaseCheckResult.LatestRelease.Pass = true // 最新发行版存在

	// 获取 tag 名称
	releaseCheckResult.LatestRelease.Tag = githubRelease.GetTagName()
	releaseCheckResult.LatestRelease.URL = githubRelease.GetHTMLURL()

	// 获取 package.zip 下载地址
	for _, asset := range githubRelease.Assets {
		if asset.GetName() == "package.zip" {
			releaseCheckResult.LatestRelease.PackageZip.Pass = true
			releaseCheckResult.LatestRelease.PackageZip.URL = asset.GetBrowserDownloadURL()
			break
		}
	}

	// 获取 tag
	// REF https://pkg.go.dev/github.com/google/go-github/v52/github#GitService.GetRef
	githubReference, _, err := githubClient.Git.GetRef(githubContext, repoOwner, repoName, "tags/"+releaseCheckResult.LatestRelease.Tag)
	if nil != err {
		logger.Warnf("get repo [%s/%s] reference tag [%s] failed: %s", repoOwner, repoName, releaseCheckResult.LatestRelease.Tag, err)
		return
	}

	referenceType := githubReference.GetObject().GetType()

	switch referenceType {
	case "commit":
		if githubReference.GetObject().GetSHA() == "" {
			logger.Warnf("parse repo [%s/%s] reference tag [%s] failed: empty commit sha", repoOwner, repoName, releaseCheckResult.LatestRelease.Tag)
			return
		}
	case "tag":
		tagSha := githubReference.GetObject().GetSHA()

		// 获取 commit hash
		// REF https://pkg.go.dev/github.com/google/go-github/v52/github#GitService.GetTag
		githubTag, _, err := githubClient.Git.GetTag(githubContext, repoOwner, repoName, tagSha)
		if nil != err {
			logger.Warnf("get repo [%s/%s] tag [%s:%s] failed: %s", repoOwner, repoName, releaseCheckResult.LatestRelease.Tag, tagSha, err)
			return
		}
		if githubTag.GetObject().GetSHA() == "" {
			logger.Warnf("parse repo [%s/%s] tag [%s:%s] failed: empty commit sha", repoOwner, repoName, releaseCheckResult.LatestRelease.Tag, tagSha)
			return
		}
	default:
		logger.Warnf("parse repo [%s/%s] reference tag [%s] failed: unknown type [%s]", repoOwner, repoName, releaseCheckResult.LatestRelease.Tag, referenceType)
		return
	}

	releaseCheckResult.Pass = releaseCheckResult.LatestRelease.Pass &&
		releaseCheckResult.LatestRelease.PackageZip.Pass // 通过发行版检查

	return
}
