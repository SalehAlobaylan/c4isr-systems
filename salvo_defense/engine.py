"""Closed-Loop Multi-Threat Salvo & Layered Air Defense Simulation Engine.

Steps through a mixed multi-wave raid at ``dt = 0.25 s`` over verified terrain:
  1. Propagates sealed ``EXERCISE_TRUTH`` raid trajectories + frozen civilian airliners
  2. Scans ``MultiSensorNetwork`` (LOS-masked) -> updates ``EXERCISE_OBSERVATION`` tracks
  3. Queries pluggable WTA policy (``greedy_earliest`` | ``combinatorial_wta`` | ``ai_policy``)
  4. Launches 3-DOF lead-pursuit interceptors from finite-magazine 3-tier batteries
  5. Resolves engagements (seeded Pk rolls -> real Shoot-Look-Shoot re-engagements!)
  6. Propagates wind-drifted shrapnel per kill & aggregates WorldPop + OSM road disruption
  7. Scores leakers, defense cost ($k), decoy waste, airspace safety, and composite score
"""
from __future__ import annotations

from dataclasses import dataclass, field
import datetime as dt
import hashlib
import json
import math
from pathlib import Path
import time
from typing import Any

import numpy as np

from sim.contracts import ShrapnelConfig
from sim.physics import simulate_shrapnel_cloud
from sim_core.atmosphere import AtmosphereModel
from sim_core.environment import EnvironmentModel
from sim_core.road_impact import get_road_analyzer
from sim_core.traffic import CivilianTrack, check_airspace_deconfliction, load_frozen_traffic

from .contracts import (
    DERIVATION_LABEL,
    SALVO_ENGINE_VERSION,
    SALVO_REPLAY_SCHEMA,
    SCOPE_EVALUATION,
    SCOPE_EXERCISE_INPUT,
    SCOPE_EXERCISE_OBSERVATION,
    SCOPE_EXERCISE_TRUTH,
    SalvoScenarioSpec,
)
from .fusion import MultiSensorNetwork, MultiTrackFusionEngine, SensorDetection
from .raid import ThreatTruth, build_raid_truth
from .wta import BatteryRuntime, choose_assignments, load_ai_wta_model


@dataclass
class _InterceptorFlight:
    id: str
    battery_id: str
    battery_tier: str
    track_id: str
    target_truth_id: str | None  # resolved strictly inside truth physics loop for proximity fuze
    launch_s: float
    pos: np.ndarray
    vel: np.ndarray
    speed_mps: float
    pk: float
    cost_k_usd: float
    aim_enu: np.ndarray
    active: bool = True
    outcome: str = "in_flight"  # in_flight | kill | miss | expired
    end_s: float | None = None
    samples: list[list[float]] = field(default_factory=list)


def _match_track_to_truth(
    track_pos: np.ndarray,
    active_threats: dict[str, ThreatTruth],
    t_s: float,
    max_dist_m: float = 650.0,
) -> str | None:
    """Resolve physical interceptor seeker lock onto the nearest airborne truth target (TRUTH layer only)."""
    best_id: str | None = None
    best_d = max_dist_m
    for tid, th in active_threats.items():
        pos, _, airborne = th.state_at(t_s)
        if not airborne:
            continue
        d = float(np.linalg.norm(pos - track_pos))
        if d < best_d:
            best_d = d
            best_id = tid
    return best_id


def run_salvo_policy(
    spec: SalvoScenarioSpec,
    *,
    env: EnvironmentModel,
    atm: AtmosphereModel,
    policy: str | None = None,
    surrogate: Any = None,
    ai_weights: np.ndarray | None = None,
    record_replay: bool = True,
) -> dict[str, Any]:
    """Execute one deterministic multi-threat salvo run under a single WTA policy."""
    t_wall0 = time.perf_counter()
    active_policy = policy or spec.policy
    rng_pk = np.random.default_rng(spec.seed + 77713)

    # 1. Sealed EXERCISE_TRUTH Raid + Frozen EXERCISE_INPUT Civilian Airspace
    raid_truth = build_raid_truth(spec.threats, spec.assets, env, dt_s=spec.dt_s, duration_s=spec.duration_s)
    truth_by_id = {th.id: th for th in raid_truth}
    alive_threats: dict[str, ThreatTruth] = dict(truth_by_id)
    threat_status: dict[str, dict[str, Any]] = {
        th.id: {"status": "pending", "endS": th.arrival_s, "killedBy": None} for th in raid_truth
    }
    civilian_tracks: list[CivilianTrack] = (
        load_frozen_traffic(env, duration_s=spec.duration_s, seed=spec.seed)
        if spec.enable_civilian_traffic
        else []
    )
    asset_by_id = {a.id: a for a in spec.assets}

    # 2. Multi-Sensor Observation Network & GNN/Kalman Fusion Engine
    prof = atm.profile_summary(0.0, 0.0)
    surf_wind = float(prof[0]["speedMps"]) if prof else 4.0
    sensor_net = MultiSensorNetwork(
        spec.sensors,
        env,
        visibility_km=spec.visibility_km,
        surface_wind_mps=surf_wind,
        seed=spec.seed,
    )
    fusion = MultiTrackFusionEngine(env)
    pending_dets: list[SensorDetection] = []

    # 3. Finite-Magazine Layered Batteries
    batteries: list[BatteryRuntime] = []
    batt_by_id: dict[str, BatteryRuntime] = {}
    for bspec in spec.batteries:
        res = bspec.resolved()
        g = env.height_at(bspec.east, bspec.north, fallback=env.up_median)
        br = BatteryRuntime(
            spec=bspec,
            pos_enu=np.array([bspec.east, bspec.north, g + bspec.height_agl_m], dtype=np.float64),
            magazine_remaining=int(bspec.magazine),
            magazine_initial=int(bspec.magazine),
            resolved=res,
        )
        batteries.append(br)
        batt_by_id[br.id] = br

    # 4. Closed-loop Discrete Simulation
    dt_s = float(spec.dt_s)
    n_steps = int(math.ceil(spec.duration_s / dt_s)) + 1
    interceptors: list[_InterceptorFlight] = []
    events: list[dict[str, Any]] = []
    track_frames: list[dict[str, Any]] = []
    magazine_timeline: list[dict[str, Any]] = []
    shrapnel_impacts_all: list[dict[str, Any]] = []
    shrapnel_e_all: list[float] = []
    shrapnel_n_all: list[float] = []
    shrapnel_hz_all: list[bool] = []

    total_los_clear = 0
    total_los_blocked = 0
    total_detections = 0
    decoy_shots = 0
    airspace_violations = 0
    total_building_hits = 0
    total_road_hits = 0
    total_pop_exposed = 0.0
    total_hazardous = 0
    intercept_count = 0
    leakers_lethal = 0
    damage_suffered = 0.0
    next_int_seq = 1

    for step in range(n_steps):
        t_s = round(step * dt_s, 3)

        # 4a. Check threat arrivals (leakers hitting assets)
        for tid in list(alive_threats.keys()):
            th = alive_threats[tid]
            if t_s < th.spawn_s:
                continue
            if threat_status[tid]["status"] == "pending":
                threat_status[tid]["status"] = "airborne"
            pos, _, airborne = th.state_at(t_s)
            tgt = asset_by_id.get(th.target_asset_id, spec.assets[0])
            dist_to_asset = math.hypot(float(pos[0]) - tgt.east, float(pos[1]) - tgt.north)
            if (not airborne) or (t_s >= th.arrival_s - 1e-6) or dist_to_asset <= max(60.0, tgt.radius_m * 0.25):
                del alive_threats[tid]
                if th.lethal:
                    threat_status[tid] = {"status": "leaker", "endS": t_s, "killedBy": None}
                    leakers_lethal += 1
                    damage_suffered += th.damage_value
                    events.append(
                        {
                            "tS": t_s,
                            "kind": "leaker_impact",
                            "threatId": tid,
                            "threatKind": th.kind,
                            "assetId": tgt.id,
                            "enu": [round(float(v), 1) for v in pos],
                        }
                    )
                else:
                    threat_status[tid] = {"status": "decoy_expired", "endS": t_s, "killedBy": None}

        # 4b. Scan multi-sensor network every 0.5 s (strictly opaque detections!)
        if step % max(1, int(round(0.5 / dt_s))) == 0:
            airborne_entities: list[dict[str, Any]] = []
            for th in alive_threats.values():
                pos, _, airborne = th.state_at(t_s)
                if airborne:
                    airborne_entities.append(
                        {"pos": pos, "rcs_m2": th.rcs_m2, "ir_signature": th.ir_signature, "iff_cooperative": False}
                    )
            for ct in civilian_tracks:
                if ct.kind == "civilian_airliner":
                    airborne_entities.append(
                        {
                            "pos": ct.position_at(t_s),
                            "rcs_m2": 18.0,
                            "ir_signature": 0.75,
                            "iff_cooperative": ct.iff_cooperative,
                            "squawk": ct.squawk,
                        }
                    )
            if airborne_entities:
                dets, lc, lb = sensor_net.scan_entities(t_s, airborne_entities)
                total_los_clear += lc
                total_los_blocked += lb
                total_detections += len(dets)
                pending_dets.extend(dets)

        # Deliver delayed EXERCISE_OBSERVATION detections to fusion engine
        ready_dets = [d for d in pending_dets if d.deliver_s <= t_s + 1e-6]
        if ready_dets:
            pending_dets = [d for d in pending_dets if d.deliver_s > t_s + 1e-6]
        tracks = fusion.ingest(t_s, ready_dets)

        # 4c. Propagate active interceptors & resolve engagements
        for flt in interceptors:
            if not flt.active:
                continue
            target_th = alive_threats.get(flt.target_truth_id) if flt.target_truth_id else None
            if target_th is not None:
                t_pos, t_vel, airborne = target_th.state_at(t_s)
                if not airborne:
                    target_th = None
            if target_th is not None:
                rel = t_pos - flt.pos
                dist = float(np.linalg.norm(rel))
                t_go = dist / max(flt.speed_mps, 1.0)
                lead_pt = t_pos + t_vel * min(t_go, 4.0)
                desired_dir = lead_pt - flt.pos
            else:
                desired_dir = flt.aim_enu - flt.pos
                dist = float(np.linalg.norm(desired_dir))

            d_norm = float(np.linalg.norm(desired_dir))
            if d_norm > 1e-3:
                flt.vel = (desired_dir / d_norm) * flt.speed_mps
            step_move = flt.speed_mps * dt_s
            flt.pos = flt.pos + flt.vel * dt_s
            if record_replay and (len(flt.samples) == 0 or step % 2 == 0):
                flt.samples.append([t_s, round(float(flt.pos[0]), 1), round(float(flt.pos[1]), 1), round(float(flt.pos[2]), 1)])

            # Check proximity fuze / intercept resolution
            hit_now = False
            if target_th is not None:
                t_pos_after, t_vel_after, _ = target_th.state_at(t_s)
                if float(np.linalg.norm(t_pos_after - flt.pos)) <= max(35.0, step_move * 0.95):
                    hit_now = True
                    flt.pos = t_pos_after.copy()
            elif dist <= max(40.0, step_move):
                flt.active = False
                flt.outcome = "expired"
                flt.end_s = t_s
                batt_by_id[flt.battery_id].active_interceptors = max(0, batt_by_id[flt.battery_id].active_interceptors - 1)
                if record_replay:
                    flt.samples.append([t_s, round(float(flt.pos[0]), 1), round(float(flt.pos[1]), 1), round(float(flt.pos[2]), 1)])
                continue

            if hit_now and target_th is not None:
                flt.active = False
                flt.end_s = t_s
                batt = batt_by_id[flt.battery_id]
                batt.active_interceptors = max(0, batt.active_interceptors - 1)
                if record_replay:
                    flt.samples.append([t_s, round(float(flt.pos[0]), 1), round(float(flt.pos[1]), 1), round(float(flt.pos[2]), 1)])

                # Check airspace safety at actual intercept instant
                air_chk = check_airspace_deconfliction(civilian_tracks, flt.pos, t_s)
                if not air_chk["safe"]:
                    airspace_violations += 1

                if float(rng_pk.random()) <= flt.pk:
                    flt.outcome = "kill"
                    batt.kills += 1
                    intercept_count += 1
                    del alive_threats[target_th.id]
                    threat_status[target_th.id] = {
                        "status": "intercepted",
                        "endS": t_s,
                        "killedBy": batt.id,
                        "interceptEnu": [round(float(v), 1) for v in flt.pos],
                    }
                    # Lightweight wind-drifted shrapnel cloud per kill
                    sh_cfg = ShrapnelConfig(fragment_count=target_th.fragment_count, dt_s=0.2, max_flight_s=95.0)
                    sh = simulate_shrapnel_cloud(
                        flt.pos,
                        t_vel_after,
                        sh_cfg,
                        env,
                        atm,
                        seed=spec.seed + 3000 + intercept_count * 97,
                        interceptor_vel=flt.vel,
                        record_trails=False,
                        protected_center=(spec.assets[0].east, spec.assets[0].north),
                        protected_radius_m=spec.assets[0].radius_m,
                    )
                    exp = sh["exposure"]
                    total_building_hits += int(exp["buildingHits"])
                    total_road_hits += int(exp["roadHits"])
                    total_pop_exposed += float(exp.get("estimatedPopulationExposed") or 0.0)
                    total_hazardous += int(exp.get("hazardousHits") or 0)
                    for f in sh["fragments"]:
                        shrapnel_e_all.append(float(f["impactEnu"][0]))
                        shrapnel_n_all.append(float(f["impactEnu"][1]))
                        shrapnel_hz_all.append(bool(f["hazardous"]))
                        if record_replay:
                            shrapnel_impacts_all.append(
                                {
                                    "tS": round(t_s + float(f["impactTimeAfterInterceptS"]), 2),
                                    "burstS": t_s,
                                    "enu": f["impactEnu"],
                                    "category": f["exposureCategory"],
                                    "hazardous": bool(f["hazardous"]),
                                }
                            )
                    events.append(
                        {
                            "tS": t_s,
                            "kind": "intercept_kill",
                            "batteryId": batt.id,
                            "batteryTier": batt.tier,
                            "threatId": target_th.id,
                            "threatKind": target_th.kind,
                            "enu": [round(float(v), 1) for v in flt.pos],
                            "buildingHits": int(exp["buildingHits"]),
                            "popExposed": round(float(exp.get("estimatedPopulationExposed") or 0.0), 1),
                            "airspaceWarning": air_chk["warningLevel"],
                        }
                    )
                else:
                    flt.outcome = "miss"
                    events.append(
                        {
                            "tS": t_s,
                            "kind": "intercept_miss",
                            "batteryId": batt.id,
                            "threatId": target_th.id,
                            "enu": [round(float(v), 1) for v in flt.pos],
                        }
                    )
            elif t_s - flt.launch_s > 40.0:
                flt.active = False
                flt.outcome = "expired"
                flt.end_s = t_s
                batt_by_id[flt.battery_id].active_interceptors = max(0, batt_by_id[flt.battery_id].active_interceptors - 1)

        # 4d. Query observation-only WTA policy every 0.5 s
        if step % max(1, int(round(0.5 / dt_s))) == 0 and tracks:
            eng_counts: dict[str, int] = {}
            for flt in interceptors:
                if flt.active:
                    eng_counts[flt.track_id] = eng_counts.get(flt.track_id, 0) + 1

            fire_orders = choose_assignments(
                t_s=t_s,
                tracks=tracks,
                batteries=batteries,
                assets=spec.assets,
                civilian_tracks=civilian_tracks,
                engaged_track_counts=eng_counts,
                env=env,
                atm=atm,
                policy=active_policy,
                surrogate=surrogate,
                ai_weights=ai_weights,
            )
            for fo in fire_orders:
                batt = batt_by_id[fo.battery_id]
                if not batt.is_ready(t_s):
                    continue
                trk = fusion.tracks.get(fo.track_id)
                if trk is None:
                    continue
                batt.magazine_remaining -= 1
                batt.shots_fired += 1
                batt.active_interceptors += 1
                batt.next_ready_s = t_s + float(batt.resolved["reloadS"])

                truth_lock_id = _match_track_to_truth(trk.predict(t_s), alive_threats, t_s)
                if truth_lock_id and not truth_by_id[truth_lock_id].lethal:
                    decoy_shots += 1

                init_dir = fo.predicted_intercept_enu - batt.pos_enu
                dn = max(float(np.linalg.norm(init_dir)), 1.0)
                spd = float(batt.resolved["speedMps"])
                flt = _InterceptorFlight(
                    id=f"int_{next_int_seq:03d}",
                    battery_id=batt.id,
                    battery_tier=batt.tier,
                    track_id=fo.track_id,
                    target_truth_id=truth_lock_id,
                    launch_s=t_s,
                    pos=batt.pos_enu.copy(),
                    vel=(init_dir / dn) * spd,
                    speed_mps=spd,
                    pk=fo.predicted_pk,
                    cost_k_usd=fo.cost_k_usd,
                    aim_enu=fo.predicted_intercept_enu.copy(),
                    samples=[[t_s, round(float(batt.pos_enu[0]), 1), round(float(batt.pos_enu[1]), 1), round(float(batt.pos_enu[2]), 1)]] if record_replay else [],
                )
                next_int_seq += 1
                interceptors.append(flt)
                events.append(
                    {
                        "tS": t_s,
                        "kind": "launch",
                        "interceptorId": flt.id,
                        "batteryId": batt.id,
                        "batteryTier": batt.tier,
                        "trackId": fo.track_id,
                        "costKUsd": fo.cost_k_usd,
                        "reason": fo.reason,
                    }
                )

        # Record compact observation & magazine timeline every 1.0 s for 3D HUD
        if record_replay and step % max(1, int(round(1.0 / dt_s))) == 0:
            track_frames.append({"tS": t_s, "tracks": [tr.snapshot() for tr in tracks]})
            magazine_timeline.append(
                {
                    "tS": t_s,
                    "magazines": {b.id: b.magazine_remaining for b in batteries},
                }
            )

        # Early exit once all threats resolved and all interceptors done
        if not alive_threats and all(not f.active for f in interceptors) and t_s > max((th.spawn_s for th in raid_truth), default=0.0) + 2.0:
            break

    # 5. Aggregate Road Disruption & Evaluation Scorecard
    road_impact = get_road_analyzer(env).evaluate_debris_impact(
        np.asarray(shrapnel_e_all, dtype=np.float64),
        np.asarray(shrapnel_n_all, dtype=np.float64),
        np.asarray(shrapnel_hz_all, dtype=bool),
        protected_center=(spec.assets[0].east, spec.assets[0].north),
    )

    total_lethal = sum(1 for th in raid_truth if th.lethal)
    total_decoys = sum(1 for th in raid_truth if not th.lethal)
    lethal_intercepted = sum(
        1 for th in raid_truth if th.lethal and threat_status[th.id]["status"] == "intercepted"
    )
    total_shots = sum(b.shots_fired for b in batteries)
    total_cost_k = round(sum(b.shots_fired * float(b.resolved["costKUsd"]) for b in batteries), 1)
    defense_success_rate = round(lethal_intercepted / max(total_lethal, 1), 4)

    # Composite defense score (higher is better, 0..100 scale): rewards killing all lethal threats,
    # conserving interceptor dollars, rejecting decoys, protecting airspace & minimizing civil debris.
    raw_score = (
        100.0 * defense_success_rate
        - 16.0 * leakers_lethal
        - 0.0025 * total_cost_k
        - 3.5 * decoy_shots
        - 25.0 * airspace_violations
        - 0.15 * min(total_building_hits, 80)
        - 0.003 * min(total_pop_exposed, 4000.0)
    )
    composite_score = round(max(0.0, min(100.0, raw_score)), 2)

    battery_summary = [
        {
            "id": b.id,
            "tier": b.tier,
            "label": b.resolved["label"],
            "color": b.resolved["color"],
            "enu": [round(float(v), 1) for v in b.pos_enu],
            "maxRangeM": b.resolved["maxRangeM"],
            "costKUsd": b.resolved["costKUsd"],
            "magazineInitial": b.magazine_initial,
            "magazineRemaining": b.magazine_remaining,
            "shotsFired": b.shots_fired,
            "kills": b.kills,
            "spendKUsd": round(b.shots_fired * float(b.resolved["costKUsd"]), 1),
        }
        for b in batteries
    ]

    evaluation = {
        "scope": SCOPE_EVALUATION,
        "policy": active_policy,
        "compositeScore": composite_score,
        "totalThreats": len(raid_truth),
        "lethalThreats": total_lethal,
        "decoyThreats": total_decoys,
        "lethalIntercepted": lethal_intercepted,
        "leakers": leakers_lethal,
        "defenseSuccessRate": defense_success_rate,
        "damageSuffered": round(damage_suffered, 1),
        "shotsFired": total_shots,
        "totalCostKUsd": total_cost_k,
        "decoyShotsWasted": decoy_shots,
        "airspaceViolations": airspace_violations,
        "debrisBuildingHits": total_building_hits,
        "debrisRoadHits": total_road_hits,
        "debrisHazardousHits": total_hazardous,
        "estimatedPopulationExposed": round(total_pop_exposed, 1),
        "roadImpact": road_impact,
        "sensing": {
            "detectionsTotal": total_detections,
            "losClearChecks": total_los_clear,
            "losBlockedChecks": total_los_blocked,
            "occlusionFraction": round(total_los_blocked / max(total_los_clear + total_los_blocked, 1), 4),
        },
        "batteries": battery_summary,
        "elapsedS": round(time.perf_counter() - t_wall0, 3),
    }

    if not record_replay:
        return {"policy": active_policy, "evaluation": evaluation}

    # Compact truth trajectories trimmed at kill/impact instant
    threats_replay = []
    for th in raid_truth:
        st = threat_status[th.id]
        end_s = float(st["endS"])
        pts = []
        for k in range(0, len(th.times_s), 2):
            tk = float(th.times_s[k])
            if tk > end_s + 1e-6:
                break
            pts.append([round(tk, 2), round(float(th.positions[k, 0]), 1), round(float(th.positions[k, 1]), 1), round(float(th.positions[k, 2]), 1)])
        ep, _, _ = th.state_at(end_s)
        pts.append([round(end_s, 2), round(float(ep[0]), 1), round(float(ep[1]), 1), round(float(ep[2]), 1)])
        threats_replay.append(
            {
                "id": th.id,
                "kind": th.kind,
                "label": th.label,
                "lethal": th.lethal,
                "wave": th.wave,
                "spawnS": th.spawn_s,
                "endS": round(end_s, 2),
                "status": st["status"],
                "killedBy": st.get("killedBy"),
                "samples": pts,
            }
        )

    interceptors_replay = [
        {
            "id": f.id,
            "batteryId": f.battery_id,
            "batteryTier": f.battery_tier,
            "trackId": f.track_id,
            "launchS": f.launch_s,
            "endS": round(float(f.end_s or spec.duration_s), 2),
            "outcome": f.outcome,
            "costKUsd": f.cost_k_usd,
            "samples": f.samples,
        }
        for f in interceptors
    ]

    return {
        "policy": active_policy,
        "evaluation": evaluation,
        "truth": {
            "scope": SCOPE_EXERCISE_TRUTH,
            "threats": threats_replay,
            "interceptors": interceptors_replay,
            "shrapnelImpacts": shrapnel_impacts_all,
        },
        "observations": {
            "scope": SCOPE_EXERCISE_OBSERVATION,
            "trackFrames": track_frames,
        },
        "magazineTimeline": magazine_timeline,
        "events": events,
        "civilianTraffic": [
            {**ct.to_dict(), "samples": ct.sample_trajectory(spec.duration_s, dt_s=2.0)} for ct in civilian_tracks
        ],
    }


def run_salvo_comparison(
    spec: SalvoScenarioSpec,
    *,
    env: EnvironmentModel,
    atm: AtmosphereModel,
    policies: tuple[str, ...] | None = None,
    surrogate: Any = None,
    source_draft: dict[str, Any] | None = None,
) -> dict[str, Any]:
    """Run Greedy vs Combinatorial WTA (plus AI Policy when trained) and assemble unified replay bundle."""
    t0 = time.perf_counter()
    if policies is None:
        has_ai = load_ai_wta_model() is not None
        policies = ("greedy_earliest", "combinatorial_wta", "ai_policy") if has_ai else ("greedy_earliest", "combinatorial_wta")

    runs: dict[str, dict[str, Any]] = {}
    for pol in policies:
        runs[pol] = run_salvo_policy(spec, env=env, atm=atm, policy=pol, surrogate=surrogate, record_replay=True)

    # Policy comparison delta table
    comp_rows = {
        pol: {
            "compositeScore": r["evaluation"]["compositeScore"],
            "leakers": r["evaluation"]["leakers"],
            "lethalIntercepted": r["evaluation"]["lethalIntercepted"],
            "lethalThreats": r["evaluation"]["lethalThreats"],
            "defenseSuccessRate": r["evaluation"]["defenseSuccessRate"],
            "shotsFired": r["evaluation"]["shotsFired"],
            "totalCostKUsd": r["evaluation"]["totalCostKUsd"],
            "decoyShotsWasted": r["evaluation"]["decoyShotsWasted"],
            "airspaceViolations": r["evaluation"]["airspaceViolations"],
            "debrisBuildingHits": r["evaluation"]["debrisBuildingHits"],
            "estimatedPopulationExposed": r["evaluation"]["estimatedPopulationExposed"],
            "emergencyDetourDelayS": r["evaluation"]["roadImpact"]["emergencyDetourDelayS"],
        }
        for pol, r in runs.items()
    }

    wta_vs_greedy: dict[str, Any] = {}
    if "greedy_earliest" in comp_rows and "combinatorial_wta" in comp_rows:
        g, w = comp_rows["greedy_earliest"], comp_rows["combinatorial_wta"]
        cost_saved = round(g["totalCostKUsd"] - w["totalCostKUsd"], 1)
        wta_vs_greedy = {
            "scoreDelta": round(w["compositeScore"] - g["compositeScore"], 2),
            "leakersDelta": w["leakers"] - g["leakers"],
            "costSavedKUsd": cost_saved,
            "costSavedPct": round((cost_saved / g["totalCostKUsd"]) * 100.0, 1) if g["totalCostKUsd"] > 0 else 0.0,
            "decoyWasteDelta": w["decoyShotsWasted"] - g["decoyShotsWasted"],
            "airspaceViolationsDelta": w["airspaceViolations"] - g["airspaceViolations"],
        }

    # Precompute ENU coordinates for assets, sensors, batteries
    assets_out = []
    for a in spec.assets:
        ag = env.height_at(a.east, a.north, fallback=env.up_median)
        lon, lat, _ = env.enu_to_geodetic(a.east, a.north, ag)
        assets_out.append(
            {"id": a.id, "label": a.label, "enu": [round(a.east, 1), round(a.north, 1), round(ag, 1)], "lon": round(lon, 6), "lat": round(lat, 6), "radiusM": a.radius_m, "value": a.value}
        )
    sensors_out = []
    for s in spec.sensors:
        sg = env.height_at(s.east, s.north, fallback=env.up_median)
        lon, lat, _ = env.enu_to_geodetic(s.east, s.north, sg + s.mast_agl_m)
        sensors_out.append(
            {"id": s.id, "type": s.type, "label": s.label, "enu": [round(s.east, 1), round(s.north, 1), round(sg + s.mast_agl_m, 1)], "lon": round(lon, 6), "lat": round(lat, 6), "maxRangeM": s.max_range_m}
        )

    first_run = next(iter(runs.values()))
    replay = {
        "schema": SALVO_REPLAY_SCHEMA,
        "engineVersion": SALVO_ENGINE_VERSION,
        "derivation": DERIVATION_LABEL,
        "generatedAt": dt.datetime.now(dt.timezone.utc).isoformat().replace("+00:00", "Z"),
        "elapsedS": round(time.perf_counter() - t0, 3),
        "scenario": {"scope": SCOPE_EXERCISE_INPUT, **spec.to_dict()},
        "environment": env.metadata(),
        "atmosphere": atm.metadata(),
        "viewerContext": env.export_viewer_context(),
        "assets": assets_out,
        "sensors": sensors_out,
        "civilianTraffic": first_run["civilianTraffic"],
        "policies": list(policies),
        "runs": {pol: {k: v for k, v in r.items() if k != "civilianTraffic"} for pol, r in runs.items()},
        "comparison": {
            "scope": SCOPE_EVALUATION,
            "policies": comp_rows,
            "wtaVsGreedy": wta_vs_greedy,
        },
        "sourceDraft": source_draft,
    }
    return replay
