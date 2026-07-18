{{ .Marker }}
### [{{ .OwnerRepo }}]({{ repoURL .OwnerRepo }}) (`{{ .PackageType }}`)
{{- if .Release.URL }}

最新 Release / Latest Release: [{{ .Release.Tag }}]({{ .Release.URL }}){{- if .Hash }} · hash `{{ .Hash }}`{{- end }}
{{- else if .Hash }}

hash `{{ .Hash }}`
{{- end }}
{{- if .Issues }}

检测到以下问题，请在修复之后提升清单字段 `version`，重新打包 `package.zip` 并发布新的 GitHub Release（标记为 Latest）。

We found the following issues. Please fix them, bump the manifest `version`, rebuild `package.zip`, and publish a new GitHub Release marked as Latest.

---
{{- end }}
{{- $issues := .Issues }}
{{- range $i, $issue := $issues }}

[{{ issueIndex $i (len $issues) }}]

{{ $issue.MessageZh }}

{{ $issue.MessageEn }}

---
{{- end }}
{{- if .WorkflowRunURL }}

工作流 / Workflow: {{ .WorkflowRunURL }}
{{- end }}
