{{define "prFlowShapeRules" -}}
每个 Pull Request 仅允许以下之一：
1. 仅添加 1 个新包；
2. 更换维护者（添加 1 个新 `owner/repo`，并删除同类型、同 GitHub 仓库名的旧 `owner/repo`）；
3. 仅下架一个或多个包。

Each Pull Request may only be one of:
1. Add exactly 1 new package;
2. Change maintainer (add 1 new `owner/repo` and delete the old `owner/repo` with the same type and same GitHub repository name);
3. Delist one or more packages only.

{{end}}
{{define "onePackageLimit" -}}
本 PR 添加或更换了 {{.Total}} 个集市包，但每个 Pull Request 最多只能添加或更换 1 个包。请将每个包拆成独立的 Pull Request 后再提交。

This PR adds or changes {{.Total}} bazaar packages, but each Pull Request may add or change at most 1 package. Please split each package into its own Pull Request.

{{template "prFlowShapeRules" . -}}
涉及的仓库 / Involved repos:
{{range .NewRepos}}- `{{.ListFile}}`: {{.Repos}}
{{end}}{{end}}
{{define "mixedAddDelist" -}}
本 PR 在添加或更换集市包的同时，还下架了其他包。添加/更换与无关下架不能放在同一个 Pull Request 中。

This PR both adds or changes a bazaar package and delists unrelated package(s). Adding/changing and unrelated delistings cannot be combined in the same Pull Request.

{{template "prFlowShapeRules" . -}}
涉及的新增或更换 / Involved add or change:
{{range .NewRepos}}- `{{.ListFile}}`: {{.Repos}}
{{end}}
涉及的无关下架 / Involved unrelated delistings:
{{range .PureDeleted}}- `{{.ListFile}}`: {{.Repos}}
{{end}}{{end}}
