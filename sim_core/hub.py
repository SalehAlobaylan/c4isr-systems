"""Unified Python Simulation Compute Hub (``python -m sim_core service``).

Hosts all four EMAD Python simulation engines and all three Three.js 3D workbenches
inside **one** OS process sharing:
- A single ``SharedBiomeCache`` (1x RAM footprint for large 24-30 km biomes)
- A single ``SensorNetworkStore`` (instant cross-tool network handoff)
- Two-lane execution isolation: fast interactive LOS/preview queries run immediately
  while heavy multi-policy runs & benchmarks share a bounded ``ThreadPoolExecutor``
- Optional constant-time ``X-EMAD-Worker-Secret`` loopback authentication when fronted
  by Go's ``c4isr-server`` (``internal/simgateway``).
"""
from __future__ import annotations

from concurrent.futures import ThreadPoolExecutor
from functools import partial
from http import HTTPStatus
import json
import os
from pathlib import Path
import threading
from typing import Any
from urllib.parse import parse_qs, unquote, urlparse

from .cache import SharedBiomeCache
from .handoff import SensorNetworkStore
from .http_base import (
    BaseSimHandler,
    ExclusiveThreadingHTTPServer,
    REPO_ROOT,
    file_response,
    json_response,
    serve_three_vendor,
    verify_worker_secret,
)

SIM_HUB_VERSION = "emad-sim-hub-0.4.0"


class UnifiedSimHub:
    """Single-process coordinator owning shared memory caches and all 4 engines."""

    def __init__(
        self,
        *,
        repo_root: Path = REPO_ROOT,
        runs_base: Path | None = None,
        public_origin: str = "http://127.0.0.1:8765",
        max_loaded_biomes: int | None = None,
        heavy_workers: int = 2,
        worker_secret: str | None = None,
    ) -> None:
        from salvo_defense.service import SalvoService
        from sensor_coverage.service import CoverageService
        from sim.service import SimulationService
        from sim_bench.service import BenchService

        self.repo_root = Path(repo_root).resolve()
        self.runs_base = Path(runs_base).resolve() if runs_base else (self.repo_root / "tmp")
        self.public_origin = public_origin.rstrip("/")
        self.worker_secret = worker_secret
        self.cache = SharedBiomeCache(self.repo_root, max_loaded_biomes=max_loaded_biomes)
        self.networks = SensorNetworkStore(self.runs_base / "shared-layouts")
        self.heavy_pool = ThreadPoolExecutor(max_workers=max(1, int(heavy_workers)), thread_name_prefix="emad-sim-heavy")
        self._lock = threading.Lock()
        self._env_summary_cache: list[dict[str, Any]] | None = None

        self.sim = SimulationService(
            repo_root=self.repo_root,
            runs_root=self.runs_base / "sim-runs",
            public_origin=self.public_origin,
            biome_cache=self.cache,
            network_store=self.networks,
            compute_executor=self.heavy_pool,
            viewer_path_prefix="/viewer/sim",
        )
        self.coverage = CoverageService(
            repo_root=self.repo_root,
            runs_root=self.runs_base / "cov-runs",
            public_origin=self.public_origin,
            biome_cache=self.cache,
            network_store=self.networks,
            compute_executor=self.heavy_pool,
            viewer_path_prefix="/viewer/coverage",
        )
        self.salvo = SalvoService(
            repo_root=self.repo_root,
            runs_root=self.runs_base / "salvo-runs",
            public_origin=self.public_origin,
            biome_cache=self.cache,
            network_store=self.networks,
            compute_executor=self.heavy_pool,
            viewer_path_prefix="/viewer/salvo",
        )
        self.bench = BenchService(
            repo_root=self.repo_root,
            public_origin=self.public_origin,
            biome_cache=self.cache,
        )

    def set_public_origin(self, origin: str) -> None:
        origin = origin.rstrip("/")
        self.public_origin = origin
        self.sim.public_origin = origin
        self.coverage.public_origin = origin
        self.salvo.public_origin = origin
        self.bench.public_origin = origin

    def health(self, *, authenticated: bool = True) -> dict[str, Any]:
        base = {
            "status": "ok",
            "service": "emad-sim-hub",
            "version": SIM_HUB_VERSION,
            "authRequired": bool(self.worker_secret),
            "engines": ["sim", "coverage", "salvo", "bench"],
            "capabilities": [
                "unified-hub",
                "shared-biome-cache",
                "two-lane-compute",
                "los-profile",
                "preview-sensor",
                "adversarial-route",
                "ew-jamming",
                "export-network",
                "multi-sensor-fusion",
                "layered-wta",
                "monte-carlo",
                "ml-surrogate",
                "ai-wta-policy",
                "live-salvo-sessions",
                "truth-partition-views",
            ],
        }
        if not authenticated:
            return base
        return {
            **base,
            "viewerOrigin": self.public_origin,
            "cache": self.cache.stats(),
            "subservices": {
                "sim": self.sim.health(),
                "coverage": self.coverage.health(),
                "salvo": self.salvo.health(),
                "bench": self.bench.health(),
            },
        }

    def list_biomes_summary(self) -> list[dict[str, Any]]:
        """Lightweight catalog of all discovered biomes without thrashing the heavy LRU cache."""
        with self._lock:
            if self._env_summary_cache is not None:
                return list(self._env_summary_cache)
        out = []
        for p in self.cache.presets.values():
            out.append(
                {
                    "id": p.id,
                    "title": p.title,
                    "biome": p.biome,
                    "description": p.description,
                    "expectedManifestSha256": p.expected_manifest_sha256,
                    "hasGlb": bool(p.glb_path and p.glb_path.is_file()),
                    "hasImagery": bool(p.imagery_path and p.imagery_path.is_file()),
                    "hasGhsObat": bool(p.ghs_obat_path and p.ghs_obat_path.is_file()),
                    "hasWorldPop": bool(p.worldpop_path and p.worldpop_path.is_file()),
                    "hasGebco": bool(p.gebco_path and p.gebco_path.is_file()),
                    "hasCamsDust": bool(p.cams_dust_path and p.cams_dust_path.is_file()),
                }
            )
        with self._lock:
            self._env_summary_cache = out
        return list(out)

    def biome_assets(self, env_id: str) -> tuple[Path | None, Path | None] | None:
        preset = self.cache.presets.get(env_id)
        if preset is None:
            return None
        glb = preset.glb_path if (preset.glb_path and preset.glb_path.is_file()) else None
        imagery = preset.imagery_path if (preset.imagery_path and preset.imagery_path.is_file()) else None
        return glb, imagery

    def shutdown(self) -> None:
        self.heavy_pool.shutdown(wait=False, cancel_futures=True)


class HubRouter:
    """Socket-free unified HTTP router multiplexing all 4 engines and 3 viewers."""

    def __init__(self, hub: UnifiedSimHub) -> None:
        from salvo_defense.viewer.server import SalvoRouter
        from sensor_coverage.viewer.server import CoverageRouter
        from sim.viewer.server import ReplayRouter
        from sim_bench.server import BenchRouter

        self.hub = hub
        self.worker_secret = hub.worker_secret
        self._sim_router = ReplayRouter(service=hub.sim)
        self._cov_router = CoverageRouter(service=hub.coverage)
        self._salvo_router = SalvoRouter(service=hub.salvo)
        self._bench_router = BenchRouter(service=hub.bench)

    def _forward(
        self,
        router: Any,
        raw_path: str,
        prefix: str,
        replacement: str,
        *,
        method: str,
        body: bytes,
    ) -> tuple[int, str, bytes]:
        parsed = urlparse(raw_path)
        tail = parsed.path[len(prefix) :]
        new_path = replacement + tail
        if parsed.query:
            new_path = f"{new_path}?{parsed.query}"
        return router.resolve(new_path, method=method, body=body)

    def resolve(
        self,
        raw_path: str,
        method: str = "GET",
        body: bytes = b"",
        headers: dict[str, str] | None = None,
    ) -> tuple[int, str, bytes]:
        parsed = urlparse(raw_path)
        req_path = unquote(parsed.path)

        # Static browser assets (Three.js + 3D workbenches)
        if method == "GET" and req_path == "/favicon.ico":
            return int(HTTPStatus.NO_CONTENT), "image/x-icon", b""
        if method == "GET" and req_path.startswith("/vendor/three/"):
            res = serve_three_vendor(req_path)
            if res is not None:
                return res

        # 3D Workbench viewers hosted cleanly under /viewer/{sim,coverage,salvo}/
        for engine_name, viewer_dir in (
            ("sim", self.hub.repo_root / "sim" / "viewer"),
            ("coverage", self.hub.repo_root / "sensor_coverage" / "viewer"),
            ("salvo", self.hub.repo_root / "salvo_defense" / "viewer"),
        ):
            base = f"/viewer/{engine_name}"
            if method == "GET" and req_path == base:
                target = f"{base}/" + (f"?{parsed.query}" if parsed.query else "")
                return int(HTTPStatus.FOUND), f"redirect:{target}", b""
            if method == "GET" and req_path in (f"{base}/", f"{base}/index.html"):
                return file_response(viewer_dir / "index.html", "text/html")
            if method == "GET" and req_path == f"{base}/app.js":
                return file_response(viewer_dir / "app.js", "application/javascript")

        # Default root falls back cleanly to Sim 3D workbench for legacy bookmarks
        if method == "GET" and req_path in ("/", "/index.html"):
            return file_response(self.hub.repo_root / "sim" / "viewer" / "index.html", "text/html")
        if method == "GET" and req_path == "/app.js":
            return file_response(self.hub.repo_root / "sim" / "viewer" / "app.js", "application/javascript")

        # Liveness probe (redacts internals unless authenticated when secret is active)
        if method == "GET" and req_path == "/api/health":
            authed = verify_worker_secret(headers, self.worker_secret)
            return json_response(self.hub.health(authenticated=authed))

        # Enforce loopback worker-secret authentication on all API / internal routes
        if (req_path.startswith("/api/") or req_path.startswith("/internal/")) and not verify_worker_secret(
            headers, self.worker_secret
        ):
            return json_response({"error": "worker authentication required"}, HTTPStatus.UNAUTHORIZED)

        # Hub-level endpoints
        if method == "GET" and req_path == "/api/cache/stats":
            return json_response(self.hub.cache.stats())
        if method == "GET" and req_path == "/api/biomes":
            return json_response({"biomes": self.hub.list_biomes_summary()})
        if method == "GET" and req_path.startswith("/api/biomes/"):
            parts = req_path.split("/")
            if len(parts) == 5 and parts[4] in ("terrain.glb", "imagery.json"):
                assets = self.hub.biome_assets(parts[3])
                if assets is None:
                    return int(HTTPStatus.NOT_FOUND), "text/plain; charset=utf-8", b"Biome not found"
                glb, imagery = assets
                if parts[4] == "terrain.glb":
                    if glb is None:
                        return int(HTTPStatus.NOT_FOUND), "text/plain; charset=utf-8", b"No GLB for biome"
                    return file_response(glb, "model/gltf-binary")
                if imagery is None:
                    return int(HTTPStatus.NOT_FOUND), "text/plain; charset=utf-8", b"No imagery for biome"
                return file_response(imagery, "application/json")
        if method == "GET" and req_path == "/api/networks":
            qs = parse_qs(parsed.query)
            env_id = qs.get("environmentId", [None])[0]
            return json_response({"networks": self.hub.networks.list(environment_id=env_id)})

        # Namespaced engine routes: /api/{sim,coverage,salvo,bench}/* and /internal/{sim,coverage,salvo,bench}/*
        for api_root in ("/api", "/internal"):
            if req_path.startswith(f"{api_root}/sim/"):
                return self._forward(self._sim_router, raw_path, f"{api_root}/sim", "/api", method=method, body=body)
            if req_path.startswith(f"{api_root}/coverage/"):
                return self._forward(
                    self._cov_router, raw_path, f"{api_root}/coverage", "/api", method=method, body=body
                )
            if req_path.startswith(f"{api_root}/salvo/"):
                return self._forward(self._salvo_router, raw_path, f"{api_root}/salvo", "/api", method=method, body=body)
            if req_path.startswith(f"{api_root}/bench/"):
                return self._forward(self._bench_router, raw_path, f"{api_root}/bench", "/api", method=method, body=body)

        # Backwards-compatible unprefixed /api/* routes resolve against Sim engine (legacy port 8765 behavior)
        if req_path.startswith("/api/"):
            return self._sim_router.resolve(raw_path, method=method, body=body)

        return int(HTTPStatus.NOT_FOUND), "text/plain; charset=utf-8", b"Not found"


class _HubHandler(BaseSimHandler):
    log_prefix = "[sim-hub]"


def create_server(
    *,
    hub: UnifiedSimHub,
    host: str = "127.0.0.1",
    port: int = 8765,
) -> ExclusiveThreadingHTTPServer:
    router = HubRouter(hub)
    handler = partial(_HubHandler, router=router)
    return ExclusiveThreadingHTTPServer((host, port), handler)


def serve_hub(
    *,
    host: str = "127.0.0.1",
    port: int = 8765,
    runs_base: Path | None = None,
    max_loaded_biomes: int | None = None,
    worker_secret: str | None = None,
) -> None:
    secret = worker_secret if worker_secret is not None else (os.environ.get("C4ISR_SIM_WORKER_SECRET") or None)
    origin = f"http://{host}:{port}"
    hub = UnifiedSimHub(
        runs_base=runs_base,
        public_origin=origin,
        max_loaded_biomes=max_loaded_biomes,
        worker_secret=secret,
    )
    server = create_server(hub=hub, host=host, port=port)
    actual_port = server.server_address[1]
    hub.set_public_origin(f"http://{host}:{actual_port}")
    auth_mode = "worker-secret required" if secret else "standalone loopback"
    print(f"EMAD Unified Simulation Compute Hub running at http://{host}:{actual_port}/ ({auth_mode})", flush=True)
    print(f"  Engines: /api/sim · /api/coverage · /api/salvo · /api/bench", flush=True)
    print(f"  Biomes ({len(hub.cache.presets)}): {', '.join(hub.cache.presets.keys()) or 'none'}", flush=True)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        hub.shutdown()
        server.server_close()
