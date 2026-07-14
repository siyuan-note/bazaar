## Pull Request Check Report

{{ if .ParseError }}
### Package list parse error

{{ .ParseError }}

Please fix the corresponding `.txt` file in the repo root: one `owner/repo` per line. Then push a new commit or re-run PR Check.

---
{{ end }}
{{ define "repoCheck" }}
#### [{{ .RepoInfo.Path }}]({{ .RepoInfo.Home }}){{ if .MaintainerChanged }} (Change Maintainer){{ end }}

{{ range $j, $issue := .Issues }}
{{ issueIndex $j }} [{{ $issue.Rule }}]

{{ $issue.MessageZh }}

{{ $issue.MessageEn }}

---
{{ end }}
{{ if not .Issues }}
Check passed.{{ if .Release.URL }} Latest Release: [{{ .Release.Tag }}]({{ .Release.URL }}){{ end }}

---
{{ end }}
{{ end }}
{{ if .Icons }}
### Add `{{ len .Icons }}` Icon Repo

{{ range .Icons }}{{ template "repoCheck" . }}{{ end }}
{{ end }}
{{ if .IconsDeleted }}

### Remove `{{ len .IconsDeleted }}` Icon Repo

{{ range .IconsDeleted }}
- [{{ . }}](https://github.com/{{ . }})
{{ end }}
{{ end }}

{{ if .Plugins }}
### Add `{{ len .Plugins }}` Plugin Repo

{{ range .Plugins }}{{ template "repoCheck" . }}{{ end }}
{{ end }}
{{ if .PluginsDeleted }}

### Remove `{{ len .PluginsDeleted }}` Plugin Repo

{{ range .PluginsDeleted }}
- [{{ . }}](https://github.com/{{ . }})
{{ end }}
{{ end }}

{{ if .Templates }}
### Add `{{ len .Templates }}` Template Repo

{{ range .Templates }}{{ template "repoCheck" . }}{{ end }}
{{ end }}
{{ if .TemplatesDeleted }}

### Remove `{{ len .TemplatesDeleted }}` Template Repo

{{ range .TemplatesDeleted }}
- [{{ . }}](https://github.com/{{ . }})
{{ end }}
{{ end }}

{{ if .Themes }}
### Add `{{ len .Themes }}` Theme Repo

{{ range .Themes }}{{ template "repoCheck" . }}{{ end }}
{{ end }}
{{ if .ThemesDeleted }}

### Remove `{{ len .ThemesDeleted }}` Theme Repo

{{ range .ThemesDeleted }}
- [{{ . }}](https://github.com/{{ . }})
{{ end }}
{{ end }}

{{ if .Widgets }}
### Add `{{ len .Widgets }}` Widget Repo

{{ range .Widgets }}{{ template "repoCheck" . }}{{ end }}
{{ end }}
{{ if .WidgetsDeleted }}

### Remove `{{ len .WidgetsDeleted }}` Widget Repo

{{ range .WidgetsDeleted }}
- [{{ . }}](https://github.com/{{ . }})
{{ end }}
{{ end }}

{{ if and (not .Icons) (not .Plugins) (not .Templates) (not .Themes) (not .Widgets) (not .IconsDeleted) (not .PluginsDeleted) (not .TemplatesDeleted) (not .ThemesDeleted) (not .WidgetsDeleted) }}
No actual changes to the bazaar package list (or changes are already in main). Please check your commit.
{{ end }}
