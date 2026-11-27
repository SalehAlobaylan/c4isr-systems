"""Operational Civilian Airspace & Maritime Traffic Snapshot Loader (EXERCISE_INPUT).

Freezes either live EMAD Go API external tracks (``GET /api/v1/external-tracks``
from OpenSky ADS-B / AIS) or deterministic biome-appropriate Saudi civil
aviation & maritime corridors into ``EXERCISE_INPUT`` civilian tracks with
protected safety volumes (1,500 m horiz x 450 m vert for airliners).

Enables realistic airspace deconfliction (hold-fire / safe-window avoidance) in
both the 3D Intercept Workbench (``sim``) and Multi-Threat Salvo Defense
(``salvo_defense``) without ever leaking live operational traffic into truth.
"""
from __future__ import annotations

from dataclasses import asdict, dataclass
import hashlib
import json
import math
from pathlib import Path
from typing import Any
import urllib.request

import numpy as np

from .environment import EnvironmentModel

AIRLINER_SAFETY_RADIUS_M = 1500.0
AIRLINER_SAFETY_HALF_HEIGHT_M = 450.0
VESSEL_SAFETY_RADIUS_M = 450.0
VESSEL_SAFETY_HALF_HEIGHT_M = 120.0


@dataclass(frozen=True)
class CivilianTrack:
    id: str
    callsign: str
    kind: str  # civilian_airliner | commercial_vessel
    source: str  # frozen-external-tracks | synthetic/saudi-civil-corridor
    start_enu: list[float]  # [east, north, up]
    vel_enu: list[float]  # [ve, vn, vu]
    safety_radius_m: float = AIRLINER_SAFETY_RADIUS_M
    safety_half_height_m: float = AIRLINER_SAFETY_HALF_HEIGHT_M
    squawk: str = "2401"
    iff_cooperative: bool = True

    def position_at(self, t_s: float) -> np.ndarray:
        s = np.asarray(self.start_enu, dtype=np.float64)
        v = np.asarray(self.vel_enu, dtype=np.float64)
        return s + float(t_s) * v

    def sample_trajectory(self, duration_s: float, dt_s: float = 1.0) -> list[dict[str, Any]]:
        n = max(2, int(math.ceil(duration_s / max(dt_s, 0.2))) + 1)
        out = []
        for i in range(n):
            t = min(duration_s, i * dt_s)
            p = self.position_at(t)
            out.append({"tS": round(t, 2), "enu": [round(float(v), 1) for v in p]})
        return out

    def to_dict(self) -> dict[str, Any]:
        return asdict(self)

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "CivilianTrack":
        return cls(
            id=str(data["id"]),
            callsign=str(data.get("callsign") or data["id"]),
            kind=str(data.get("kind", "civilian_airliner")),
            source=str(data.get("source", "synthetic/saudi-civil-corridor")),
            start_enu=[float(v) for v in data["start_enu"]],
            vel_enu=[float(v) for v in data["vel_enu"]],
            safety_radius_m=float(data.get("safety_radius_m", AIRLINER_SAFETY_RADIUS_M)),
            safety_half_height_m=float(data.get("safety_half_height_m", AIRLINER_SAFETY_HALF_HEIGHT_M)),
            squawk=str(data.get("squawk", "2401")),
            iff_cooperative=bool(data.get("iff_cooperative", True)),
        )


def _try_fetch_live_external_tracks(
    env: EnvironmentModel,
    *,
    api_origin: str = "http://127.0.0.1:8080",
    token_path: Path | None = None,
    timeout_s: float = 0.8,
) -> list[CivilianTrack]:
    """Optional best-effort read of EMAD Go API /api/v1/external-tracks into frozen ENU."""
    if not token_path or not token_path.is_file():
        return []
    try:
        token = token_path.read_text(encoding="utf-8").strip()
        req = urllib.request.Request(
            f"{api_origin.rstrip('/')}/api/v1/external-tracks",
            headers={"Authorization": f"Bearer {token}", "Accept": "application/json"},
        )
        with urllib.request.urlopen(req, timeout=timeout_s) as resp:
            if resp.status != 200:
                return []
            payload = json.loads(resp.read().decode("utf-8"))
    except Exception:
        return []

    w, s, e, n = env.bounds_wgs84
    pad = 0.12
    items = payload.get("tracks") or payload.get("items") or (payload if isinstance(payload, list) else [])
    tracks: list[CivilianTrack] = []
    for idx, item in enumerate(items[:40]):
        if not isinstance(item, dict):
            continue
        rep = item.get("reported") or item
        lon = rep.get("longitude") or rep.get("lon")
        lat = rep.get("latitude") or rep.get("lat")
        if lon is None or lat is None:
            continue
        lon_f, lat_f = float(lon), float(lat)
        if not (w - pad <= lon_f <= e + pad and s - pad <= lat_f <= n + pad):
            continue
        alt_m = float(rep.get("altitudeM") or rep.get("altitude_m") or 1600.0)
        spd_mps = float(rep.get("speedMps") or rep.get("groundSpeedMps") or 170.0)
        hdg_deg = float(rep.get("headingDeg") or rep.get("courseDeg") or 90.0)
        hdg_rad = math.radians(hdg_deg)
        domain = str(item.get("domain") or rep.get("domain") or "air").lower()
        is_vessel = domain in ("maritime", "sea", "surface", "vessel")
        pe, pn, pu = env.geodetic_to_enu(lon_f, lat_f, alt_m if not is_vessel else env.up_median)
        tracks.append(
            CivilianTrack(
                id=str(item.get("id") or f"ext-{idx + 1}"),
                callsign=str(item.get("callsign") or item.get("name") or f"CIV{idx + 1}"),
                kind="commercial_vessel" if is_vessel else "civilian_airliner",
                source="frozen-external-tracks",
                start_enu=[round(pe, 1), round(pn, 1), round(pu, 1)],
                vel_enu=[round(math.sin(hdg_rad) * spd_mps, 2), round(math.cos(hdg_rad) * spd_mps, 2), 0.0],
                safety_radius_m=VESSEL_SAFETY_RADIUS_M if is_vessel else AIRLINER_SAFETY_RADIUS_M,
                safety_half_height_m=VESSEL_SAFETY_HALF_HEIGHT_M if is_vessel else AIRLINER_SAFETY_HALF_HEIGHT_M,
                squawk=str(rep.get("squawk") or "2401"),
            )
        )
        if len(tracks) >= 4:
            break
    return tracks


def synthesize_civilian_traffic(
    env: EnvironmentModel,
    *,
    duration_s: float = 120.0,
    seed: int = 20260417,
    n_airliners: int = 2,
) -> list[CivilianTrack]:
    """Deterministic biome-scaled civilian airliner (& coastal vessel) tracks frozen as EXERCISE_INPUT."""
    span_e = env.east_max - env.east_min
    span_n = env.north_max - env.north_min
    cx = 0.5 * (env.east_min + env.east_max)
    cy = 0.5 * (env.north_min + env.north_max)
    g_med = float(env.up_median)

    # Deterministic seed mixing manifest hash + scenario seed
    h_int = int(hashlib.sha256(f"{env.manifest_sha256}:{seed}".encode("utf-8")).hexdigest()[:8], 16)
    rng = np.random.default_rng(h_int)

    callsigns = ["SVA142", "NAS418", "FYN206", "SVA891"]
    tracks: list[CivilianTrack] = []

    for i in range(max(0, min(4, n_airliners))):
        if i == 0:
            # Commercial approach corridor crossing N->S or W->E at moderate altitude (~950-1250 m AGL)
            start_e = env.east_min - 0.10 * span_e
            start_n = cy + (0.18 + float(rng.uniform(-0.08, 0.08))) * span_n
            end_e = env.east_max + 0.10 * span_e
            end_n = cy - (0.12 + float(rng.uniform(-0.08, 0.08))) * span_n
            agl_m = float(rng.uniform(850.0, 1150.0))
            spd = min(195.0, max(75.0, math.hypot(end_e - start_e, end_n - start_n) / max(duration_s * 0.78, 35.0)))
        else:
            # Higher overflight corridor crossing S->N (~1450-1950 m AGL)
            start_e = cx + (0.24 if i % 2 else -0.24) * span_e
            start_n = env.north_min - 0.12 * span_n
            end_e = cx - (0.18 if i % 2 else -0.18) * span_e
            end_n = env.north_max + 0.12 * span_n
            agl_m = float(rng.uniform(1350.0, 1850.0))
            spd = min(220.0, max(90.0, math.hypot(end_e - start_e, end_n - start_n) / max(duration_s * 0.82, 35.0)))

        dx, dy = end_e - start_e, end_n - start_n
        d_len = max(math.hypot(dx, dy), 1.0)
        ve, vn = spd * dx / d_len, spd * dy / d_len
        tracks.append(
            CivilianTrack(
                id=f"civ-air-{i + 1}",
                callsign=callsigns[i % len(callsigns)],
                kind="civilian_airliner",
                source="synthetic/saudi-civil-corridor",
                start_enu=[round(start_e, 1), round(start_n, 1), round(g_med + agl_m, 1)],
                vel_enu=[round(ve, 2), round(vn, 2), 0.0],
                safety_radius_m=AIRLINER_SAFETY_RADIUS_M,
                safety_half_height_m=AIRLINER_SAFETY_HALF_HEIGHT_M,
                squawk=f"24{10 + i}",
            )
        )

    # Add commercial maritime vessel on coastal biomes (Jubail / Jeddah)
    if getattr(env, "_gebco_data", None) is not None or env.up_min < 15.0:
        vx, vy = 4.5, -5.8
        tracks.append(
            CivilianTrack(
                id="civ-ship-1",
                callsign="MV-ARABIAN-STAR",
                kind="commercial_vessel",
                source="synthetic/saudi-civil-corridor",
                start_enu=[round(env.east_max - 0.14 * span_e, 1), round(env.north_max - 0.18 * span_n, 1), round(max(0.0, env.up_min + 2.0), 1)],
                vel_enu=[vx, vy, 0.0],
                safety_radius_m=VESSEL_SAFETY_RADIUS_M,
                safety_half_height_m=VESSEL_SAFETY_HALF_HEIGHT_M,
                squawk="AIS-403110",
            )
        )

    return tracks


def load_frozen_traffic(
    env: EnvironmentModel,
    *,
    duration_s: float = 120.0,
    seed: int = 20260417,
    try_live_api: bool = False,
    repo_root: Path | None = None,
    n_airliners: int = 2,
) -> list[CivilianTrack]:
    """Load live external tracks if explicitly requested and reachable, else deterministic fallback."""
    if try_live_api:
        root = repo_root or Path(__file__).resolve().parents[1]
        token_path = root / "private/local/credentials/local-map-analyst.token"
        live = _try_fetch_live_external_tracks(env, token_path=token_path)
        if live:
            return live
    return synthesize_civilian_traffic(env, duration_s=duration_s, seed=seed, n_airliners=n_airliners)


def check_airspace_deconfliction(
    tracks: list[CivilianTrack],
    point_enu: np.ndarray | list[float],
    t_s: float,
    *,
    extra_buffer_m: float = 0.0,
) -> dict[str, Any]:
    """Evaluate 3D cylindrical safety-volume clearance around an intercept/engagement point."""
    airliners = [tr for tr in tracks if tr.kind == "civilian_airliner"]
    if not airliners:
        return {
            "safe": True,
            "warningLevel": "CLEAR",
            "minHorizontalSeparationM": None,
            "minVerticalSeparationM": None,
            "closestCallsign": None,
            "violations": [],
        }

    p = np.asarray(point_enu, dtype=np.float64)
    violations: list[dict[str, Any]] = []
    best_dist = float("inf")
    best_horiz = float("inf")
    best_vert = float("inf")
    closest_cs: str | None = None

    for tr in airliners:
        cp = tr.position_at(t_s)
        dh = math.hypot(float(p[0] - cp[0]), float(p[1] - cp[1]))
        dv = abs(float(p[2] - cp[2]))
        d3 = math.hypot(dh, dv)
        if d3 < best_dist:
            best_dist = d3
            best_horiz = dh
            best_vert = dv
            closest_cs = tr.callsign

        rad = tr.safety_radius_m + extra_buffer_m
        hh = tr.safety_half_height_m + 0.2 * extra_buffer_m
        if dh <= rad and dv <= hh:
            violations.append(
                {
                    "trackId": tr.id,
                    "callsign": tr.callsign,
                    "tS": round(float(t_s), 2),
                    "horizontalSepM": round(dh, 1),
                    "verticalSepM": round(dv, 1),
                    "safetyRadiusM": rad,
                    "safetyHalfHeightM": hh,
                }
            )

    if violations:
        level = "HOLD_FIRE"
    elif best_horiz <= AIRLINER_SAFETY_RADIUS_M * 1.45 and best_vert <= AIRLINER_SAFETY_HALF_HEIGHT_M * 1.5:
        level = "ADVISORY"
    else:
        level = "CLEAR"

    return {
        "safe": len(violations) == 0,
        "warningLevel": level,
        "minHorizontalSeparationM": round(best_horiz, 1) if math.isfinite(best_horiz) else None,
        "minVerticalSeparationM": round(best_vert, 1) if math.isfinite(best_vert) else None,
        "closestCallsign": closest_cs,
        "violations": violations,
    }
