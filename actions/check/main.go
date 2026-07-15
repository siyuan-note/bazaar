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
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/88250/gulu"
	"github.com/google/go-github/v89/github"
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
8. 对最终新增列表：Latest Release / package.zip → 下载解压 → check.Check，并在 Bot 回复中列出删除列表、更换维护者列表

Check 流程：
1. 获取仓库最新 release 与 package.zip
2. 下载并解压 package.zip
3. 调用 check.Check（OccupiedNames / AllowThemeJS；PR 新仓 OldName/OldVersion 为空）
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
	githubClient  *github.Client
)

//go:embed check-result.md.tpl
var checkResultTemplateText string

func formatIssueIndex(i, total int) string {
	if total < 1 {
		total = 1
	}
	// 前导零，按 issue 总数决定序号位数，至少两位
	width := max(len(strconv.Itoa(total)), 2)
	return fmt.Sprintf("%0*d", width, i+1)
}

func parseCheckResultTemplate() (*template.Template, error) {
	return template.New("check-result.md.tpl").Funcs(template.FuncMap{
		"issueIndex": formatIssueIndex,
	}).Parse(checkResultTemplateText)
}

func main() {
	logger.Infof("PR Check started")

	var err error
	githubClient, err = util.NewGitHubClient(GITHUB_TOKEN, REQUEST_TIMEOUT)
	if err != nil {
		logger.Fatalf("create github client failed: %s", err)
	}

	checkResultTemplate, err := parseCheckResultTemplate()
	if nil != err {
		logger.Fatalf("load check result template failed: %s", err)
	}

	checkResultOutputFile, err := os.OpenFile(CHECK_RESULT_OUTPUT, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if nil != err {
		logger.Fatalf("open check result output file [%s] failed: %s", CHECK_RESULT_OUTPUT, err)
	}
	defer checkResultOutputFile.Close()

	checkResult := &CheckResult{}

	occupiedNames, err := util.LoadOccupiedNames(BAZAAR_HEAD_PATH)
	if err != nil {
		logger.Fatalf("load occupied names failed: %s", err)
	}

	var occupiedNamesMu sync.Mutex // 跨类型共享，保证同 PR 内 OccupiedNames 唯一性
	var parseErrorMu sync.Mutex
	checkTypes := check.AllPackageTypes()
	wg := &sync.WaitGroup{}
	wg.Add(len(checkTypes))

	for _, packageType := range checkTypes {
		go checkRepos(packageType, checkResult, occupiedNames, &occupiedNamesMu, &parseErrorMu, wg)
	}

	wg.Wait()

	// 将检查结果写入文件
	checkResultTemplate.Execute(checkResultOutputFile, checkResult)

	logger.Infof("PR Check finished")
}

// parseReposFromRootTxt 从集市包列表 TXT（每行一个 owner/repo）解析出路径列表、路径集合和 name->owner 映射
func parseReposFromRootTxt(filePath string) (paths []string, pathSet Set, nameToOwner map[string]string, err error) {
	repos, err := util.ParseReposFromTxt(filePath)
	if err != nil {
		return
	}
	paths = make([]string, 0, len(repos))
	pathSet = make(Set, len(repos))
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
		pathSet[s] = struct{}{}
		nameToOwner[name] = owner
	}
	return
}

// checkRepos 检查集市资源仓库列表
func checkRepos(
	packageType check.PackageType,
	checkResult *CheckResult,
	occupiedNames map[string]struct{},
	occupiedNamesMu *sync.Mutex,
	parseErrorMu *sync.Mutex,
	waitGroup *sync.WaitGroup,
) {
	defer waitGroup.Done()

	repoListTxtName := packageType.ReposListFile()
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

	var themeJsAllowSet Set
	if packageType == check.TypeTheme {
		ap := filepath.Join(PR_HEAD_PATH, util.ThemeJsAllowlistRelPath)
		paths, errAllow := util.ParseReposFromTxt(ap)
		if errAllow != nil {
			parseErrorMu.Lock()
			checkResult.ParseError += fmt.Sprintf("[%s] PR head: %v\n", util.ThemeJsAllowlistRelPath, errAllow)
			parseErrorMu.Unlock()
			logger.Warnf("load theme.js allowlist [%s] failed: %v, skip this type", ap, errAllow)
			return
		}
		themeJsAllowSet = make(Set, len(paths))
		for _, p := range paths {
			themeJsAllowSet[p] = struct{}{}
		}
	}

	// 按 base/head diff 并过滤：新增 = head 有而 base 无且不在 bazaar head（多为解决冲突时从 bazaar head 合并来的）；删除 = base 有而 head 无且仍在 bazaar head（确属本 PR 删除）
	newRepos := make([]string, 0)
	for _, path := range headPaths {
		_, inBase := baseSet[path]
		_, inBazaarHead := bazaarHeadRepoSet[path]
		if !inBase && !inBazaarHead {
			newRepos = append(newRepos, path)
		}
	}
	deletedRepos := make([]string, 0)
	for _, path := range basePaths {
		_, inHead := headSet[path]
		_, inBazaarHead := bazaarHeadRepoSet[path]
		if !inHead && inBazaarHead {
			deletedRepos = append(deletedRepos, path)
		}
	}

	// 将本 PR 的删除列表写入检查结果
	if !checkResult.setDeleted(packageType, deletedRepos) {
		panic("checkRepos: invalid package type")
	}

	// 更换维护者：在 PR base 与 PR head 中，repo name 相同但 owner 不同，则视为更换维护者
	maintainerChangedSet := make(Set)
	for _, path := range newRepos {
		// newRepos 包含了更换维护者的仓库
		parts := strings.Split(path, "/")
		if len(parts) != 2 {
			continue
		}
		newOwner := parts[0]
		name := parts[1]
		oldOwner, oldExists := baseNameToOwner[name]
		if !oldExists {
			// base 中不存在该 name，是新增，不是更换维护者
			continue
		}
		if oldOwner != newOwner {
			// base 中有 oldOwner/name，head 中有 newOwner/name，是更换维护者
			maintainerChangedSet[path] = struct{}{}
		}
	}

	resultChannel := make(chan checkOutput, 4)
	waitGroupCheck := &sync.WaitGroup{}
	waitGroupResult := &sync.WaitGroup{}

	// 收集检查结果时，根据是否在 maintainerChangedSet 中打上 MaintainerChanged 标记，供模板区分展示
	waitGroupResult.Go(func() {
		for result := range resultChannel {
			packageCheck := result.packageCheck
			if _, ok := maintainerChangedSet[packageCheck.RepoInfo.Path]; ok {
				packageCheck.MaintainerChanged = true
			}
			checkResult.appendCheck(result.packageType, packageCheck)
		}
	})

	// 检查新增的集市包（更换维护者的集市包也按新集市包处理），限制并发数为 8
	sem := make(chan struct{}, 8)
	for _, ownerRepo := range newRepos {
		waitGroupCheck.Add(1)
		go func(repo string) {
			defer waitGroupCheck.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			var allowThemeJS bool
			if packageType == check.TypeTheme {
				_, allowThemeJS = themeJsAllowSet[repo]
			}
			checkRepo(repo, occupiedNames, packageType, resultChannel, occupiedNamesMu, allowThemeJS)
		}(ownerRepo)
	}
	waitGroupCheck.Wait()  // 等待检查完成
	close(resultChannel)   // 关闭检查结果输出通道
	waitGroupResult.Wait() // 等待检查结果处理完成
	logger.Infof("finish repos check [%s]", prHeadReposPath)
}

// checkRepo 检查集市资源仓库：Latest Release / package.zip → 下载解压 → check.Check
func checkRepo(
	ownerRepo string,
	occupiedNames map[string]struct{},
	packageType check.PackageType,
	resultChannel chan checkOutput,
	nameSetMutex *sync.Mutex,
	allowThemeJS bool,
) {
	logger.Infof("start repo check [%s]", ownerRepo)

	repoOwner, repoName, _ := strings.Cut(ownerRepo, "/")
	repoInfo := RepoInfo{
		Path: ownerRepo,
		Home: fmt.Sprintf("https://github.com/%s/%s", repoOwner, repoName),
	}
	releaseInfo, releaseIssues := checkRepoLatestRelease(repoOwner, repoName)

	out := PackageCheck{
		RepoInfo: repoInfo,
		Release:  releaseInfo,
		Issues:   releaseIssues,
	}

	// Release / package.zip 未通过时只报告流程层 Issue，不调用 check.Check
	if len(out.Issues) > 0 {
		resultChannel <- checkOutput{packageType: packageType, packageCheck: out}
		logger.Infof("finish repo check [%s] (release issues)", ownerRepo)
		return
	}

	tmpUnzipPath, _, cleanup, err := util.DownloadAndUnzipPackageZip(githubContext, githubClient, repoOwner, repoName, releaseInfo.PackageZipAssetID)
	if err != nil {
		logger.Warnf("download/unzip [%s] failed: %s", ownerRepo, err)
		out.Issues = append(out.Issues, check.Issue{
			Rule:      "release/package_zip",
			MessageZh: fmt.Sprintf("下载或解压 package.zip 失败：%s。请确认 Latest Release 中的 package.zip 可访问且为合法 zip。", err),
			MessageEn: fmt.Sprintf("Failed to download or unzip package.zip: %s. Ensure package.zip in the Latest Release is reachable and a valid zip.", err),
		})
		resultChannel <- checkOutput{packageType: packageType, packageCheck: out}
		logger.Infof("finish repo check [%s] (download failed)", ownerRepo)
		return
	}
	defer cleanup()

	// 持锁执行 check.Check 并登记 name：读写的 OccupiedNames 与实时一致，且同 PR 内唯一
	nameSetMutex.Lock()
	result := check.Check(check.Input{
		PackageRoot:   tmpUnzipPath,
		OwnerRepo:     ownerRepo,
		Type:          packageType,
		OldName:       "", // PR 新仓按首发处理
		OldVersion:    "",
		OccupiedNames: occupiedNames,
		AllowThemeJS:  allowThemeJS,
	})
	if result.OK && result.Package.Name != "" {
		occupiedNames[strings.ToLower(result.Package.Name)] = struct{}{}
	}
	nameSetMutex.Unlock()

	out.Issues = append(out.Issues, result.Issues...)
	resultChannel <- checkOutput{packageType: packageType, packageCheck: out}
	logger.Infof("finish repo check [%s]", ownerRepo)
}

// checkRepoLatestRelease 检查 Latest Release / package.zip / tag，失败直接返回 Issue。
func checkRepoLatestRelease(repoOwner, repoName string) (release util.LatestRelease, issues []check.Issue) {
	release, err := util.FetchLatestRelease(githubContext, githubClient, repoOwner, repoName)
	if err == nil {
		return release, nil
	}

	logger.Warnf("fetch repo [%s/%s] latest release failed: %s", repoOwner, repoName, err)
	switch {
	case errors.Is(err, util.ErrNoLatestRelease):
		return release, []check.Issue{{
			Rule:      "release/latest",
			MessageZh: "仓库没有可用的 Latest Release（或 API 读取失败）。请在 GitHub 上创建一个 Release，并确保该仓库对集市检查所用令牌可见。",
			MessageEn: "No usable Latest Release was found (or the GitHub API call failed). Create a GitHub Release and ensure the repo is visible to the bazaar checker token.",
		}}
	case errors.Is(err, util.ErrNoPackageZip):
		return release, []check.Issue{{
			Rule:      "release/package_zip",
			MessageZh: "Latest Release 中缺少名为 package.zip 的资源文件。请把打包好的 package.zip 作为 Release Asset 上传（文件名必须完全是 package.zip）。",
			MessageEn: "The Latest Release has no asset named package.zip. Upload package.zip as a Release asset (exact filename package.zip).",
		}}
	case errors.Is(err, util.ErrReleaseTag):
		return release, []check.Issue{{
			Rule:      "release/tag",
			MessageZh: "已找到 Latest Release 与 package.zip，但无法解析 Release 对应的 Git 标签/提交。请确认 tag 指向有效 commit。",
			MessageEn: "Latest Release and package.zip were found, but the release tag/commit could not be resolved. Ensure the tag points to a valid commit.",
		}}
	default:
		return release, []check.Issue{{
			Rule:      "release/latest",
			MessageZh: "仓库没有可用的 Latest Release（或 API 读取失败）。请在 GitHub 上创建一个 Release，并确保该仓库对集市检查所用令牌可见。",
			MessageEn: "No usable Latest Release was found (or the GitHub API call failed). Create a GitHub Release and ensure the repo is visible to the bazaar checker token.",
		}}
	}
}
