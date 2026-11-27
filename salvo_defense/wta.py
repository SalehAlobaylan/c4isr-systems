"""Weapon-Target Assignment (WTA) Policies for Multi-Threat Layered Air Defense.

STRICT PARTITION ISOLATION:
  Every policy consumes ONLY ``EXERCISE_OBSERVATION`` fused tracks (``FusedTrack``),
  public battery status (``BatteryRuntime``), defended asset positions, frozen
  ``EXERCISE_INPUT`` civilian airspace tracks, and environment/atmosphere models.
  No policy EVER accesses ``EXERCISE_TRUTH``.

Three pluggable policies:
  1. ``greedy_earliest``     — Baseline: fires the first ready battery in range at any
                               non-civilian track as soon as hits >= 2. Wastes Tier-1 SAMs
                               on cheap UAVs/decoys and ignores wind-drifted debris.
  2. ``combinatorial_wta``   — Layered optimizer: Shoot-Look-Shoot discipline, Bayesian
                               decoy filtering, civilian airspace HOLD_FIRE, wind/WorldPop
                               debris-risk scoring (or Phase 6 ML surrogate), and optimal
                               tier-matched bipartite assignment maximizing expected
                               asset-value saved per interceptor dollar.
  3. ``ai_policy``           — Observation-only learned WTA policy (weights loaded from
                               ``models/ai_wta_v1.npz`` trained via Phase 6 RL env).
"""
from __future__ import annotations

from dataclasses import dataclass, field
import math
from pathlib import Path
from typing import Any

import numpy as np

from sim_core.atmosphere import AtmosphereModel
from sim_core.environment import EnvironmentModel
from sim_core.traffic import CivilianTrack, check_airspace_deconfliction

from .contracts import BATTERY_TIER_SPECS, SalvoAssetSpec, SalvoBatterySpec
from .fusion import FusedTrack

REPO_ROOT = Path(__file__).resolve().parents[1]
DEFAULT_AI_WTA_PATH = REPO_ROOT / "models" / "ai_wta_v1.npz"

# Expected lethal threat damage value per inferred class (used strictly from OBSERVED class probs!)
CLASS_THREAT_VALUE = {
    "civilian_airliner": 0.0,
    "decoy": 0.0,
    "one_way_uav": 40.0,
    "cruise_missile": 82.0,
    "ballistic_missile": 115.0,
}

# Feature dimensionality shared between wta.py and sim_bench/rl_env.py
WTA_FEATURE_DIM = 14


@dataclass
class BatteryRuntime:
    spec: SalvoBatterySpec
    pos_enu: np.ndarray
    magazine_remaining: int
    magazine_initial: int
    next_ready_s: float = 0.0
    active_interceptors: int = 0
    shots_fired: int = 0
    kills: int = 0
    resolved: dict[str, Any] = field(default_factory=dict)

    @property
    def id(self) -> str:
        return self.spec.id

    @property
    def tier(self) -> str:
        return self.spec.tier

    def is_ready(self, t_s: float) -> bool:
        return (
            self.magazine_remaining > 0
            and t_s >= self.next_ready_s - 1e-6
            and self.active_interceptors < int(self.resolved["maxSimultaneous"])
        )


@dataclass(frozen=True)
class FireOrder:
    t_s: float
    battery_id: str
    battery_tier: str
    track_id: str
    predicted_intercept_s: float
    predicted_intercept_enu: np.ndarray
    predicted_pk: float
    cost_k_usd: float
    reason: str
    debris_risk: float = 0.0
    airspace_warning: str = "CLEAR"


def predict_intercept(
    trk: FusedTrack,
    batt: BatteryRuntime,
    t_s: float,
) -> tuple[float, np.ndarray, float] | None:
    """Predict lead-pursuit intercept time, 3D ENU point, and slant range from OBSERVED track."""
    p0 = trk.predict(t_s)
    v = trk.vel
    spd = float(batt.resolved["speedMps"])
    min_r = float(batt.resolved["minRangeM"])
    max_r = float(batt.resolved["maxRangeM"])
    max_alt = float(batt.resolved["maxAltitudeM"])

    t_go = float(np.linalg.norm(p0 - batt.pos_enu)) / max(spd, 1.0)
    for _ in range(5):
        p_int = p0 + v * t_go
        dist = float(np.linalg.norm(p_int - batt.pos_enu))
        t_go = dist / max(spd * 0.94, 1.0)
        if t_go > 38.0:
            return None
    p_int = p0 + v * t_go
    dist = float(np.linalg.norm(p_int - batt.pos_enu))
    if dist < min_r or dist > max_r:
        return None
    if float(p_int[2]) - float(batt.pos_enu[2]) > max_alt:
        return None
    return t_s + t_go, p_int, dist


def estimate_tti_s(trk: FusedTrack, assets: list[SalvoAssetSpec], t_s: float) -> tuple[float, float]:
    """Return (min_time_to_impact_s, min_distance_to_asset_m) using strictly OBSERVED track state."""
    p = trk.predict(t_s)
    v = trk.vel
    spd = max(15.0, float(np.linalg.norm(v[:2])))
    best_tti = 999.0
    best_dist = 99999.0
    for a in assets:
        dx = a.east - float(p[0])
        dy = a.north - float(p[1])
        d = math.hypot(dx, dy)
        if d < best_dist:
            best_dist = d
        # Closing velocity along line to asset
        closing = (dx * float(v[0]) + dy * float(v[1])) / max(d, 1.0)
        eff_close = max(closing, spd * 0.35)
        tti = max(0.0, (d - a.radius_m) / max(eff_close, 10.0))
        if tti < best_tti:
            best_tti = tti
    return best_tti, best_dist


def estimate_pk(batt: BatteryRuntime, trk: FusedTrack, intercept_dist_m: float, p_int: np.ndarray, env: EnvironmentModel) -> float:
    """Modeled single-shot probability of kill (synthetic/derived) from tier, range, and target speed."""
    base = float(batt.resolved["pkBase"])
    max_r = max(1.0, float(batt.resolved["maxRangeM"]))
    range_frac = min(1.0, intercept_dist_m / max_r)
    # Slight degradation at extreme range edge and for very fast ballistic targets against short-range tiers
    range_pen = 0.16 * (range_frac ** 2)
    spd_pen = 0.0
    if trk.speed_mps > 260.0 and batt.tier == "tier3_point_defense":
        spd_pen = 0.42
    elif trk.speed_mps > 280.0 and batt.tier == "tier2_shorad":
        spd_pen = 0.18
    g = env.height_at(float(p_int[0]), float(p_int[1]), fallback=env.up_median)
    agl = max(0.0, float(p_int[2]) - g)
    low_pen = 0.12 if (batt.tier == "tier1_sam" and agl < 70.0) else 0.0
    return float(np.clip(base - range_pen - spd_pen - low_pen, 0.12, 0.96))


def estimate_debris_risk(
    p_int: np.ndarray,
    vel_enu: np.ndarray,
    env: EnvironmentModel,
    atm: AtmosphereModel,
    surrogate: Any = None,
) -> float:
    """Fast normalized urban/WorldPop debris risk in [0, 1] for candidate intercept point.

    Uses Phase 6 pure-NumPy ML debris surrogate (< 0.2 ms) when provided, else
    computes an analytical wind-drift footprint centroid + WorldPop/building probe.
    """
    g = env.height_at(float(p_int[0]), float(p_int[1]), fallback=env.up_median)
    agl = max(20.0, float(p_int[2]) - g)
    we, wn, _, _, _ = atm.sample(
        np.array([float(p_int[0])]),
        np.array([float(p_int[1])]),
        np.array([float(p_int[2])]),
        ground_up_m=np.array([g]),
    )
    wind_e, wind_n = float(we[0]), float(wn[0])

    if surrogate is not None:
        try:
            pred = surrogate.predict_exposure(
                intercept_enu=p_int,
                threat_vel_enu=vel_enu,
                wind_en=(wind_e, wind_n),
                agl_m=agl,
                env=env,
            )
            return float(np.clip(pred.get("riskScore", 0.0), 0.0, 1.0))
        except Exception:
            pass

    # Analytical ballistic-drag drift estimate (fall time ~ sqrt(2*agl/g) * 1.45)
    fall_s = math.sqrt(2.0 * agl / 9.81) * 1.45
    drift_e = float(p_int[0]) + 0.35 * float(vel_enu[0]) * min(fall_s, 12.0) + 0.65 * wind_e * fall_s
    drift_n = float(p_int[1]) + 0.35 * float(vel_enu[1]) * min(fall_s, 12.0) + 0.65 * wind_n * fall_s
    # Sample 5-point cross around predicted debris centroid
    spread = min(450.0, 120.0 + 0.25 * agl)
    pts_e = np.array([drift_e, drift_e + spread, drift_e - spread, drift_e, drift_e], dtype=np.float64)
    pts_n = np.array([drift_n, drift_n, drift_n, drift_n + spread, drift_n - spread], dtype=np.float64)
    exp = env.classify_impacts(pts_e, pts_n)
    urb = float(exp["summary"]["urbanExposureFraction"])
    pop = float(exp["summary"].get("estimatedPopulationExposed") or 0.0)
    return float(np.clip(0.65 * urb + 0.35 * min(1.0, pop / 40.0), 0.0, 1.0))


def compute_pair_features(
    batt: BatteryRuntime,
    trk: FusedTrack,
    t_int: float,
    p_int: np.ndarray,
    dist_m: float,
    tti_s: float,
    pk: float,
    debris_risk: float,
    air_warn: str,
    engaged_count: int,
    t_s: float,
) -> np.ndarray:
    """Build 14-dim normalized observation-only feature vector for (battery, track) scoring."""
    p_civ = float(trk.class_probs.get("civilian_airliner", 0.0))
    p_dec = float(trk.class_probs.get("decoy", 0.0))
    p_uav = float(trk.class_probs.get("one_way_uav", 0.0))
    p_cru = float(trk.class_probs.get("cruise_missile", 0.0))
    p_bal = float(trk.class_probs.get("ballistic_missile", 0.0))

    exp_val = (
        p_uav * CLASS_THREAT_VALUE["one_way_uav"]
        + p_cru * CLASS_THREAT_VALUE["cruise_missile"]
        + p_bal * CLASS_THREAT_VALUE["ballistic_missile"]
    ) / 115.0  # [0..1]

    urgency = 1.0 / max(1.0, tti_s / 14.0)  # spikes as TTI -> 0
    urgency = min(2.5, urgency)
    cost_norm = math.log10(max(10.0, float(batt.resolved["costKUsd"]))) / 3.2  # ~0.39 (T3), 0.70 (T2), 0.96 (T1)
    mag_frac = batt.magazine_remaining / max(1, batt.magazine_initial)
    tier_mismatch = 1.0 if (batt.tier == "tier1_sam" and (p_uav + p_dec) > 0.65) else 0.0
    wait_for_id = 1.0 if (trk.hits < 4 and tti_s > 22.0 and p_bal < 0.35) else 0.0
    air_pen = 1.0 if air_warn == "HOLD_FIRE" else (0.25 if air_warn == "ADVISORY" else 0.0)

    return np.array(
        [
            exp_val * pk,  # 0: expected value neutralized
            urgency * exp_val,  # 1: time-critical lethal threat
            p_bal,  # 2: ballistic probability
            p_cru,  # 3: cruise probability
            p_dec,  # 4: decoy probability
            p_civ,  # 5: civilian probability
            cost_norm,  # 6: normalized interceptor dollar cost
            tier_mismatch,  # 7: wasting Tier-1 SAM on cheap UAV/decoy
            float(min(engaged_count, 3)),  # 8: already-in-flight interceptors on this track
            debris_risk,  # 9: wind-drifted urban/WorldPop debris risk
            air_pen,  # 10: civilian airspace conflict penalty
            mag_frac,  # 11: battery magazine fraction remaining
            wait_for_id,  # 12: safe margin to wait for EO/IR decoy discrimination
            1.0 - min(1.0, dist_m / max(1.0, float(batt.resolved["maxRangeM"]))),  # 13: range margin
        ],
        dtype=np.float64,
    )


# Default calibrated linear weights for combinatorial_wta (and fallback for ai_policy before RL training)
COMBINATORIAL_WEIGHTS = np.array(
    [
        4.20,   # 0: exp_val * pk
        2.95,   # 1: urgency * exp_val
        1.45,   # 2: p_bal
        0.95,   # 3: p_cru
        -3.80,  # 4: p_dec (strong decoy suppression)
        -18.0,  # 5: p_civ (never shoot civilians)
        -1.65,  # 6: cost_norm (prefer cheaper tier when Pk is adequate)
        -2.40,  # 7: tier_mismatch (preserve Tier-1 SAMs for ballistic/cruise)
        -4.50,  # 8: shoot-look-shoot discipline (avoid double-shooting)
        -1.35,  # 9: wind/WorldPop debris risk
        -22.0,  # 10: civilian airspace HOLD_FIRE
        0.55,   # 11: magazine balance
        -1.15,  # 12: wait briefly for EO/IR ID if plenty of TTI margin
        0.35,   # 13: range margin
    ],
    dtype=np.float64,
)

_CACHED_AI_MODEL: dict[str, Any] | None = None


def load_ai_wta_model(path: Path | None = None) -> dict[str, Any] | None:
    """Load pure-NumPy trained AI WTA policy weights from ``models/ai_wta_v1.npz`` if available."""
    global _CACHED_AI_MODEL
    target = path or DEFAULT_AI_WTA_PATH
    if not target.is_file():
        return None
    if _CACHED_AI_MODEL is not None and _CACHED_AI_MODEL.get("_path") == str(target):
        return _CACHED_AI_MODEL
    data = np.load(target, allow_pickle=False)
    model = {
        "_path": str(target),
        "weights": np.asarray(data["weights"], dtype=np.float64),
        "bias": float(data["bias"]) if "bias" in data else 0.0,
        "w2": np.asarray(data["w2"], dtype=np.float64) if "w2" in data else None,
        "b2": np.asarray(data["b2"], dtype=np.float64) if "b2" in data else None,
        "threshold": float(data["threshold"]) if "threshold" in data else 0.55,
    }
    _CACHED_AI_MODEL = model
    return model


def _score_ai_model(feats: np.ndarray, model: dict[str, Any]) -> float:
    w = model["weights"]
    if w.ndim == 1:
        return float(np.dot(feats, w) + model.get("bias", 0.0))
    # 1-hidden-layer tanh MLP policy
    h = np.tanh(feats @ w + model.get("bias", 0.0))
    return float(np.dot(h, model["w2"]) + float(model["b2"]))


def _solve_max_weight_matching(score_matrix: np.ndarray, threshold: float) -> list[tuple[int, int]]:
    """Exact small-scale optimal bipartite assignment (recursive branch-and-bound for <=8 batteries, greedy fallback)."""
    n_b, n_t = score_matrix.shape
    if n_b == 0 or n_t == 0:
        return []

    if n_b <= 7 and n_t <= 14:
        best_val = 0.0
        best_pairs: list[tuple[int, int]] = []
        used_t = [False] * n_t
        # Upper bound per battery for fast pruning
        row_max = np.maximum(0.0, np.max(score_matrix - threshold, axis=1))
        suffix_ub = np.zeros(n_b + 1, dtype=np.float64)
        for i in range(n_b - 1, -1, -1):
            suffix_ub[i] = suffix_ub[i + 1] + row_max[i]

        cur_pairs: list[tuple[int, int]] = []

        def dfs(bi: int, acc: float) -> None:
            nonlocal best_val, best_pairs
            if acc + suffix_ub[bi] <= best_val + 1e-9:
                return
            if bi == n_b:
                if acc > best_val:
                    best_val = acc
                    best_pairs = list(cur_pairs)
                return
            # Option A: assign battery bi to a feasible track above threshold (ordered best-first)
            cands = [
                (float(score_matrix[bi, ti]) - threshold, ti)
                for ti in range(n_t)
                if not used_t[ti] and score_matrix[bi, ti] > threshold
            ]
            cands.sort(reverse=True)
            for gain, ti in cands[:4]:
                used_t[ti] = True
                cur_pairs.append((bi, ti))
                dfs(bi + 1, acc + gain)
                cur_pairs.pop()
                used_t[ti] = False
            # Option B: hold battery bi in reserve
            dfs(bi + 1, acc)

        dfs(0, 0.0)
        return best_pairs

    # Fast greedy matching fallback for very large battery counts
    triples = [
        (float(score_matrix[bi, ti]), bi, ti)
        for bi in range(n_b)
        for ti in range(n_t)
        if score_matrix[bi, ti] > threshold
    ]
    triples.sort(reverse=True)
    used_b: set[int] = set()
    used_t_s: set[int] = set()
    out: list[tuple[int, int]] = []
    for _, bi, ti in triples:
        if bi not in used_b and ti not in used_t_s:
            used_b.add(bi)
            used_t_s.add(ti)
            out.append((bi, ti))
    return out


def choose_assignments(
    *,
    t_s: float,
    tracks: list[FusedTrack],
    batteries: list[BatteryRuntime],
    assets: list[SalvoAssetSpec],
    civilian_tracks: list[CivilianTrack],
    engaged_track_counts: dict[str, int],
    env: EnvironmentModel,
    atm: AtmosphereModel,
    policy: str = "combinatorial_wta",
    surrogate: Any = None,
    ai_weights: np.ndarray | None = None,
) -> list[FireOrder]:
    """Select observation-only fire orders for the current simulation tick."""
    ready_batts = [b for b in batteries if b.is_ready(t_s)]
    if not ready_batts or not tracks:
        return []

    orders: list[FireOrder] = []

    if policy == "greedy_earliest":
        # Naive baseline: fires first ready battery (often Tier-1 SAM because it has the longest range!)
        # on the first non-civilian track with >=2 hits, ignoring decoy uncertainty, cost tier, and debris!
        assigned_tracks: set[str] = set()
        # Sort batteries by max range descending (longest-range Tier-1 SAM sees target first!)
        for batt in sorted(ready_batts, key=lambda b: float(b.resolved["maxRangeM"]), reverse=True):
            for trk in sorted(tracks, key=lambda tr: tr.first_seen_s):
                if trk.hits < 2 or trk.track_id in assigned_tracks:
                    continue
                if trk.iff_cooperative or trk.class_probs.get("civilian_airliner", 0.0) > 0.70:
                    continue
                if engaged_track_counts.get(trk.track_id, 0) >= 2:
                    continue
                pred = predict_intercept(trk, batt, t_s)
                if pred is None:
                    continue
                t_int, p_int, dist_m = pred
                air = check_airspace_deconfliction(civilian_tracks, p_int, t_int)
                pk = estimate_pk(batt, trk, dist_m, p_int, env)
                orders.append(
                    FireOrder(
                        t_s=t_s,
                        battery_id=batt.id,
                        battery_tier=batt.tier,
                        track_id=trk.track_id,
                        predicted_intercept_s=t_int,
                        predicted_intercept_enu=p_int,
                        predicted_pk=pk,
                        cost_k_usd=float(batt.resolved["costKUsd"]),
                        reason="greedy-earliest-in-range",
                        debris_risk=0.0,
                        airspace_warning=air["warningLevel"],
                    )
                )
                assigned_tracks.add(trk.track_id)
                break
        return orders

    # Combinatorial WTA or AI RL Policy — build exact score matrix over ready (battery, track) pairs
    cand_tracks = [
        tr for tr in tracks
        if tr.hits >= 2 and not tr.iff_cooperative and tr.class_probs.get("civilian_airliner", 0.0) < 0.35
    ]
    if not cand_tracks:
        return []

    ai_model = None
    if policy == "ai_policy":
        if ai_weights is not None:
            ai_model = {"weights": ai_weights, "bias": 0.0, "threshold": 0.55}
        else:
            ai_model = load_ai_wta_model()

    n_b, n_t = len(ready_batts), len(cand_tracks)
    scores = np.full((n_b, n_t), -999.0, dtype=np.float64)
    meta: dict[tuple[int, int], tuple[float, np.ndarray, float, float, str]] = {}

    for ti, trk in enumerate(cand_tracks):
        tti_s, _ = estimate_tti_s(trk, assets, t_s)
        eng_cnt = engaged_track_counts.get(trk.track_id, 0)
        # Strict decoy rejection when EO/IR + radar confirm decoy
        if trk.class_probs.get("decoy", 0.0) > 0.58 and tti_s > 8.0:
            continue
        for bi, batt in enumerate(ready_batts):
            pred = predict_intercept(trk, batt, t_s)
            if pred is None:
                continue
            t_int, p_int, dist_m = pred
            air = check_airspace_deconfliction(civilian_tracks, p_int, t_int)
            # Hard safety rule: NEVER fire into a civilian airliner's protected safety cylinder!
            if not air["safe"]:
                continue
            pk = estimate_pk(batt, trk, dist_m, p_int, env)
            d_risk = estimate_debris_risk(p_int, trk.vel, env, atm, surrogate=surrogate)
            feats = compute_pair_features(
                batt, trk, t_int, p_int, dist_m, tti_s, pk, d_risk, air["warningLevel"], eng_cnt, t_s
            )
            if ai_model is not None:
                sc = _score_ai_model(feats, ai_model)
            else:
                sc = float(np.dot(feats, COMBINATORIAL_WEIGHTS))
            scores[bi, ti] = sc
            meta[(bi, ti)] = (t_int, p_int, pk, d_risk, air["warningLevel"])

    thresh = float(ai_model.get("threshold", 0.55)) if ai_model is not None else 0.65
    matched = _solve_max_weight_matching(scores, threshold=thresh)
    for bi, ti in matched:
        batt = ready_batts[bi]
        trk = cand_tracks[ti]
        t_int, p_int, pk, d_risk, air_warn = meta[(bi, ti)]
        orders.append(
            FireOrder(
                t_s=t_s,
                battery_id=batt.id,
                battery_tier=batt.tier,
                track_id=trk.track_id,
                predicted_intercept_s=t_int,
                predicted_intercept_enu=p_int,
                predicted_pk=pk,
                cost_k_usd=float(batt.resolved["costKUsd"]),
                reason="ai-policy-wta" if (policy == "ai_policy" and ai_model is not None) else "combinatorial-layered-wta",
                debris_risk=d_risk,
                airspace_warning=air_warn,
            )
        )
    return orders
