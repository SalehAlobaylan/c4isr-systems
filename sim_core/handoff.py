"""Cross-simulation sensor-network handoff store (``emad.sensor-network.v1``).

Allows an operator to export an optimized multi-sensor layout from
``sensor_coverage`` and import it directly into ``sim`` (Intercept) or
``salvo_defense`` (Layered Defense), pinned to the exact verified environment
manifest SHA-256.
"""
from __future__ import annotations

import datetime as dt
import json
from pathlib import Path
import re
import secrets
from typing import Any

SENSOR_NETWORK_SCHEMA = "emad.sensor-network.v1"
REPO_ROOT = Path(__file__).resolve().parents[1]
DEFAULT_LAYOUTS_ROOT = REPO_ROOT / "tmp" / "shared-layouts"
LAYOUT_ID_RE = re.compile(r"^[a-zA-Z0-9_-]{4,48}$")


class SensorNetworkStore:
    def __init__(self, root: Path = DEFAULT_LAYOUTS_ROOT) -> None:
        self.root = Path(root).resolve()
        self.root.mkdir(parents=True, exist_ok=True)

    def save(
        self,
        *,
        name: str,
        environment_id: str,
        manifest_sha256: str,
        sensors: list[dict[str, Any]],
        source_job_id: str | None = None,
        metrics: dict[str, Any] | None = None,
        layout_id: str | None = None,
    ) -> dict[str, Any]:
        if not sensors:
            raise ValueError("Cannot export an empty sensor network.")
        lid = layout_id if (layout_id and LAYOUT_ID_RE.fullmatch(layout_id)) else ("net_" + secrets.token_hex(6))
        now = dt.datetime.now(dt.timezone.utc).isoformat().replace("+00:00", "Z")
        cleaned = []
        for idx, s in enumerate(sensors):
            cleaned.append(
                {
                    "id": str(s.get("id") or f"sensor-{idx + 1}"),
                    "type": str(s.get("type") or "radar"),
                    "label": str(s.get("label") or s.get("id") or f"Sensor {idx + 1}"),
                    "lon": round(float(s["lon"]), 6),
                    "lat": round(float(s["lat"]), 6),
                    "mastAglM": float(s.get("mastAglM", 15.0)),
                    "maxRangeM": float(s.get("maxRangeM", 9000.0)),
                    "minElevationDeg": float(s.get("minElevationDeg", -1.0)),
                    "clutterFloorAglM": float(s.get("clutterFloorAglM", 10.0)),
                    "sectorCenterDeg": float(s.get("sectorCenterDeg", 0.0)),
                    "sectorWidthDeg": float(s.get("sectorWidthDeg", 360.0)),
                    "origin": str(s.get("origin", "optimized")),
                }
            )
        doc = {
            "schema": SENSOR_NETWORK_SCHEMA,
            "layoutId": lid,
            "name": str(name or f"{environment_id} sensor network")[:100],
            "environmentId": str(environment_id),
            "manifestSha256": str(manifest_sha256),
            "sourceJobId": source_job_id,
            "createdAt": now,
            "sensorCount": len(cleaned),
            "sensors": cleaned,
            "metrics": metrics,
        }
        (self.root / f"{lid}.json").write_text(json.dumps(doc, indent=2), encoding="utf-8")
        return doc

    def list(self, *, environment_id: str | None = None) -> list[dict[str, Any]]:
        out = []
        for p in sorted(self.root.glob("*.json")):
            try:
                doc = json.loads(p.read_text(encoding="utf-8"))
                if doc.get("schema") != SENSOR_NETWORK_SCHEMA:
                    continue
                if environment_id and doc.get("environmentId") != environment_id:
                    continue
                out.append(doc)
            except Exception:
                continue
        out.sort(key=lambda d: str(d.get("createdAt", "")), reverse=True)
        return out[:40]

    def get(self, layout_id: str) -> dict[str, Any] | None:
        if not LAYOUT_ID_RE.fullmatch(layout_id):
            return None
        p = self.root / f"{layout_id}.json"
        if not p.is_file():
            return None
        try:
            return json.loads(p.read_text(encoding="utf-8"))
        except Exception:
            return None
