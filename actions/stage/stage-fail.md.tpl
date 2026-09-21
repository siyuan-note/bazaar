{{ .Marker }}
@{{ repoOwner .OwnerRepo }}

您好。您的集市包 [{{ .OwnerRepo }}]({{ repoURL .OwnerRepo }})（`{{ .PackageType }}`）在社区集市自动索引更新过程中未通过检查，因此未能更新。

Hello. Your bazaar package [{{ .OwnerRepo }}]({{ repoURL .OwnerRepo }}) (`{{ .PackageType }}`) did not pass the check during the automated community bazaar index update, and therefore was not updated.

请先修复下列问题，再重新打包 `package.zip`，并发布（或更新）标记为 Latest 的 GitHub Release，确保其中包含新的 `package.zip`。无需另行提交 Pull Request；索引一般会在数小时内自动同步。

Please fix the issues listed below first, then rebuild `package.zip`, and publish (or update) a GitHub Release marked as Latest that includes the new `package.zip`. A separate pull request is not required; the index is usually synchronized automatically within a few hours.

若对检查结果有疑问，可直接在本 Issue 中回复，集市维护者看到后会处理。

If you have questions about the check result, please reply in this issue. Bazaar maintainers will follow up when they see it.
{{- if .Release.URL }}

检查的 Release / Checked release: [{{ .Release.Tag }}]({{ .Release.URL }}){{- if .Hash }} · package hash `{{ .Hash }}`{{- end }}
{{- else if .Hash }}

package hash `{{ .Hash }}`
{{- end }}
{{- if .Issues }}

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

工作流日志 / Workflow log: {{ .WorkflowRunURL }}
{{- end }}
