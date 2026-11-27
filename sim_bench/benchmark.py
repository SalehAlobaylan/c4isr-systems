"""Frozen Multi-Biome Multi-Seed AI & Defense Benchmark Harness.

Compares ``greedy_earliest`` vs ``combinatorial_wta`` vs trained ``ai_policy``
across verified Saudi biomes (Riyadh, Asir Highlands, Jubail Industrial Coast,
and Owned Fixture) and writes a reproducible scorecard to
``tmp/benchmarks/latest.json``.
"""
from __future__ import annotations

import datetime as dt
import json
from pathlib import Path
import time
from typing import Any

import numpy as np

from salvo_defense.wta import load_ai_wta_model

from .rl_env import SalvoRLEnv
from .surrogate import load_debris_surrogate

REPO_ROOT = Path(__file__).resolve().parents[1]
DEFAULT_BENCH_OUT = REPO_ROOT / "tmp" / "benchmarks" / "latest.json"


def run_benchmark_suite(
    *,
    env_ids: list[str] | None = None,
    seeds: tuple[int, ...] = (1001, 2002),
    out_path: Path = DEFAULT_BENCH_OUT,
) -> dict[str, Any]:
    """Run frozen multi-biome benchmark across all available WTA policies."""
    t0 = time.perf_counter()
    surrogate = load_debris_surrogate()
    ai_model = load_ai_wta_model()
    policies = ("greedy_earliest", "combinatorial_wta", "ai_policy") if ai_model is not None else ("greedy_earliest", "combinatorial_wta")

    rl_env = SalvoRLEnv(env_ids=env_ids, surrogate=surrogate)
    raid_profiles = [
        ("mixed_swarm", {"decoys": 3, "uavs": 5, "cruiseMissiles": 2, "ballisticMissiles": 1}),
        ("saturation", {"decoys": 3, "uavs": 6, "cruiseMissiles": 3, "ballisticMissiles": 2}),
    ]

    cases: list[dict[str, Any]] = []
    agg: dict[str, list[dict[str, Any]]] = {p: [] for p in policies}

    for eid in rl_env.env_ids:
        for prof_name, raid in raid_profiles:
            for sd in seeds:
                row: dict[str, Any] = {"biome": eid, "profile": prof_name, "seed": sd, "policies": {}}
                for pol in policies:
                    ev = rl_env.evaluate_episode(env_id=eid, seed=sd, raid=raid, policy=pol)
                    slim = {
                        "compositeScore": ev["compositeScore"],
                        "leakers": ev["leakers"],
                        "defenseSuccessRate": ev["defenseSuccessRate"],
                        "totalCostKUsd": ev["totalCostKUsd"],
                        "decoyShotsWasted": ev["decoyShotsWasted"],
                        "airspaceViolations": ev["airspaceViolations"],
                        "debrisBuildingHits": ev["debrisBuildingHits"],
                        "estimatedPopulationExposed": ev["estimatedPopulationExposed"],
                        "emergencyDetourDelayS": ev["roadImpact"]["emergencyDetourDelayS"],
                    }
                    row["policies"][pol] = slim
                    agg[pol].append(slim)
                cases.append(row)

    leaderboard: list[dict[str, Any]] = []
    for pol in policies:
        items = agg[pol]
        leaderboard.append(
            {
                "policy": pol,
                "episodes": len(items),
                "meanCompositeScore": round(float(np.mean([x["compositeScore"] for x in items])), 2),
                "meanDefenseSuccessPct": round(float(np.mean([x["defenseSuccessRate"] for x in items])) * 100.0, 1),
                "meanLeakers": round(float(np.mean([x["leakers"] for x in items])), 2),
                "meanCostKUsd": round(float(np.mean([x["totalCostKUsd"] for x in items])), 1),
                "meanDecoyWaste": round(float(np.mean([x["decoyShotsWasted"] for x in items])), 2),
                "totalAirspaceViolations": int(sum(x["airspaceViolations"] for x in items)),
                "meanPopExposed": round(float(np.mean([x["estimatedPopulationExposed"] for x in items])), 1),
            }
        )
    leaderboard.sort(key=lambda x: x["meanCompositeScore"], reverse=True)

    report = {
        "schema": "emad.sim-benchmark.v1",
        "generatedAt": dt.datetime.now(dt.timezone.utc).isoformat().replace("+00:00", "Z"),
        "elapsedS": round(time.perf_counter() - t0, 2),
        "biomes": rl_env.env_ids,
        "seeds": list(seeds),
        "surrogateActive": surrogate is not None,
        "aiPolicyActive": ai_model is not None,
        "leaderboard": leaderboard,
        "cases": cases,
    }
    out_path.parent.mkdir(parents=True, exist_ok=True)
    out_path.write_text(json.dumps(report, indent=2), encoding="utf-8")
    return report
