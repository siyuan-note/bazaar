# 维护者下架流程

维护者主动下架集市包时，**先开 issue，再直接提交到 `main`**。不要开 PR。作者自己提的下架 PR 不走本流程。

## 1. 开 issue

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

## 2. 改列表并提交到 main

确认 issue 已创建并记下编号，再改对应列表：`plugins.txt` / `themes.txt` / `icons.txt` / `templates.txt` / `widgets.txt`。只删目标 `owner/repo` 那一行，保持 LF，不要夹带新增或其他下架。

提交信息（第一行用 `because` 写清英文原因，不要只写 `Delist owner/repo`）：

```
Delist owner/repo, because <英文原因>

close https://github.com/siyuan-note/bazaar/issues/<编号>
```

推送到 `origin/main`。Stage 随后会更新索引，issue 会因 `close` 自动关闭。
