## 拉取请求检查报告 / Pull Request Check Report

{{- if .ParseError }}

### 包列表解析错误 / Package list parse error

{{ .ParseError }}

请修复仓库根目录下对应的 `.txt` 文件，确保每行包含一个 `owner/repo`，然后推送新提交。

Please fix the corresponding `.txt` file in the repo root, ensuring each line contains one `owner/repo`, then push a new commit.

---
{{- end }}

{{- define "repoCheck" }}
#### [{{ .RepoInfo.Path }}]({{ .RepoInfo.Home }}){{ if .MaintainerChanged }} (更换维护者 / Change Maintainer){{ end }}

  {{- if .Release.URL }}

最新 Release / Latest Release: [{{ .Release.Tag }}]({{ .Release.URL }})
  {{- end }}

  {{- if .Issues }}

检测到以下问题，请在修复之后重新打包 `package.zip` 发布新的 Release，并将 Release 标记为 Latest。

The following issues were found. After fixing them, rebuild `package.zip`, publish a new Release, and mark that Release as Latest.
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

{{- if .Plugins }}

### 新增 `{{ len .Plugins }}` 个插件仓库 / Add `{{ len .Plugins }}` Plugin Repo

  {{- range .Plugins }}
{{ template "repoCheck" . }}
  {{- end }}
{{- end }}

{{- if .PluginsDeleted }}

### 移除 `{{ len .PluginsDeleted }}` 个插件仓库 / Remove `{{ len .PluginsDeleted }}` Plugin Repo
  {{- range .PluginsDeleted }}
- [{{ . }}](https://github.com/{{ . }})
  {{- end }}
{{- end }}

{{- if .Themes }}

### 新增 `{{ len .Themes }}` 个主题仓库 / Add `{{ len .Themes }}` Theme Repo

  {{- range .Themes }}
{{ template "repoCheck" . }}
  {{- end }}
{{- end }}

{{- if .ThemesDeleted }}

### 移除 `{{ len .ThemesDeleted }}` 个主题仓库 / Remove `{{ len .ThemesDeleted }}` Theme Repo
  {{- range .ThemesDeleted }}
- [{{ . }}](https://github.com/{{ . }})
  {{- end }}
{{- end }}

{{- if .Icons }}

### 新增 `{{ len .Icons }}` 个图标仓库 / Add `{{ len .Icons }}` Icon Repo

  {{- range .Icons }}
{{ template "repoCheck" . }}
  {{- end }}
{{- end }}

{{- if .IconsDeleted }}

### 移除 `{{ len .IconsDeleted }}` 个图标仓库 / Remove `{{ len .IconsDeleted }}` Icon Repo
  {{- range .IconsDeleted }}
- [{{ . }}](https://github.com/{{ . }})
  {{- end }}
{{- end }}

{{- if .Templates }}

### 新增 `{{ len .Templates }}` 个模板仓库 / Add `{{ len .Templates }}` Template Repo

  {{- range .Templates }}
{{ template "repoCheck" . }}
  {{- end }}
{{- end }}

{{- if .TemplatesDeleted }}

### 移除 `{{ len .TemplatesDeleted }}` 个模板仓库 / Remove `{{ len .TemplatesDeleted }}` Template Repo
  {{- range .TemplatesDeleted }}
- [{{ . }}](https://github.com/{{ . }})
  {{- end }}
{{- end }}

{{- if .Widgets }}

### 新增 `{{ len .Widgets }}` 个挂件仓库 / Add `{{ len .Widgets }}` Widget Repo

  {{- range .Widgets }}
{{ template "repoCheck" . }}
  {{- end }}
{{- end }}

{{- if .WidgetsDeleted }}

### 移除 `{{ len .WidgetsDeleted }}` 个挂件仓库 / Remove `{{ len .WidgetsDeleted }}` Widget Repo
  {{- range .WidgetsDeleted }}
- [{{ . }}](https://github.com/{{ . }})
  {{- end }}
{{- end }}

{{- if and (not .Plugins) (not .Themes) (not .Icons) (not .Templates) (not .Widgets) (not .PluginsDeleted) (not .ThemesDeleted) (not .IconsDeleted) (not .TemplatesDeleted) (not .WidgetsDeleted) }}

集市包列表无实际变更（或变更已在 main 中），请检查你的提交。

No actual changes to the bazaar package list (or changes are already in main), please check your commit.
{{- end }}
