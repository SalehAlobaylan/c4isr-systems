"""Local offline HTTP server for EMAD Multi-Threat Salvo Defense Workbench (Port 8767)."""
from __future__ import annotations

from functools import partial
from http import HTTPStatus
import json
from pathlib import Path
import re
from typing import Any
from urllib.parse import parse_qs, unquote, urlparse

from sim_core.http_base import (
    BaseSimHandler,
    ExclusiveThreadingHTTPServer,
    THREE_ROOT,
    file_response,
    json_response,
    serve_three_vendor,
)
from sim_core.truth_filter import filter_salvo_replay

from ..service import SalvoService

VIEWER_DIR = Path(__file__).resolve().parent
REPO_ROOT = VIEWER_DIR.parents[1]
RUN_ASSET_RE = re.compile(r"^/api/runs/([a-zA-Z0-9_-]+)(?:/(replay\.json|terrain\.glb|imagery\.json|cancel))?$")
SESSION_RE = re.compile(r"^/api/sessions/([a-zA-Z0-9_-]+)(?:/(step|commands|stop))?$")


class SalvoRouter:
    """Socket-free route resolver for the Salvo Defense service & 3D workbench."""

    def __init__(self, service: SalvoService) -> None:
        self.service = service

    def resolve(self, raw_path: str, method: str = "GET", body: bytes = b"") -> tuple[int, str, bytes]:
        req_path = unquote(urlparse(raw_path).path)

        if method == "GET" and req_path == "/favicon.ico":
            return int(HTTPStatus.NO_CONTENT), "image/x-icon", b""
        if method == "GET" and req_path in ("/", "/index.html"):
            return file_response(VIEWER_DIR / "index.html", "text/html")
        if method == "GET" and req_path == "/app.js":
            return file_response(VIEWER_DIR / "app.js", "application/javascript")
        if method == "GET" and req_path.startswith("/vendor/three/"):
            res = serve_three_vendor(req_path)
            if res is not None:
                return res

        if method == "GET" and req_path == "/api/health":
            return json_response(self.service.health())
        if method == "GET" and req_path == "/api/environments":
            return json_response({"environments": self.service.list_environments()})
        if method == "GET" and req_path == "/api/runs":
            return json_response({"runs": self.service.list_runs()})
        if method == "GET" and req_path == "/api/networks":
            return json_response({"networks": self.service.list_networks()})

        # Live interactive command-in-the-loop sessions (Phase 4)
        if method == "GET" and req_path == "/api/sessions":
            return json_response({"sessions": self.service.list_sessions()})
        if method == "POST" and req_path == "/api/sessions":
            try:
                payload = json.loads(body.decode("utf-8") or "{}")
                if not isinstance(payload, dict):
                    raise ValueError("Request body must be a JSON object")
                sess = self.service.start_session(payload)
                return json_response(sess, HTTPStatus.CREATED)
            except (ValueError, KeyError) as exc:
                return json_response({"error": str(exc)}, HTTPStatus.BAD_REQUEST)
            except Exception as exc:
                return json_response({"error": str(exc)}, HTTPStatus.INTERNAL_SERVER_ERROR)

        smatch = SESSION_RE.match(req_path)
        if smatch:
            sid, saction = smatch.group(1), smatch.group(2)
            if method == "GET" and saction is None:
                sess = self.service.get_session(sid)
                if sess is None:
                    return json_response({"error": "Session not found"}, HTTPStatus.NOT_FOUND)
                return json_response(sess)
            if method == "POST" and saction == "step":
                try:
                    payload = json.loads(body.decode("utf-8") or "{}")
                    ticks = int(payload.get("ticks", 1))
                    cmds = payload.get("commands")
                    sess = self.service.step_session(sid, ticks=ticks, commands=cmds if isinstance(cmds, list) else None)
                    if sess is None:
                        return json_response({"error": "Session not found"}, HTTPStatus.NOT_FOUND)
                    return json_response(sess)
                except (ValueError, TypeError) as exc:
                    return json_response({"error": str(exc)}, HTTPStatus.BAD_REQUEST)
            if method == "POST" and saction == "commands":
                try:
                    payload = json.loads(body.decode("utf-8") or "{}")
                    if not isinstance(payload, dict) or not payload.get("kind"):
                        raise ValueError("Command payload must include kind")
                    sess = self.service.command_session(sid, payload)
                    if sess is None:
                        return json_response({"error": "Session not found"}, HTTPStatus.NOT_FOUND)
                    return json_response(sess, HTTPStatus.ACCEPTED)
                except ValueError as exc:
                    return json_response({"error": str(exc)}, HTTPStatus.BAD_REQUEST)
            if method == "POST" and saction == "stop":
                sess = self.service.stop_session(sid)
                if sess is None:
                    return json_response({"error": "Session not found"}, HTTPStatus.NOT_FOUND)
                return json_response(sess)

        if method == "POST" and req_path == "/api/runs":
            try:
                payload = json.loads(body.decode("utf-8") or "{}")
                if not isinstance(payload, dict):
                    raise ValueError("Request body must be a JSON object")
                sync = bool(payload.pop("_sync", False))
                job = self.service.start_run(payload, sync=sync)
                return json_response(job, HTTPStatus.ACCEPTED)
            except (ValueError, KeyError) as exc:
                return json_response({"error": str(exc)}, HTTPStatus.BAD_REQUEST)
            except Exception as exc:
                return json_response({"error": str(exc)}, HTTPStatus.INTERNAL_SERVER_ERROR)

        match = RUN_ASSET_RE.match(req_path)
        if method == "POST" and match and match.group(2) == "cancel":
            job = self.service.cancel_run(match.group(1))
            if job is None:
                return json_response({"error": "Run not found"}, HTTPStatus.NOT_FOUND)
            return json_response(job)
        if method == "GET" and match:
            job_id, asset = match.group(1), match.group(2)
            if asset is None:
                job = self.service.get_run(job_id)
                if job is None:
                    return json_response({"error": "Run not found"}, HTTPStatus.NOT_FOUND)
                return json_response(job)
            assets = self.service.run_assets(job_id)
            if assets is None:
                return int(HTTPStatus.NOT_FOUND), "text/plain; charset=utf-8", b"Run not found"
            run_dir, glb, imagery = assets
            if asset == "replay.json":
                rp = run_dir / "replay.json"
                if not rp.is_file():
                    return int(HTTPStatus.NOT_FOUND), "text/plain; charset=utf-8", b"Not found"
                qs = parse_qs(urlparse(raw_path).query)
                view = (qs.get("view") or ["debrief"])[0].lower()
                if view == "operator":
                    doc = json.loads(rp.read_text(encoding="utf-8"))
                    return json_response(filter_salvo_replay(doc, view="operator"))
                return file_response(rp, "application/json")
            if asset == "terrain.glb":
                if glb is None:
                    return int(HTTPStatus.NOT_FOUND), "text/plain; charset=utf-8", b"No GLB"
                return file_response(glb, "model/gltf-binary")
            if asset == "imagery.json":
                if imagery is None:
                    return int(HTTPStatus.NOT_FOUND), "text/plain; charset=utf-8", b"No imagery"
                return file_response(imagery, "application/json")

        return int(HTTPStatus.NOT_FOUND), "text/plain; charset=utf-8", b"Not found"


class _SalvoHandler(BaseSimHandler):
    log_prefix = "[salvo-http]"


def create_server(*, service: SalvoService, host: str = "127.0.0.1", port: int = 8767) -> ExclusiveThreadingHTTPServer:
    router = SalvoRouter(service)
    handler = partial(_SalvoHandler, router=router)
    return ExclusiveThreadingHTTPServer((host, port), handler)


def serve_service(*, host: str = "127.0.0.1", port: int = 8767, runs_root: Path | None = None) -> None:
    origin = f"http://{host}:{port}"
    svc = SalvoService(
        runs_root=runs_root if runs_root else (REPO_ROOT / "tmp" / "salvo-runs"),
        public_origin=origin,
    )
    server = create_server(service=svc, host=host, port=port)
    actual_port = server.server_address[1]
    svc.public_origin = f"http://{host}:{actual_port}"
    print(f"EMAD 3D Multi-Threat Salvo Defense Service running at http://{host}:{actual_port}/", flush=True)
    print(f"  Discovered environments: {', '.join(svc._presets.keys()) or 'none'}", flush=True)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()
