# 思源笔记社区集市 <a title="Hits" target="_blank" href="https://github.com/siyuan-note/bazaar"><img src="https://hits.b3log.org/siyuan-note/bazaar.svg"></a>

[English](https://github.com/siyuan-note/bazaar/blob/main/README.md)

## 概述

思源笔记社区集市分为四个部分：

* 主题集市
* 模板集市
* 图标集市
* 挂件集市

请分别参考下面的方式进行上架。

## 上架主题集市

上架前请确认你的主题仓库根路径下至少包含以下文件（[仓库示例](https://github.com/88250/Comfortably-Numb)）：

* theme.css
* theme.json（请确保 JSON 格式正确）
* preview.png（请压缩图片大小在 512 KB 以内）
* README.md（请注意大小写）
* 在 GitHub 上创建 Release

确认无误以后请通过对[社区集市](https://github.com/siyuan-note/bazaar)仓库[发起 pull request](https://docs.github.com/cn/free-pro-team@latest/github/collaborating-with-issues-and-pull-requests/creating-a-pull-request)，修改 themes.json 文件。该文件是所有社区主题仓库的索引，格式为：

```json
{
  "repos": [
    "username/reponame"
  ]
}
```

如果你开发的主题更新了版本，请记得：

* 更新你的主题配置 theme.json 中的 version 字段
* 在 GitHub 上创建一个新的 Release

## 上架模板集市

上架前请确认你的模板仓库根路径下至少包含以下文件（[仓库示例](https://github.com/88250/November-Rain)）：

* template.json（请确保 JSON 格式正确）
* preview.png（请压缩图片大小在 512 KB 以内）
* README.md（请注意大小写）
* 在 GitHub 上创建 Release

确认无误以后请通过对[社区集市](https://github.com/siyuan-note/bazaar)仓库[发起 pull request](https://docs.github.com/cn/free-pro-team@latest/github/collaborating-with-issues-and-pull-requests/creating-a-pull-request)，修改 templates.json 文件。该文件是所有社区模板仓库的索引，格式为：

```json
{
  "repos": [
    "username/reponame"
  ]
}
```

如果你开发的模板更新了版本，请记得：

* 更新你的模板配置 template.json 中的 version 字段
* 在 GitHub 上创建一个新的 Release

## 上架图标集市

上架前请确认你的图标仓库根路径下至少包含以下文件：

* icon.json（请确保 JSON 格式正确）
* icon.js
* preview.png（请压缩图片大小在 512 KB 以内）
* README.md（请注意大小写）
* 在 GitHub 上创建 Release

确认无误以后请通过对[社区集市](https://github.com/siyuan-note/bazaar)仓库[发起 pull request](https://docs.github.com/cn/free-pro-team@latest/github/collaborating-with-issues-and-pull-requests/creating-a-pull-request)，修改 icons.json 文件。该文件是所有社区图标仓库的索引，格式为：

```json
{
  "repos": [
    "username/reponame"
  ]
}
```

如果你开发的图标更新了版本，请记得：

* 更新你的图标配置 icon.json 中的 version 字段
* 在 GitHub 上创建一个新的 Release

## 上架挂件集市

上架前请确认你的挂件仓库根路径下至少包含以下文件（[仓库示例](https://github.com/88250/Stairway-To-Heaven)）：

* index.html
* widget.json（请确保 JSON 格式正确）
* preview.png（请压缩图片大小在 512 KB 以内）
* README.md（请注意大小写）
* 在 GitHub 上创建 Release

确认无误以后请通过对[社区集市](https://github.com/siyuan-note/bazaar)仓库[发起 pull request](https://docs.github.com/cn/free-pro-team@latest/github/collaborating-with-issues-and-pull-requests/creating-a-pull-request)，修改 widgets.json 文件。该文件是所有社区挂件仓库的索引，格式为：

```json
{
  "repos": [
    "username/reponame"
  ]
}
```

如果你开发的挂件更新了版本，请记得：

* 更新你的挂件配置 widget.json 中的 version 字段
* 在 GitHub 上创建一个新的 Release

---

## 仓库名称由来

该仓库的名称来源于书籍《[The Cathedral and the Bazaar](https://en.wikipedia.org/wiki/The_Cathedral_and_the_Bazaar)》。

我们的本意不是为了标新立异，而是为了延续开源软件的传统。
