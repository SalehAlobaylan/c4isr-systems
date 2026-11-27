"""Pure-NumPy Cross-Entropy Method (CEM) RL Policy Trainer for Layered Salvo WTA.

Trains an observation-only 14-feature linear/scoring policy over ``SalvoRLEnv``
scenarios (mixed swarms, RF decoys, ballistic/cruise waves, civilian airliners),
validates against ``greedy_earliest`` and ``combinatorial_wta`` on held-out
seeds, and writes ``models/ai_wta_v1.npz`` — immediately usable live as
Policy #3 (``ai_policy``) in the 3D Salvo Defense Workbench!
"""
from __future__ import annotations

import json
from pathlib import Path
import time
from typing import Any

import numpy as np

from salvo_defense.wta import COMBINATORIAL_WEIGHTS, DEFAULT_AI_WTA_PATH, WTA_FEATURE_DIM

from .rl_env import SalvoRLEnv
from .surrogate import load_debris_surrogate


def train_ai_wta_policy(
    *,
    out_path: Path = DEFAULT_AI_WTA_PATH,
    generations: int = 8,
    population: int = 12,
    elite_k: int = 4,
    seed: int = 20261127,
) -> dict[str, Any]:
    """Run deterministic CEM optimization over SalvoRLEnv and persist ``models/ai_wta_v1.npz``."""
    t0 = time.perf_counter()
    rng = np.random.default_rng(seed)
    surrogate = load_debris_surrogate()
    rl_env = SalvoRLEnv(env_ids=["riyadh-observation-v1", "owned-fixture-v1"], surrogate=surrogate)

    # Diverse training episodes (varying raid mixtures & seeds)
    train_cases = [
        (rl_env.env_ids[0], 101, {"decoys": 3, "uavs": 5, "cruiseMissiles": 2, "ballisticMissiles": 1}),
        (rl_env.env_ids[0], 202, {"decoys": 4, "uavs": 6, "cruiseMissiles": 2, "ballisticMissiles": 2}),
        (rl_env.env_ids[-1], 303, {"decoys": 3, "uavs": 4, "cruiseMissiles": 2, "ballisticMissiles": 1}),
    ]
    val_cases = [
        (rl_env.env_ids[0], 801, {"decoys": 3, "uavs": 5, "cruiseMissiles": 2, "ballisticMissiles": 1}),
        (rl_env.env_ids[0], 802, {"decoys": 4, "uavs": 5, "cruiseMissiles": 3, "ballisticMissiles": 1}),
        (rl_env.env_ids[-1], 803, {"decoys": 3, "uavs": 4, "cruiseMissiles": 2, "ballisticMissiles": 1}),
    ]

    # Warm-start CEM distribution around calibrated domain prior with broad exploration sigma
    mean_w = COMBINATORIAL_WEIGHTS.copy()
    std_w = np.full(WTA_FEATURE_DIM, 0.75, dtype=np.float64)
    # Keep safety-critical civilian avoidance weights strictly negative
    std_w[5] = 0.2
    std_w[10] = 0.2

    def eval_weights(w: np.ndarray, cases: list[tuple[str, int, dict[str, Any]]]) -> tuple[float, float, float]:
        scores, costs, leakers = [], [], []
        for eid, sd, raid in cases:
            ev = rl_env.evaluate_episode(env_id=eid, seed=sd, raid=raid, policy="ai_policy", ai_weights=w)
            scores.append(float(ev["compositeScore"]))
            costs.append(float(ev["totalCostKUsd"]))
            leakers.append(float(ev["leakers"]))
        return float(np.mean(scores)), float(np.mean(costs)), float(np.mean(leakers))

    best_w = mean_w.copy()
    best_score, _, _ = eval_weights(best_w, train_cases)
    history: list[dict[str, Any]] = []

    for gen in range(generations):
        cands = [best_w]
        for _ in range(population - 1):
            w = mean_w + rng.normal(0.0, std_w)
            w[5] = min(w[5], -10.0)   # never engage civilian airliners
            w[10] = min(w[10], -15.0)  # never violate airspace cylinder
            cands.append(w)

        scored = []
        for w in cands:
            sc, cst, lk = eval_weights(w, train_cases)
            scored.append((sc, cst, lk, w))
        scored.sort(key=lambda x: x[0], reverse=True)

        if scored[0][0] > best_score:
            best_score = scored[0][0]
            best_w = scored[0][3].copy()

        elites = np.stack([x[3] for x in scored[:elite_k]])
        mean_w = 0.35 * mean_w + 0.65 * elites.mean(axis=0)
        std_w = np.maximum(0.12, 0.85 * std_w)
        history.append(
            {
                "generation": gen + 1,
                "bestTrainScore": round(scored[0][0], 2),
                "eliteMeanScore": round(float(np.mean([x[0] for x in scored[:elite_k]])), 2),
                "bestCostKUsd": round(scored[0][1], 1),
            }
        )

    # Held-out validation comparison: Greedy vs Combinatorial vs AI Policy
    def eval_named(pol: str, w: np.ndarray | None = None) -> dict[str, float]:
        evs = [rl_env.evaluate_episode(env_id=e, seed=s, raid=r, policy=pol, ai_weights=w) for e, s, r in val_cases]
        return {
            "meanScore": round(float(np.mean([x["compositeScore"] for x in evs])), 2),
            "meanCostKUsd": round(float(np.mean([x["totalCostKUsd"] for x in evs])), 1),
            "meanLeakers": round(float(np.mean([x["leakers"] for x in evs])), 2),
            "meanDecoyWaste": round(float(np.mean([x["decoyShotsWasted"] for x in evs])), 2),
            "airspaceViolations": int(sum(x["airspaceViolations"] for x in evs)),
        }

    val_greedy = eval_named("greedy_earliest")
    val_comb = eval_named("combinatorial_wta")
    val_ai = eval_named("ai_policy", best_w)

    meta = {
        "schema": "emad.ai-wta-policy.v1",
        "derivation": "derived/cem-rl-observation-only-wta",
        "featureDim": WTA_FEATURE_DIM,
        "generations": generations,
        "population": population,
        "trainElapsedS": round(time.perf_counter() - t0, 2),
        "validation": {
            "greedy_earliest": val_greedy,
            "combinatorial_wta": val_comb,
            "ai_policy": val_ai,
        },
        "history": history,
    }

    out_path.parent.mkdir(parents=True, exist_ok=True)
    np.savez(
        out_path,
        weights=best_w,
        bias=np.array(0.0, dtype=np.float64),
        threshold=np.array(0.60, dtype=np.float64),
        metadata_json=np.array(json.dumps(meta)),
    )
    return meta
