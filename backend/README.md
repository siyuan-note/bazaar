# 思源插件兼容 API（Flask，本地文件库）

与 `siyuan-custompic` 插件约定一致的轻量后端：**文件落盘在本地**，不使用数据库。`GET /api/documents/` 为占位接口（`results` 恒为空），无法在服务端按标题去重。

## 插件侧配置

- **服务器地址**：`http://<本机或局域网 IP>:<端口>`（默认端口见下方 `FLASK_PORT`）
- **Token（可选）**：若在 `.env` 中设置 `API_KEY`（兼容旧变量 `PAPERLESS_TOKEN`），则上传、文档列表等受保护接口须在请求头携带 `Authorization: Token <同一值>`（插件里可写裸 token，由插件补前缀）

## 运行

```bash
cd backend
python -m venv .venv
.venv\Scripts\activate   # Windows
pip install -r requirements.txt
copy .env.example .env   # 按需修改
python app.py
```

- 默认监听 **`0.0.0.0:5000`**（局域网可访问）；仅本机可设 `FLASK_HOST=127.0.0.1`，此时插件地址应填 `http://127.0.0.1:5000`。
- **`PUBLIC_DIR`**：实际上传文件保存目录（如 VuePress 的 `.vuepress/public`）；未设置时默认为 **`{DOCUMENT_DATA_DIR}/files`**，其中 **`DOCUMENT_DATA_DIR`** 默认 `backend/data`。
- `GET /health` 会返回 `data_dir` 与 `public_dir` 便于核对路径。

若出现 **连接被重置**：确认监听地址、Windows 防火墙放行端口，以及插件里的 IP 是否为运行本服务的机器。

若浏览器控制台出现 **`net::ERR_CONNECTION_TIMED_OUT`**（插件对内网地址使用直连，不走思源内核代发）：

1. 在 **运行 Flask 的那台电脑** 上执行 `python app.py`，并确认默认监听 **`0.0.0.0:5000`**（勿误用仅 `127.0.0.1` 却在外部设备填局域网 IP）。
2. 在该机防火墙中 **放行入站 TCP 5000**（或你设置的 `FLASK_PORT`）。
3. 插件地址中的 IP 必须是 **能 ping/访问到的那台机器**；客户端隔离的 Wi‑Fi 可能导致无法访问局域网服务。
4. 若配置了 `API_KEY`，插件里 Token 须与 `.env` 一致，否则会得到 401 而非超时（超时表示 TCP 未连上）。

## 上传与存储路径

`POST /api/documents/post_document/` 使用 **multipart/form-data**：

| 字段         | 必填 | 说明                                        |
| ------------ | ---- | ------------------------------------------- |
| `document` | 是   | 文件内容                                    |
| `title`    | 建议 | 与插件一致，作展示名                        |
| `path`     | 否   | 思源工作区路径，如 `/data/assets/xxx.png` |

- **带 `path`**：经规范化后写入 `files/` 下**相对路径**（自动去掉前导 `data/`，路径段做 `secure_filename`，禁止 `..`）。例如 `/data/assets/a.png` → `files/assets/a.png`。
- **不带 `path`**：保存为 **`{uuid}_{安全文件名}{扩展名}`**，与 `GET /api/files/<uuid>`（按 `{uuid}_*` 查找）兼容。

成功响应示例：

```json
{
  "success": true,
  "id": "<uuid>",
  "file_url": "/api/files/assets/xxx.png"
}
```

`file_url` 为相对路径，插件会用配置的 `baseURL` 拼成完整地址。

## 路由一览

| 方法     | 路径                              | 鉴权              | 说明                                                                                                                 |
| -------- | --------------------------------- | ----------------- | -------------------------------------------------------------------------------------------------------------------- |
| GET/POST | `/api/testConnection`           | 若配置 Token 则需 | 连通性检测，返回 `{"success": true}`                                                                               |
| GET      | `/api/documents/`               | 若配置 Token 则需 | 占位列表；`results` 为空                                                                                           |
| POST     | `/api/documents/post_document/` | 若配置 Token 则需 | 见上表                                                                                                               |
| GET      | `/api/files/<path:subpath>`     | **否**      | 读取已上传文件：根目录为 `PUBLIC_DIR`；多段路径按相对路径；**单段且为 UUID** 时按 `{uuid}_*` 匹配（无 `path` 上传方式） |
| GET      | `/health`                       | 否                | `{"ok": true, "data_dir": "...", "public_dir": "..."}`                                                                                  |

## 安全说明

1. **`GET /api/files/...` 不校验 Token**，以便笔记里 `<img src="...">` 能直接加载；请勿把本服务暴露到公网，或应在前置反向代理上限制来源 IP。
2. 未设置 `API_KEY`（或旧变量 `PAPERLESS_TOKEN`）时，任意能访问该端口的客户端均可调用需鉴权的接口（上传、列表等），仅限可信网络使用。
3. `path` 已做目录穿越防护，文件始终限制在 **`PUBLIC_DIR`**（未设置时等同 `{DOCUMENT_DATA_DIR}/files`）内。

## 项目结构（与本仓库）

- `app.py`：Flask 入口、CORS、`/health`
- `siyuan_updata_route.py`：上述 API 蓝图，可通过 `register_routes(app)` 挂到其他 Flask 应用
