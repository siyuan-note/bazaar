{{ if eq .Reason "manual-review" -}}
本拉取请求打开已超过 {{ .Days }} 天，且仍带有 `ci-passed` 与 `manual-review` 标签，因此由机器人自动关闭。

This pull request has been open for more than {{ .Days }} days and still has both the `ci-passed` and `manual-review` labels, so it is being closed automatically by the bot.

请先按维护者的审核意见完成修改，然后**打开一个新的拉取请求**。

Please apply the changes requested by maintainers first, then **open a new pull request**.
{{ else -}}
本拉取请求打开已超过 {{ .Days }} 天，且仍带有 `ci-failed` 标签，因此由机器人自动关闭。

This pull request has been open for more than {{ .Days }} days and still has the `ci-failed` label, so it is being closed automatically by the bot.

请先根据检查评论修复问题，然后**打开一个新的拉取请求**。

Please fix the issues from the check comments first, then **open a new pull request**.
{{ end }}

<sub>若维护者需要保留本 PR 不被自动关闭，请打上 `ci-skip` 标签。</sub>
<sub>If maintainers need to keep this PR open without auto-close, please add the `ci-skip` label.</sub>
