"""Shared loopback HTTP server primitives for EMAD local simulation services."""
from __future__ import annotations

import hmac
from http import HTTPStatus
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import inspect
import json
import mimetypes
from pathlib import Path
import socket
import sys
import threading
from typing import Any
from urllib.parse import unquote, urlparse

REPO_ROOT = Path(__file__).resolve().parents[1]
THREE_ROOT = REPO_ROOT / "apps" / "operator-ui" / "node_modules" / "three"

mimetypes.add_type("application/javascript", ".js")
mimetypes.add_type("application/javascript", ".mjs")
mimetypes.add_type("model/gltf-binary", ".glb")


def cors_origin(origin: str | None) -> str | None:
    if not origin:
        return None
    parsed = urlparse(origin)
    if parsed.scheme in ("http", "https") and parsed.hostname in ("127.0.0.1", "localhost", "::1"):
        return origin
    return None


def json_response(payload: Any, status: int = HTTPStatus.OK) -> tuple[int, str, bytes]:
    return (
        int(status),
        "application/json; charset=utf-8",
        json.dumps(payload, separators=(",", ":")).encode("utf-8"),
    )


def file_response(path: Path, content_type: str | None = None) -> tuple[int, str, bytes]:
    if not path.is_file():
        return int(HTTPStatus.NOT_FOUND), "text/plain; charset=utf-8", b"Not found"
    ctype = content_type or mimetypes.guess_type(str(path))[0] or "application/octet-stream"
    if ctype.startswith("text/") or ctype in ("application/javascript", "application/json"):
        ctype = f"{ctype}; charset=utf-8"
    return int(HTTPStatus.OK), ctype, path.read_bytes()


def serve_three_vendor(req_path: str) -> tuple[int, str, bytes] | None:
    if not req_path.startswith("/vendor/three/"):
        return None
    rel = req_path[len("/vendor/three/") :]
    target = (THREE_ROOT / rel).resolve()
    if not str(target).startswith(str(THREE_ROOT.resolve())):
        return int(HTTPStatus.FORBIDDEN), "text/plain; charset=utf-8", b"Forbidden"
    return file_response(target, "application/javascript")


def verify_worker_secret(headers: dict[str, str] | None, expected_secret: str | None) -> bool:
    """Constant-time validation of loopback Go->Python worker secret header."""
    if not expected_secret:
        return True
    if not headers:
        return False
    supplied = ""
    for k, v in headers.items():
        if k.lower() == "x-emad-worker-secret":
            supplied = str(v)
            break
    if not supplied:
        return False
    return hmac.compare_digest(supplied.encode("utf-8"), expected_secret.encode("utf-8"))


class ExclusiveThreadingHTTPServer(ThreadingHTTPServer):
    """ThreadingHTTPServer that refuses silent port sharing on Windows."""

    allow_reuse_address = False

    def server_bind(self) -> None:
        if hasattr(socket, "SO_EXCLUSIVEADDRUSE"):
            self.socket.setsockopt(socket.SOL_SOCKET, socket.SO_EXCLUSIVEADDRUSE, 1)
        super().server_bind()


class BaseSimHandler(BaseHTTPRequestHandler):
    log_prefix = "[emad-http]"

    def __init__(self, *args, router: Any, **kwargs) -> None:
        self.router = router
        self._accepts_headers = "headers" in inspect.signature(router.resolve).parameters
        super().__init__(*args, **kwargs)

    def log_message(self, format: str, *args) -> None:
        msg = format % args
        if "POST " in msg or " 40" in msg or " 50" in msg:
            print(f"{self.log_prefix} {msg}", file=sys.stderr, flush=True)

    def _write_cors(self) -> None:
        origin = cors_origin(self.headers.get("Origin"))
        if origin:
            self.send_header("Access-Control-Allow-Origin", origin)
            self.send_header("Vary", "Origin")
            self.send_header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
            self.send_header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-EMAD-Worker-Secret")

    def do_OPTIONS(self) -> None:
        self.send_response(HTTPStatus.NO_CONTENT)
        self._write_cors()
        self.end_headers()

    def _dispatch(self, method: str) -> None:
        length = int(self.headers.get("Content-Length") or 0)
        if length > 8 * 1024 * 1024:
            self.send_response(HTTPStatus.REQUEST_ENTITY_TOO_LARGE)
            self._write_cors()
            self.end_headers()
            return
        body = self.rfile.read(length) if length > 0 else b""
        hdrs = {k: v for k, v in self.headers.items()}
        if self._accepts_headers:
            status, ctype, data = self.router.resolve(self.path, method=method, body=body, headers=hdrs)
        else:
            status, ctype, data = self.router.resolve(self.path, method=method, body=body)
        if status in (301, 302, 307, 308) and ctype.startswith("redirect:"):
            target = ctype[len("redirect:") :]
            self.send_response(status)
            self._write_cors()
            self.send_header("Location", target)
            self.send_header("Content-Length", "0")
            self.end_headers()
            return
        self.send_response(status)
        self._write_cors()
        self.send_header("Content-Type", ctype)
        self.send_header("Content-Length", str(len(data)))
        if ctype == "model/gltf-binary":
            self.send_header("Cache-Control", "private, max-age=86400")
        else:
            self.send_header("Cache-Control", "no-store")
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self) -> None:
        self._dispatch("GET")

    def do_POST(self) -> None:
        req_path = unquote(urlparse(self.path).path)
        if req_path == "/api/shutdown" and (
            getattr(self.router, "service", None) is not None or getattr(self.router, "hub", None) is not None
        ):
            expected_secret = getattr(self.router, "worker_secret", None)
            hdrs = {k: v for k, v in self.headers.items()}
            if not verify_worker_secret(hdrs, expected_secret):
                data = json.dumps({"error": "worker authentication required"}).encode("utf-8")
                self.send_response(HTTPStatus.UNAUTHORIZED)
                self._write_cors()
                self.send_header("Content-Type", "application/json; charset=utf-8")
                self.send_header("Content-Length", str(len(data)))
                self.end_headers()
                self.wfile.write(data)
                return
            data = json.dumps({"status": "shutting_down"}).encode("utf-8")
            self.send_response(HTTPStatus.OK)
            self._write_cors()
            self.send_header("Content-Type", "application/json; charset=utf-8")
            self.send_header("Content-Length", str(len(data)))
            self.end_headers()
            self.wfile.write(data)
            threading.Thread(target=self.server.shutdown, daemon=True).start()
            return
        self._dispatch("POST")
