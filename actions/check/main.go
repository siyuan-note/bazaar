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
	"image"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/88250/gulu"
	"github.com/google/go-github/v52/github"
	"github.com/panjf2000/ants/v2"
	"github.com/parnurzeal/gorequest"
	"github.com/siyuan-note/bazaar/actions/util"
)

/*
Diff 流程（以 plugins.json 为例）：
1. 签出 bazaar head（主分支最新）：读 plugins.json，得到 bazaar head 的 owner/repo 集合与 name 集合，用于过滤和 name 唯一性检查
2. 签出 PR head：读 plugins.json（PR 当前状态）
3. 签出 PR base：读 plugins.json（PR 创建时的基准状态）
4. 比较 base 与 head：候选新增 = head 有 base 无，候选删除 = base 有 head 无
5. 过滤候选新增：排除已在 bazaar head 中的仓库（可能是解决冲突时从 bazaar head 合并来的）
6. 过滤候选删除：排除在 bazaar head 中已不存在的仓库（可能是其他 PR 删除的）
7. name 唯一性检查使用 bazaar head 的 name 集合
8. 对最终新增列表做 release/文件/属性检查，并在 Bot 回复中列出删除列表、更换维护者列表

Check 流程：
1. 获取仓库最新 release
2. 获取仓库最新 release 的 tag
3. 获取仓库最新 release 的 hash
4. 检查必要的文件是否存在
5. 获取清单文件 *.json
		检查清单文件是否存在必要的字段
			检查清单中 name 字段值是否与 stage/*.json 中的重复
6. 生成检查结果并输出文件 (使用 go 模板)
7. 使用 thollander/actions-comment-pull-request 将检查结果输出到到 PR 中
*/

var (
	BAZAAR_HEAD_PATH    = os.Getenv("BAZAAR_HEAD_PATH")    // bazaar 主分支最新代码目录（用于过滤与 name 唯一性）
	PR_HEAD_PATH        = os.Getenv("PR_HEAD_PATH")        // 本 PR 当前提交的代码目录（PR head）
	PR_BASE_PATH        = os.Getenv("PR_BASE_PATH")        // 本 PR 分叉点的代码目录（PR base，做 diff 的旧侧）
	GITHUB_TOKEN        = os.Getenv("PAT")                 // GitHub Token
	CHECK_RESULT_OUTPUT = os.Getenv("CHECK_RESULT_OUTPUT") // 检查结果输出文件路径

	REQUEST_TIMEOUT        = 30 * time.Second // 请求超时时间
	REQUEST_RETRY_COUNT    = 3                // 请求重试次数
	REQUEST_RETRY_DURATION = 10 * time.Second // 请求重试间隔时间

	logger        = gulu.Log.NewLogger(os.Stdout)
	githubContext = context.Background()
	githubClient  = github.NewTokenClient(githubContext, GITHUB_TOKEN)
)

func main() {
	logger.Infof("PR Check running...")

	// 获取检查结果模板文件
	checkResultTemplate, err := template.ParseFiles(FILE_PATH_CHECK_RESULT_TEMPLATE)
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

	// 取消注释以下代码，使用测试数据渲染检查结果模板
	// check_result_template.Execute(check_result_output_file, CheckResultTestExample)
	// return

	checkResult := &CheckResult{
		Icons:     []Icon{},
		Plugins:   []Plugin{},
		Templates: []Template{},
		Themes:    []Theme{},
		Widgets:   []Widget{},
	} // 检查结果

	githubClient.Client().Timeout = REQUEST_TIMEOUT // 设置请求超时时间

	wg := &sync.WaitGroup{}
	wg.Add(5)

	go checkRepos(icons, checkResult, wg)
	go checkRepos(plugins, checkResult, wg)
	go checkRepos(templates, checkResult, wg)
	go checkRepos(themes, checkResult, wg)
	go checkRepos(widgets, checkResult, wg)

	wg.Wait() // 等待所有检查完成

	// 将检查结果写入文件
	checkResultTemplate.Execute(checkResultOutputFile, checkResult)

	logger.Infof("PR Check finished")
}

// parseReposFromRootJSON 从集市包列表 JSON（repos 为字符串数组）解析出路径列表、路径集合、名称集合和 name->owner 映射
func parseReposFromRootJSON(filePath string) (paths []string, pathSet StringSet, nameSet StringSet, nameToOwner map[string]string, err error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return
	}
	var m map[string]interface{}
	if err = gulu.JSON.UnmarshalJSON(data, &m); err != nil {
		return
	}
	repos, _ := m["repos"].([]interface{})
	if repos == nil {
		paths = []string{}
		pathSet = make(StringSet)
		nameSet = make(StringSet)
		nameToOwner = make(map[string]string)
		return
	}
	paths = make([]string, 0, len(repos))
	pathSet = make(StringSet, len(repos))
	nameSet = make(StringSet, len(repos))
	nameToOwner = make(map[string]string, len(repos))
	for _, r := range repos {
		if s, ok := r.(string); ok {
			parts := strings.Split(s, "/")
			if len(parts) != 2 {
				err = fmt.Errorf("invalid repo path: %s", s)
				return
			}
			owner := parts[0]
			name := parts[1]
			paths = append(paths, s)
			pathSet[s] = nil
			nameSet[name] = nil
			nameToOwner[name] = owner
		}
	}
	return
}

// checkRepos 检查集市资源仓库列表
func checkRepos(
	resourceType ResourceType,
	checkResult *CheckResult,
	waitGroup *sync.WaitGroup,
) {
	defer waitGroup.Done()

	var repoListJSONName string
	switch resourceType {
	case icons:
		repoListJSONName = "icons.json"
	case plugins:
		repoListJSONName = "plugins.json"
	case templates:
		repoListJSONName = "templates.json"
	case themes:
		repoListJSONName = "themes.json"
	case widgets:
		repoListJSONName = "widgets.json"
	default:
		panic("checkRepos: invalid resource type")
	}
	bazaarHeadReposPath := filepath.Join(BAZAAR_HEAD_PATH, repoListJSONName)
	prBaseReposPath := filepath.Join(PR_BASE_PATH, repoListJSONName)
	prHeadReposPath := filepath.Join(PR_HEAD_PATH, repoListJSONName)
	logger.Infof("start repos check [%s]", prHeadReposPath)

	// 加载三个版本的集市包列表：bazaar head（用于过滤与 name 唯一性检查）、PR base、PR head（用于 diff）
	_, bazaarHeadRepoSet, bazaarHeadNameSet, _, err := parseReposFromRootJSON(bazaarHeadReposPath)
	if err != nil {
		logger.Fatalf("load bazaar head repos [%s] failed: %s", bazaarHeadReposPath, err)
		panic(err)
	}
	basePaths, baseSet, _, baseNameToOwner, err := parseReposFromRootJSON(prBaseReposPath)
	if err != nil {
		logger.Fatalf("load PR base repos [%s] failed: %s", prBaseReposPath, err)
		panic(err)
	}
	headPaths, headSet, _, _, err := parseReposFromRootJSON(prHeadReposPath)
	if err != nil {
		logger.Fatalf("load PR head repos [%s] failed: %s", prHeadReposPath, err)
		panic(err)
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

	// 新增与更换维护者合并为待检查列表，统一做 release/文件/属性/name 唯一性检查（更换维护者按新集市包处理）
	maintainerChangedSet := make(StringSet, len(maintainerChanged))
	for _, path := range maintainerChanged {
		maintainerChangedSet[path] = nil
	}
	resultChannel := make(chan interface{}, 4)
	waitGroupCheck := &sync.WaitGroup{}
	waitGroupResult := &sync.WaitGroup{}
	nameSetMutex := &sync.Mutex{}

	// setMaintainerChangedIfNeeded 如果 path 在 maintainerChangedSet 中，则设置 maintainerChanged 为 true
	setMaintainerChangedIfNeeded := func(path *string, maintainerChanged *bool, maintainerChangedSet StringSet) {
		if isKeyInSet(*path, maintainerChangedSet) {
			*maintainerChanged = true
		}
	}

	waitGroupResult.Add(1)
	// 收集检查结果时，根据是否在 maintainerChangedSet 中打上 MaintainerChanged 标记，供模板区分展示
	go func() {
		for result := range resultChannel {
			switch v := result.(type) {
			case *Icon:
				icon := *v
				setMaintainerChangedIfNeeded(&icon.RepoInfo.Path, &icon.MaintainerChanged, maintainerChangedSet)
				checkResult.Icons = append(checkResult.Icons, icon)
			case *Plugin:
				plugin := *v
				setMaintainerChangedIfNeeded(&plugin.RepoInfo.Path, &plugin.MaintainerChanged, maintainerChangedSet)
				checkResult.Plugins = append(checkResult.Plugins, plugin)
			case *Template:
				templateItem := *v
				setMaintainerChangedIfNeeded(&templateItem.RepoInfo.Path, &templateItem.MaintainerChanged, maintainerChangedSet)
				checkResult.Templates = append(checkResult.Templates, templateItem)
			case *Theme:
				theme := *v
				setMaintainerChangedIfNeeded(&theme.RepoInfo.Path, &theme.MaintainerChanged, maintainerChangedSet)
				checkResult.Themes = append(checkResult.Themes, theme)
			case *Widget:
				widget := *v
				setMaintainerChangedIfNeeded(&widget.RepoInfo.Path, &widget.MaintainerChanged, maintainerChangedSet)
				checkResult.Widgets = append(checkResult.Widgets, widget)
			default:
			}
		}
		waitGroupResult.Done()
	}()

	// 检查新增的集市包（包含更换维护者的集市包），限制并发数为 8
	p, _ := ants.NewPoolWithFunc(8, func(arg interface{}) {
		defer waitGroupCheck.Done()
		repo := arg.(string)
		checkRepo(repo, bazaarHeadNameSet, resourceType, resultChannel, nameSetMutex)
	})
	defer p.Release()

	for _, repo := range newRepos {
		waitGroupCheck.Add(1)
		p.Invoke(repo)
	}
	waitGroupCheck.Wait()  // 等待检查完成
	close(resultChannel)   // 关闭检查结果输出通道
	waitGroupResult.Wait() // 等待检查结果处理完成
	logger.Infof("finish repos check [%s]", prHeadReposPath)
}

// checkRepo 检查集市资源仓库
func checkRepo(
	repoPath string,
	nameSet StringSet,
	resourceType ResourceType,
	resultChannel chan interface{},
	nameSetMutex *sync.Mutex,
) {

	logger.Infof("start repo check [%s]", repoPath)
	var err error

	// 检查 latest release
	repoMeta := strings.Split(repoPath, "/")
	repoOwner := repoMeta[0]
	repoName := repoMeta[1]
	repoInfo := &RepoInfo{
		Owner: repoOwner,
		Name:  repoName,
		Path:  repoPath,
		Home:  buildRepoHomeURL(repoOwner, repoName),
	}
	releaseCheckResult := checkRepoLatestRelease(repoOwner, repoName)

	if releaseCheckResult.LatestRelease.Hash != "" {
		// 获得 latest release 成功, 可以进一步检查文件与属性

		// 检查清单文件中的属性
		var attrsCheckResult *Attrs
		var manifestFilePath string // 清单文件路径
		switch resourceType {
		case icons:
			manifestFilePath = "icon.json"
		case plugins:
			manifestFilePath = "plugin.json"
		case templates:
			manifestFilePath = "template.json"
		case themes:
			manifestFilePath = "theme.json"
		case widgets:
			manifestFilePath = "widget.json"
		default:
		}
		manifestFileUrl := buildFileRawURL(
			repoOwner,
			repoName,
			releaseCheckResult.LatestRelease.Hash,
			manifestFilePath,
		) // 清单文件下载地址

		if attrsCheckResult, err = checkManifestAttrs(manifestFileUrl); err != nil {
			logger.Warnf("check repo [%s] manifest file [%s] failed: %s", repoPath, manifestFileUrl, err)
		} else {
			// 有效性检查
			attrsCheckResult.Name.Valid = isValidName(attrsCheckResult.Name.Value)
			if attrsCheckResult.Name.Valid {
				// name 必须和 repo name 一致
				attrsCheckResult.Name.Valid = attrsCheckResult.Name.Value == repoName
				if !attrsCheckResult.Name.Valid {
					logger.Warnf("repo [%s] name [%s] is not equal to repo name [%s]", repoPath, attrsCheckResult.Name.Value, repoName)
				}
			} else {
				logger.Warnf("repo [%s] name [%s] is invalid", repoPath, attrsCheckResult.Name.Value)
			}

			// 唯一性检查
			if attrsCheckResult.Name.Valid {
				name := attrsCheckResult.Name.Value
				nameSetMutex.Lock() // 保护 nameSet 的并发访问
				if isKeyInSet(name, nameSet) {
					logger.Warnf("repo [%s] name [%s] already exists", repoPath, name)
				} else {
					nameSet[name] = nil // 新的 name 添加到检查集合中

					attrsCheckResult.Name.Unique = true // name 通过唯一性检查
				}
				nameSetMutex.Unlock()
			}
		}

		attrsCheckResult.Name.Pass = attrsCheckResult.Name.Exist &&
			attrsCheckResult.Name.Valid &&
			attrsCheckResult.Name.Unique

		attrsCheckResult.Pass = attrsCheckResult.Name.Pass &&
			attrsCheckResult.Version.Pass &&
			attrsCheckResult.Author.Pass &&
			attrsCheckResult.URL.Pass

		// 检查文件
		var filesCheckResult interface{} // 文件检查结果

		// 检查所有类型集市资源必要的文件
		iconPngCheckResult, err := checkFileExist(
			repoOwner,
			repoName,
			releaseCheckResult.LatestRelease.Hash,
			FILE_PATH_ICON_PNG,
		)
		if err != nil {
			logger.Warn(err.Error())
		}

		previewPngCheckResult, err := checkFileExist(
			repoOwner,
			repoName,
			releaseCheckResult.LatestRelease.Hash,
			FILE_PATH_PREVIEW_PNG,
		)
		if err != nil {
			logger.Warn(err.Error())
		}

		readmeMdCheckResult, err := checkFileExist(
			repoOwner,
			repoName,
			releaseCheckResult.LatestRelease.Hash,
			FILE_PATH_README_MD,
		)
		if err != nil {
			logger.Warn(err.Error())
		}

		// 检查各类型集市资源其他必要的文件
		switch resourceType {
		case icons:
			{
				iconJsonCheckResult, err := checkFileExist(
					repoOwner,
					repoName,
					releaseCheckResult.LatestRelease.Hash,
					FILE_PATH_ICON_JSON,
				)
				if err != nil {
					logger.Warn(err.Error())
				}

				filesCheckResult = &IconFiles{
					Pass: iconJsonCheckResult.Pass &&
						iconPngCheckResult.Pass &&
						previewPngCheckResult.Pass &&
						readmeMdCheckResult.Pass,

					IconJson: *iconJsonCheckResult,

					IconPng:    *iconPngCheckResult,
					PreviewPng: *previewPngCheckResult,
					ReadmeMd:   *readmeMdCheckResult,
				}
			}
		case plugins:
			{
				pluginJsonCheckResult, err := checkFileExist(
					repoOwner,
					repoName,
					releaseCheckResult.LatestRelease.Hash,
					FILE_PATH_PLUGIN_JSON,
				)
				if err != nil {
					logger.Warn(err.Error())
				}

				filesCheckResult = &PluginFiles{
					Pass: pluginJsonCheckResult.Pass &&
						iconPngCheckResult.Pass &&
						previewPngCheckResult.Pass &&
						readmeMdCheckResult.Pass,

					PluginJson: *pluginJsonCheckResult,

					IconPng:    *iconPngCheckResult,
					PreviewPng: *previewPngCheckResult,
					ReadmeMd:   *readmeMdCheckResult,
				}
			}
		case templates:
			{
				templateJsonCheckResult, err := checkFileExist(
					repoOwner,
					repoName,
					releaseCheckResult.LatestRelease.Hash,
					FILE_PATH_TEMPLATE_JSON,
				)
				if err != nil {
					logger.Warn(err.Error())
				}

				filesCheckResult = &TemplateFiles{
					Pass: templateJsonCheckResult.Pass &&
						iconPngCheckResult.Pass &&
						previewPngCheckResult.Pass &&
						readmeMdCheckResult.Pass,

					TemplateJson: *templateJsonCheckResult,

					IconPng:    *iconPngCheckResult,
					PreviewPng: *previewPngCheckResult,
					ReadmeMd:   *readmeMdCheckResult,
				}
			}
		case themes:
			{
				themeJsonCheckResult, err := checkFileExist(
					repoOwner,
					repoName,
					releaseCheckResult.LatestRelease.Hash,
					FILE_PATH_THEME_JSON,
				)
				if err != nil {
					logger.Warn(err.Error())
				}

				filesCheckResult = &ThemeFiles{
					Pass: themeJsonCheckResult.Pass &&
						iconPngCheckResult.Pass &&
						previewPngCheckResult.Pass &&
						readmeMdCheckResult.Pass,

					ThemeJson: *themeJsonCheckResult,

					IconPng:    *iconPngCheckResult,
					PreviewPng: *previewPngCheckResult,
					ReadmeMd:   *readmeMdCheckResult,
				}
			}
		case widgets:
			{
				widgetJsonCheckResult, err := checkFileExist(
					repoOwner,
					repoName,
					releaseCheckResult.LatestRelease.Hash,
					FILE_PATH_WIDGET_JSON,
				)
				if err != nil {
					logger.Warn(err.Error())
				}

				filesCheckResult = &WidgetFiles{
					Pass: widgetJsonCheckResult.Pass &&
						iconPngCheckResult.Pass &&
						previewPngCheckResult.Pass &&
						readmeMdCheckResult.Pass,

					WidgetJson: *widgetJsonCheckResult,

					IconPng:    *iconPngCheckResult,
					PreviewPng: *previewPngCheckResult,
					ReadmeMd:   *readmeMdCheckResult,
				}
			}
		default:
			logger.Errorf("repo [%s] invalid resourceType: %d", repoPath, resourceType)
			return
		}

		// 返回检查结果
		switch resourceType {
		case icons:
			resultChannel <- &Icon{
				RepoInfo: *repoInfo,
				Release:  *releaseCheckResult,
				Files:    *filesCheckResult.(*IconFiles),
				Attrs:    *attrsCheckResult,
			}
		case plugins:
			resultChannel <- &Plugin{
				RepoInfo: *repoInfo,
				Release:  *releaseCheckResult,
				Files:    *filesCheckResult.(*PluginFiles),
				Attrs:    *attrsCheckResult,
			}
		case templates:
			resultChannel <- &Template{
				RepoInfo: *repoInfo,
				Release:  *releaseCheckResult,
				Files:    *filesCheckResult.(*TemplateFiles),
				Attrs:    *attrsCheckResult,
			}
		case themes:
			resultChannel <- &Theme{
				RepoInfo: *repoInfo,
				Release:  *releaseCheckResult,
				Files:    *filesCheckResult.(*ThemeFiles),
				Attrs:    *attrsCheckResult,
			}
		case widgets:
			resultChannel <- &Widget{
				RepoInfo: *repoInfo,
				Release:  *releaseCheckResult,
				Files:    *filesCheckResult.(*WidgetFiles),
				Attrs:    *attrsCheckResult,
			}
		}
	} else {
		// 无法检查文件与属性, 直接返回结果
		switch resourceType {
		case icons:
			resultChannel <- &Icon{
				RepoInfo: *repoInfo,
				Release:  *releaseCheckResult,
			}
		case plugins:
			resultChannel <- &Plugin{
				RepoInfo: *repoInfo,
				Release:  *releaseCheckResult,
			}
		case templates:
			resultChannel <- &Template{
				RepoInfo: *repoInfo,
				Release:  *releaseCheckResult,
			}
		case themes:
			resultChannel <- &Theme{
				RepoInfo: *repoInfo,
				Release:  *releaseCheckResult,
			}
		case widgets:
			resultChannel <- &Widget{
				RepoInfo: *repoInfo,
				Release:  *releaseCheckResult,
			}
		default:
		}
	}

	logger.Infof("finish repo check [%s]", repoPath)
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
		releaseCheckResult.LatestRelease.Hash = githubReference.GetObject().GetSHA()
	case "tag":
		tagSha := githubReference.GetObject().GetSHA()

		// 获取 commit hash
		// REF https://pkg.go.dev/github.com/google/go-github/v52/github#GitService.GetTag
		githubTag, _, err := githubClient.Git.GetTag(githubContext, repoOwner, repoName, tagSha)
		if nil != err {
			logger.Warnf("get repo [%s/%s] tag [%s:%s] failed: %s", repoOwner, repoName, releaseCheckResult.LatestRelease.Tag, tagSha, err)
			return
		}

		releaseCheckResult.LatestRelease.Hash = githubTag.GetObject().GetSHA()
	default:
		logger.Warnf("parse repo [%s/%s] reference tag [%s] failed: unknown type [%s]", repoOwner, repoName, releaseCheckResult.LatestRelease.Tag, referenceType)
		return
	}

	releaseCheckResult.Pass = releaseCheckResult.LatestRelease.Pass &&
		releaseCheckResult.LatestRelease.PackageZip.Pass // 通过发行版检查

	return
}

// checkFileExist 检查文件是否存在
func checkFileExist(
	repoOwner string,
	repoName string,
	hash string,
	filePath string,
) (
	fileCheckResult *File,
	err error,
) {
	fileCheckResult = &File{}
	rawUrl := buildFileRawURL(
		repoOwner,
		repoName,
		hash,
		filePath,
	) // 文件访问地址
	fileCheckResult.URL = buildFilePreviewURL(
		repoOwner,
		repoName,
		hash,
		filePath,
	) // 文件预览地址

	response, data, errs := gorequest.
		New().
		Head(rawUrl).
		Set("User-Agent", util.UserAgent).
		Retry(REQUEST_RETRY_COUNT, REQUEST_RETRY_DURATION).
		Timeout(REQUEST_TIMEOUT).
		End()
	if nil != errs {
		logger.Fatalf("HTTP HEAD request [%s] failed: %s", rawUrl, errs)
		panic(errs)
	}
	if response.StatusCode == http.StatusOK {
		if strings.HasSuffix(filePath, ".png") && 0 < len(data) {
			if strings.HasSuffix(filePath, "icon.png") {
				// 图标大小 160*160
				img, _, decodeErr := image.DecodeConfig(strings.NewReader(data))
				if decodeErr != nil {
					logger.Warnf("check icon.png file [%s] size failed: %s", rawUrl, decodeErr)
				} else {
					if img.Width != 160 || img.Height != 160 {
						logger.Warnf("icon.png file [%s] size is not 160x160", rawUrl)
						fileCheckResult.Pass = false
						return
					}
				}
			} else if strings.HasSuffix(filePath, "preview.png") {
				// 预览图大小 1024*768
				img, _, decodeErr := image.DecodeConfig(strings.NewReader(data))
				if decodeErr != nil {
					logger.Warnf("check preview.png file [%s] size failed: %s", rawUrl, decodeErr)
				} else {
					if img.Width != 1024 || img.Height != 768 {
						logger.Warnf("preview.png file [%s] size is not 1024x768", rawUrl)
						fileCheckResult.Pass = false
						return
					}
				}
			}
		}

		fileCheckResult.Pass = true
		return
	} else if response.StatusCode == http.StatusNotFound {
		fileCheckResult.Pass = false
		return
	}

	err = fmt.Errorf("HTTP HEAD request [%s] failed: %s", rawUrl, response.Status)
	return
}

// checkManifestAttrs 检查清单属性
func checkManifestAttrs(fileURL string) (attrsCheckResult *Attrs, err error) {
	attrsCheckResult = &Attrs{}
	response, data, errs := gorequest.
		New().
		Get(fileURL).
		Set("User-Agent", util.UserAgent).
		Retry(REQUEST_RETRY_COUNT, REQUEST_RETRY_DURATION).
		Timeout(REQUEST_TIMEOUT).
		EndBytes()
	if nil != errs {
		logger.Fatalf("HTTP Get request [%s] failed: %s", fileURL, errs)
		panic(errs)
	}
	if response.StatusCode != http.StatusOK {
		err = fmt.Errorf("HTTP HEAD request [%s] failed: %s", fileURL, response.Status)
		return
	}

	// 解析清单
	manifest := map[string]interface{}{}
	if err = gulu.JSON.UnmarshalJSON(data, &manifest); nil != err {
		return
	}

	// 检查清单文件
	if name := manifest["name"]; name != nil {
		if value := name.(string); value != "" {
			attrsCheckResult.Name.Value = value
			attrsCheckResult.Name.Exist = true
		}
	}
	if version := manifest["version"]; version != nil {
		if value := version.(string); value != "" {
			attrsCheckResult.Version.Value = value
			attrsCheckResult.Version.Pass = true
		}
	}
	if author := manifest["author"]; author != nil {
		if value := author.(string); value != "" {
			attrsCheckResult.Author.Value = value
			attrsCheckResult.Author.Pass = true
		}
	}
	if url := manifest["url"]; url != nil {
		if value := url.(string); value != "" {
			attrsCheckResult.URL.Value = value
			attrsCheckResult.URL.Pass = true
		}
	}
	return
}
