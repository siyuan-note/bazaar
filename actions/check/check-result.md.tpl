## 拉取请求自动化检查 / Pull Request Automated Check

{{- if .ParseError }}

### 包列表解析错误 / Package list parse error

{{ .ParseError }}

请修复仓库根目录下对应的 `.txt` 文件，确保每行包含一个 `owner/repo`，然后推送新提交。

Please fix the matching `.txt` file at the repo root so each line is one `owner/repo`, then push a new commit.

---
{{- end }}

{{- if .FlowError }}

### 流程规则未通过 / Flow rule failed

{{ .FlowError }}
---
{{- end }}

{{- define "repoCheck" }}
#### [{{ .RepoInfo.Path }}]({{ .RepoInfo.Home }}){{ if .MaintainerChanged }} (更换维护者 / Change Maintainer){{ end }}

  {{- if .MaintainerChanged }}

检测到更换维护者。请阅读流程说明，并在本 PR 中 `@` 原维护者请求确认后才会合并：

[更换维护者]({{ bazaarDocURL "README.zh-CN.md" "更换维护者" }})

This PR changes the package maintainer. Please read the process guide and `@` the original maintainer in this PR for confirmation before merge:

[Changing maintainers]({{ bazaarDocURL "README.md" "changing-maintainers" }})

  {{- end }}

  {{- if .Release.URL }}

最新 Release / Latest Release: [{{ .Release.Tag }}]({{ .Release.URL }})
  {{- end }}

  {{- if .Issues }}

检测到以下问题，请在修复之后重新打包 `package.zip` 发布新的 Release，并将 Release 标记为 Latest。

We found the following issues. Please fix them, rebuild `package.zip`, publish a new Release, and mark that Release as Latest.

---
  {{- end }}

  {{- $issues := .Issues }}
  {{- range $j, $issue := $issues }}

[{{ issueIndex $j (len $issues) }}]

{{ $issue.MessageZh }}

{{ $issue.MessageEn }}

---
  {{- end }}

  {{- if not .Issues }}

检查通过。

Check passed.

---
  {{- end }}
{{- end }}

{{- /* 一次一包失败时不展示移除列表；通过时移除放在新增前面便于审阅 */ -}}
{{- if not .FlowError }}
{{- if .PluginsDeleted }}

### 移除插件仓库 / Remove Plugin Repo
  {{- range .PluginsDeleted }}
- [{{ . }}](https://github.com/{{ . }})
  {{- end }}
{{- end }}
{{- if .ThemesDeleted }}

### 移除主题仓库 / Remove Theme Repo
  {{- range .ThemesDeleted }}
- [{{ . }}](https://github.com/{{ . }})
  {{- end }}
{{- end }}
{{- if .IconsDeleted }}

### 移除图标仓库 / Remove Icon Repo
  {{- range .IconsDeleted }}
- [{{ . }}](https://github.com/{{ . }})
  {{- end }}
{{- end }}
{{- if .TemplatesDeleted }}

### 移除模板仓库 / Remove Template Repo
  {{- range .TemplatesDeleted }}
- [{{ . }}](https://github.com/{{ . }})
  {{- end }}
{{- end }}
{{- if .WidgetsDeleted }}

### 移除挂件仓库 / Remove Widget Repo
  {{- range .WidgetsDeleted }}
- [{{ . }}](https://github.com/{{ . }})
  {{- end }}
{{- end }}
{{- end }}

{{- if .Plugins }}

### 新增插件仓库 / Add Plugin Repo

  {{- range .Plugins }}
{{ template "repoCheck" . }}
  {{- end }}
{{- end }}

{{- if .Themes }}

### 新增主题仓库 / Add Theme Repo

  {{- range .Themes }}
{{ template "repoCheck" . }}
  {{- end }}
{{- end }}

{{- if .Icons }}

### 新增图标仓库 / Add Icon Repo

  {{- range .Icons }}
{{ template "repoCheck" . }}
  {{- end }}
{{- end }}

{{- if .Templates }}

### 新增模板仓库 / Add Template Repo

  {{- range .Templates }}
{{ template "repoCheck" . }}
  {{- end }}
{{- end }}

{{- if .Widgets }}

### 新增挂件仓库 / Add Widget Repo

  {{- range .Widgets }}
{{ template "repoCheck" . }}
  {{- end }}
{{- end }}

{{- if and (not .ParseError) (not .FlowError) (not .Plugins) (not .Themes) (not .Icons) (not .Templates) (not .Widgets) (not .PluginsDeleted) (not .ThemesDeleted) (not .IconsDeleted) (not .TemplatesDeleted) (not .WidgetsDeleted) }}

集市包列表无实际变更（或变更已在 main 中），请检查你的提交。

There's no actual change to the bazaar package list (or the change is already on main). Please double-check your commit.
{{- end }}
