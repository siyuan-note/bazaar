# PR Check Result:
## Add **`{{ len .Icons }}`** Icon Repo

{{ range $i, $repo := .Icons }}### [{{ $repo.Path }}]({{ $repo.Home }})

- {{ if $repo.Release.Pass }}[x]{{ else }}[ ]{{ end }} Necessary Release
  - {{ if $repo.Release.LatestRelease.Pass }}[x] [Current Latest Release]({{ $repo.Release.LatestRelease.URL }}){{ else }}[ ] Current Latest Release{{ end }}
  - {{ if $repo.Release.LatestRelease.PackageZip.Pass }}[x] [package.zip]({{ $repo.Release.LatestRelease.PackageZip.URL }}){{ else }}[ ] `package.zip`{{ end }}
- {{ if $repo.Files.Pass }}[x]{{ else }}[ ]{{ end }} Necessary Files
  - {{ if $repo.Files.IconJs.Pass }}[x] [{{ $repo.Files.IconJs.Name }}]({{ $repo.Files.IconJs.URL }}){{ else }}[ ] `{{ $repo.Files.IconJs.Name }}`{{ end }}
  - {{ if $repo.Files.IconJson.Pass }}[x] [{{ $repo.Files.IconJson.Name }}]({{ $repo.Files.IconJson.URL }}){{ else }}[ ] `{{ $repo.Files.IconJson.Name }}`{{ end }}
  - {{ if $repo.Files.PreviewPng.Pass }}[x] [{{ $repo.Files.PreviewPng.Name }}]({{ $repo.Files.PreviewPng.URL }}){{ else }}[ ] `{{ $repo.Files.PreviewPng.Name }}`{{ end }}
  - {{ if $repo.Files.ReadmeMd.Pass }}[x] [{{ $repo.Files.ReadmeMd.Name }}]({{ $repo.Files.ReadmeMd.URL }}){{ else }}[ ] `{{ $repo.Files.ReadmeMd.Name }}`{{ end }}
- {{ if $repo.Attrs.Pass }}[x]{{ else }}[ ]{{ end }} Necessary Attributes for `icon.json`
  - {{ if $repo.Attrs.Name.Pass }}[x] `name`: `{{ $repo.Attrs.Name.Value }}`{{ else }}[ ] `name`{{ end }} 
    - {{ if $repo.Attrs.Name.Unique }}[x]{{ else }}[ ]{{ end }} Unique
  - {{ if $repo.Attrs.Version.Pass }}[x] `version`: `{{ $repo.Attrs.Version.Value }}`{{ else }}[ ] `version`{{ end }} 

---
{{ end }}
## Add **`{{ len .Plugins }}`** Plugin Repo

{{ range $i, $repo := .Plugins }}### [{{ $repo.Path }}]({{ $repo.Home }})

- {{ if $repo.Release.Pass }}[x]{{ else }}[ ]{{ end }} Necessary Release
  - {{ if $repo.Release.LatestRelease.Pass }}[x] [Current Latest Release]({{ $repo.Release.LatestRelease.URL }}){{ else }}[ ] Current Latest Release{{ end }}
  - {{ if $repo.Release.LatestRelease.PackageZip.Pass }}[x] [package.zip]({{ $repo.Release.LatestRelease.PackageZip.URL }}){{ else }}[ ] `package.zip`{{ end }}
- {{ if $repo.Files.Pass }}[x]{{ else }}[ ]{{ end }} Necessary Files
  - {{ if $repo.Files.IndexJs.Pass }}[x] [{{ $repo.Files.IndexJs.Name }}]({{ $repo.Files.IndexJs.URL }}){{ else }}[ ] `{{ $repo.Files.IndexJs.Name }}`{{ end }}
  - {{ if $repo.Files.PluginJson.Pass }}[x] [{{ $repo.Files.PluginJson.Name }}]({{ $repo.Files.PluginJson.URL }}){{ else }}[ ] `{{ $repo.Files.PluginJson.Name }}`{{ end }}
  - {{ if $repo.Files.PreviewPng.Pass }}[x] [{{ $repo.Files.PreviewPng.Name }}]({{ $repo.Files.PreviewPng.URL }}){{ else }}[ ] `{{ $repo.Files.PreviewPng.Name }}`{{ end }}
  - {{ if $repo.Files.ReadmeMd.Pass }}[x] [{{ $repo.Files.ReadmeMd.Name }}]({{ $repo.Files.ReadmeMd.URL }}){{ else }}[ ] `{{ $repo.Files.ReadmeMd.Name }}`{{ end }}
- {{ if $repo.Attrs.Pass }}[x]{{ else }}[ ]{{ end }} Necessary Attributes for `plugin.json`
  - {{ if $repo.Attrs.Name.Pass }}[x] `name`: `{{ $repo.Attrs.Name.Value }}`{{ else }}[ ] `name`{{ end }} 
    - {{ if $repo.Attrs.Name.Unique }}[x]{{ else }}[ ]{{ end }} Unique
  - {{ if $repo.Attrs.Version.Pass }}[x] `version`: `{{ $repo.Attrs.Version.Value }}`{{ else }}[ ] `version`{{ end }} 

---
{{ end }}
## Add **`{{ len .Templates }}`** Template Repo

{{ range $i, $repo := .Templates }}### [{{ $repo.Path }}]({{ $repo.Home }})

- {{ if $repo.Release.Pass }}[x]{{ else }}[ ]{{ end }} Necessary Release
  - {{ if $repo.Release.LatestRelease.Pass }}[x] [Current Latest Release]({{ $repo.Release.LatestRelease.URL }}){{ else }}[ ] Current Latest Release{{ end }}
  - {{ if $repo.Release.LatestRelease.PackageZip.Pass }}[x] [package.zip]({{ $repo.Release.LatestRelease.PackageZip.URL }}){{ else }}[ ] `package.zip`{{ end }}
- {{ if $repo.Files.Pass }}[x]{{ else }}[ ]{{ end }} Necessary Files
  - {{ if $repo.Files.PreviewPng.Pass }}[x] [{{ $repo.Files.PreviewPng.Name }}]({{ $repo.Files.PreviewPng.URL }}){{ else }}[ ] `{{ $repo.Files.PreviewPng.Name }}`{{ end }}
  - {{ if $repo.Files.ReadmeMd.Pass }}[x] [{{ $repo.Files.ReadmeMd.Name }}]({{ $repo.Files.ReadmeMd.URL }}){{ else }}[ ] `{{ $repo.Files.ReadmeMd.Name }}`{{ end }}
  - {{ if $repo.Files.TemplateJson.Pass }}[x] [{{ $repo.Files.TemplateJson.Name }}]({{ $repo.Files.TemplateJson.URL }}){{ else }}[ ] `{{ $repo.Files.TemplateJson.Name }}`{{ end }}
- {{ if $repo.Attrs.Pass }}[x]{{ else }}[ ]{{ end }} Necessary Attributes for `template.json`
  - {{ if $repo.Attrs.Name.Pass }}[x] `name`: `{{ $repo.Attrs.Name.Value }}`{{ else }}[ ] `name`{{ end }} 
    - {{ if $repo.Attrs.Name.Unique }}[x]{{ else }}[ ]{{ end }} Unique
  - {{ if $repo.Attrs.Version.Pass }}[x] `version`: `{{ $repo.Attrs.Version.Value }}`{{ else }}[ ] `version`{{ end }} 

---
{{ end }}
## Add **`{{ len .Themes }}`** Theme Repo

{{ range $i, $repo := .Themes }}### [{{ $repo.Path }}]({{ $repo.Home }})

- {{ if $repo.Release.Pass }}[x]{{ else }}[ ]{{ end }} Necessary Release
  - {{ if $repo.Release.LatestRelease.Pass }}[x] [Current Latest Release]({{ $repo.Release.LatestRelease.URL }}){{ else }}[ ] Current Latest Release{{ end }}
  - {{ if $repo.Release.LatestRelease.PackageZip.Pass }}[x] [package.zip]({{ $repo.Release.LatestRelease.PackageZip.URL }}){{ else }}[ ] `package.zip`{{ end }}
- {{ if $repo.Files.Pass }}[x]{{ else }}[ ]{{ end }} Necessary Files
  - {{ if $repo.Files.PreviewPng.Pass }}[x] [{{ $repo.Files.PreviewPng.Name }}]({{ $repo.Files.PreviewPng.URL }}){{ else }}[ ] `{{ $repo.Files.PreviewPng.Name }}`{{ end }}
  - {{ if $repo.Files.ReadmeMd.Pass }}[x] [{{ $repo.Files.ReadmeMd.Name }}]({{ $repo.Files.ReadmeMd.URL }}){{ else }}[ ] `{{ $repo.Files.ReadmeMd.Name }}`{{ end }}
  - {{ if $repo.Files.ThemeCss.Pass }}[x] [{{ $repo.Files.ThemeCss.Name }}]({{ $repo.Files.ThemeCss.URL }}){{ else }}[ ] `{{ $repo.Files.ThemeCss.Name }}`{{ end }}
  - {{ if $repo.Files.ThemeJson.Pass }}[x] [{{ $repo.Files.ThemeJson.Name }}]({{ $repo.Files.ThemeJson.URL }}){{ else }}[ ] `{{ $repo.Files.ThemeJson.Name }}`{{ end }}
- {{ if $repo.Attrs.Pass }}[x]{{ else }}[ ]{{ end }} Necessary Attributes for `theme.json`
  - {{ if $repo.Attrs.Name.Pass }}[x] `name`: `{{ $repo.Attrs.Name.Value }}`{{ else }}[ ] `name`{{ end }} 
    - {{ if $repo.Attrs.Name.Unique }}[x]{{ else }}[ ]{{ end }} Unique
  - {{ if $repo.Attrs.Version.Pass }}[x] `version`: `{{ $repo.Attrs.Version.Value }}`{{ else }}[ ] `version`{{ end }} 

---
{{ end }}
## Add **`{{ len .Widgets }}`** Widget Repo

{{ range $i, $repo := .Widgets }}### [{{ $repo.Path }}]({{ $repo.Home }})

- {{ if $repo.Release.Pass }}[x]{{ else }}[ ]{{ end }} Necessary Release
  - {{ if $repo.Release.LatestRelease.Pass }}[x] [Current Latest Release]({{ $repo.Release.LatestRelease.URL }}){{ else }}[ ] Current Latest Release{{ end }}
  - {{ if $repo.Release.LatestRelease.PackageZip.Pass }}[x] [package.zip]({{ $repo.Release.LatestRelease.PackageZip.URL }}){{ else }}[ ] `package.zip`{{ end }}
- {{ if $repo.Files.Pass }}[x]{{ else }}[ ]{{ end }} Necessary Files
  - {{ if $repo.Files.IndexHtml.Pass }}[x] [{{ $repo.Files.IndexHtml.Name }}]({{ $repo.Files.IndexHtml.URL }}){{ else }}[ ] `{{ $repo.Files.IndexHtml.Name }}`{{ end }}
  - {{ if $repo.Files.PreviewPng.Pass }}[x] [{{ $repo.Files.PreviewPng.Name }}]({{ $repo.Files.PreviewPng.URL }}){{ else }}[ ] `{{ $repo.Files.PreviewPng.Name }}`{{ end }}
  - {{ if $repo.Files.ReadmeMd.Pass }}[x] [{{ $repo.Files.ReadmeMd.Name }}]({{ $repo.Files.ReadmeMd.URL }}){{ else }}[ ] `{{ $repo.Files.ReadmeMd.Name }}`{{ end }}
  - {{ if $repo.Files.WidgetJson.Pass }}[x] [{{ $repo.Files.WidgetJson.Name }}]({{ $repo.Files.WidgetJson.URL }}){{ else }}[ ] `{{ $repo.Files.WidgetJson.Name }}`{{ end }}
- {{ if $repo.Attrs.Pass }}[x]{{ else }}[ ]{{ end }} Necessary Attributes for `widget.json`
  - {{ if $repo.Attrs.Name.Pass }}[x] `name`: `{{ $repo.Attrs.Name.Value }}`{{ else }}[ ] `name`{{ end }} 
    - {{ if $repo.Attrs.Name.Unique }}[x]{{ else }}[ ]{{ end }} Unique
  - {{ if $repo.Attrs.Version.Pass }}[x] `version`: `{{ $repo.Attrs.Version.Value }}`{{ else }}[ ] `version`{{ end }} 

---
{{ end }}
