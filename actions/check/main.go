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
	获取配置文件 *.json
		检查配置文件是否具有必要的字段
		检查资源名称是否与 stage/*.json 中的重复
	检查必要的文件是否存在
	生成检查结果并输出文件 (使用 go 模板)
	使用 thollander/actions-comment-pull-request@v2.3.1 将检查结果输出到到 PR 中
*/

var (
	PR_REPO_PATH                  = os.Getenv("PR_REPO_PATH")                  // PR 仓库目录路径
	GITHUB_TOKEN                  = os.Getenv("PAT")                           // GitHub Token
	FILE_PATH_CHECK_RESULT_OUTPUT = os.Getenv("FILE_PATH_CHECK_RESULT_OUTPUT") // 检查结果输出文件路径

	REQUEST_TIMEOUT        = 30 * time.Second // 请求超时时间
	REQUEST_RETRY_COUNT    = 3                // 请求重试次数
	REQUEST_RETRY_DURATION = 10 * time.Second // 请求重试间隔时间

	logger         = gulu.Log.NewLogger(os.Stdout)
	github_contest = context.Background()
	github_client  = github.NewTokenClient(github_contest, GITHUB_TOKEN)
)

func main() {
	logger.Infof("PR Check running...")

	/* 获取检查结果模板文件 */
	check_result_template, err := template.ParseFiles(FILE_PATH_CHECK_RESULT_TEMPLATE)
	if nil != err {
		logger.Fatalf("load check result template file <\033[7m%s\033[0m> failed: %s", FILE_PATH_CHECK_RESULT_TEMPLATE, err)
		panic(err)
	}

	/* 打开检查结果输出文件 */
	check_result_output_file, err := os.OpenFile(FILE_PATH_CHECK_RESULT_OUTPUT, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if nil != err {
		logger.Fatalf("open check result output file <\033[7m%s\033[0m> failed: %s", FILE_PATH_CHECK_RESULT_OUTPUT, err)
		panic(err)
	}

	check_result := &CheckResult{
		Icons:     []Icon{},
		Plugins:   []Plugin{},
		Templates: []Template{},
		Themes:    []Theme{},
		Widgets:   []Widget{},
	} // 检查结果

	github_client.Client().Timeout = REQUEST_TIMEOUT // 设置请求超时时间

	wait_group := &sync.WaitGroup{}
	wait_group.Add(5)

	go checkRepos(PR_REPO_PATH+"/icons.json", "./stage/icons.json", icons, check_result, wait_group)
	go checkRepos(PR_REPO_PATH+"/plugins.json", "./stage/plugins.json", plugins, check_result, wait_group)
	go checkRepos(PR_REPO_PATH+"/templates.json", "./stage/templates.json", templates, check_result, wait_group)
	go checkRepos(PR_REPO_PATH+"/themes.json", "./stage/themes.json", themes, check_result, wait_group)
	go checkRepos(PR_REPO_PATH+"/widgets.json", "./stage/widgets.json", widgets, check_result, wait_group)

	wait_group.Wait() // 等待所有检查完成

	/* 将检查结果写入文件 */
	// check_result_template.Execute(check_result_output_file, CheckResultTestExample)
	check_result_template.Execute(check_result_output_file, check_result)

	logger.Infof("PR Check finished")
}

/* 检查集市资源仓库列表 */
func checkRepos(
	targetFilePath,
	originFilePath string,
	resourceType ResourceType,
	checkResult *CheckResult,
	waitGroup *sync.WaitGroup,
) {
	defer waitGroup.Done()
	logger.Infof("start repes check <\033[7m%s\033[0m>", targetFilePath)

	/* 读取 PR 中的文件 */
	target_file, err := os.ReadFile(targetFilePath)
	if nil != err {
		logger.Fatalf("read file <\033[7m%s\033[0m> failed: %s", targetFilePath, err)
		panic(err)
	}
	target := map[string]interface{}{}
	if err = gulu.JSON.UnmarshalJSON(target_file, &target); nil != err {
		logger.Fatalf("unmarshal file <\033[7m%s\033[0m> failed: %s", targetFilePath, err)
		panic(err)
	}

	/* 读取 stage 中的文件 */
	origin_file, err := os.ReadFile(originFilePath)
	if nil != err {
		logger.Fatalf("read file <\033[7m%s\033[0m> failed: %s", originFilePath, err)
		panic(err)
	}
	origin := map[string]interface{}{}
	if err = gulu.JSON.UnmarshalJSON(origin_file, &origin); nil != err {
		logger.Fatalf("unmarshal file <\033[7m%s\033[0m> failed: %s", originFilePath, err)
		panic(err)
	}

	/* 获取新增的仓库列表 */
	target_repos := target["repos"].([]interface{})       // PR 中的仓库列表
	origin_repos := origin["repos"].([]interface{})       // stage 中的仓库列表
	origin_repo_set := make(StringSet, len(origin_repos)) // stage 中的仓库 owner/name 集合
	origin_name_set := make(StringSet, len(origin_repos)) // stage 中的 name 字段集合
	for _, origin_repo := range origin_repos {
		origin_url := origin_repo.(map[string]interface{})["url"].(string)
		origin_repo_set[strings.Split(origin_url, "@")[0]] = nil

		origin_name := origin_repo.(map[string]interface{})["package"].(map[string]interface{})["name"].(string)
		origin_name_set[origin_name] = nil
	}

	new_repos := []string{} // 新增的仓库列表
	for _, target_repo := range target_repos {
		target_repo_path := target_repo.(string)
		if !isKeyInSet(target_repo_path, origin_repo_set) {
			new_repos = append(new_repos, target_repo_path)
		}
	}

	/* 检查每个集市资源仓库 */
	result_channel := make(chan interface{}, 4) // 检查结果输出通道
	wait_group_check := &sync.WaitGroup{}       // 等待所有检查完成
	wait_group_result := &sync.WaitGroup{}      // 等待所有检查结果处理完成

	wait_group_result.Add(1)
	go func() { // 处理输出地检查结果
		for result := range result_channel {
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
		wait_group_result.Done()
	}()

	for _, new_repo := range new_repos {
		wait_group_check.Add(1)
		go checkRepo(new_repo, origin_name_set, resourceType, result_channel, wait_group_check) // 检查每个仓库
	}
	wait_group_check.Wait()  // 等待检查完成
	close(result_channel)    // 关闭检查结果输出通道
	wait_group_result.Wait() // 等待检查结果处理完成
	logger.Infof("finish repos check <\033[7m%s\033[0m>", targetFilePath)
}

/* 检查集市资源仓库 */
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

	/* 检查 latest release */
	repo_meta := strings.Split(repoPath, "/")
	repo_owner := repo_meta[0]
	repo_name := repo_meta[1]
	repo_info := &RepoInfo{
		Owner: repo_owner,
		Name:  repo_name,
		Path:  repoPath,
		Home:  buildRepoHomeURL(repo_owner, repo_name),
	}
	release_check_result := checkRepoLatestRelease(repo_owner, repo_name)

	if release_check_result.LatestRelease.Hash != "" {
		/* 获得 latest release 成功, 可以进一步检查文件与属性 */

		/* 检查清单文件中的属性 */
		var attrs_check_result *Attrs
		var manifest_file_path string // 清单文件路径
		switch resourceType {
		case icons:
			manifest_file_path = "icon.json"
		case plugins:
			manifest_file_path = "plugin.json"
		case templates:
			manifest_file_path = "template.json"
		case themes:
			manifest_file_path = "theme.json"
		case widgets:
			manifest_file_path = "widget.json"
		default:
		}
		manifest_file_url := buildFileDownloadURL(
			repo_owner,
			repo_name,
			release_check_result.LatestRelease.Hash,
			manifest_file_path,
		) // 清单文件下载地址

		if attrs_check_result, err = checkManifestAttrs(manifest_file_url); err != nil {
			logger.Warnf("check repo <\033[7m%s\033[0m> manifest file <\033[7m%s\033[0m> failed: %s", repoPath, manifest_file_url, err)
		} else {
			if attrs_check_result.Name.Value != "" {
				/* 字段唯一性检查 */
				if isKeyInSet(attrs_check_result.Name.Value, nameSet) {
					logger.Warnf("repo <\033[7m%s\033[0m> name <\033[7m%s\033[0m> already exists", repoPath, attrs_check_result.Name.Value)
				} else {
					nameSet[attrs_check_result.Name.Value] = nil // 新的 name 添加到检查集合中
					attrs_check_result.Name.Unique = true        // name 字段通过唯一性检查
					attrs_check_result.Name.Pass = true          // name 字段通过检查

					attrs_check_result.Pass = attrs_check_result.Name.Pass &&
						attrs_check_result.Version.Pass &&
						attrs_check_result.Author.Pass &&
						attrs_check_result.URL.Pass
				}
			}
		}

		/* 检查文件 */
		var files_check_result interface{} // 文件检查结果

		/* 检查所有类型集市资源必要的文件 */
		icon_png_check_result, err := checkFileExist(buildFileDownloadURL(
			repo_owner,
			repo_name,
			release_check_result.LatestRelease.Hash,
			FILE_PATH_ICON_PNG,
		))
		if err != nil {
			logger.Warn(err.Error())
		}

		preview_png_check_result, err := checkFileExist(buildFileDownloadURL(
			repo_owner,
			repo_name,
			release_check_result.LatestRelease.Hash,
			FILE_PATH_PREVIEW_PNG,
		))
		if err != nil {
			logger.Warn(err.Error())
		}

		readme_md_check_result, err := checkFileExist(buildFileDownloadURL(
			repo_owner,
			repo_name,
			release_check_result.LatestRelease.Hash,
			FILE_PATH_README_MD,
		))
		if err != nil {
			logger.Warn(err.Error())
		}

		/* 检查各类型集市资源其他必要的文件 */
		switch resourceType {
		case icons:
			{
				icon_json_check_result, err := checkFileExist(buildFileDownloadURL(
					repo_owner,
					repo_name,
					release_check_result.LatestRelease.Hash,
					FILE_PATH_ICON_JSON,
				))
				if err != nil {
					logger.Warn(err.Error())
				}

				files_check_result = &IconFiles{
					Pass: icon_json_check_result.Pass &&
						icon_png_check_result.Pass &&
						preview_png_check_result.Pass &&
						readme_md_check_result.Pass,

					IconJson: *icon_json_check_result,

					IconPng:    *icon_png_check_result,
					PreviewPng: *preview_png_check_result,
					ReadmeMd:   *readme_md_check_result,
				}
			}
		case plugins:
			{
				plugin_json_check_result, err := checkFileExist(buildFileDownloadURL(
					repo_owner,
					repo_name,
					release_check_result.LatestRelease.Hash,
					FILE_PATH_PLUGIN_JSON,
				))
				if err != nil {
					logger.Warn(err.Error())
				}

				files_check_result = &PluginFiles{
					Pass: plugin_json_check_result.Pass &&
						icon_png_check_result.Pass &&
						preview_png_check_result.Pass &&
						readme_md_check_result.Pass,

					PluginJson: *plugin_json_check_result,

					IconPng:    *icon_png_check_result,
					PreviewPng: *preview_png_check_result,
					ReadmeMd:   *readme_md_check_result,
				}
			}
		case templates:
			{
				template_json_check_result, err := checkFileExist(buildFileDownloadURL(
					repo_owner,
					repo_name,
					release_check_result.LatestRelease.Hash,
					FILE_PATH_TEMPLATE_JSON,
				))
				if err != nil {
					logger.Warn(err.Error())
				}

				files_check_result = &TemplateFiles{
					Pass: template_json_check_result.Pass &&
						icon_png_check_result.Pass &&
						preview_png_check_result.Pass &&
						readme_md_check_result.Pass,

					TemplateJson: *template_json_check_result,

					IconPng:    *icon_png_check_result,
					PreviewPng: *preview_png_check_result,
					ReadmeMd:   *readme_md_check_result,
				}
			}
		case themes:
			{
				theme_json_check_result, err := checkFileExist(buildFileDownloadURL(
					repo_owner,
					repo_name,
					release_check_result.LatestRelease.Hash,
					FILE_PATH_THEME_JSON,
				))
				if err != nil {
					logger.Warn(err.Error())
				}

				files_check_result = &ThemeFiles{
					Pass: theme_json_check_result.Pass &&
						icon_png_check_result.Pass &&
						preview_png_check_result.Pass &&
						readme_md_check_result.Pass,

					ThemeJson: *theme_json_check_result,

					IconPng:    *icon_png_check_result,
					PreviewPng: *preview_png_check_result,
					ReadmeMd:   *readme_md_check_result,
				}
			}
		case widgets:
			{
				widget_json_check_result, err := checkFileExist(buildFileDownloadURL(
					repo_owner,
					repo_name,
					release_check_result.LatestRelease.Hash,
					FILE_PATH_WIDGET_JSON,
				))
				if err != nil {
					logger.Warn(err.Error())
				}

				files_check_result = &WidgetFiles{
					Pass: widget_json_check_result.Pass &&
						icon_png_check_result.Pass &&
						preview_png_check_result.Pass &&
						readme_md_check_result.Pass,

					WidgetJson: *widget_json_check_result,

					IconPng:    *icon_png_check_result,
					PreviewPng: *preview_png_check_result,
					ReadmeMd:   *readme_md_check_result,
				}
			}
		default:
		}

		/* 返回检查结果 */
		switch resourceType {
		case icons:
			resultChannel <- &Icon{
				RepoInfo: *repo_info,
				Release:  *release_check_result,
				Files:    *files_check_result.(*IconFiles),
				Attrs:    *attrs_check_result,
			}
		case plugins:
			resultChannel <- &Plugin{
				RepoInfo: *repo_info,
				Release:  *release_check_result,
				Files:    *files_check_result.(*PluginFiles),
				Attrs:    *attrs_check_result,
			}
		case templates:
			resultChannel <- &Template{
				RepoInfo: *repo_info,
				Release:  *release_check_result,
				Files:    *files_check_result.(*TemplateFiles),
				Attrs:    *attrs_check_result,
			}
		case themes:
			resultChannel <- &Theme{
				RepoInfo: *repo_info,
				Release:  *release_check_result,
				Files:    *files_check_result.(*ThemeFiles),
				Attrs:    *attrs_check_result,
			}
		case widgets:
			resultChannel <- &Widget{
				RepoInfo: *repo_info,
				Release:  *release_check_result,
				Files:    *files_check_result.(*WidgetFiles),
				Attrs:    *attrs_check_result,
			}
		default:
		}
	} else {
		/* 无法检查文件与属性, 直接返回结果 */
		switch resourceType {
		case icons:
			resultChannel <- &Icon{
				RepoInfo: *repo_info,
				Release:  *release_check_result,
			}
		case plugins:
			resultChannel <- &Plugin{
				RepoInfo: *repo_info,
				Release:  *release_check_result,
			}
		case templates:
			resultChannel <- &Template{
				RepoInfo: *repo_info,
				Release:  *release_check_result,
			}
		case themes:
			resultChannel <- &Theme{
				RepoInfo: *repo_info,
				Release:  *release_check_result,
			}
		case widgets:
			resultChannel <- &Widget{
				RepoInfo: *repo_info,
				Release:  *release_check_result,
			}
		default:
		}
	}

	logger.Infof("finish repo check <\033[7m%s\033[0m>", repoPath)
}

/* 检查最新发行信息 */
func checkRepoLatestRelease(
	repoOwner string,
	repoName string,
) (releaseCheckResult *Release) {
	releaseCheckResult = &Release{}

	/* 获取 latest release */
	github_release, _, err := github_client.Repositories.GetLatestRelease(github_contest, repoOwner, repoName)
	if nil != err {
		logger.Warnf("get repo <\033[7m%s/%s\033[0m> latest release failed: %s", repoOwner, repoName, err)
		return
	}

	releaseCheckResult.LatestRelease.Pass = true // 最新发行版存在

	/* 获取 tag 名称 */
	releaseCheckResult.LatestRelease.Tag = github_release.GetTagName()
	releaseCheckResult.LatestRelease.URL = github_release.GetHTMLURL()

	/* 获取 package.zip 下载地址 */
	for _, asset := range github_release.Assets {
		if asset.GetName() == "package.zip" {
			releaseCheckResult.LatestRelease.PackageZip.Pass = true
			releaseCheckResult.LatestRelease.PackageZip.URL = asset.GetBrowserDownloadURL()
			break
		}
	}

	/* 获取 hash */
	// REF https://pkg.go.dev/github.com/google/go-github/v52/github#GitService.GetRef
	github_reference, _, err := github_client.Git.GetRef(github_contest, repoOwner, repoName, "tags/"+releaseCheckResult.LatestRelease.Tag)
	if nil != err {
		logger.Warnf("get repo <\033[7m%s/%s\033[0m> tag <\033[7m%s\033[0m> failed: %s", repoOwner, repoName, releaseCheckResult.LatestRelease.Tag, err)
		return
	}

	releaseCheckResult.LatestRelease.Hash = github_reference.GetObject().GetSHA()
	releaseCheckResult.Pass = releaseCheckResult.LatestRelease.Pass &&
		releaseCheckResult.LatestRelease.PackageZip.Pass // 通过发行版检查

	return
}

/* 检查文件是否存在 */
func checkFileExist(url string) (
	fileCheckResult *File,
	err error,
) {
	fileCheckResult = &File{}
	fileCheckResult.URL = url
	response, _, errs := gorequest.
		New().
		Head(fileCheckResult.URL).
		Set("User-Agent", util.UserAgent).
		Retry(REQUEST_RETRY_COUNT, REQUEST_RETRY_DURATION).
		Timeout(REQUEST_TIMEOUT).
		End()
	if nil != errs {
		logger.Fatalf("HTTP HEAD request <\033[7m%s\033[0m> failed: %s", fileCheckResult.URL, errs)
		panic(errs)
	}
	if response.StatusCode == http.StatusOK {
		fileCheckResult.Pass = true
		return
	} else if response.StatusCode == http.StatusNotFound {
		fileCheckResult.Pass = false
		return
	} else {
		err = fmt.Errorf("HTTP HEAD request <\033[7m%s\033[0m> failed: %s", fileCheckResult.URL, response.Status)
		return
	}
}

/* 检查清单属性 */
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

	/* 解析清单 */
	manifest := map[string]interface{}{}
	if err = gulu.JSON.UnmarshalJSON(data, &manifest); nil != err {
		return
	}

	/* 检查清单文件 */
	if name := manifest["name"]; name != nil {
		if value := name.(string); value != "" {
			attrsCheckResult.Name.Value = value
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
