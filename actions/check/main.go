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
Diff 流程（以 plugins.txt 为例）：
1. 签出 bazaar head（主分支最新）：读 plugins.txt，得到 bazaar head 的 owner/repo 集合，用于过滤
2. 签出 PR head：读 plugins.txt（PR 当前状态）
3. 签出 PR base（merge base）：读 plugins.txt（与 GitHub "Files changed" 的基准一致）
4. 比较 base 与 head：候选新增 = head 有 base 无，候选删除 = base 有 head 无
5. 过滤候选新增：排除已在 bazaar head 中的仓库（可能是解决冲突时从 bazaar head 合并来的）
6. 过滤候选删除：排除在 bazaar head 中已不存在的仓库（可能是其他 PR 删除的）
7. 流程规则：添加或更换维护者合计只能为 1（移除不限）；违反则写 FlowError 评论并跳过包检查（亦不展示移除列表）
8. 按本 PR 实际涉及的 *.txt 同步类型标签（plugin/theme/icon/template/widget）：每次运行对账，缺则补、多则删；解析失败的类型也打标以便发现
9. 一次一包通过后：自动改 PR 标题（Add/Remove [type] owner/repo，插件省略类型；换维护者附 (maintainer change)；多仓纯移除为 Remove N packages）
10. OccupiedNames 使用 bazaar head 的 stage/*.json 中所有类型的 package.name 集合（跨类型；比较前统一转小写）
11. 一次一包通过后：对新增列表做 Latest Release / package.zip → 下载解压 → rules.Check；Bot 回复中列出移除列表、检查结果与更换维护者标记

Check 流程：
1. 获取仓库最新 release 与 package.zip
2. 下载并解压 package.zip
3. 调用 rules.Check（OccupiedNames / AllowThemeJS；PR 新仓 OldName/OldVersion 为空）
4. 通过后将 name 写入 OccupiedNames（同 PR 内唯一性）
5. 生成检查结果并输出文件（使用 go 模板）
6. 使用 thollander/actions-comment-pull-request 将检查结果输出到 PR 中
*/

var (
	BAZAAR_HEAD_PATH    = os.Getenv("BAZAAR_HEAD_PATH")    // bazaar 主分支最新代码目录（用于过滤与 OccupiedNames）
	PR_HEAD_PATH        = os.Getenv("PR_HEAD_PATH")        // 本 PR 当前提交的代码目录（PR head）
	PR_BASE_PATH        = os.Getenv("PR_BASE_PATH")        // 本 PR 的 merge base 代码目录（做 diff 的旧侧，与 GitHub "Files changed" 一致）
	GITHUB_TOKEN        = os.Getenv("PAT")                 // GitHub Token（用于 Release API 与改 PR 标题）
	CHECK_RESULT_OUTPUT = os.Getenv("CHECK_RESULT_OUTPUT") // 检查结果输出文件路径

	REQUEST_TIMEOUT = 30 * time.Second // 请求超时时间

	logger        = gulu.Log.NewLogger(os.Stdout)
	githubContext context.Context
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
	githubClient, err = util.NewGitHubClient(GITHUB_TOKEN, REQUEST_TIMEOUT)
	if err != nil {
		logger.Fatalf("create github client failed: %s", err)
	}

	checkResultTemplate, err := parseCheckResultTemplate()
	if err != nil {
		logger.Fatalf("load check result template failed: %s", err)
	}

	checkResultOutputFile, err := os.OpenFile(CHECK_RESULT_OUTPUT, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		logger.Fatalf("open check result output file [%s] failed: %s", CHECK_RESULT_OUTPUT, err)
	}
	defer checkResultOutputFile.Close()

	checkResult := &CheckResult{}
	var parseErrorBuilder strings.Builder

	// 第一阶段：各类型并行算 diff（不下载、不跑 Pkg Check）
	packageTypes := rules.AllPackageTypes()
	plans := make([]typeCheckPlan, len(packageTypes))
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

	// 类型标签与 diff 对账（与一次一包是否通过无关；解析失败的类型也打标）
	syncPRTypeLabels(plans)

	// 流程规则：添加或更换维护者合计只能为 1（移除不限）
	if addedOrChanged > 1 {
		checkResult.FlowError = formatOnePackageLimitError(addedOrChanged, plans)
		logger.Errorf("one-package limit violated: %d packages added or changed", addedOrChanged)
	} else {
		// 一次一包通过后：自动改 PR 标题（含纯移除；多仓移除为 Remove N packages）
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

	// 将检查结果写入文件
	if err := checkResultTemplate.Execute(checkResultOutputFile, checkResult); err != nil {
		logger.Fatalf("write check result failed: %s", err)
	}

	logger.Infof("PR Check completed")
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
		ap := filepath.Join(PR_HEAD_PATH, util.ThemeJsAllowlistRelPath)
		paths, errAllow := util.ParseReposFromTxt(ap)
		if errAllow != nil {
			plan.parseError = formatParseErrorLabel(fmt.Sprintf("%s PR head", util.ThemeJsAllowlistRelPath), errAllow)
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
// 一次一包规则下通常只有一个仓库；更换维护者的仓库也在 plan.diff.New 中，按新集市包处理。
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
		packageCheck := checkNewRepo(ownerRepo, occupiedNames, plan.packageType, allowThemeJS)
		if _, ok := plan.diff.MaintainerChanged[ownerRepo]; ok {
			packageCheck.MaintainerChanged = true
		}
		checkResult.appendCheck(plan.packageType, packageCheck)
	}
	logger.Infof("finish repos check [%s]", prHeadReposPath)
}

// checkNewRepo 检查 PR 新增的集市包仓库：Latest Release / package.zip → 下载解压 → rules.Check
func checkNewRepo(
	ownerRepo string,
	occupiedNames map[string]struct{},
	packageType rules.PackageType,
	allowThemeJS bool,
) PackageCheck {
	logger.Infof("start repo check [%s]", ownerRepo)

	repoOwner, repoName, _ := strings.Cut(ownerRepo, "/")
	repoInfo := RepoInfo{
		Path: ownerRepo,
		Home: util.GitHubRepoURL(ownerRepo),
	}
	// 检查 Latest Release / package.zip / tag
	releaseInfo, err := util.FetchLatestRelease(githubContext, githubClient, repoOwner, repoName)
	out := PackageCheck{
		RepoInfo: repoInfo,
		Release:  releaseInfo,
	}
	if err != nil {
		logger.Errorf("fetch repo [%s/%s] latest release failed: %s", repoOwner, repoName, err)
		out.Issues = []rules.Issue{rules.IssueFromErr(err)}
	}

	// Release / package.zip 未通过时只报告流程层 Issue，不调用 rules.Check
	if len(out.Issues) > 0 {
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
		OldName:       "",
		OldVersion:    "",
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
