"""Local loopback HTTP server for EMAD Monte Carlo & AI Benchmarks Service (Port 8768)."""
from __future__ import annotations

from functools import partial
from http import HTTPStatus
import json
from typing import Any
from urllib.parse import unquote, urlparse

from sim_core.http_base import BaseSimHandler, ExclusiveThreadingHTTPServer, json_response

from .service import BenchService


class BenchRouter:
    def __init__(self, service: BenchService) -> None:
        self.service = service

    def resolve(self, raw_path: str, method: str = "GET", body: bytes = b"") -> tuple[int, str, bytes]:
        req_path = unquote(urlparse(raw_path).path)
        if method == "GET" and req_path == "/api/health":
            return json_response(self.service.health())
        if method == "GET" and req_path == "/api/environments":
            return json_response({"environments": self.service.list_environments()})
        if method == "GET" and req_path == "/api/models":
            return json_response(self.service.models_status())
        if method == "GET" and req_path == "/api/leaderboard":
            lb = self.service.get_leaderboard()
            return json_response({"leaderboard": lb})
        if method == "POST" and req_path == "/api/monte-carlo":
            try:
                payload = json.loads(body.decode("utf-8") or "{}")
                return json_response(self.service.run_monte_carlo(payload))
            except (ValueError, KeyError) as exc:
                return json_response({"error": str(exc)}, HTTPStatus.BAD_REQUEST)
            except Exception as exc:
                return json_response({"error": str(exc)}, HTTPStatus.INTERNAL_SERVER_ERROR)
        if method == "POST" and req_path == "/api/benchmark":
            try:
                payload = json.loads(body.decode("utf-8") or "{}")
                return json_response(self.service.run_leaderboard(payload))
            except Exception as exc:
                return json_response({"error": str(exc)}, HTTPStatus.INTERNAL_SERVER_ERROR)
        return int(HTTPStatus.NOT_FOUND), "text/plain; charset=utf-8", b"Not found"


class _BenchHandler(BaseSimHandler):
    log_prefix = "[bench-http]"


def serve_service(*, host: str = "127.0.0.1", port: int = 8768) -> None:
    svc = BenchService(public_origin=f"http://{host}:{port}")
    router = BenchRouter(svc)
    server = ExclusiveThreadingHTTPServer((host, port), partial(_BenchHandler, router=router))
    actual_port = server.server_address[1]
    svc.public_origin = f"http://{host}:{actual_port}"
    print(f"EMAD Monte Carlo & AI Benchmark Service running at http://{host}:{actual_port}/", flush=True)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()
