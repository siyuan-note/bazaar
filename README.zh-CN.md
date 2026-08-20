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
   - 每个 PR 仅允许以下之一：仅添加 1 个新包；更换维护者（添加 1 个新 `owner/repo` 并删除同类型、同 GitHub 仓库名的旧 `owner/repo`）；或仅下架一个或多个包。不要把添加/更换与无关下架混在同一个 PR。

3. **提交 PR**

   提交更改并创建 Pull Request 到本仓库的 `main` 分支。

4. **等待检查与合并**

   PR Check 工作流会自动运行以检查新增包是否符合集市规范（如 release、必要文件、清单字段等），维护者也会进行审核，请根据要求进行相应修改。

   如果 PR Check 工作流检查不成功，请根据检查结果进行修改，不要重新创建 PR。修好包仓库（例如更新 Latest Release / `package.zip`）后，定时任务约每 20 分钟会按活跃度复检开放中尚未 `ci-passed` 的 PR（包仓 Release 有变化时优先；长期无变化则逐渐降低频率）并更新检查评论；维护者也可手动打 `Check` 标签或触发工作流立即复检。

5. **成功上架**

   审核通过与合并 PR 后，在数分钟内集市索引会自动更新，该包即可在思源笔记集市中展示（需要重启一次思源笔记以刷新集市索引缓存）。

## 更新集市包

无需再提交 PR，只需在集市包仓库中发布新版本，集市索引会自动拉取更新。

一般情况下，社区集市仓库会每一到三小时自动更新索引并部署，你可以在 [Stage 工作流页面](https://github.com/siyuan-note/bazaar/actions/workflows/stage.yml) 查看部署状态。

如果长时间未更新，可能是更新包存在问题（例如未提升清单中的 version）。请先查看带 `stage-fail` 标签的 [Stage 检查失败 Issue](https://github.com/siyuan-note/bazaar/issues?q=is%3Aissue+is%3Aopen+label%3Astage-fail) 中是否有对应仓库，也可检查最新的 Stage 工作流日志。

## 弃用集市包

弃用是一种推荐状态，适用于仍可使用、但已不适合作为默认选择的包，例如包已停止维护或已有持续维护的替代品。弃用不同于下架或安全封禁：弃用包仍保留在对应类型的 TXT 列表中，并继续参与索引、发布、安装、更新和启用。

集市维护者负责维护集中的 [`deprecated.json`](./deprecated.json) 注册表，并对每项弃用做最终决定。包作者和社区成员可以通过独立 PR 提议变更，但包不得在 `plugin.json`、`theme.json` 或其它包清单中自行声明弃用元数据。

弃用 PR 只能修改 `deprecated.json`。根对象包含 `plugins`、`themes`、`icons`、`templates` 和 `widgets`，各对象以列表中精确的 `owner/repo` 为键；键存在即表示该包已弃用。空对象使用客户端通用提示；可选的多语言 `reason` 必须包含 `default`；可选的 `alternatives` 是同类型 `owner/repo` 的有序列表。

```json
{
  "plugins": {
    "owner/old-plugin": {
      "reason": {
        "default": "This package is no longer maintained",
        "zh-CN": "该包已停止维护"
      },
      "alternatives": ["owner/new-plugin"]
    }
  },
  "themes": {},
  "icons": {},
  "templates": {},
  "widgets": {}
}
```

弃用源包及每个替代包都必须继续存在于对应类型的 TXT 列表中。替代包可以同样处于弃用状态，但维护者应确认该推荐仍有意义。要恢复为普通状态，只需从 `deprecated.json` 删除对应键；下一次 Stage 会清除生成的弃用字段。不要手工修改 `stage/*.json`。

## 更换维护者

若原作者无力继续维护，可由新维护者接手已上架的集市包。更换维护者需要单独提交 PR（一次只更换一个包），并经过原维护者确认后才会合并。

### 提交流程

1. **准备新仓库**

   新维护者应拥有可发布 Release 的 GitHub 仓库（常见做法是 Fork 原仓库，或原作者将仓库转移给新维护者）。仓库中需包含符合规范的集市包，并已发布带 `package.zip` 的 Latest Release。

2. **修改集市包列表 TXT 文件**

   在对应类型的列表文件中：删除原 `owner/repo` 一行，新增新的 `owner/repo` 一行。

   - 示例：将 `alice/foo-plugin` 改为 `bob/foo-plugin`。
   - 清单中的 `name` 应与原先已上架的包名保持一致，以便用户侧仍识别为同一集市包；`url` 须改为新仓库地址。
   - 更换维护者视同包更新：清单 `version` 必须高于原先已上架版本，并发布带新 `package.zip` 的 Latest Release。

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

<!-- test-deprecate-workflow: do not merge -->

