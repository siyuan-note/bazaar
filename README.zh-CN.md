# 思源社区集市 <a title="Hits" target="_blank" href="https://github.com/siyuan-note/bazaar"><img src="https://hits.b3log.org/siyuan-note/bazaar.svg"></a>

[English](./README.md) | **简体中文**

## 集市包开发示例

- [思源插件示例](https://github.com/siyuan-note/plugin-sample)，由思源官方维护，插件相关规范请参阅该仓库的 README。
- [思源插件示例（Vite & Svelte）](https://github.com/siyuan-note/plugin-sample-vite-svelte)，由社区维护
- [思源插件示例（Vite & Vue3）](https://github.com/siyuan-note/plugin-sample-vite-vue)，由社区维护
- [思源主题示例](https://github.com/siyuan-note/theme-sample)
- [思源图标示例](https://github.com/siyuan-note/icon-sample)
- [思源模板示例](https://github.com/siyuan-note/template-sample)
- [思源挂件示例](https://github.com/siyuan-note/widget-sample)

## 提交集市包

若你已开发好插件、主题、图标、模板、挂件，并希望将其加入思源社区集市，请按以下步骤提交集市包：

1. **Fork 本仓库**

   在 GitHub 上 Fork 本仓库 [siyuan-note/bazaar](https://github.com/siyuan-note/bazaar)。如果已经 Fork 过了，需要先同步最新代码。

2. **修改集市包列表 TXT 文件**

   根目录下按类型有五个包列表文件：`plugins.txt`、`themes.txt`、`icons.txt`、`templates.txt`、`widgets.txt`。
   在**对应类型**的文件中新增一行，格式为：`owner/repo`（owner 是指 GitHub 用户名或组织名，repo 是指集市包仓库名）。

   - 每行一个 `owner/repo`，不要有多余逗号或空行。
   - 示例：`siyuan-note/plugin-sample`。
   - 一次只添加 1 个包（跨类型合计也只能 1 个）；需要上架多个包时请拆成多个 PR。

3. **提交 PR**

   提交更改并创建 Pull Request 到本仓库的 `main` 分支。

4. **等待检查与合并**

   PR Check 工作流会自动运行以检查新增包是否符合集市规范（如 release、必要文件、清单字段等），维护者也会进行审核，请根据要求进行相应修改。

   如果 PR Check 工作流检查不成功，请根据检查结果进行修改，不要重新创建 PR，维护者会手动再次运行 PR Check 工作流检查。

5. **成功上架**

   审核通过与合并 PR 后，在数分钟内集市索引会自动更新，该包即可在思源笔记集市中展示（需要重启一次思源笔记以刷新集市索引缓存）。

## 更新集市包

无需再提交 PR，只需在集市包仓库中发布新版本，集市索引会自动拉取更新。

一般情况下，社区集市仓库会每一到三小时自动更新索引并部署，你可以在 [Stage 工作流页面](https://github.com/siyuan-note/bazaar/actions/workflows/stage.yml) 查看部署状态。

如果长时间未更新，可能是更新包存在问题（例如未提升清单中的 version），可以检查最新的 Stage 工作流日志。

## 更换维护者

若原作者无力继续维护，可由新维护者接手已上架的集市包。更换维护者需要单独提交 PR（一次只更换一个包），并经过原维护者确认后才会合并。

### 提交流程

1. **准备新仓库**

   新维护者应拥有可发布 Release 的 GitHub 仓库（常见做法是 Fork 原仓库，或原作者将仓库转移给新维护者）。仓库中需包含符合规范的集市包，并已发布带 `package.zip` 的 Latest Release。

2. **修改集市包列表 TXT 文件**

   在对应类型的列表文件中：删除原 `owner/repo` 一行，新增新的 `owner/repo` 一行。

   - 示例：将 `alice/foo-plugin` 改为 `bob/foo-plugin`。
   - 清单中的 `name` 应与原先已上架的包名保持一致，以便用户侧仍识别为同一集市包；`url` 须改为新仓库地址。

3. **提交 PR 并请求原维护者确认**

   创建 Pull Request 到本仓库的 `main` 分支后，在 PR 中 `@` 原维护者，说明接手原因与后续维护计划，请其确认同意更换。

   如果原维护者未同意，那么 PR 不会被合并。

4. **等待检查与合并**

   PR Check 会对新仓库按上架规范检查。原维护者确认、检查通过后，维护者才会合并 PR。合并后集市索引会更新为新的 `owner/repo`。

### 关于下载量等统计数据

集市包的统计数据目前按 `owner/repo` 区分。更换维护者后，**默认不继承**旧仓库的统计数据（如下载量）。

若需要迁移统计数据，请同时满足：

1. 新维护者在 PR 中，或者在本仓库提交新的 [issue](https://github.com/siyuan-note/bazaar/issues) 说明迁移请求；
2. 原作者在回复同意；
3. 经确认后，由思源维护者在服务端手工迁移。

## 为什么仓库叫 bazaar？

仓库名灵感来自《[The Cathedral and the Bazaar](https://en.wikipedia.org/wiki/The_Cathedral_and_the_Bazaar)》一书。初衷并非标新立异，而是延续开源软件的传统。

## 其他疑问

请提交 [issue](https://github.com/siyuan-note/bazaar/issues)
