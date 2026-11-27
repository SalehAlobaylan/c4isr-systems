"""Pure-NumPy Sub-Millisecond ML Debris Footprint Surrogate (12 -> 48 -> 48 -> 5 MLP).

Trained directly on headless EMAD 3-DOF wind-drifted shrapnel physics samples
across verified Saudi biomes, serialized to ``models/debris_surrogate_v1.npz``.
Provides < 0.5 ms debris drift & urban/WorldPop exposure predictions for live
Safe-Window & Salvo Combinatorial WTA scoring, always tagged as
``derived/ml-debris-surrogate-v1``.
"""
from __future__ import annotations

import json
import math
from pathlib import Path
import time
from typing import Any

import numpy as np

from sim.contracts import ShrapnelConfig
from sim.physics import simulate_shrapnel_cloud
from sim_core.atmosphere import AtmosphereModel, AtmosphereOverrideConfig
from sim_core.environment import EnvironmentModel
from sim_core.registry import discover_biomes

REPO_ROOT = Path(__file__).resolve().parents[1]
DEFAULT_SURROGATE_PATH = REPO_ROOT / "models" / "debris_surrogate_v1.npz"

FEATURE_DIM = 12
TARGET_DIM = 5  # [drift_e_km, drift_n_km, spread_km, urban_frac, pop_norm]


def _local_context_features(env: EnvironmentModel, east: float, north: float, agl_m: float, wind_en: tuple[float, float], vel_enu: np.ndarray) -> np.ndarray:
    """Compute 12 normalized kinematic + atmospheric + downwind land-use features."""
    fall_est = math.sqrt(2.0 * max(agl_m, 20.0) / 9.81) * 1.4
    pred_e = east + 0.35 * float(vel_enu[0]) * min(fall_est, 12.0) + 0.65 * wind_en[0] * fall_est
    pred_n = north + 0.35 * float(vel_enu[1]) * min(fall_est, 12.0) + 0.65 * wind_en[1] * fall_est

    # Fast 9-point probe on cached ObstructionGrid & WorldPop around predicted downwind landing zone
    obs = env.obstruction_grid()
    r = max(120.0, min(420.0, 0.25 * agl_m + 100.0))
    offsets = np.array([[0, 0], [r, 0], [-r, 0], [0, r], [0, -r], [r * 0.7, r * 0.7], [-r * 0.7, r * 0.7], [r * 0.7, -r * 0.7], [-r * 0.7, -r * 0.7]])
    pe = pred_e + offsets[:, 0]
    pn = pred_n + offsets[:, 1]
    surf, inside = obs.sample(pe, pn, buildings=True)
    terr, _ = obs.sample(pe, pn, buildings=False)
    bldg_dens = float(np.mean(inside & ((surf - terr) > 1.5)))
    pop_mean = float(np.mean(env.population_at(pe, pn))) / 100.0  # normalized residents per 100m cell

    spd = float(np.linalg.norm(vel_enu))
    w_mag = math.hypot(wind_en[0], wind_en[1])
    return np.array(
        [
            agl_m / 1000.0,
            spd / 300.0,
            float(vel_enu[0]) / 300.0,
            float(vel_enu[1]) / 300.0,
            float(vel_enu[2]) / 200.0,
            wind_en[0] / 25.0,
            wind_en[1] / 25.0,
            w_mag / 25.0,
            bldg_dens,
            min(3.0, pop_mean),
            fall_est / 25.0,
            1.0 if bldg_dens > 0.25 else 0.0,
        ],
        dtype=np.float64,
    )


class DebrisSurrogate:
    """Pure-NumPy 2-layer ReLU MLP debris footprint surrogate (< 0.3 ms inference)."""

    def __init__(self, weights: dict[str, Any]) -> None:
        self.x_mean = np.asarray(weights["x_mean"], dtype=np.float64)
        self.x_std = np.maximum(np.asarray(weights["x_std"], dtype=np.float64), 1e-6)
        self.W1 = np.asarray(weights["W1"], dtype=np.float64)
        self.b1 = np.asarray(weights["b1"], dtype=np.float64)
        self.W2 = np.asarray(weights["W2"], dtype=np.float64)
        self.b2 = np.asarray(weights["b2"], dtype=np.float64)
        self.W3 = np.asarray(weights["W3"], dtype=np.float64)
        self.b3 = np.asarray(weights["b3"], dtype=np.float64)
        meta_raw = weights.get("metadata_json")
        self.metadata: dict[str, Any] = json.loads(str(meta_raw)) if meta_raw is not None else {}

    def forward_batch(self, X: np.ndarray) -> np.ndarray:
        xn = (X - self.x_mean) / self.x_std
        h1 = np.maximum(0.0, xn @ self.W1 + self.b1)
        h2 = np.maximum(0.0, h1 @ self.W2 + self.b2)
        return h2 @ self.W3 + self.b3

    def predict_exposure(
        self,
        *,
        intercept_enu: np.ndarray,
        threat_vel_enu: np.ndarray,
        wind_en: tuple[float, float],
        agl_m: float,
        env: EnvironmentModel,
    ) -> dict[str, Any]:
        t0 = time.perf_counter()
        feat = _local_context_features(env, float(intercept_enu[0]), float(intercept_enu[1]), float(agl_m), wind_en, np.asarray(threat_vel_enu, dtype=np.float64))
        out = self.forward_batch(feat[None, :])[0]
        dt_ms = (time.perf_counter() - t0) * 1000.0
        drift_e_m = float(out[0]) * 1000.0
        drift_n_m = float(out[1]) * 1000.0
        spread_m = max(40.0, float(out[2]) * 1000.0)
        urban_frac = float(np.clip(out[3], 0.0, 1.0))
        pop_exp = max(0.0, float(out[4]) * 40.0)
        risk = float(np.clip(0.65 * urban_frac + 0.35 * min(1.0, pop_exp / 35.0), 0.0, 1.0))
        return {
            "derivation": "derived/ml-debris-surrogate-v1",
            "driftEnuM": [round(drift_e_m, 1), round(drift_n_m, 1)],
            "driftDistanceM": round(math.hypot(drift_e_m, drift_n_m), 1),
            "spreadRadiusM": round(spread_m, 1),
            "urbanExposureFraction": round(urban_frac, 4),
            "estimatedPopulationExposed": round(pop_exp, 1),
            "riskScore": round(risk, 4),
            "inferenceMs": round(dt_ms, 3),
        }


_CACHED_SURROGATE: DebrisSurrogate | None = None


def load_debris_surrogate(path: Path | None = None) -> DebrisSurrogate | None:
    global _CACHED_SURROGATE
    target = path or DEFAULT_SURROGATE_PATH
    if not target.is_file():
        return None
    if _CACHED_SURROGATE is not None and _CACHED_SURROGATE.metadata.get("_path") == str(target):
        return _CACHED_SURROGATE
    raw = np.load(target, allow_pickle=False)
    d = {k: raw[k] for k in raw.files}
    model = DebrisSurrogate(d)
    model.metadata["_path"] = str(target)
    _CACHED_SURROGATE = model
    return model


def train_debris_surrogate(
    *,
    out_path: Path = DEFAULT_SURROGATE_PATH,
    n_samples: int = 320,
    epochs: int = 380,
    seed: int = 20261127,
) -> dict[str, Any]:
    """Generate physics training samples on verified EMAD biomes and train the pure-NumPy MLP."""
    t0 = time.perf_counter()
    rng = np.random.default_rng(seed)
    biomes = discover_biomes(REPO_ROOT)
    # Prefer Riyadh pilot + owned fixture for fast diverse urban/rural sampling
    chosen = [b for b in biomes if b.id in ("riyadh-observation-v1", "asir-highlands-30km-v1", "owned-fixture-v1")]
    if not chosen:
        chosen = biomes[:1]

    envs = [
        EnvironmentModel(
            b.package_path,
            expected_manifest_sha256=b.expected_manifest_sha256,
            ghs_obat_path=b.ghs_obat_path,
            worldpop_path=b.worldpop_path,
        )
        for b in chosen
    ]
    base_atms = [AtmosphereModel.load(e, external_path=b.weather_field_path) for b, e in zip(chosen, envs)]

    X_list = []
    Y_list = []
    sh_cfg = ShrapnelConfig(fragment_count=48, dt_s=0.2, max_flight_s=100.0)

    for i in range(n_samples):
        ei = i % len(envs)
        env = envs[ei]
        span_e = env.east_max - env.east_min
        span_n = env.north_max - env.north_min
        pe = float(rng.uniform(env.east_min + 0.15 * span_e, env.east_max - 0.15 * span_e))
        pn = float(rng.uniform(env.north_min + 0.15 * span_n, env.north_max - 0.15 * span_n))
        g = env.height_at(pe, pn, fallback=env.up_median)
        agl = float(rng.uniform(80.0, 1400.0))
        p_int = np.array([pe, pn, g + agl], dtype=np.float64)

        spd = float(rng.uniform(45.0, 340.0))
        az = float(rng.uniform(0.0, 2.0 * math.pi))
        vz = float(rng.uniform(-90.0, 30.0))
        vel = np.array([spd * math.sin(az), spd * math.cos(az), vz], dtype=np.float64)

        add_e = float(rng.uniform(-16.0, 16.0))
        add_n = float(rng.uniform(-16.0, 16.0))
        atm = base_atms[ei].with_perturbation((add_e, add_n), scale=float(rng.uniform(0.6, 1.8)))
        we, wn, _, _, _ = atm.sample(np.array([pe]), np.array([pn]), np.array([g + agl]), ground_up_m=np.array([g]))
        wind_en = (float(we[0]), float(wn[0]))

        res = simulate_shrapnel_cloud(p_int, vel, sh_cfg, env, atm, seed=seed + i * 31, record_trails=False)
        fp = res["footprint"]
        exp = res["exposure"]
        drift_e = float(fp["centroidEnu"][0]) - pe
        drift_n = float(fp["centroidEnu"][1]) - pn
        spread_m = 0.5 * (float(fp["ellipse90SemiMajorM"]) + float(fp["ellipse90SemiMinorM"]))
        urb = float(exp["urbanExposureFraction"])
        pop = float(exp.get("estimatedPopulationExposed") or 0.0)

        X_list.append(_local_context_features(env, pe, pn, agl, wind_en, vel))
        Y_list.append(np.array([drift_e / 1000.0, drift_n / 1000.0, spread_m / 1000.0, urb, min(2.0, pop / 40.0)], dtype=np.float64))

    X = np.stack(X_list)
    Y = np.stack(Y_list)
    perm = rng.permutation(len(X))
    n_train = int(0.8 * len(X))
    tr_idx, va_idx = perm[:n_train], perm[n_train:]

    X_tr, Y_tr = X[tr_idx], Y[tr_idx]
    X_va, Y_va = X[va_idx], Y[va_idx]
    x_mean = X_tr.mean(axis=0)
    x_std = np.maximum(X_tr.std(axis=0), 1e-6)
    Xn_tr = (X_tr - x_mean) / x_std
    Xn_va = (X_va - x_mean) / x_std

    # 12 -> 48 -> 48 -> 5 ReLU MLP with Adam optimizer + L2 decay
    h_dim = 48
    params = {
        "W1": rng.normal(0.0, math.sqrt(2.0 / FEATURE_DIM), size=(FEATURE_DIM, h_dim)),
        "b1": np.zeros(h_dim),
        "W2": rng.normal(0.0, math.sqrt(2.0 / h_dim), size=(h_dim, h_dim)),
        "b2": np.zeros(h_dim),
        "W3": rng.normal(0.0, math.sqrt(2.0 / h_dim), size=(h_dim, TARGET_DIM)),
        "b3": np.zeros(TARGET_DIM),
    }
    m = {k: np.zeros_like(v) for k, v in params.items()}
    v = {k: np.zeros_like(v) for k, v in params.items()}
    lr = 0.008
    l2 = 1e-4
    batch = 32
    step_t = 0

    for _ in range(epochs):
        order = rng.permutation(n_train)
        for s in range(0, n_train, batch):
            bi = order[s : s + batch]
            xb, yb = Xn_tr[bi], Y_tr[bi]
            nb = len(bi)
            z1 = xb @ params["W1"] + params["b1"]
            a1 = np.maximum(0.0, z1)
            z2 = a1 @ params["W2"] + params["b2"]
            a2 = np.maximum(0.0, z2)
            pred = a2 @ params["W3"] + params["b3"]

            diff = (2.0 / nb) * (pred - yb)
            grads = {}
            grads["W3"] = a2.T @ diff + l2 * params["W3"]
            grads["b3"] = diff.sum(axis=0)
            da2 = diff @ params["W3"].T
            dz2 = da2 * (z2 > 0.0)
            grads["W2"] = a1.T @ dz2 + l2 * params["W2"]
            grads["b2"] = dz2.sum(axis=0)
            da1 = dz2 @ params["W2"].T
            dz1 = da1 * (z1 > 0.0)
            grads["W1"] = xb.T @ dz1 + l2 * params["W1"]
            grads["b1"] = dz1.sum(axis=0)

            step_t += 1
            for k in params:
                m[k] = 0.9 * m[k] + 0.1 * grads[k]
                v[k] = 0.999 * v[k] + 0.001 * (grads[k] ** 2)
                mh = m[k] / (1.0 - 0.9 ** step_t)
                vh = v[k] / (1.0 - 0.999 ** step_t)
                params[k] -= lr * mh / (np.sqrt(vh) + 1e-8)

    # Validation metrics
    model = DebrisSurrogate({"x_mean": x_mean, "x_std": x_std, **params})
    va_pred = model.forward_batch(X_va)
    ss_res = np.sum((Y_va - va_pred) ** 2, axis=0)
    ss_tot = np.maximum(np.sum((Y_va - Y_va.mean(axis=0)) ** 2, axis=0), 1e-9)
    r2_per_target = 1.0 - ss_res / ss_tot
    drift_mae_m = float(np.mean(np.hypot((Y_va[:, 0] - va_pred[:, 0]) * 1000.0, (Y_va[:, 1] - va_pred[:, 1]) * 1000.0)))
    urban_mae = float(np.mean(np.abs(Y_va[:, 3] - np.clip(va_pred[:, 3], 0.0, 1.0))))

    # Measure inference speed
    t_inf0 = time.perf_counter()
    for _ in range(100):
        model.forward_batch(X_va[:1])
    inf_ms = ((time.perf_counter() - t_inf0) / 100.0) * 1000.0

    meta = {
        "schema": "emad.debris-surrogate.v1",
        "derivation": "derived/ml-debris-surrogate-v1",
        "architecture": f"{FEATURE_DIM}->{h_dim}->{h_dim}->{TARGET_DIM} ReLU MLP (pure NumPy)",
        "biomesUsed": [b.id for b in chosen],
        "nSamples": n_samples,
        "trainSamples": n_train,
        "valSamples": len(va_idx),
        "epochs": epochs,
        "valR2DriftE": round(float(r2_per_target[0]), 4),
        "valR2DriftN": round(float(r2_per_target[1]), 4),
        "valR2Spread": round(float(r2_per_target[2]), 4),
        "valR2Urban": round(float(r2_per_target[3]), 4),
        "valDriftMaeM": round(drift_mae_m, 1),
        "valUrbanMae": round(urban_mae, 4),
        "inferenceMs": round(inf_ms, 3),
        "trainElapsedS": round(time.perf_counter() - t0, 2),
    }

    out_path.parent.mkdir(parents=True, exist_ok=True)
    np.savez(
        out_path,
        x_mean=x_mean,
        x_std=x_std,
        W1=params["W1"],
        b1=params["b1"],
        W2=params["W2"],
        b2=params["b2"],
        W3=params["W3"],
        b3=params["b3"],
        metadata_json=np.array(json.dumps(meta)),
    )
    global _CACHED_SURROGATE
    _CACHED_SURROGATE = None
    return meta
