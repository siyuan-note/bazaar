"""
思源 Paperless 插件兼容 API 入口：本地文档库，见 siyuan_updata_route.register_routes。
"""

from __future__ import annotations

import os

from dotenv import load_dotenv
from flask import Flask
from flask_cors import CORS

from siyuan_updata_route import register_routes

load_dotenv()

_document_data_dir = os.environ.get(
    "DOCUMENT_DATA_DIR",
    os.path.abspath(os.path.join(os.path.dirname(__file__), "data")),
)
#_public_dir = os.environ.get("PUBLIC_DIR")
_public_dir = "/www/wwwroot/public"
if _public_dir:
    _public_dir = os.path.abspath(_public_dir)
else:
    _public_dir = os.path.abspath(os.path.join(_document_data_dir, "files"))

app = Flask(__name__)
app.config.setdefault("DOCUMENT_DATA_DIR", _document_data_dir)
app.config.setdefault("PUBLIC_DIR", _public_dir)

CORS(app, resources={r"/api/*": {"origins": "*"}})
register_routes(app)


@app.get("/health")
def health():
    return {
        "ok": True,
        "data_dir": app.config["DOCUMENT_DATA_DIR"],
        "public_dir": app.config["PUBLIC_DIR"],
    }


if __name__ == "__main__":
    # 默认 0.0.0.0：允许用本机局域网 IP（如 192.168.x.x）访问；仅本机可设 FLASK_HOST=127.0.0.1
    host = os.environ.get("FLASK_HOST", "0.0.0.0")
    port = int(os.environ.get("FLASK_PORT", "5000"))
    app.run(
        host=host,
        port=port,
        debug=os.environ.get("FLASK_DEBUG") == "1",
        threaded=True,
    )
