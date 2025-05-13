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
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/88250/gulu"
	"github.com/google/go-github/v52/github"
	"github.com/parnurzeal/gorequest"
	"github.com/siyuan-note/bazaar/actions/util"
)

/*
检查更改的文件内的仓库是否有不在 stage/*.json 对应文件的仓库列表中
	获取 PR 中 *.json 中的仓库列表
	获取 stage/*.json 中的仓库列表
	判断是否有仅存在 *.json 而不存在 stage/*.json 中的仓库
如果有
	获取仓库最新 release
	获取仓库最新 release 的 tag
	获取仓库最新 release 的 hash
	检查必要的文件是否存在
	获取清单文件 *.json
		检查清单文件是否存在必要的字段
			检查清单中 name 字段值是否与 stage/*.json 中的重复
	生成检查结果并输出文件 (使用 go 模板)
	使用 thollander/actions-comment-pull-request 将检查结果输出到到 PR 中
*/

var (
	PR_REPO_PATH                  = os.Getenv("PR_REPO_PATH")                  // PR 仓库目录路径
	GITHUB_TOKEN                  = os.Getenv("PAT")                           // GitHub Token
	FILE_PATH_CHECK_RESULT_OUTPUT = os.Getenv("FILE_PATH_CHECK_RESULT_OUTPUT") // 检查结果输出文件路径

	REQUEST_TIMEOUT        = 30 * time.Second // 请求超时时间
	REQUEST_RETRY_COUNT    = 3                // 请求重试次数
	REQUEST_RETRY_DURATION = 10 * time.Second // 请求重试间隔时间

	logger        = gulu.Log.NewLogger(os.Stdout)
	githubContest = context.Background()
	githubClient  = github.NewTokenClient(githubContest, GITHUB_TOKEN)
)

func main() {
	logger.Infof("PR Check running...")

	// 获取检查结果模板文件
	checkResultTemplate, err := template.ParseFiles(FILE_PATH_CHECK_RESULT_TEMPLATE)
	if nil != err {
		logger.Fatalf("load check result template file <\033[7m%s\033[0m> failed: %s", FILE_PATH_CHECK_RESULT_TEMPLATE, err)
		panic(err)
	}

	// 打开检查结果输出文件
	checkResultOutputFile, err := os.OpenFile(FILE_PATH_CHECK_RESULT_OUTPUT, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if nil != err {
		logger.Fatalf("open check result output file <\033[7m%s\033[0m> failed: %s", FILE_PATH_CHECK_RESULT_OUTPUT, err)
		panic(err)
	}

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

	go checkRepos(PR_REPO_PATH+"/icons.json", "./stage/icons.json", icons, checkResult, wg)
	go checkRepos(PR_REPO_PATH+"/plugins.json", "./stage/plugins.json", plugins, checkResult, wg)
	go checkRepos(PR_REPO_PATH+"/templates.json", "./stage/templates.json", templates, checkResult, wg)
	go checkRepos(PR_REPO_PATH+"/themes.json", "./stage/themes.json", themes, checkResult, wg)
	go checkRepos(PR_REPO_PATH+"/widgets.json", "./stage/widgets.json", widgets, checkResult, wg)

	wg.Wait() // 等待所有检查完成

	// 将检查结果写入文件
	checkResultTemplate.Execute(checkResultOutputFile, checkResult)

	logger.Infof("PR Check finished")
}

// checkRepos 检查集市资源仓库列表
func checkRepos(
	targetFilePath string,
	originFilePath string,
	resourceType ResourceType,
	checkResult *CheckResult,
	waitGroup *sync.WaitGroup,
) {
	defer waitGroup.Done()
	logger.Infof("start repes check <\033[7m%s\033[0m>", targetFilePath)

	// 读取 PR 中的文件
	targetFile, err := os.ReadFile(targetFilePath)
	if nil != err {
		logger.Fatalf("read file <\033[7m%s\033[0m> failed: %s", targetFilePath, err)
		panic(err)
	}
	target := map[string]interface{}{}
	if err = gulu.JSON.UnmarshalJSON(targetFile, &target); nil != err {
		logger.Fatalf("unmarshal file <\033[7m%s\033[0m> failed: %s", targetFilePath, err)
		panic(err)
	}

	// 读取 stage 中的文件
	originFile, err := os.ReadFile(originFilePath)
	if nil != err {
		logger.Fatalf("read file <\033[7m%s\033[0m> failed: %s", originFilePath, err)
		panic(err)
	}
	origin := map[string]interface{}{}
	if err = gulu.JSON.UnmarshalJSON(originFile, &origin); nil != err {
		logger.Fatalf("unmarshal file <\033[7m%s\033[0m> failed: %s", originFilePath, err)
		panic(err)
	}

	// 获取新增的仓库列表
	targetRepos := target["repos"].([]interface{})     // PR 中的仓库列表
	originRepos := origin["repos"].([]interface{})     // stage 中的仓库列表
	originRepoSet := make(StringSet, len(originRepos)) // stage 中的仓库 owner/name 集合
	originNameSet := make(StringSet, len(originRepos)) // stage 中的 name 字段集合
	for _, originRepo := range originRepos {
		originUrl := originRepo.(map[string]interface{})["url"].(string)
		originUrl = strings.Split(originUrl, "@")[0]
		originUrl = strings.ToLower(originUrl)
		originRepoSet[originUrl] = nil

		originPackage := originRepo.(map[string]interface{})["package"]
		if originPackage != nil {
			originPackageName := originPackage.(map[string]interface{})["name"]
			if originPackageName != nil {
				originName := strings.ToLower(originPackageName.(string))
				originNameSet[originName] = nil
			}
		}
	}

	newRepos := []string{} // 新增的仓库列表
	for _, targetRepo := range targetRepos {
		targetRepoPath := targetRepo.(string)
		targetRepoPath = strings.ToLower(targetRepoPath)
		if !isKeyInSet(targetRepoPath, originRepoSet) {
			newRepos = append(newRepos, targetRepoPath)
		}
	}

	// 检查每个集市资源仓库
	resultChannel := make(chan interface{}, 4) // 检查结果输出通道
	waitGroupCheck := &sync.WaitGroup{}        // 等待所有检查完成
	waitGroupResult := &sync.WaitGroup{}       // 等待所有检查结果处理完成

	waitGroupResult.Add(1)
	// 处理输出地检查结果
	go func() {
		for result := range resultChannel {
			switch result.(type) {
			case *Icon:
				checkResult.Icons = append(checkResult.Icons, *result.(*Icon))
			case *Plugin:
				checkResult.Plugins = append(checkResult.Plugins, *result.(*Plugin))
			case *Template:
				checkResult.Templates = append(checkResult.Templates, *result.(*Template))
			case *Theme:
				checkResult.Themes = append(checkResult.Themes, *result.(*Theme))
			case *Widget:
				checkResult.Widgets = append(checkResult.Widgets, *result.(*Widget))
			default:
			}
		}
		waitGroupResult.Done()
	}()

	for _, new_repo := range newRepos {
		waitGroupCheck.Add(1)
		go checkRepo(new_repo, originNameSet, resourceType, resultChannel, waitGroupCheck) // 检查每个仓库
	}
	waitGroupCheck.Wait()  // 等待检查完成
	close(resultChannel)   // 关闭检查结果输出通道
	waitGroupResult.Wait() // 等待检查结果处理完成
	logger.Infof("finish repos check <\033[7m%s\033[0m>", targetFilePath)
}

// checkRepo 检查集市资源仓库
func checkRepo(
	repoPath string,
	nameSet StringSet,
	resourceType ResourceType,
	resultChannel chan interface{},
	waitGroup *sync.WaitGroup,
) {
	defer waitGroup.Done()

	logger.Infof("start repo check <\033[7m%s\033[0m>", repoPath)
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
			logger.Warnf("check repo <\033[7m%s\033[0m> manifest file <\033[7m%s\033[0m> failed: %s", repoPath, manifestFileUrl, err)
		} else {
			// 有效性检查
			attrsCheckResult.Name.Valid = isValidName(attrsCheckResult.Name.Value)
			if attrsCheckResult.Name.Valid {
				// name 必须和 repo name 一致
				attrsCheckResult.Name.Valid = attrsCheckResult.Name.Value == repoName
			}

			// 唯一性检查
			if attrsCheckResult.Name.Valid {
				name := strings.ToLower(attrsCheckResult.Name.Value)
				if isKeyInSet(name, nameSet) {
					logger.Warnf("repo <\033[7m%s\033[0m> name <\033[7m%s\033[0m> already exists", repoPath, name)
				} else {
					nameSet[name] = nil // 新的 name 添加到检查集合中

					attrsCheckResult.Name.Unique = true // name 通过唯一性检查
				}
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
		default:
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

	logger.Infof("finish repo check <\033[7m%s\033[0m>", repoPath)
}

// checkRepoLatestRelease 检查最新发行信息
func checkRepoLatestRelease(
	repoOwner string,
	repoName string,
) (releaseCheckResult *Release) {
	releaseCheckResult = &Release{}

	// 获取 latest release
	githubRelease, _, err := githubClient.Repositories.GetLatestRelease(githubContest, repoOwner, repoName)
	if nil != err {
		logger.Warnf("get repo <\033[7m%s/%s\033[0m> latest release failed: %s", repoOwner, repoName, err)
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
	githubReference, _, err := githubClient.Git.GetRef(githubContest, repoOwner, repoName, "tags/"+releaseCheckResult.LatestRelease.Tag)
	if nil != err {
		logger.Warnf("get repo <\033[7m%s/%s\033[0m> reference tag <\033[7m%s\033[0m> failed: %s", repoOwner, repoName, releaseCheckResult.LatestRelease.Tag, err)
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
		githubTag, _, err := githubClient.Git.GetTag(githubContest, repoOwner, repoName, tagSha)
		if nil != err {
			logger.Warnf("get repo <\033[7m%s/%s\033[0m> tag <\033[7m%s:%s\033[0m> failed: %s", repoOwner, repoName, releaseCheckResult.LatestRelease.Tag, tagSha, err)
			return
		}

		releaseCheckResult.LatestRelease.Hash = githubTag.GetObject().GetSHA()
	default:
		logger.Warnf("parse repo <\033[7m%s/%s\033[0m> reference tag <\033[7m%s\033[0m> failed: unknown type <\033[7m%s\033[0m>", repoOwner, repoName, releaseCheckResult.LatestRelease.Tag, referenceType)
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
		logger.Fatalf("HTTP HEAD request <\033[7m%s\033[0m> failed: %s", rawUrl, errs)
		panic(errs)
	}
	if response.StatusCode == http.StatusOK {
		if strings.HasSuffix(filePath, ".png") && 0 < len(data) {
			if strings.HasSuffix(filePath, "icon.png") {
				// 图标大小 160*160
				img, _, decodeErr := image.DecodeConfig(strings.NewReader(data))
				if decodeErr != nil {
					logger.Warnf("check icon.png file <\033[7m%s\033[0m> size failed: %s", rawUrl, decodeErr)
				} else {
					if img.Width != 160 || img.Height != 160 {
						logger.Warnf("icon.png file <\033[7m%s\033[0m> size is not 160x160", rawUrl)
						fileCheckResult.Pass = false
						return
					}
				}
			} else if strings.HasSuffix(filePath, "preview.png") {
				// 预览图大小 1024*768
				img, _, decodeErr := image.DecodeConfig(strings.NewReader(data))
				if decodeErr != nil {
					logger.Warnf("check preview.png file <\033[7m%s\033[0m> size failed: %s", rawUrl, decodeErr)
				} else {
					if img.Width != 1024 || img.Height != 768 {
						logger.Warnf("preview.png file <\033[7m%s\033[0m> size is not 1024x768", rawUrl)
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
	} else {
		err = fmt.Errorf("HTTP HEAD request <\033[7m%s\033[0m> failed: %s", rawUrl, response.Status)
		return
	}
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
		logger.Fatalf("HTTP Get request <\033[7m%s\033[0m> failed: %s", fileURL, errs)
		panic(errs)
	}
	if response.StatusCode != http.StatusOK {
		err = fmt.Errorf("HTTP HEAD request <\033[7m%s\033[0m> failed: %s", fileURL, response.Status)
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
