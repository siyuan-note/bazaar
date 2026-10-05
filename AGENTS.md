# 维护者流程

## 上架集市包

上架指把作者仓库加入集市清单。作者或维护者都可以提 PR，流程相同。

1. 确认仓库已有 GitHub Release，且附件含 `package.zip`；缺失时 PR Check 必然失败
2. 从 `main` 拉分支 `add-<type>-<name>`（插件 `add-plugin-<name>`，其他类型同理）
3. 只改对应的 `plugins.txt` / `themes.txt` / `icons.txt` / `templates.txt` / `widgets.txt`，追加一行 `owner/repo`，保持 LF；一个 PR 只加一个包，不要夹带其他改动
4. 提交信息 `Add owner/repo`，推送到自己的 fork 后开 PR 到 `siyuan-note/bazaar:main`，标题同为 `Add owner/repo`
5. PR 正文用 `.github/PULL_REQUEST_TEMPLATE.md` 的内容，末尾附 `仓库：https://github.com/owner/repo`；Windows 上把正文写入 UTF-8 无 BOM 临时文件后用 `--body-file`，勿提交临时文件
6. 人工审阅通过后合并；Stage 会重建索引，无需手改 `stage/*.json`

## 弃用集市包

弃用用于仍上架、仍可安装和更新，但已不建议新用户选用的包。它不承担安全封禁或下架职责；需要移除分发时走下方「下架集市包」流程。

`deprecated.json` 由集市维护者集中维护。作者或社区成员可以提议，但维护者负责核实停止维护、替代关系等事实并做最终决定。弃用变更使用独立 PR，只能修改根目录的 `deprecated.json`，不得同时修改包列表、Stage 索引或其它文件。

- 根对象固定包含 `plugins` / `themes` / `icons` / `templates` / `widgets`
- 每项键使用对应 TXT 列表中精确的 `owner/repo`；键存在即表示弃用，空对象表示使用通用原因
- `reason` 可省略；存在时必须包含非空 `default`，其它键为 locale，内容应简洁、可验证
- `alternatives` 可省略；存在时仅填写同类型、仍上架的 `owner/repo`，按推荐顺序排列，不得包含自身或重复项
- 不要在包清单中加入 `deprecated`、`deprecatedReason` 或 `alternatives`，也不要手改 `stage/*.json`
- 恢复普通状态时删除注册表条目（例如作者取消归档并恢复维护）；Stage 会先清空所有旧生成字段再重新覆盖

PR Check 只验证结构与引用关系；检查通过后仍须人工审阅弃用结论、原因及替代包。合并后 Stage 会在不访问包仓库 API 的情况下重建五类索引。

## 巡检已归档仓库

作者归档仓库后不会再发 Release，Stage 的「asset id 未变」快路径与 `deprecated.json` 变更路径都不会再打仓库 API，因此「仓库已归档」不会自动反映到集市里，需要偶尔人工巡检。

1. 在仓库根目录运行 `go run ./actions/audit-archived -out report.md`（需 GitHub 令牌；未设置 `GITHUB_TOKEN` / `PAT` 时回退到 `gh auth token`，已登录 `gh` 即可直接跑）
2. 查看报告「已归档但未登记弃用」表中的条目，逐个核实是否确已停止维护
3. 需要弃用时按上方「弃用集市包」流程提独立 PR，只改 `deprecated.json`；`reason` 写明作者已归档仓库并停止维护
4. 查看报告「已登记弃用但仓库未归档」表：归档可被作者取消，取消后作者可能已恢复维护，此时原弃用条目已陈旧，应按「弃用集市包」流程提独立 PR 删除该条目；表中「最后推送」时间可辅助判断是否已恢复维护
5. 报告中的「仓库不存在」按「下架集市包」流程处理；「清单条目 owner/repo 已过期」只是仓库改名，GitHub 会自动重定向，不影响 PR Check，必要时单独提 PR 修正列表行

注意：

- 归档状态可增可减，本工具只是**当下的一次快照**，不记录历史，也不能当作长期结论
- 归档时间只能取自 GraphQL 的 `Repository.archivedAt`；REST 的 `archived_at` 已被 GitHub 废弃，`go-github` v89 中已无该字段，故脚本必须走 GraphQL
- GraphQL 把「仓库不存在」与「无权查看（私有）」都返回为 `null`，令牌看不到的私有仓库会被误报为不存在
- 每个条目约消耗 1 个 GraphQL 配额点，按 80 个一批提交
- 该脚本不参与任何 CI 工作流，不会自动修改 `deprecated.json`；Windows 上不要用 `> report.md` 重定向，改用 `-out`，否则 PowerShell 会按控制台代码页重编码并损毁中文

## 下架集市包

维护者主动下架集市包时，**先开 issue，再直接提交到 `main`**。不要开 PR。作者自己提的下架 PR 不走本流程。

### 1. 开 issue

在 [siyuan-note/bazaar](https://github.com/siyuan-note/bazaar/issues) 创建 issue。

- 标题：插件用 `Delist owner/repo`；其他类型用 `Delist <type> owner/repo`（`theme` / `icon` / `template` / `widget`）
- 正文用中英双语，说明原因，附证据（截图、链滴帖、插件仓 issue、404 等）
- `cc @作者用户名`
- Windows 上不要用 bash heredoc；把正文写入临时文件，用 `gh issue create --body-file`，创建后删除临时文件，勿提交

正文模板：

```markdown
### 下架插件 / Delist Package

**插件 / Package**: [`owner/repo`](https://github.com/owner/repo)

**下架原因 / Reason**:

中文原因，因此从社区集市下架。

English reason, so it will be delisted from the community bazaar.

cc @author
```

非插件时把「插件 / Package」改成对应类型（主题 / Theme 等）。

参考：[样式冲突 #2106](https://github.com/siyuan-note/bazaar/issues/2106)、[破坏界面 #1983](https://github.com/siyuan-note/bazaar/issues/1983)、[仓库 404 #1992](https://github.com/siyuan-note/bazaar/issues/1992)

### 2. 改列表并提交到 main

确认 issue 已创建并记下编号，再改对应列表：`plugins.txt` / `themes.txt` / `icons.txt` / `templates.txt` / `widgets.txt`。只删目标 `owner/repo` 那一行，保持 LF，不要夹带新增或其他下架。

若目标包已在 `deprecated.json` 中，同一维护动作还需删除对应弃用条目，避免留下过期注册表数据；除此之外不要夹带其它变更。

提交信息（第一行用 `because` 写清英文原因，不要只写 `Delist owner/repo`）：

```
Delist owner/repo, because <英文原因>

close https://github.com/siyuan-note/bazaar/issues/<编号>
```

推送到 `origin/main`。Stage 随后会更新索引，issue 会因 `close` 自动关闭。
