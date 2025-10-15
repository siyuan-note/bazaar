## Pull Request Check Report

### Add `{{ len .Icons }}` Icon Repo

{{ range $i, $repo := .Icons }}#### [{{ $repo.RepoInfo.Path }}](<{{ $repo.RepoInfo.Home }}>)

- {{ if $repo.Release.Pass }}[x]{{ else }}[ ]{{ end }} Release that must exist
  - {{ if $repo.Release.LatestRelease.Pass }}[x] [Current Latest Release](<{{ $repo.Release.LatestRelease.URL }}>){{ else }}[ ] Current Latest Release{{ end }}
  - {{ if $repo.Release.LatestRelease.PackageZip.Pass }}[x] [package.zip](<{{ $repo.Release.LatestRelease.PackageZip.URL }}>){{ else }}[ ] `package.zip`{{ end }}
- {{ if $repo.Files.Pass }}[x]{{ else }}[ ]{{ end }} Files that must exist
  - {{ if $repo.Files.IconJson.Pass }}[x] [icon.json](<{{ $repo.Files.IconJson.URL }}>){{ else }}[ ] `icon.json`{{ end }}
  - {{ if $repo.Files.IconPng.Pass }}[x] [icon.png](<{{ $repo.Files.IconPng.URL }}>){{ else }}[ ] `icon.png`{{ end }}
  - {{ if $repo.Files.PreviewPng.Pass }}[x] [preview.png](<{{ $repo.Files.PreviewPng.URL }}>){{ else }}[ ] `preview.png`{{ end }}
  - {{ if $repo.Files.ReadmeMd.Pass }}[x] [README.md](<{{ $repo.Files.ReadmeMd.URL }}>){{ else }}[ ] `README.md`{{ end }}
- {{ if $repo.Attrs.Pass }}[x]{{ else }}[ ]{{ end }} Attributes that must exist in `icon.json`
  - {{ if $repo.Attrs.Name.Pass }}[x] `name`: `{{ $repo.Attrs.Name.Value }}`{{ else }}[ ] `name`{{ end }}
    - {{ if $repo.Attrs.Name.Valid }}[x]{{ else }}[ ]{{ end }} Is a valid name
    - {{ if $repo.Attrs.Name.Unique }}[x]{{ else }}[ ]{{ end }} Not conflict with other icon name
  - {{ if $repo.Attrs.Version.Pass }}[x] `version`: `{{ $repo.Attrs.Version.Value }}`{{ else }}[ ] `version`{{ end }}
  - {{ if $repo.Attrs.Author.Pass }}[x] `author`: `{{ $repo.Attrs.Author.Value }}`{{ else }}[ ] `author`{{ end }}
  - {{ if $repo.Attrs.URL.Pass }}[x] `url`: [{{ $repo.Attrs.URL.Value }}](<{{ $repo.Attrs.URL.Value }}>){{ else }}[ ] `url`{{ end }}

---
{{ end }}
### Add `{{ len .Plugins }}` Plugin Repo

{{ range $i, $repo := .Plugins }}#### [{{ $repo.RepoInfo.Path }}](<{{ $repo.RepoInfo.Home }}>)

- {{ if $repo.Release.Pass }}[x]{{ else }}[ ]{{ end }} Release that must exist
  - {{ if $repo.Release.LatestRelease.Pass }}[x] [Current Latest Release](<{{ $repo.Release.LatestRelease.URL }}>){{ else }}[ ] Current Latest Release{{ end }}
  - {{ if $repo.Release.LatestRelease.PackageZip.Pass }}[x] [package.zip](<{{ $repo.Release.LatestRelease.PackageZip.URL }}>){{ else }}[ ] `package.zip`{{ end }}
- {{ if $repo.Files.Pass }}[x]{{ else }}[ ]{{ end }} Files that must exist
  - {{ if $repo.Files.PluginJson.Pass }}[x] [plugin.json](<{{ $repo.Files.PluginJson.URL }}>){{ else }}[ ] `plugin.json`{{ end }}
  - {{ if $repo.Files.IconPng.Pass }}[x] [icon.png](<{{ $repo.Files.IconPng.URL }}>){{ else }}[ ] `icon.png`{{ end }}
  - {{ if $repo.Files.PreviewPng.Pass }}[x] [preview.png](<{{ $repo.Files.PreviewPng.URL }}>){{ else }}[ ] `preview.png`{{ end }}
  - {{ if $repo.Files.ReadmeMd.Pass }}[x] [README.md](<{{ $repo.Files.ReadmeMd.URL }}>){{ else }}[ ] `README.md`{{ end }}
- {{ if $repo.Attrs.Pass }}[x]{{ else }}[ ]{{ end }} Attributes that must exist in `plugin.json`
  - {{ if $repo.Attrs.Name.Pass }}[x] `name`: `{{ $repo.Attrs.Name.Value }}`{{ else }}[ ] `name`{{ end }}
    - {{ if $repo.Attrs.Name.Exist }}[x]{{ else }}[ ]{{ end }} The attribute exists
    - {{ if $repo.Attrs.Name.Valid }}[x]{{ else }}[ ]{{ end }} Is a valid name
    - {{ if $repo.Attrs.Name.Unique }}[x]{{ else }}[ ]{{ end }} Not conflict with other plugin name
  - {{ if $repo.Attrs.Version.Pass }}[x] `version`: `{{ $repo.Attrs.Version.Value }}`{{ else }}[ ] `version`{{ end }}
  - {{ if $repo.Attrs.Author.Pass }}[x] `author`: `{{ $repo.Attrs.Author.Value }}`{{ else }}[ ] `author`{{ end }}
  - {{ if $repo.Attrs.URL.Pass }}[x] `url`: [{{ $repo.Attrs.URL.Value }}](<{{ $repo.Attrs.URL.Value }}>){{ else }}[ ] `url`{{ end }}

---
{{ end }}
### Add `{{ len .Templates }}` Template Repo

{{ range $i, $repo := .Templates }}#### [{{ $repo.RepoInfo.Path }}](<{{ $repo.RepoInfo.Home }}>)

- {{ if $repo.Release.Pass }}[x]{{ else }}[ ]{{ end }} Release that must exist
  - {{ if $repo.Release.LatestRelease.Pass }}[x] [Current Latest Release](<{{ $repo.Release.LatestRelease.URL }}>){{ else }}[ ] Current Latest Release{{ end }}
  - {{ if $repo.Release.LatestRelease.PackageZip.Pass }}[x] [package.zip](<{{ $repo.Release.LatestRelease.PackageZip.URL }}>){{ else }}[ ] `package.zip`{{ end }}
- {{ if $repo.Files.Pass }}[x]{{ else }}[ ]{{ end }} Files that must exist
  - {{ if $repo.Files.TemplateJson.Pass }}[x] [template.json](<{{ $repo.Files.TemplateJson.URL }}>){{ else }}[ ] `template.json`{{ end }}
  - {{ if $repo.Files.IconPng.Pass }}[x] [icon.png](<{{ $repo.Files.IconPng.URL }}>){{ else }}[ ] `icon.png`{{ end }}
  - {{ if $repo.Files.PreviewPng.Pass }}[x] [preview.png](<{{ $repo.Files.PreviewPng.URL }}>){{ else }}[ ] `preview.png`{{ end }}
  - {{ if $repo.Files.ReadmeMd.Pass }}[x] [README.md](<{{ $repo.Files.ReadmeMd.URL }}>){{ else }}[ ] `README.md`{{ end }}
- {{ if $repo.Attrs.Pass }}[x]{{ else }}[ ]{{ end }} Attributes that must exist in `template.json`
  - {{ if $repo.Attrs.Name.Pass }}[x] `name`: `{{ $repo.Attrs.Name.Value }}`{{ else }}[ ] `name`{{ end }}
    - {{ if $repo.Attrs.Name.Valid }}[x]{{ else }}[ ]{{ end }} Is a valid name
    - {{ if $repo.Attrs.Name.Unique }}[x]{{ else }}[ ]{{ end }} Not conflict with other template name
  - {{ if $repo.Attrs.Version.Pass }}[x] `version`: `{{ $repo.Attrs.Version.Value }}`{{ else }}[ ] `version`{{ end }}
  - {{ if $repo.Attrs.Author.Pass }}[x] `author`: `{{ $repo.Attrs.Author.Value }}`{{ else }}[ ] `author`{{ end }}
  - {{ if $repo.Attrs.URL.Pass }}[x] `url`: [{{ $repo.Attrs.URL.Value }}](<{{ $repo.Attrs.URL.Value }}>){{ else }}[ ] `url`{{ end }}

---
{{ end }}
### Add `{{ len .Themes }}` Theme Repo

{{ range $i, $repo := .Themes }}#### [{{ $repo.RepoInfo.Path }}](<{{ $repo.RepoInfo.Home }}>)

- {{ if $repo.Release.Pass }}[x]{{ else }}[ ]{{ end }} Release that must exist
  - {{ if $repo.Release.LatestRelease.Pass }}[x] [Current Latest Release](<{{ $repo.Release.LatestRelease.URL }}>){{ else }}[ ] Current Latest Release{{ end }}
  - {{ if $repo.Release.LatestRelease.PackageZip.Pass }}[x] [package.zip](<{{ $repo.Release.LatestRelease.PackageZip.URL }}>){{ else }}[ ] `package.zip`{{ end }}
- {{ if $repo.Files.Pass }}[x]{{ else }}[ ]{{ end }} Files that must exist
  - {{ if $repo.Files.ThemeJson.Pass }}[x] [theme.json](<{{ $repo.Files.ThemeJson.URL }}>){{ else }}[ ] `theme.json`{{ end }}
  - {{ if $repo.Files.IconPng.Pass }}[x] [icon.png](<{{ $repo.Files.IconPng.URL }}>){{ else }}[ ] `icon.png`{{ end }}
  - {{ if $repo.Files.PreviewPng.Pass }}[x] [preview.png](<{{ $repo.Files.PreviewPng.URL }}>){{ else }}[ ] `preview.png`{{ end }}
  - {{ if $repo.Files.ReadmeMd.Pass }}[x] [README.md](<{{ $repo.Files.ReadmeMd.URL }}>){{ else }}[ ] `README.md`{{ end }}
- {{ if $repo.Attrs.Pass }}[x]{{ else }}[ ]{{ end }} Attributes that must exist in `theme.json`
  - {{ if $repo.Attrs.Name.Pass }}[x] `name`: `{{ $repo.Attrs.Name.Value }}`{{ else }}[ ] `name`{{ end }}
    - {{ if $repo.Attrs.Name.Valid }}[x]{{ else }}[ ]{{ end }} Is a valid name
    - {{ if $repo.Attrs.Name.Unique }}[x]{{ else }}[ ]{{ end }} Not conflict with other theme name
  - {{ if $repo.Attrs.Version.Pass }}[x] `version`: `{{ $repo.Attrs.Version.Value }}`{{ else }}[ ] `version`{{ end }}
  - {{ if $repo.Attrs.Author.Pass }}[x] `author`: `{{ $repo.Attrs.Author.Value }}`{{ else }}[ ] `author`{{ end }}
  - {{ if $repo.Attrs.URL.Pass }}[x] `url`: [{{ $repo.Attrs.URL.Value }}](<{{ $repo.Attrs.URL.Value }}>){{ else }}[ ] `url`{{ end }}

---
{{ end }}
### Add `{{ len .Widgets }}` Widget Repo

{{ range $i, $repo := .Widgets }}#### [{{ $repo.RepoInfo.Path }}](<{{ $repo.RepoInfo.Home }}>)

- {{ if $repo.Release.Pass }}[x]{{ else }}[ ]{{ end }} Release that must exist
  - {{ if $repo.Release.LatestRelease.Pass }}[x] [Current Latest Release](<{{ $repo.Release.LatestRelease.URL }}>){{ else }}[ ] Current Latest Release{{ end }}
  - {{ if $repo.Release.LatestRelease.PackageZip.Pass }}[x] [package.zip](<{{ $repo.Release.LatestRelease.PackageZip.URL }}>){{ else }}[ ] `package.zip`{{ end }}
- {{ if $repo.Files.Pass }}[x]{{ else }}[ ]{{ end }} Files that must exist
  - {{ if $repo.Files.WidgetJson.Pass }}[x] [widget.json](<{{ $repo.Files.WidgetJson.URL }}>){{ else }}[ ] `widget.json`{{ end }}
  - {{ if $repo.Files.IconPng.Pass }}[x] [icon.png](<{{ $repo.Files.IconPng.URL }}>){{ else }}[ ] `icon.png`{{ end }}
  - {{ if $repo.Files.PreviewPng.Pass }}[x] [preview.png](<{{ $repo.Files.PreviewPng.URL }}>){{ else }}[ ] `preview.png`{{ end }}
  - {{ if $repo.Files.ReadmeMd.Pass }}[x] [README.md](<{{ $repo.Files.ReadmeMd.URL }}>){{ else }}[ ] `README.md`{{ end }}
- {{ if $repo.Attrs.Pass }}[x]{{ else }}[ ]{{ end }} Attributes that must exist in `widget.json`
  - {{ if $repo.Attrs.Name.Pass }}[x] `name`: `{{ $repo.Attrs.Name.Value }}`{{ else }}[ ] `name`{{ end }}
    - {{ if $repo.Attrs.Name.Valid }}[x]{{ else }}[ ]{{ end }} Is a valid name
    - {{ if $repo.Attrs.Name.Unique }}[x]{{ else }}[ ]{{ end }} Not conflict with other widget name
  - {{ if $repo.Attrs.Version.Pass }}[x] `version`: `{{ $repo.Attrs.Version.Value }}`{{ else }}[ ] `version`{{ end }}
  - {{ if $repo.Attrs.Author.Pass }}[x] `author`: `{{ $repo.Attrs.Author.Value }}`{{ else }}[ ] `author`{{ end }}
  - {{ if $repo.Attrs.URL.Pass }}[x] `url`: [{{ $repo.Attrs.URL.Value }}](<{{ $repo.Attrs.URL.Value }}>){{ else }}[ ] `url`{{ end }}

---
{{ end }}
