"""Local offline HTTP server for the EMAD 3D Simulation Service & Replay Viewer.

Supports two clean modes:
1. **Service mode** (``python -m sim service --port 8765`` — auto-started by
   ``task dev``): exposes ``/api/health``, ``/api/environments``, ``/api/runs``,
   ``/api/runs/<id>``, ``/api/runs/<id>/replay.json``, ``/api/runs/<id>/terrain.glb``,
   and ``/api/runs/<id>/imagery.json`` alongside the Three.js 3D replay viewer at
   ``http://127.0.0.1:8765/?run=<jobId>``.
2. **Single-bundle mode** (``python -m sim serve <run_dir>``): serves one static
   replay directory directly.

Three.js modules are served directly from ``apps/operator-ui/node_modules/three``
with zero external CDN requests.
"""
from __future__ import annotations

from functools import partial
from http import HTTPStatus
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import mimetypes
from pathlib import Path
import re
import socket
import sys
from typing import Any
from urllib.parse import unquote, urlparse

from ..service import SimulationService

VIEWER_DIR = Path(__file__).resolve().parent
REPO_ROOT = VIEWER_DIR.parents[1]
THREE_ROOT = REPO_ROOT / "apps" / "operator-ui" / "node_modules" / "three"

mimetypes.add_type("application/javascript", ".js")
mimetypes.add_type("application/javascript", ".mjs")
mimetypes.add_type("model/gltf-binary", ".glb")

RUN_ASSET_RE = re.compile(r"^/api/runs/([a-zA-Z0-9_-]+)(?:/(replay\.json|terrain\.glb|imagery\.json))?$")


def _cors_origin(origin: str | None) -> str | None:
    if not origin:
        return None
    parsed = urlparse(origin)
    if parsed.scheme in ("http", "https") and parsed.hostname in ("127.0.0.1", "localhost", "::1"):
        return origin
    return None


class ReplayRouter:
    """Pure, socket-free route resolver for both single-bundle and multi-run service modes."""

    def __init__(
        self,
        run_dir: Path | None = None,
        *,
        glb_path: Path | None = None,
        imagery_path: Path | None = None,
        service: SimulationService | None = None,
    ) -> None:
        self.run_dir = Path(run_dir).resolve() if run_dir else None
        self.glb_path = Path(glb_path).resolve() if glb_path else None
        self.imagery_path = Path(imagery_path).resolve() if imagery_path else None
        self.service = service

    @staticmethod
    def _json_response(payload: Any, status: int = HTTPStatus.OK) -> tuple[int, str, bytes]:
        return (
            int(status),
            "application/json; charset=utf-8",
            json.dumps(payload, separators=(",", ":")).encode("utf-8"),
        )

    @staticmethod
    def _file_response(path: Path, content_type: str | None = None) -> tuple[int, str, bytes]:
        if not path.is_file():
            return int(HTTPStatus.NOT_FOUND), "text/plain; charset=utf-8", b"Not found"
        ctype = content_type or mimetypes.guess_type(str(path))[0] or "application/octet-stream"
        if ctype.startswith("text/") or ctype in ("application/javascript", "application/json"):
            ctype = f"{ctype}; charset=utf-8"
        return int(HTTPStatus.OK), ctype, path.read_bytes()

    def resolve(self, raw_path: str, method: str = "GET", body: bytes = b"") -> tuple[int, str, bytes]:
        req_path = unquote(urlparse(raw_path).path)

        # Static viewer assets (always available in both modes)
        if method == "GET" and req_path == "/favicon.ico":
            return int(HTTPStatus.NO_CONTENT), "image/x-icon", b""
        if method == "GET" and req_path in ("/", "/index.html"):
            return self._file_response(VIEWER_DIR / "index.html", "text/html")
        if method == "GET" and req_path == "/app.js":
            return self._file_response(VIEWER_DIR / "app.js", "application/javascript")
        if method == "GET" and req_path.startswith("/vendor/three/"):
            rel = req_path[len("/vendor/three/") :]
            target = (THREE_ROOT / rel).resolve()
            if not str(target).startswith(str(THREE_ROOT.resolve())):
                return int(HTTPStatus.FORBIDDEN), "text/plain; charset=utf-8", b"Forbidden"
            return self._file_response(target, "application/javascript")

        # Multi-run interactive service API
        if self.service is not None:
            if method == "GET" and req_path == "/api/health":
                return self._json_response(self.service.health())
            if method == "GET" and req_path == "/api/environments":
                return self._json_response({"environments": self.service.list_environments()})
            if method == "GET" and req_path == "/api/runs":
                return self._json_response({"runs": self.service.list_runs()})
            if method == "GET" and req_path == "/api/networks":
                return self._json_response({"networks": self.service.list_sensor_networks()})
            if method == "POST" and req_path == "/api/runs":
                try:
                    payload = json.loads(body.decode("utf-8") or "{}")
                    if not isinstance(payload, dict):
                        raise ValueError("Request body must be a JSON object")
                    sync = bool(payload.pop("_sync", False))
                    job = self.service.start_run(payload, sync=sync)
                    return self._json_response(job, HTTPStatus.ACCEPTED)
                except (ValueError, KeyError) as exc:
                    return self._json_response({"error": str(exc)}, HTTPStatus.BAD_REQUEST)
                except Exception as exc:
                    return self._json_response({"error": str(exc)}, HTTPStatus.INTERNAL_SERVER_ERROR)

            match = RUN_ASSET_RE.match(req_path)
            if method == "GET" and match:
                job_id, asset = match.group(1), match.group(2)
                if asset is None:
                    job = self.service.get_run(job_id)
                    if job is None:
                        return self._json_response({"error": "Run not found"}, HTTPStatus.NOT_FOUND)
                    return self._json_response(job)
                assets = self.service.run_assets(job_id)
                if assets is None:
                    return int(HTTPStatus.NOT_FOUND), "text/plain; charset=utf-8", b"Run not found"
                run_dir, glb, imagery = assets
                if asset == "replay.json":
                    return self._file_response(run_dir / "replay.json", "application/json")
                if asset == "terrain.glb":
                    if glb is None:
                        return int(HTTPStatus.NOT_FOUND), "text/plain; charset=utf-8", b"No GLB for environment"
                    return self._file_response(glb, "model/gltf-binary")
                if asset == "imagery.json":
                    if imagery is None:
                        return int(HTTPStatus.NOT_FOUND), "text/plain; charset=utf-8", b"No imagery for environment"
                    return self._file_response(imagery, "application/json")

        # Single-bundle legacy/direct viewer routes
        if method == "GET" and req_path == "/data/meta.json":
            meta = {
                "serviceMode": self.service is not None,
                "hasGlb": bool(self.glb_path and self.glb_path.is_file()),
                "hasImagery": bool(self.imagery_path and self.imagery_path.is_file()),
                "hasThree": THREE_ROOT.is_dir(),
                "runDir": str(self.run_dir) if self.run_dir else None,
            }
            return self._json_response(meta)
        if method == "GET" and req_path == "/data/replay.json" and self.run_dir is not None:
            return self._file_response(self.run_dir / "replay.json", "application/json")
        if method == "GET" and req_path == "/data/terrain.glb":
            if self.glb_path and self.glb_path.is_file():
                return self._file_response(self.glb_path, "model/gltf-binary")
            return int(HTTPStatus.NOT_FOUND), "text/plain; charset=utf-8", b"No GLB configured"
        if method == "GET" and req_path == "/data/imagery.json":
            if self.imagery_path and self.imagery_path.is_file():
                return self._file_response(self.imagery_path, "application/json")
            return int(HTTPStatus.NOT_FOUND), "text/plain; charset=utf-8", b"No imagery configured"

        return int(HTTPStatus.NOT_FOUND), "text/plain; charset=utf-8", b"Not found"


class ReplayHandler(BaseHTTPRequestHandler):
    def __init__(self, *args, router: ReplayRouter, **kwargs) -> None:
        self.router = router
        super().__init__(*args, **kwargs)

    def log_message(self, format: str, *args) -> None:
        # Log errors and POST requests for diagnostics without flooding logs
        msg = format % args
        if "POST " in msg or " 40" in msg or " 50" in msg:
            print(f"[sim-http] {msg}", file=sys.stderr, flush=True)

    def _write_cors(self) -> None:
        origin = _cors_origin(self.headers.get("Origin"))
        if origin:
            self.send_header("Access-Control-Allow-Origin", origin)
            self.send_header("Vary", "Origin")
            self.send_header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
            self.send_header("Access-Control-Allow-Headers", "Content-Type")

    def do_OPTIONS(self) -> None:
        self.send_response(HTTPStatus.NO_CONTENT)
        self._write_cors()
        self.end_headers()

    def _dispatch(self, method: str) -> None:
        length = int(self.headers.get("Content-Length") or 0)
        if length > 2 * 1024 * 1024:
            self.send_response(HTTPStatus.REQUEST_ENTITY_TOO_LARGE)
            self._write_cors()
            self.end_headers()
            return
        body = self.rfile.read(length) if length > 0 else b""
        status, ctype, data = self.router.resolve(self.path, method=method, body=body)
        self.send_response(status)
        self._write_cors()
        self.send_header("Content-Type", ctype)
        self.send_header("Content-Length", str(len(data)))
        self.send_header("Cache-Control", "no-store")
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self) -> None:
        self._dispatch("GET")

    def do_POST(self) -> None:
        if unquote(urlparse(self.path).path) == "/api/shutdown" and self.router.service is not None:
            data = json.dumps({"status": "shutting_down"}).encode("utf-8")
            self.send_response(HTTPStatus.OK)
            self._write_cors()
            self.send_header("Content-Type", "application/json; charset=utf-8")
            self.send_header("Content-Length", str(len(data)))
            self.end_headers()
            self.wfile.write(data)
            import threading

            threading.Thread(target=self.server.shutdown, daemon=True).start()
            return
        self._dispatch("POST")


class ExclusiveThreadingHTTPServer(ThreadingHTTPServer):
    """ThreadingHTTPServer that refuses silent port sharing on Windows."""

    allow_reuse_address = False

    def server_bind(self) -> None:
        if hasattr(socket, "SO_EXCLUSIVEADDRUSE"):
            self.socket.setsockopt(socket.SOL_SOCKET, socket.SO_EXCLUSIVEADDRUSE, 1)
        super().server_bind()


def create_server(
    run_dir: Path | None = None,
    *,
    glb_path: Path | None = None,
    imagery_path: Path | None = None,
    service: SimulationService | None = None,
    host: str = "127.0.0.1",
    port: int = 8765,
) -> ThreadingHTTPServer:
    if service is None:
        if run_dir is None or not (Path(run_dir) / "replay.json").is_file():
            raise FileNotFoundError(f"replay.json not found in {run_dir}")
    router = ReplayRouter(run_dir, glb_path=glb_path, imagery_path=imagery_path, service=service)
    handler = partial(ReplayHandler, router=router)
    return ExclusiveThreadingHTTPServer((host, port), handler)


def serve_replay(
    run_dir: Path,
    *,
    glb_path: Path | None = None,
    imagery_path: Path | None = None,
    host: str = "127.0.0.1",
    port: int = 8765,
) -> None:
    server = create_server(run_dir, glb_path=glb_path, imagery_path=imagery_path, host=host, port=port)
    actual_port = server.server_address[1]
    print(f"EMAD 3D Simulation Replay Viewer running at http://{host}:{actual_port}/", flush=True)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()


def serve_service(*, host: str = "127.0.0.1", port: int = 8765, runs_root: Path | None = None) -> None:
    origin = f"http://{host}:{port}"
    svc = SimulationService(
        runs_root=runs_root if runs_root else (REPO_ROOT / "tmp" / "sim-runs"),
        public_origin=origin,
    )
    server = create_server(service=svc, host=host, port=port)
    actual_port = server.server_address[1]
    svc.public_origin = f"http://{host}:{actual_port}"
    print(f"EMAD 3D Simulation Service running at http://{host}:{actual_port}/", flush=True)
    print(f"  Discovered environments: {', '.join(svc._presets.keys()) or 'none'}", flush=True)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()
