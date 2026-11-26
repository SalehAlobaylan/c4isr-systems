"""End-to-End Deterministic Simulation Runner & Safe-Window Intercept Optimizer.

Orchestrates all four modular stages on real verified EMAD environments:
  1. Inbound 3-DOF threat trajectory over real DSM terrain
  2. Terrain-masked synthetic radar -> imperfect EXERCISE_OBSERVATION -> tracker
  3. Intercept decision policy ("earliest" baseline vs. "safe_window" wind-aware optimizer)
  4. 3-DOF interceptor pursuit -> intercept -> wind-drifted shrapnel cloud & exposure scoring

Emits strictly separated EMAD partition artifacts:
  - manifest.json      (EXERCISE_INPUT)
  - truth.json         (EXERCISE_TRUTH)
  - observations.json  (EXERCISE_OBSERVATION)
  - evaluation.json    (EVALUATION)
  - replay.json        (3D viewer bundle with explicit truth/observation separation)
"""
from __future__ import annotations

import hashlib
import json
import math
from pathlib import Path
from typing import Any

import numpy as np

from .atmosphere import AtmosphereModel
from .contracts import (
    FIDELITY_KINEMATIC_3DOF,
    SCHEMA_EVALUATION,
    SCHEMA_OBSERVATIONS,
    SCHEMA_REPLAY,
    SCHEMA_RUN_MANIFEST,
    SCHEMA_TRUTH,
    SCOPE_EVALUATION,
    SCOPE_EXERCISE_INPUT,
    SCOPE_EXERCISE_OBSERVATION,
    SCOPE_EXERCISE_TRUTH,
    SIM_ENGINE_VERSION,
    ScenarioSpec,
)
from .environment import EnvironmentModel
from .physics import (
    TrajectorySample,
    build_threat_trajectory,
    simulate_interceptor_pursuit,
    simulate_shrapnel_cloud,
)
from .sensors import ConstantVelocityTracker, TerrainMaskedRadar, TrackEstimate


def _resolve_path(base_dir: Path, raw_path: str) -> Path:
    p = Path(raw_path)
    if p.is_absolute():
        return p
    # Check relative to scenario directory first, then repository/current working directory
    rel = base_dir / p
    if rel.exists():
        return rel.resolve()
    return p.resolve()


def _estimate_intercept_geometry(
    est: TrackEstimate,
    launch_pos: np.ndarray,
    *,
    launch_time_s: float,
    launch_delay_s: float,
    interceptor_speed_mps: float,
) -> tuple[float, np.ndarray] | None:
    """Predict intercept time and ENU point from OBSERVATION-ONLY track estimate."""
    liftoff_s = launch_time_s + launch_delay_s
    dt_to_liftoff = max(0.0, liftoff_s - est.t_s)
    p_at_liftoff = est.pos_enu + est.vel_enu * dt_to_liftoff

    # Fixed-point iteration for lead intercept: ||(p + v*t_go) - launch|| = speed * t_go
    t_go = float(np.linalg.norm(p_at_liftoff - launch_pos)) / max(interceptor_speed_mps, 1.0)
    for _ in range(6):
        pred = p_at_liftoff + est.vel_enu * t_go
        dist = float(np.linalg.norm(pred - launch_pos))
        t_go = dist / max(interceptor_speed_mps * 0.92, 1.0)
        if t_go > 35.0:
            return None
    pred_pos = p_at_liftoff + est.vel_enu * t_go
    return liftoff_s + t_go, pred_pos


def _choose_engagement(
    spec: ScenarioSpec,
    env: EnvironmentModel,
    atm: AtmosphereModel,
    track_estimates: list[TrackEstimate],
    launch_pos: np.ndarray,
    protected_center: tuple[float, float],
) -> dict[str, Any]:
    """Select interceptor launch time using strictly OBSERVATION-ONLY track estimates."""
    site = spec.interceptor_site
    firm = [e for e in track_estimates if e.update_count >= 3]
    if not firm:
        return {"decided": False, "reason": "no-firm-track-established", "candidates": []}

    if spec.policy == "earliest":
        for est in firm:
            pred = _estimate_intercept_geometry(
                est,
                launch_pos,
                launch_time_s=est.t_s,
                launch_delay_s=site.launch_delay_s,
                interceptor_speed_mps=site.speed_mps,
            )
            if pred is None:
                continue
            t_int, p_int = pred
            dist_from_site = float(np.linalg.norm(p_int - launch_pos))
            if site.min_range_m <= dist_from_site <= site.max_range_m:
                return {
                    "decided": True,
                    "policy": "earliest",
                    "decisionTimeS": round(est.t_s, 3),
                    "launchTimeS": round(est.t_s, 3),
                    "predictedInterceptTimeS": round(t_int, 3),
                    "predictedInterceptEnu": [round(float(v), 2) for v in p_int],
                    "candidates": [],
                }
        return {"decided": False, "reason": "never-entered-interceptor-envelope", "candidates": []}

    if spec.policy == "safe_window":
        # Wait for an initial firm track, then sweep candidate launch delays over the planning
        # horizon, previewing wind-drifted shrapnel exposure for every feasible window.
        first_est = firm[0]
        # Prefer using a slightly refined track (4th update if available within 2s) for better velocity vector
        plan_est = firm[min(1, len(firm) - 1)]
        if plan_est.t_s - first_est.t_s > 2.5:
            plan_est = first_est

        candidates: list[dict[str, Any]] = []
        best: dict[str, Any] | None = None

        # Adapt sweep cadence automatically for fast cruise / ballistic threats
        est_speed = float(np.linalg.norm(plan_est.vel_enu))
        step_s = min(spec.safe_window_step_s, 0.5) if est_speed > 140.0 else spec.safe_window_step_s
        horizon_s = min(spec.safe_window_horizon_s, 14.0) if est_speed > 140.0 else spec.safe_window_horizon_s
        n_cands = max(1, int(math.floor(horizon_s / step_s))) + 1
        for k in range(n_cands):
            cand_launch_s = round(plan_est.t_s + k * step_s, 3)
            if cand_launch_s >= spec.duration_s - 2.0:
                break
            pred = _estimate_intercept_geometry(
                plan_est,
                launch_pos,
                launch_time_s=cand_launch_s,
                launch_delay_s=site.launch_delay_s,
                interceptor_speed_mps=site.speed_mps,
            )
            if pred is None:
                continue
            t_int, p_int = pred
            # Clamp predicted point above local terrain for preview realism
            g_pred = env.height_at(float(p_int[0]), float(p_int[1]), fallback=env.up_median)
            if not (env.east_min <= p_int[0] <= env.east_max and env.north_min <= p_int[1] <= env.north_max):
                continue
            p_int[2] = max(p_int[2], g_pred + 25.0)

            dist_from_site = float(np.linalg.norm(p_int - launch_pos))
            dist_to_protected = math.hypot(p_int[0] - protected_center[0], p_int[1] - protected_center[1])
            feasible = (
                site.min_range_m <= dist_from_site <= site.max_range_m
                and dist_to_protected >= spec.protected_zone_radius_m * 1.15
            )
            if not feasible:
                candidates.append(
                    {
                        "candidateLaunchS": cand_launch_s,
                        "predictedInterceptS": round(t_int, 2),
                        "predictedInterceptEnu": [round(float(v), 1) for v in p_int],
                        "feasible": False,
                        "reason": "outside-envelope-or-too-close-to-protected-zone",
                    }
                )
                continue

            preview = simulate_shrapnel_cloud(
                p_int,
                plan_est.vel_enu,
                spec.shrapnel,
                env,
                atm,
                seed=spec.seed + 7001 + k,
                fragment_count_override=spec.safe_window_preview_fragments,
                record_trails=False,
                protected_center=protected_center,
                protected_radius_m=spec.protected_zone_radius_m,
            )
            exp = preview["exposure"]
            total_f = max(exp["total"], 1)
            bldg_frac = exp["buildingHits"] / total_f
            road_frac = exp["roadHits"] / total_f
            zone_frac = exp["protectedZoneHits"] / total_f

            # Exposure objective + tiny preference for earlier intercept when exposure is tied
            delay_tiebreak = 0.004 * (cand_launch_s - plan_est.t_s)
            cost = (
                spec.safe_window_building_weight * bldg_frac
                + spec.safe_window_road_weight * road_frac
                + spec.safe_window_protected_zone_weight * zone_frac
                + delay_tiebreak
            )
            cand_record = {
                "candidateLaunchS": cand_launch_s,
                "predictedInterceptS": round(t_int, 2),
                "predictedInterceptEnu": [round(float(v), 1) for v in p_int],
                "feasible": True,
                "previewBuildingHits": exp["buildingHits"],
                "previewRoadHits": exp["roadHits"],
                "previewProtectedHits": exp["protectedZoneHits"],
                "previewUrbanFraction": exp["urbanExposureFraction"],
                "previewDriftM": preview["footprint"]["driftDistanceM"],
                "objectiveCost": round(cost, 4),
            }
            candidates.append(cand_record)
            if best is None or cost < best["objectiveCost"]:
                best = cand_record

        if best is None:
            # Fallback to earliest feasible intercept if a high-speed threat leaves no margin window
            for est in firm:
                pred = _estimate_intercept_geometry(
                    est,
                    launch_pos,
                    launch_time_s=est.t_s,
                    launch_delay_s=site.launch_delay_s,
                    interceptor_speed_mps=site.speed_mps,
                )
                if pred is None:
                    continue
                t_int, p_int = pred
                dist_from_site = float(np.linalg.norm(p_int - launch_pos))
                if site.min_range_m <= dist_from_site <= site.max_range_m:
                    return {
                        "decided": True,
                        "policy": "safe_window",
                        "fallbackReason": "earliest-feasible-high-speed-fallback",
                        "decisionTimeS": round(est.t_s, 3),
                        "launchTimeS": round(est.t_s, 3),
                        "predictedInterceptTimeS": round(t_int, 3),
                        "predictedInterceptEnu": [round(float(v), 2) for v in p_int],
                        "candidates": candidates,
                    }
            return {"decided": False, "reason": "no-feasible-safe-window", "candidates": candidates}

        best["selected"] = True
        return {
            "decided": True,
            "policy": "safe_window",
            "decisionTimeS": round(plan_est.t_s, 3),
            "launchTimeS": best["candidateLaunchS"],
            "predictedInterceptTimeS": best["predictedInterceptS"],
            "predictedInterceptEnu": best["predictedInterceptEnu"],
            "selectedCost": best["objectiveCost"],
            "candidates": candidates,
        }

    raise ValueError(f"Unknown engagement policy: {spec.policy}")


def run_simulation(
    spec: ScenarioSpec,
    *,
    base_dir: Path | None = None,
    env: EnvironmentModel | None = None,
    atm: AtmosphereModel | None = None,
) -> dict[str, Any]:
    """Execute one complete deterministic simulation run and return all 4 EMAD partitions."""
    root = base_dir or Path.cwd()
    if env is None:
        pkg_path = _resolve_path(root, spec.environment_package)
        env = EnvironmentModel(
            pkg_path,
            expected_manifest_sha256=spec.expected_manifest_sha256,
            derived_building_height_m=spec.derived_building_height_m,
            road_buffer_m=spec.road_exposure_buffer_m,
        )
    if atm is None:
        ext_weather = _resolve_path(root, spec.weather_field_path) if spec.weather_field_path else None
        atm = AtmosphereModel.load(env, external_path=ext_weather, override=spec.atmosphere)

    # Resolve launch site and protected zone center
    se, sn, sagl = spec.interceptor_site.position_enu
    site_ground = env.height_at(float(se), float(sn), fallback=env.up_median)
    launch_pos = np.array([float(se), float(sn), site_ground + float(sagl)], dtype=np.float64)

    if spec.protected_zone_center_enu is not None:
        protected_center = (float(spec.protected_zone_center_enu[0]), float(spec.protected_zone_center_enu[1]))
    else:
        last_wp = spec.threat.waypoints_enu[-1]
        protected_center = (float(last_wp[0]), float(last_wp[1]))
    protected_ground = env.height_at(protected_center[0], protected_center[1], fallback=env.up_median)

    # Stage 1: Inbound Threat Truth Trajectory
    threat_samples = build_threat_trajectory(spec.threat, env, duration_s=spec.duration_s, dt_s=spec.dt_s)

    # Stage 2: Synthetic Terrain-Masked Radar -> Imperfect EXERCISE_OBSERVATION -> Tracker
    radar = TerrainMaskedRadar(spec.radar, env, seed=spec.seed)
    radar_scan = radar.scan_trajectory(spec.threat.id, spec.threat.rcs_m2, threat_samples, dt_s=spec.dt_s)
    tracker = ConstantVelocityTracker()
    track_estimates: list[TrackEstimate] = [tracker.ingest(obs) for obs in radar_scan["observations"]]

    # Stage 3: Policy Engagement Decision (strictly using OBSERVATION track estimates!)
    decision = _choose_engagement(spec, env, atm, track_estimates, launch_pos, protected_center)

    # Stage 4: Interceptor Pursuit & Wind-Drifted Shrapnel Propagation
    interceptor_result: dict[str, Any] = {"intercepted": False, "reason": decision.get("reason", "no-launch"), "samples": []}
    shrapnel_result: dict[str, Any] | None = None
    intercept_idx: int | None = None

    if decision["decided"]:
        aim_pt = np.asarray(decision["predictedInterceptEnu"], dtype=np.float64)
        interceptor_result = simulate_interceptor_pursuit(
            spec.interceptor_site,
            env,
            threat_samples,
            launch_time_s=float(decision["launchTimeS"]),
            dt_s=spec.dt_s,
            aim_lead_pos=aim_pt,
        )
        if interceptor_result["intercepted"]:
            intercept_idx = int(interceptor_result["interceptIndex"])
            shrapnel_result = simulate_shrapnel_cloud(
                interceptor_result["interceptPosEnu"],
                interceptor_result["threatVelEnu"],
                spec.shrapnel,
                env,
                atm,
                seed=spec.seed + 99991,
                interceptor_vel=interceptor_result.get("interceptorVelEnu"),
                record_trails=True,
                protected_center=protected_center,
                protected_radius_m=spec.protected_zone_radius_m,
            )

    # Trim threat truth trajectory & post-intercept observations after intercept instant
    end_t_s = float(interceptor_result["interceptTimeS"]) if interceptor_result["intercepted"] else spec.duration_s
    active_threat_truth = [
        {
            "tS": round(ts.t_s, 3),
            "enu": [round(float(v), 2) for v in ts.pos],
            "velEnu": [round(float(v), 2) for v in ts.vel],
            "active": ts.active,
        }
        for i, ts in enumerate(threat_samples)
        if ts.active and (intercept_idx is None or i <= intercept_idx)
    ]

    visible_obs = [o for o in radar_scan["observations"] if float(o["sampleS"]) <= end_t_s + 1e-6]
    visible_tracks = [
        {
            "contactId": e.contact_id,
            "tS": round(e.t_s, 3),
            "sampleS": round(e.sample_s, 3),
            "estimatedEnu": [round(float(v), 2) for v in e.pos_enu],
            "estimatedVelEnu": [round(float(v), 2) for v in e.vel_enu],
            "updateCount": e.update_count,
            "confidence": e.confidence,
        }
        for e in track_estimates
        if e.sample_s <= end_t_s + 1e-6
    ]
    visible_los_truth = [r for r in radar_scan["losTruth"] if float(r["sampleS"]) <= end_t_s + 1e-6]

    # Compute Evaluation Metrics (comparing EXERCISE_OBSERVATION vs sealed EXERCISE_TRUTH)
    threat_by_t = {round(ts.t_s, 3): ts for ts in threat_samples}
    track_errors_m = []
    for e in track_estimates:
        if e.sample_s > end_t_s + 1e-6:
            continue
        ts_match = threat_by_t.get(round(e.t_s, 1))
        if ts_match is not None and ts_match.active:
            track_errors_m.append(float(np.linalg.norm(e.pos_enu - ts_match.pos)))

    n_scans = len(visible_los_truth)
    n_los_clear = sum(1 for r in visible_los_truth if r["losClear"])
    n_terrain_blocked = sum(1 for r in visible_los_truth if r["blockedBy"] == "terrain")
    n_bldg_blocked = sum(1 for r in visible_los_truth if r["blockedBy"] == "building")
    first_los_s = next((r["sampleS"] for r in visible_los_truth if r["losClear"]), None)
    first_det_s = visible_obs[0]["deliverS"] if visible_obs else None
    firm_track_s = next((e.t_s for e in track_estimates if e.update_count >= 3), None)

    intercept_metrics: dict[str, Any] = {"intercepted": bool(interceptor_result["intercepted"])}
    if interceptor_result["intercepted"]:
        ip = interceptor_result["interceptPosEnu"]
        ig = env.height_at(float(ip[0]), float(ip[1]), fallback=env.up_median)
        dist_prot = math.hypot(float(ip[0]) - protected_center[0], float(ip[1]) - protected_center[1])
        intercept_metrics.update(
            {
                "launchTimeS": interceptor_result["launchTimeS"],
                "liftoffTimeS": interceptor_result["liftoffTimeS"],
                "interceptTimeS": interceptor_result["interceptTimeS"],
                "flightDurationS": interceptor_result["flightDurationS"],
                "interceptEnu": [round(float(v), 2) for v in ip],
                "interceptAglM": round(float(ip[2] - ig), 2),
                "distanceToProtectedCenterM": round(dist_prot, 2),
                "protectedMarginM": round(dist_prot - spec.protected_zone_radius_m, 2),
                "missDistanceM": interceptor_result["missDistanceM"],
            }
        )
    else:
        intercept_metrics["reason"] = interceptor_result.get("reason")

    run_id_seed = json.dumps(
        {
            "scenario": spec.to_dict(),
            "manifest": env.manifest_sha256,
            "weather": atm.source_sha256,
        },
        sort_keys=True,
    )
    run_id = "simrun_" + hashlib.sha256(run_id_seed.encode("utf-8")).hexdigest()[:16]

    manifest_doc = {
        "schema": SCHEMA_RUN_MANIFEST,
        "scope": SCOPE_EXERCISE_INPUT,
        "runId": run_id,
        "engineVersion": SIM_ENGINE_VERSION,
        "fidelityProfile": FIDELITY_KINEMATIC_3DOF,
        "seed": spec.seed,
        "policy": spec.policy,
        "scenario": spec.to_dict(),
        "environment": env.metadata(),
        "atmosphere": atm.metadata(),
        "radarPositionEnu": radar_scan["radarPositionEnu"],
        "launcherPositionEnu": [round(float(v), 2) for v in launch_pos],
        "protectedZone": {
            "centerEnu": [round(protected_center[0], 2), round(protected_center[1], 2), round(protected_ground, 2)],
            "radiusM": spec.protected_zone_radius_m,
        },
    }

    interceptor_truth_samples = [
        {"tS": round(s.t_s, 3), "enu": [round(float(v), 2) for v in s.pos]}
        for s in interceptor_result.get("samples", [])
    ]

    truth_doc = {
        "schema": SCHEMA_TRUTH,
        "scope": SCOPE_EXERCISE_TRUTH,
        "runId": run_id,
        "notice": "Privileged simulation ground truth. Must not be consumed by observation-only trackers or operator COP.",
        "threatId": spec.threat.id,
        "threatKind": spec.threat.kind,
        "threatTrajectory": active_threat_truth,
        "radarLosTruth": visible_los_truth,
        "interceptorTrajectory": interceptor_truth_samples,
        "intercept": intercept_metrics,
        "shrapnel": shrapnel_result,
    }

    observations_doc = {
        "schema": SCHEMA_OBSERVATIONS,
        "scope": SCOPE_EXERCISE_OBSERVATION,
        "runId": run_id,
        "notice": "Imperfect synthetic sensor detections and observation-only track estimates. Opaque contact IDs only.",
        "radarId": spec.radar.id,
        "radarPositionEnu": radar_scan["radarPositionEnu"],
        "detections": visible_obs,
        "tracks": visible_tracks,
    }

    evaluation_doc = {
        "schema": SCHEMA_EVALUATION,
        "scope": SCOPE_EVALUATION,
        "runId": run_id,
        "scenarioId": spec.scenario_id,
        "policy": spec.policy,
        "fidelityProfile": FIDELITY_KINEMATIC_3DOF,
        "sensing": {
            "totalScansBeforeEnd": n_scans,
            "losClearScans": n_los_clear,
            "terrainBlockedScans": n_terrain_blocked,
            "buildingBlockedScans": n_bldg_blocked,
            "occlusionFraction": round(1.0 - (n_los_clear / max(n_scans, 1)), 4),
            "detectionsCount": len(visible_obs),
            "firstClearLosS": first_los_s,
            "firstDetectionDeliveredS": first_det_s,
            "firmTrackEstablishedS": round(firm_track_s, 3) if firm_track_s is not None else None,
            "meanTrackErrorM": round(float(np.mean(track_errors_m)), 2) if track_errors_m else None,
        },
        "engagementDecision": decision,
        "interception": intercept_metrics,
        "shrapnelExposure": shrapnel_result["exposure"] if shrapnel_result else None,
        "shrapnelFootprint": shrapnel_result["footprint"] if shrapnel_result else None,
    }

    return {
        "runId": run_id,
        "manifest": manifest_doc,
        "truth": truth_doc,
        "observations": observations_doc,
        "evaluation": evaluation_doc,
        "_env": env,
        "_atm": atm,
        "_radar": radar,
    }


def _compact_run_for_replay(run: dict[str, Any]) -> dict[str, Any]:
    """Extract compact, stride-sampled replay data for smooth 60fps Three.js playback."""
    truth = run["truth"]
    obs = run["observations"]
    ev = run["evaluation"]
    # Keep every 2nd step (0.2s) + always include last point for crisp rendering
    t_traj = truth["threatTrajectory"]
    t_compact = t_traj[::2]
    if t_traj and (not t_compact or t_compact[-1]["tS"] != t_traj[-1]["tS"]):
        t_compact.append(t_traj[-1])

    i_traj = truth["interceptorTrajectory"]
    i_compact = i_traj[::2]
    if i_traj and (not i_compact or i_compact[-1]["tS"] != i_traj[-1]["tS"]):
        i_compact.append(i_traj[-1])

    shrapnel = truth["shrapnel"]
    shrapnel_compact = None
    if shrapnel is not None:
        shrapnel_compact = {
            "fragmentCount": shrapnel["fragmentCount"],
            "exposure": shrapnel["exposure"],
            "footprint": shrapnel["footprint"],
            "impacts": [
                {
                    "id": f["id"],
                    "origin": f["origin"],
                    "class": f["class"],
                    "massKg": f["massKg"],
                    "impactSpeedMps": f["impactSpeedMps"],
                    "impactEnergyJ": f["impactEnergyJ"],
                    "hazardous": f["hazardous"],
                    "enu": f["impactEnu"],
                    "tAfterInterceptS": f["impactTimeAfterInterceptS"],
                    "category": f["exposureCategory"],
                    "buildingSourceId": f["buildingSourceId"],
                }
                for f in shrapnel["fragments"]
            ],
            "trails": shrapnel["trails"],
        }

    sens = ev["sensing"]
    intr = truth["intercept"]
    decision = ev.get("engagementDecision") or {}
    last_impact_s = (
        round(float(intr["interceptTimeS"]) + float(shrapnel["footprint"]["maxFallTimeS"]), 2)
        if (intr.get("intercepted") and shrapnel)
        else None
    )
    timeline_events = {
        "firstClearLosS": sens.get("firstClearLosS"),
        "firstDetectionS": sens.get("firstDetectionDeliveredS"),
        "firmTrackS": sens.get("firmTrackEstablishedS"),
        "launchS": intr.get("launchTimeS") if intr.get("intercepted") else None,
        "interceptS": intr.get("interceptTimeS") if intr.get("intercepted") else None,
        "lastImpactS": last_impact_s,
    }

    return {
        "runId": run["runId"],
        "policy": ev["policy"],
        "threatKind": truth.get("threatKind", "one_way_uav"),
        "timelineEvents": timeline_events,
        "optimizerCandidates": decision.get("candidates") or [],
        "evaluation": ev,
        "truth": {
            "scope": SCOPE_EXERCISE_TRUTH,
            "threatTrajectory": t_compact,
            "interceptorTrajectory": i_compact,
            "radarLosTruth": truth["radarLosTruth"],
            "intercept": truth["intercept"],
            "shrapnel": shrapnel_compact,
        },
        "observations": {
            "scope": SCOPE_EXERCISE_OBSERVATION,
            "detections": obs["detections"],
            "tracks": obs["tracks"],
        },
    }


def save_run_bundle(
    runs: list[dict[str, Any]],
    out_dir: Path,
    *,
    comparison_summary: dict[str, Any] | None = None,
    source_draft: dict[str, Any] | None = None,
) -> Path:
    """Persist partitioned JSON artifacts and a unified 3D replay.json to ``out_dir``."""
    out_dir.mkdir(parents=True, exist_ok=True)
    primary = runs[0]
    env: EnvironmentModel = primary["_env"]
    atm: AtmosphereModel = primary["_atm"]
    radar: TerrainMaskedRadar = primary["_radar"]

    for run in runs:
        sub = out_dir if len(runs) == 1 else (out_dir / run["evaluation"]["policy"])
        sub.mkdir(parents=True, exist_ok=True)
        for key in ("manifest", "truth", "observations", "evaluation"):
            (sub / f"{key}.json").write_text(json.dumps(run[key], indent=2), encoding="utf-8")

    if comparison_summary is not None:
        (out_dir / "comparison.json").write_text(json.dumps(comparison_summary, indent=2), encoding="utf-8")

    # Compute radar terrain-masked visibility fan at the threat's initial AGL for the 3D viewer
    probe_agl = float(primary["manifest"]["scenario"]["threat"]["waypoints_enu"][0][2])
    coverage_ring = radar.coverage_profile(probe_agl_m=max(40.0, probe_agl), n_azimuths=72)

    replay = {
        "schema": SCHEMA_REPLAY,
        "engineVersion": SIM_ENGINE_VERSION,
        "fidelityProfile": FIDELITY_KINEMATIC_3DOF,
        "scenarioId": primary["manifest"]["scenario"]["scenario_id"],
        "title": primary["manifest"]["scenario"]["title"],
        "description": primary["manifest"]["scenario"]["description"],
        "environment": primary["manifest"]["environment"],
        "atmosphere": primary["manifest"]["atmosphere"],
        "radarPositionEnu": primary["manifest"]["radarPositionEnu"],
        "launcherPositionEnu": primary["manifest"]["launcherPositionEnu"],
        "protectedZone": primary["manifest"]["protectedZone"],
        "radarCoverageRing": coverage_ring,
        "windProfile": atm.profile_summary(0.0, 0.0),
        "viewerContext": env.export_viewer_context(),
        "runs": {r["evaluation"]["policy"]: _compact_run_for_replay(r) for r in runs},
        "comparison": comparison_summary,
        "sourceDraft": source_draft,
    }
    replay_path = out_dir / "replay.json"
    replay_path.write_text(json.dumps(replay, separators=(",", ":")), encoding="utf-8")
    return replay_path


def compare_policies(
    spec: ScenarioSpec,
    *,
    base_dir: Path | None = None,
    policies: tuple[str, ...] = ("earliest", "safe_window"),
) -> tuple[list[dict[str, Any]], dict[str, Any]]:
    """Run multiple policies on the exact same frozen environment & seed and build comparison."""
    root = base_dir or Path.cwd()
    pkg_path = _resolve_path(root, spec.environment_package)
    env = EnvironmentModel(
        pkg_path,
        expected_manifest_sha256=spec.expected_manifest_sha256,
        derived_building_height_m=spec.derived_building_height_m,
        road_buffer_m=spec.road_exposure_buffer_m,
    )
    ext_weather = _resolve_path(root, spec.weather_field_path) if spec.weather_field_path else None
    atm = AtmosphereModel.load(env, external_path=ext_weather, override=spec.atmosphere)

    runs = [run_simulation(spec.with_policy(p), base_dir=root, env=env, atm=atm) for p in policies]

    summary_rows = {}
    for r in runs:
        ev = r["evaluation"]
        exp = ev["shrapnelExposure"] or {}
        fp = ev["shrapnelFootprint"] or {}
        summary_rows[ev["policy"]] = {
            "intercepted": ev["interception"]["intercepted"],
            "launchTimeS": ev["interception"].get("launchTimeS"),
            "interceptTimeS": ev["interception"].get("interceptTimeS"),
            "interceptAglM": ev["interception"].get("interceptAglM"),
            "protectedMarginM": ev["interception"].get("protectedMarginM"),
            "buildingHits": exp.get("buildingHits"),
            "roadHits": exp.get("roadHits"),
            "protectedZoneHits": exp.get("protectedZoneHits"),
            "openHits": exp.get("openHits"),
            "urbanExposureFraction": exp.get("urbanExposureFraction"),
            "driftDistanceM": fp.get("driftDistanceM"),
        }

    delta = {}
    if "earliest" in summary_rows and "safe_window" in summary_rows:
        e_row = summary_rows["earliest"]
        s_row = summary_rows["safe_window"]
        if e_row["urbanExposureFraction"] is not None and s_row["urbanExposureFraction"] is not None:
            e_exp = e_row["urbanExposureFraction"]
            s_exp = s_row["urbanExposureFraction"]
            reduction = (e_exp - s_exp) / e_exp if e_exp > 0 else 0.0
            delta = {
                "buildingHitsDelta": (s_row["buildingHits"] or 0) - (e_row["buildingHits"] or 0),
                "roadHitsDelta": (s_row["roadHits"] or 0) - (e_row["roadHits"] or 0),
                "urbanExposureDelta": round(s_exp - e_exp, 4),
                "urbanExposureReductionPct": round(reduction * 100.0, 1),
            }

    comparison = {
        "schema": SCHEMA_EVALUATION,
        "scope": SCOPE_EVALUATION,
        "scenarioId": spec.scenario_id,
        "seed": spec.seed,
        "environmentManifestSha256": env.manifest_sha256,
        "policies": summary_rows,
        "safeWindowVsEarliest": delta,
    }
    return runs, comparison
