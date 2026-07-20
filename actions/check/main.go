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
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"text/template"
	"time"

	"github.com/88250/gulu"
	"github.com/google/go-github/v89/github"
	"github.com/siyuan-note/bazaar/actions/util"
	"github.com/siyuan-note/bazaar/rules"
)

/*
路径黑白名单（最先执行）：
1. git diff（PR merge base → head）得到变更文件
2. 黑名单（stage/**、config/themes-theme-js-allowlist.txt）：写 FlowError，跳过包检查
3. 白名单（五个 *.txt）：进入下方 Diff / Check；灰区文件忽略
4. 同改黑+白：优先黑名单；无白名单改动：不跑包检查（模板「无实际变更」，ci-failed）

Diff 流程（以 plugins.txt 为例）：
1. 签出 bazaar head（主分支最新）：读 plugins.txt，得到 bazaar head 的 owner/repo 集合，用于过滤
2. 签出 PR head：读 plugins.txt（PR 当前状态）
3. 签出 PR base（merge base）：读 plugins.txt（与 GitHub "Files changed" 的基准一致）
4. 比较 base 与 head：候选新增 = head 有 base 无，候选删除 = base 有 head 无
5. 过滤候选新增：排除已在 bazaar head 中的仓库（可能是解决冲突时从 bazaar head 合并来的）
6. 过滤候选删除：排除在 bazaar head 中已不存在的仓库（可能是其他 PR 删除的）
7. 流程规则：添加或更换维护者合计只能为 1（下架不限）；违反则写 FlowError，跳过后续 Check（评论亦不展示下架列表）
8. 一次一包通过后：自动改 PR 标题（Add/Delist [type] owner/repo，插件省略类型；换维护者附 (maintainer change)；多仓纯下架为 Delist N packages）

Check 流程（一次一包通过后，对 plan.diff.New 中的仓库）：
1. 从 bazaar head 的 stage/*.json 加载 OccupiedNames（跨类型；比较前统一转小写）
2. 校验仓库公开；不公开（私有 / 不可访问）则记 Issue 并跳过后续检查
3. 校验根目录 LICENSE / LICENSE.txt；缺失则记 Issue（不阻断后续）
4. 获取仓库 Latest Release 与 package.zip
5. 下载并解压 package.zip
6. 调用 rules.Check（OccupiedNames / AllowThemeJS；纯新仓 OldName/OldVersion 为空；换维护者从旧 stage 取 OldName+OldVersion，视同更新须升版）
7. 通过后将 name 写入 OccupiedNames（同 PR 内唯一性）

收尾（无论是否跑过包检查）：
1. 无实际变更且 PR 已合并/关闭：跳过结果评论与标签同步（解冲突触发检查后立刻合并的竞态，避免误打 ci-failed）
2. 用模板写出检查结果文件（含下架列表、检查 Issues；换维护者时附流程说明链接）
3. 同步标签：类型标签按涉及的 *.txt 对账；CI 状态打 ci-failed 或 ci-passed（互斥，同一次 Replace）
4. 工作流用 thollander/actions-comment-pull-request 将结果文件发到 PR
*/

var (
	BAZAAR_HEAD_PATH    = os.Getenv("BAZAAR_HEAD_PATH")    // bazaar 主分支最新代码目录（用于过滤、OccupiedNames、theme.js 白名单）
	PR_HEAD_PATH        = os.Getenv("PR_HEAD_PATH")        // 本 PR 当前提交的代码目录（PR head）
	PR_BASE_PATH        = os.Getenv("PR_BASE_PATH")        // 本 PR 的 merge base 代码目录（做 diff 的旧侧，与 GitHub "Files changed" 一致）
	PAT                 = os.Getenv("PAT")                 // 个人访问令牌（Release API）
	GITHUB_TOKEN        = os.Getenv("GITHUB_TOKEN")        // Actions 令牌（改本仓 PR 标题 / 标签 / 请求审查）
	CHECK_RESULT_OUTPUT = os.Getenv("CHECK_RESULT_OUTPUT") // 检查结果输出文件路径

	REQUEST_TIMEOUT = 30 * time.Second // 请求超时时间

	logger           = gulu.Log.NewLogger(os.Stdout)
	githubContext    context.Context
	githubClient     *github.Client // PAT：跨仓 Release
	githubRepoClient *github.Client // GITHUB_TOKEN：本仓 PR 标题 / 标签 / 请求审查
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

// bazaarDocURL 生成指向本次 bazaar head 提交的 README 锚点链接。
func bazaarDocURL(bazaarHeadSHA, file, anchor string) string {
	repo := GITHUB_REPOSITORY
	if repo == "" {
		repo = "siyuan-note/bazaar"
	}
	return fmt.Sprintf("https://github.com/%s/blob/%s/%s#%s", repo, bazaarHeadSHA, file, anchor)
}

func parseCheckResultTemplate(ctx context.Context) (*template.Template, error) {
	bazaarHeadSHA, err := gitRevParseHEAD(ctx, BAZAAR_HEAD_PATH)
	if err != nil {
		return nil, err
	}
	logger.Infof("bazaar head SHA [%s]", bazaarHeadSHA)
	return template.New("check-result.md.tpl").Funcs(template.FuncMap{
		"issueIndex": formatIssueIndex,
		"bazaarDocURL": func(file, anchor string) string {
			return bazaarDocURL(bazaarHeadSHA, file, anchor)
		},
	}).Parse(checkResultTemplateText)
}

// typeCheckPlan 单类型 diff 与后续包检查所需上下文。
type typeCheckPlan struct {
	packageType     rules.PackageType
	diff            repoDiff
	themeJsAllowSet Set
	parseError      string
}

func main() {
	logger.Infof("PR Check started")

	var stop context.CancelFunc
	githubContext, stop = signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var err error
	githubClient, err = util.NewGitHubClient(PAT, REQUEST_TIMEOUT)
	if err != nil {
		logger.Fatalf("create github client failed: %s", err)
	}
	repoToken := GITHUB_TOKEN
	if repoToken == "" {
		repoToken = PAT
		logger.Infof("GITHUB_TOKEN empty, fall back to PAT for PR title/labels/reviewers")
	}
	githubRepoClient, err = util.NewGitHubClient(repoToken, REQUEST_TIMEOUT)
	if err != nil {
		logger.Fatalf("create github repo client failed: %s", err)
	}

	checkResultTemplate, err := parseCheckResultTemplate(githubContext)
	if err != nil {
		logger.Fatalf("load check result template failed: %s", err)
	}

	checkResultOutputFile, err := os.OpenFile(CHECK_RESULT_OUTPUT, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		logger.Fatalf("open check result output file [%s] failed: %s", CHECK_RESULT_OUTPUT, err)
	}
	defer checkResultOutputFile.Close()

	checkResult := &CheckResult{}
	var plans []typeCheckPlan

	changedFiles, err := listPRChangedFiles(githubContext)
	if err != nil {
		logger.Fatalf("list PR changed files failed: %s", err)
	}
	blackFiles, whiteFiles := classifyPRFiles(changedFiles)
	logger.Infof("PR path scope: changed=%d black=%d white=%d", len(changedFiles), len(blackFiles), len(whiteFiles))

	switch {
	case len(blackFiles) > 0:
		// 黑名单优先：固定 FlowError，跳过包列表 diff 与包检查
		checkResult.FlowError = formatBlacklistFlowError()
		logger.Errorf("blacklisted path changes: %s", strings.Join(blackFiles, ", "))
	case len(whiteFiles) > 0:
		var parseErrorBuilder strings.Builder

		// 第一阶段：各类型并行算 diff（不下载、不跑 Pkg Check）
		packageTypes := rules.AllPackageTypes()
		plans = make([]typeCheckPlan, len(packageTypes))
		var planWg sync.WaitGroup
		for i, packageType := range packageTypes {
			planWg.Go(func() {
				plans[i] = prepareTypeCheckPlan(packageType)
			})
		}
		planWg.Wait()

		addedOrChanged := 0
		for _, plan := range plans {
			if plan.parseError != "" {
				parseErrorBuilder.WriteString(plan.parseError)
				continue
			}
			// 将本 PR 的删除列表写入检查结果（一次一包失败时模板不展示）
			if !checkResult.setDeleted(plan.packageType, plan.diff.Deleted) {
				panic("main: invalid package type")
			}
			addedOrChanged += len(plan.diff.New)
		}
		checkResult.ParseError = parseErrorBuilder.String()

		// 流程规则：添加或更换维护者合计只能为 1（下架不限）
		if addedOrChanged > 1 {
			checkResult.FlowError = formatOnePackageLimitError(addedOrChanged, plans)
			logger.Errorf("one-package limit violated: %d packages added or changed", addedOrChanged)
		} else {
			// 一次一包通过后：自动改 PR 标题（含纯下架；多仓下架为 Delist N packages）
			if title, ok := conventionalPRTitle(plans); ok {
				maybeUpdatePRTitle(title)
			}

			occupiedNames, err := util.LoadOccupiedNames(BAZAAR_HEAD_PATH)
			if err != nil {
				logger.Fatalf("load occupied names failed: %s", err)
			}

			// 一次一包通过后最多只有一个类型含新增，顺序检查即可
			for _, plan := range plans {
				if plan.parseError != "" || len(plan.diff.New) == 0 {
					continue
				}
				runTypePackageChecks(plan, checkResult, occupiedNames)
			}
		}
	default:
		// 无白名单改动（且未命中黑名单）：不跑包检查，模板展示「无实际变更」，ci-failed
		logger.Infof("no whitelisted list file changes; skip package checks")
	}

	// 无实际变更且 PR 已合并/关闭：多为解冲突后立刻合并，检查相对最新 main 滤空；跳过评论与标签，避免误打 ci-failed
	if isNoActualChange(checkResult) && prIsMergedOrClosed() {
		logger.Infof("no actual list change and PR already merged/closed; skip result comment and label sync")
		appendGitHubOutput("skip_side_effects", "true")
		logger.Infof("PR Check completed (skipped side effects)")
		return
	}

	checkResult.PRAuthor = prAuthorLogin()

	// 将检查结果写入文件
	if err := checkResultTemplate.Execute(checkResultOutputFile, checkResult); err != nil {
		logger.Fatalf("write check result failed: %s", err)
	}

	// 类型标签 + CI 状态标签对账（失败只记日志）
	syncPRLabels(plans, checkResult)

	// 检查通过后请求审查者（名单来自仓库 Variables，失败只记日志）
	maybeRequestReviewers(checkResult)

	logger.Infof("PR Check completed")
}

// appendGitHubOutput 向 GITHUB_OUTPUT 追加 name=value（非 Actions 环境则忽略）。
func appendGitHubOutput(name, value string) {
	path := os.Getenv("GITHUB_OUTPUT")
	if path == "" {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logger.Errorf("open GITHUB_OUTPUT failed: %s", err)
		return
	}
	defer f.Close()
	if _, err := fmt.Fprintf(f, "%s=%s\n", name, value); err != nil {
		logger.Errorf("write GITHUB_OUTPUT failed: %s", err)
	}
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
		owner, name, ok := strings.Cut(s, "/")
		if !ok {
			logger.Fatalf("internal error: repo path %q has no owner/repo separator", s)
		}
		paths = append(paths, s)
		pathSet[s] = struct{}{}
		nameToOwner[name] = owner
	}
	return
}

// prepareTypeCheckPlan 解析三侧 TXT 并计算本类型 diff；失败时只填 parseError。
func prepareTypeCheckPlan(packageType rules.PackageType) typeCheckPlan {
	repoListTxtName := packageType.ReposListFile()
	bazaarHeadReposPath := filepath.Join(BAZAAR_HEAD_PATH, repoListTxtName)
	prBaseReposPath := filepath.Join(PR_BASE_PATH, repoListTxtName)
	prHeadReposPath := filepath.Join(PR_HEAD_PATH, repoListTxtName)

	logger.Infof("start repos diff [%s]", prHeadReposPath)

	plan := typeCheckPlan{packageType: packageType}

	// 加载三个版本的集市包列表：bazaar head（用于过滤）、PR base、PR head（用于 diff）
	_, bazaarHeadRepoSet, _, err := parseReposFromRootTxt(bazaarHeadReposPath)
	if err != nil {
		plan.parseError = formatParseErrorLabel(fmt.Sprintf("%s bazaar head", repoListTxtName), err)
		logger.Errorf("load bazaar head repos [%s] failed: %s, skip this type", bazaarHeadReposPath, err)
		return plan
	}
	basePaths, baseSet, baseNameToOwner, err := parseReposFromRootTxt(prBaseReposPath)
	if err != nil {
		plan.parseError = formatParseErrorLabel(fmt.Sprintf("%s PR base", repoListTxtName), err)
		logger.Errorf("load PR base repos [%s] failed: %s, skip this type", prBaseReposPath, err)
		return plan
	}
	headPaths, headSet, _, err := parseReposFromRootTxt(prHeadReposPath)
	if err != nil {
		plan.parseError = formatParseErrorLabel(fmt.Sprintf("%s PR head", repoListTxtName), err)
		logger.Errorf("load PR head repos [%s] failed: %s, skip this type", prHeadReposPath, err)
		return plan
	}

	if packageType == rules.TypeTheme {
		// 白名单为黑名单路径（社区 PR 不可改），始终读 bazaar head
		ap := filepath.Join(BAZAAR_HEAD_PATH, util.ThemeJsAllowlistRelPath)
		paths, errAllow := util.ParseReposFromTxt(ap)
		if errAllow != nil {
			plan.parseError = formatParseErrorLabel(fmt.Sprintf("%s bazaar head", util.ThemeJsAllowlistRelPath), errAllow)
			logger.Errorf("load theme.js allowlist [%s] failed: %s, skip this type", ap, errAllow)
			return plan
		}
		plan.themeJsAllowSet = make(Set, len(paths))
		for _, p := range paths {
			plan.themeJsAllowSet[p] = struct{}{}
		}
	}

	plan.diff = computeRepoDiff(headPaths, basePaths, baseSet, headSet, bazaarHeadRepoSet, baseNameToOwner)
	logger.Infof("finish repos diff [%s]: new=%d deleted=%d", prHeadReposPath, len(plan.diff.New), len(plan.diff.Deleted))
	return plan
}

func formatParseErrorLabel(label string, err error) string {
	zh, en := rules.LocalizedMessages(err)
	return fmt.Sprintf("[%s]\n\n%s\n\n%s\n\n", label, zh, en)
}

// runTypePackageChecks 对本类型新增/换维护者仓库执行 Latest Release → package.zip → rules.Check。
// 一次一包规则下通常只有一个仓库；更换维护者时从 bazaar head stage 取旧 name/version（视同更新）。
func runTypePackageChecks(
	plan typeCheckPlan,
	checkResult *CheckResult,
	occupiedNames map[string]struct{},
) {
	prHeadReposPath := filepath.Join(PR_HEAD_PATH, plan.packageType.ReposListFile())
	logger.Infof("start repos check [%s]", prHeadReposPath)

	for _, ownerRepo := range plan.diff.New {
		var allowThemeJS bool
		if plan.packageType == rules.TypeTheme {
			_, allowThemeJS = plan.themeJsAllowSet[ownerRepo]
		}
		var oldName, oldVersion string
		var legacyIssues []rules.Issue
		previousOwnerRepo, maintainerChanged := plan.diff.PreviousRepos[ownerRepo]
		if maintainerChanged {
			oldName, oldVersion, legacyIssues = resolveMaintainerChangeLegacy(plan.packageType, ownerRepo, previousOwnerRepo)
		}
		packageCheck := checkNewRepo(ownerRepo, occupiedNames, plan.packageType, allowThemeJS, oldName, oldVersion, legacyIssues)
		packageCheck.MaintainerChanged = maintainerChanged
		checkResult.appendCheck(plan.packageType, packageCheck)
	}
	logger.Infof("finish repos check [%s]", prHeadReposPath)
}

// resolveMaintainerChangeLegacy 从 bazaar head stage 读取换维护者前旧仓库的 name/version。
//
// 调用前提：TXT diff 已判定为更换维护者（同 GitHub 仓库名、不同 owner，且旧路径已删除），oldOwnerRepo 为被替换的旧路径。
// 此处只负责取出旧清单字段，供 rules.Check 校验「name 不变、version 升高」。
func resolveMaintainerChangeLegacy(packageType rules.PackageType, newOwnerRepo, oldOwnerRepo string) (oldName, oldVersion string, issues []rules.Issue) {
	if oldOwnerRepo == "" {
		// PreviousRepos 有键时值不应为空；落到这里是内部状态异常，不是作者列表写法问题。
		return "", "", []rules.Issue{rules.IssueFromErr(rules.LocalizedErr(
			fmt.Sprintf("内部错误：已将 `%s` 识别为更换维护者，但缺少对应的原 `owner/repo`。请联系集市维护者。", newOwnerRepo),
			fmt.Sprintf("Internal error: `%s` was detected as a maintainer change, but the previous `owner/repo` is missing. Please contact a bazaar maintainer.", newOwnerRepo),
			nil,
		))}
	}
	oldStage, err := util.FindStageRepo(BAZAAR_HEAD_PATH, packageType, oldOwnerRepo)
	if err != nil {
		logger.Errorf("load stage repo [%s] for maintainer change failed: %s", oldOwnerRepo, err)
		return "", "", []rules.Issue{rules.IssueFromErr(rules.LocalizedErr(
			fmt.Sprintf("更换维护者检查失败：读取原仓库 `%s` 的 stage 条目出错：%v。请联系集市维护者。", oldOwnerRepo, err),
			fmt.Sprintf("Maintainer-change check failed: error reading the stage entry for previous repo `%s`: %v. Please contact a bazaar maintainer.", oldOwnerRepo, err),
			err,
		))}
	}
	if oldStage == nil || oldStage.Package.Name == "" {
		// TXT 侧已是「旧路径 → 新路径」，但 stage 没有旧包元数据，无法做 name/version 连续性校验。
		return "", "", []rules.Issue{rules.IssueFromErr(rules.LocalizedErr(
			fmt.Sprintf("内部错误：已将 `%s` → `%s` 识别为更换维护者，但集市 stage 中没有原仓库 `%s` 的记录。请联系集市维护者。", oldOwnerRepo, newOwnerRepo, oldOwnerRepo),
			fmt.Sprintf("Internal error: `%s` → `%s` was detected as a maintainer change, but the bazaar stage has no entry for previous repo `%s`. Please contact a bazaar maintainer.", oldOwnerRepo, newOwnerRepo, oldOwnerRepo),
			nil,
		))}
	}
	logger.Infof("maintainer change [%s] <- [%s], old name [%s] version [%s]", newOwnerRepo, oldOwnerRepo, oldStage.Package.Name, oldStage.Package.Version)
	return oldStage.Package.Name, oldStage.Package.Version, nil
}

// checkNewRepo 检查 PR 新增的集市包仓库：公开性 / LICENSE → Latest Release / package.zip → 下载解压 → rules.Check
// oldName/oldVersion 在换维护者时来自旧 stage（视同更新）；legacyIssues 为解析旧条目时的前置错误。
func checkNewRepo(
	ownerRepo string,
	occupiedNames map[string]struct{},
	packageType rules.PackageType,
	allowThemeJS bool,
	oldName, oldVersion string,
	legacyIssues []rules.Issue,
) PackageCheck {
	logger.Infof("start repo check [%s]", ownerRepo)

	repoOwner, repoName, _ := strings.Cut(ownerRepo, "/")
	repoInfo := RepoInfo{
		Path: ownerRepo,
		Home: util.GitHubRepoURL(ownerRepo),
	}
	out := PackageCheck{
		RepoInfo: repoInfo,
	}
	if len(legacyIssues) > 0 {
		out.Issues = append(out.Issues, legacyIssues...)
		logger.Infof("finish repo check [%s] (legacy issues)", ownerRepo)
		return out
	}

	// 仓库须公开；否则跳过后续检查（无法可靠读取 Release / 源码）
	if err := util.CheckRepoPublic(githubContext, githubClient, repoOwner, repoName); err != nil {
		logger.Errorf("repo [%s/%s] public check failed: %s", repoOwner, repoName, err)
		out.Issues = append(out.Issues, rules.IssueFromErr(err))
		logger.Infof("finish repo check [%s] (not public)", ownerRepo)
		return out
	}

	// LICENSE / LICENSE.txt：缺失只记 Issue，继续跑 Release / Pkg Check
	if err := util.CheckRepoLicenseFile(githubContext, githubClient, repoOwner, repoName); err != nil {
		logger.Errorf("repo [%s/%s] license check failed: %s", repoOwner, repoName, err)
		out.Issues = append(out.Issues, rules.IssueFromErr(err))
	}

	// 检查 Latest Release / package.zip / tag
	releaseInfo, err := util.FetchLatestRelease(githubContext, githubClient, repoOwner, repoName)
	out.Release = releaseInfo
	if err != nil {
		logger.Errorf("fetch repo [%s/%s] latest release failed: %s", repoOwner, repoName, err)
		out.Issues = append(out.Issues, rules.IssueFromErr(err))
		// Release / package.zip 未通过时只报告流程层 Issue（可与 LICENSE Issue 并存），不调用 rules.Check
		logger.Infof("finish repo check [%s] (release issues)", ownerRepo)
		return out
	}

	tmpUnzipPath, zipData, cleanup, err := util.DownloadAndUnzipPackageZip(githubContext, githubClient, repoOwner, repoName, releaseInfo.PackageZipAssetID)
	if err != nil {
		logger.Errorf("download/unzip [%s] failed: %s", ownerRepo, err)
		out.Issues = append(out.Issues, rules.IssueFromErr(err))
		logger.Infof("finish repo check [%s] (download failed)", ownerRepo)
		return out
	}
	defer cleanup()

	result := rules.Check(rules.Input{
		PackageRoot:   tmpUnzipPath,
		OwnerRepo:     ownerRepo,
		Type:          packageType,
		ZipData:       zipData,
		OldName:       oldName,
		OldVersion:    oldVersion,
		OccupiedNames: occupiedNames,
		AllowThemeJS:  allowThemeJS,
	})
	if result.OK && result.Package.Name != "" {
		occupiedNames[strings.ToLower(result.Package.Name)] = struct{}{}
	}

	out.Issues = append(out.Issues, result.Issues...)
	logger.Infof("finish repo check [%s]", ownerRepo)
	return out
}
