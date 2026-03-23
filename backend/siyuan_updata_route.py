"""
思源 Paperless 插件兼容 API：仅接收文件写入磁盘，不使用数据库。
GET 仍保留为插件连通性/查询占位（恒为空列表，不做去重）。
通过 register_routes(app) 挂到 Flask 应用。
"""

from __future__ import annotations

import os
import re
import uuid
from pathlib import Path

from flask import Blueprint, current_app, jsonify, request, send_file
from werkzeug.utils import secure_filename

bp = Blueprint("siyuan_paperless_local", __name__)

API_KEY = os.environ.get("API_KEY")
if not API_KEY:
    API_KEY = "PicServerAPIKey_xdfdwefddas1234567890"


def _files_dir() -> Path:
    """上传与静态读取的根目录，由环境变量 PUBLIC_DIR 或 DOCUMENT_DATA_DIR/files 决定。"""
    return Path(current_app.config["PUBLIC_DIR"]).resolve()


def _safe_relpath_from_workspace(raw: str) -> Path | None:
    """
    将前端传来的工作区路径（如 /data/assets/xxx.png）转为 PUBLIC_DIR 下的安全相对路径。
    禁止 .. 与空段，每段经 secure_filename。
    """
    if not raw or not isinstance(raw, str):
        return None
    s = raw.strip().replace("\\", "/").lstrip("/")
    if s.startswith("data/"):
        s = s[5:].lstrip("/")
    parts: list[str] = []
    for seg in s.split("/"):
        if not seg or seg == ".":
            continue
        if seg == "..":
            return None
        safe = secure_filename(seg)
        if not safe:
            return None
        parts.append(safe)
    if not parts:
        return None
    return Path(*parts)


def _is_under_files_root(path: Path) -> bool:
    try:
        path.resolve().relative_to(_files_dir().resolve())
        return True
    except ValueError:
        return False


def _auth_ok() -> bool:
    # 优先使用 API_KEY；兼容旧变量 PAPERLESS_TOKEN
    expected = (os.environ.get("API_KEY") or os.environ.get("PAPERLESS_TOKEN") or "").strip()
    if not expected:
        return True
    exp = expected[6:].strip() if expected.lower().startswith("token ") else expected
    auth = (request.headers.get("Authorization") or "").strip()
    if not auth:
        return False
    got = auth[6:].strip() if auth.lower().startswith("token ") else auth
    return got == exp


def _auth_guard():
    if _auth_ok():
        return None
    return jsonify({"detail": "unauthorized"}), 401


def init_storage(app) -> None:
    Path(app.config["PUBLIC_DIR"]).resolve().mkdir(parents=True, exist_ok=True)

@bp.route("/api/testConnection", methods=["GET", "POST"])
def test_connection():
    guard = _auth_guard()
    if guard:
        return guard
    return jsonify({"success": True})

@bp.get("/api/documentExists/")
@bp.get("/api/documentExists/<path:path>")
def documentExists(path: str | None = None):
    """
    检查资源是否已保存（按 path 对应到 files 下相对路径判断）。
    兼容两种传参：
    - /api/documentExists/?path=/data/assets/xxx.png
    - /api/documentExists/<path:path>
    返回 results 数组，便于前端沿用通用解析逻辑。
    """
    guard = _auth_guard()
    if guard:
        return guard

    raw = (request.args.get("path") or path or "").strip()
    if not raw:
        return jsonify(
            {
                "success": False,
                "detail": "missing path",
                "message": "missing path",
                "count": 0,
                "results": [],
            }
        ), 400

    rel = _safe_relpath_from_workspace(raw)
    if rel is None:
        return jsonify(
            {
                "success": True,
                "path": raw,
                "exists": False,
                "count": 0,
                "results": [],
            }
        )

    file_path = (_files_dir() / rel).resolve()
    exists = _is_under_files_root(file_path) and file_path.is_file()
    #file_url 走 flask 的 api 路径，public_url 走 nginx 的 public 路径
    results = (
        [{"path": raw, "file_url": f"/api/files/{rel.as_posix()}", "public_url": f"/public/{rel.as_posix()}"}]
        if exists
        else []
    )
    return jsonify(
        {
            "success": True,
            "path": raw,
            "exists": exists,
            "count": len(results),
            "results": results,
        }
    )


@bp.post("/api/documents/post_document/")
def post_document():
    """对齐插件：FormData title + document；成功时返回纯文本 UUID。"""
    guard = _auth_guard()
    if guard:
        return guard

    if "document" not in request.files:
        return jsonify({"success": False,"detail": "missing file field document","message":"missing file field document"}), 400

    f = request.files["document"]
    orig = f.filename or "bin"
    ext = Path(orig).suffix

    _path = (request.form.get("path") or "").strip()
    doc_id = str(uuid.uuid4())

    if _path:
        rel = _safe_relpath_from_workspace(_path)
        if rel is None or not rel.suffix:
            return jsonify(
                {"success": False, "detail": "invalid path", "message": "invalid path"},
            ), 400
        ext = rel.suffix
    else:
        safe_stub = secure_filename(Path(orig).stem) or "file"
        rel = Path(f"{doc_id}_{safe_stub}{ext}")

    file_path = (_files_dir() / rel).resolve()
    if not _is_under_files_root(file_path):
        return jsonify(
            {"success": False, "detail": "path escapes storage", "message": "path escapes storage"},
        ), 400

    file_path.parent.mkdir(parents=True, exist_ok=True)
    f.save(file_path)

    return jsonify(
        {
            "success": True,
            "id": doc_id,
            "file_url": f"/api/files/{rel.as_posix()}",
        }
    )


_UUID_RE = re.compile(r"^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$", re.I)


@bp.get("/api/files/<path:subpath>")
def serve_uploaded_file(subpath: str):
    """
    读取已上传文件（不做 Token 校验，便于笔记内引用；勿公网暴露）。
    - 单段且为 UUID：兼容旧逻辑，匹配 PUBLIC_DIR 下 {uuid}_*
    - 否则：按相对路径（与上传时 _safe_relpath_from_workspace 规则一致）读取
    """
    subpath = subpath.strip().replace("\\", "/")
    if _UUID_RE.fullmatch(subpath):
        matches = list(_files_dir().glob(f"{subpath}_*"))
        if not matches:
            return jsonify({"detail": "not found"}), 404
        return send_file(matches[0], conditional=True)

    rel = _safe_relpath_from_workspace(subpath)
    if rel is None:
        return jsonify({"detail": "invalid path"}), 400
    file_path = (_files_dir() / rel).resolve()
    if not _is_under_files_root(file_path) or not file_path.is_file():
        return jsonify({"detail": "not found"}), 404
    return send_file(file_path, conditional=True)


def register_routes(app) -> None:
    """注册路由；未配置 DOCUMENT_DATA_DIR 时使用 backend/data；PUBLIC_DIR 默认为其下 files。"""
    dd = app.config.setdefault(
        "DOCUMENT_DATA_DIR",
        os.path.abspath(os.path.join(os.path.dirname(__file__), "data")),
    )
    app.config.setdefault(
        "PUBLIC_DIR",
        os.path.abspath(os.path.join(dd, "files")),
    )
    init_storage(app)
    app.register_blueprint(bp)
