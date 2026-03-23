# Siyuan Custom Pic Server  插件

## 介绍

这是一个思源笔记图床插件：

- 用于将文档中的图片视频资源上传到你配置的服务器上，实现自建图床。

- 导出符合 vuepress-theme-vdoing 笔记文档的 .md 文件

- 本仓库提供的 **Flask 适配后端**（`backend/`，简单的图床方案）。

## 使用方法

1. 安装并启用插件；
2. 打开插件设置；
3. 填写服务器地址（baseURL）与 Token（按后端配置可选）；
4. 点击“测试连接”确认可达；
5. 在编辑器中对资源右键，执行“上传到 CustomPic”。

![插件设置截图](./img/image.png)

默认上传范围为常见图片与视频格式（jpg/png/webp/gif/mp4/mov/mkv 等）。

## 功能

1. 支持右键手动上传资源文件；
2. 支持按资源路径进行已上传检查（`documentExists`）；
3. 上传成功后可将当前块中的资源路径替换为服务端直链；
4. 提供完整  Flask 后端 API 文档，便于自定义后端对接。

## API 文档

插件在设置里填写的 **服务器地址（baseURL）** 决定请求发往哪里：

- 指向本仓库自带的 **Flask 后端**（`backend/`，见 [backend/README.md](./backend/README.md)）时，接口约定如下。以下路径均相对于 baseURL，例如 baseURL 为 `http://192.168.1.2:5000` 时，完整 URL 为 `http://192.168.1.2:5000/api/...`。

### 鉴权

若服务端环境变量配置了 **`PAPERLESS_TOKEN`**，则下列接口须在请求头携带：

```http
Authorization: Token <与 .env 中一致的值>
```

未配置 Token 时，除特别说明外可不带头。
**例外**：`GET /api/files/...` **始终不校验 Token**（便于笔记内图片直链加载），请勿将该服务暴露到不可信网络。

### `GET` / `POST` `/api/testConnection`

- **说明**：插件「测试连接」使用（直连 `fetch`，不经思源 forwardProxy）。
- **鉴权**：不需要。
- **响应示例**：

```json
{ "success": true }
```

### `GET /api/documentExists/` 或 `GET /api/documentExists/<path:path>`

- **说明**：检查资源是否已保存（按 path 映射到 `files/` 下相对路径后判断文件是否存在）。
- **鉴权**：若配置了 `PAPERLESS_TOKEN` 则需要。
- **传参方式**（二选一）：
  - 查询参数：`/api/documentExists/?path=/data/assets/xxx.png`（插件当前使用）
  - 路径参数：`/api/documentExists/data/assets/xxx.png`
- **成功响应示例（存在）**：

```json
{
  "success": true,
  "path": "/data/assets/xxx.png",
  "exists": true,
  "count": 1,
  "results": [
    {
      "path": "/data/assets/xxx.png",
      "file_url": "/api/files/assets/xxx.png"
    }
  ]
}
```

- **成功响应示例（不存在）**：

```json
{
  "success": true,
  "path": "/data/assets/not-found.png",
  "exists": false,
  "count": 0,
  "results": []
}
```

### `POST /api/documents/post_document/`

- **说明**：上传文件到本地 `DOCUMENT_DATA_DIR/files/`。
- **鉴权**：若配置了 `PAPERLESS_TOKEN` 则需要。
- **Content-Type**：`multipart/form-data`
- **表单字段**：

| 字段         | 必填 | 说明                                                                                                                               |
| ------------ | ---- | ---------------------------------------------------------------------------------------------------------------------------------- |
| `document` | 是   | 文件二进制                                                                                                                         |
| `title`    | 建议 | 展示用标题，与插件侧文件名一致                                                                                                     |
| `path`     | 否   | 思源工作区路径，如 `/data/assets/xxx.png`；有则按规则映射到 `files/` 下相对路径（去掉前缀 `data/`、段名安全化、禁止 `..`） |

- **成功响应** `200`，`Content-Type: application/json`：

```json
{
  "success": true,
  "id": "<每次请求生成的 uuid>",
  "file_url": "/api/files/assets/xxx.png"
}
```

`file_url` 为**相对路径**，插件会拼在 baseURL 后作为公网/局域网访问地址。**无 `path` 时**：磁盘文件名为 `{uuid}_{安全主名}{扩展名}`，此时 `file_url` 形如 `/api/files/该相对路径`；下载时若 URL 为**单段纯 UUID**，服务端按 `files/{uuid}_*` 通配查找。

- **失败响应**：`4xx` JSON 中含 `success: false`、`detail`、`message` 等（以实际返回为准）。

### `GET /api/files/<路径>`

- **说明**：读取已上传文件，用于浏览器/思源内嵌图片等。
- **鉴权**：**不需要**（见上文安全说明）。
- **路径规则**：
  - **多段路径**（如 `assets/xxx.png`）：与上传时写入 `files/` 的相对路径一致（需经与上传相同的安全规范化）。
  - **单段且为 UUID**（如 `550e8400-e29b-41d4-a716-446655440000`）：返回 `files` 目录下以 `{uuid}_` 开头的唯一文件（对应未传 `path` 的上传方式）。
- **成功**：返回文件流（`send_file`）。
- **失败**：`404` / `400` JSON，如 `{"detail":"not found"}`。

### `GET /health`

- **说明**：健康检查。
- **鉴权**：不需要。
- **响应示例**：

```json
{ "ok": true, "data_dir": "C:\\...\\backend\\data" }
```

### 跨域（CORS）

Flask 应用对 `/api/*` 已配置允许跨域，便于思源浏览器端访问；桌面端同样适用。

### 更多部署说明

环境变量、监听地址、防火墙与目录结构详见 **[backend/README.md](./backend/README.md)**。

本项目是参考 [Jasaxion/siyuan-paperless](https://github.com/Jasaxion/siyuan-paperless/) 修改而来。

如果你觉得本插件对你有帮助，可以点个 ⭐️Star，谢谢～
