# 思源社区集市 <a title="Hits" target="_blank" href="https://github.com/siyuan-note/bazaar"><img src="https://hits.b3log.org/siyuan-note/bazaar.svg"></a>

[English](./README.md) | 简体中文

## 集市包开发示例

- [思源插件示例](https://github.com/siyuan-note/plugin-sample)，插件相关规范请参阅该仓库的 README。
- [思源插件示例（Vite & Svelte）](https://github.com/siyuan-note/plugin-sample-vite-svelte)
- [思源插件示例（Vite & Vue3）](https://github.com/siyuan-note/plugin-sample-vite-vue)
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

3. **提交 PR**
   提交更改并创建 Pull Request 到本仓库的 `main` 分支。
4. **等待检查与合并**
   CI 会自动运行以检查新增包是否符合集市规范（如 release、必要文件、清单属性等），维护者也会进行审核，请根据要求进行相应修改。
   审核通过与合并 PR 后，在数分钟内集市索引会自动更新，该包即可在思源笔记集市中展示（需要重启一次思源笔记以刷新集市索引缓存）。

## 更新集市包

无需再提交 PR，只需在集市包仓库中发布新版本，集市索引会自动拉取更新。

一般情况下，社区集市仓库会每小时自动更新索引并部署，你可在 [https://github.com/siyuan-note/bazaar/actions](https://github.com/siyuan-note/bazaar/actions) 查看部署状态。

## 为什么仓库叫 bazaar？

仓库名灵感来自《[The Cathedral and the Bazaar](https://en.wikipedia.org/wiki/The_Cathedral_and_the_Bazaar)》一书。初衷并非标新立异，而是延续开源软件的传统。

## 其他疑问

请提交 [issue](https://github.com/siyuan-note/bazaar/issues)
