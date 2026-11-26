"""Command-line interface for running, comparing, and viewing EMAD 3D simulations."""
from __future__ import annotations

import argparse
from dataclasses import replace
import json
from pathlib import Path
import sys

from .contracts import ScenarioSpec
from .engine import compare_policies, run_simulation, save_run_bundle


def _load_scenario(path: Path, args: argparse.Namespace) -> ScenarioSpec:
    raw = json.loads(path.read_text(encoding="utf-8"))
    spec = ScenarioSpec.from_dict(raw)
    if getattr(args, "policy", None):
        spec = spec.with_policy(args.policy)
    if getattr(args, "seed", None) is not None:
        spec = replace(spec, seed=int(args.seed))
    if getattr(args, "wind_scale", None) is not None:
        spec = replace(spec, atmosphere=replace(spec.atmosphere, wind_scale=float(args.wind_scale)))
    if getattr(args, "wind_add", None) is not None:
        parts = [float(x) for x in args.wind_add.split(",")]
        if len(parts) != 3:
            raise ValueError("--wind-add requires 'east,north,up' in m/s")
        spec = replace(spec, atmosphere=replace(spec.atmosphere, wind_add_enu_mps=parts))
    return spec


def _print_eval_table(runs: list[dict], comparison: dict | None) -> None:
    for r in runs:
        ev = r["evaluation"]
        sens = ev["sensing"]
        intr = ev["interception"]
        exp = ev["shrapnelExposure"] or {}
        fp = ev["shrapnelFootprint"] or {}
        print(f"\n=== Policy: {ev['policy']} ({r['runId']}) ===")
        print(
            f"  Sensing:      scans={sens['totalScansBeforeEnd']} | "
            f"LOS clear={sens['losClearScans']} (terrain blocked={sens['terrainBlockedScans']}, "
            f"bldg blocked={sens['buildingBlockedScans']}) | detections={sens['detectionsCount']} | "
            f"firm track={sens['firmTrackEstablishedS']}s | mean track err={sens['meanTrackErrorM']}m"
        )
        if intr["intercepted"]:
            print(
                f"  Interception: HIT at t={intr['interceptTimeS']}s (launch={intr['launchTimeS']}s) | "
                f"alt={intr['interceptAglM']}m AGL | protected margin={intr['protectedMarginM']}m"
            )
            print(
                f"  Shrapnel:     {exp['total']} frags | building={exp['buildingHits']} | "
                f"road={exp['roadHits']} | protected={exp['protectedZoneHits']} | "
                f"open={exp['openHits']} | urban exposure={exp['urbanExposureFraction']*100:.1f}% | "
                f"wind drift={fp['driftDistanceM']}m | 90% ellipse={fp['ellipse90SemiMajorM']:.0f}x{fp['ellipse90SemiMinorM']:.0f}m"
            )
        else:
            print(f"  Interception: MISSED ({intr.get('reason')})")
    if comparison and comparison.get("safeWindowVsEarliest"):
        d = comparison["safeWindowVsEarliest"]
        print(
            f"\n>>> Safe-Window vs Earliest: building hits {d['buildingHitsDelta']:+d}, "
            f"road hits {d['roadHitsDelta']:+d}, urban exposure reduction {d['urbanExposureReductionPct']:+.1f}%"
        )


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(prog="python -m sim", description="EMAD 3D Simulation CLI")
    sub = parser.add_subparsers(dest="cmd", required=True)

    p_run = sub.add_parser("run", help="Run a single deterministic simulation")
    p_run.add_argument("scenario", type=Path)
    p_run.add_argument("--out", type=Path, required=True)
    p_run.add_argument("--policy", choices=["earliest", "safe_window"])
    p_run.add_argument("--seed", type=int)
    p_run.add_argument("--wind-scale", type=float)
    p_run.add_argument("--wind-add", help="Additive wind vector 'east,north,up' in m/s")

    p_cmp = sub.add_parser("compare", help="Compare earliest vs safe_window policies on identical frozen environment")
    p_cmp.add_argument("scenario", type=Path)
    p_cmp.add_argument("--out", type=Path, required=True)
    p_cmp.add_argument("--seed", type=int)
    p_cmp.add_argument("--wind-scale", type=float)
    p_cmp.add_argument("--wind-add", help="Additive wind vector 'east,north,up' in m/s")

    p_srv = sub.add_parser("serve", help="Serve one static simulation replay bundle locally")
    p_srv.add_argument("run_dir", type=Path, help="Output directory containing replay.json")
    p_srv.add_argument("--glb", type=Path, help="Optional verified EMAD terrain GLB file")
    p_srv.add_argument("--imagery", type=Path, help="Optional verified EMAD optical imagery JSON file")
    p_srv.add_argument("--port", type=int, default=8765)
    p_srv.add_argument("--host", default="127.0.0.1")

    p_svc = sub.add_parser("service", help="Run the interactive multi-run simulation service + 3D replay viewer")
    p_svc.add_argument("--port", type=int, default=8765)
    p_svc.add_argument("--host", default="127.0.0.1")
    p_svc.add_argument("--runs-dir", type=Path, help="Directory for persisted simulation runs (default: tmp/sim-runs)")

    args = parser.parse_args(argv)

    if args.cmd == "run":
        spec = _load_scenario(args.scenario, args)
        run = run_simulation(spec, base_dir=args.scenario.parent)
        replay_path = save_run_bundle([run], args.out)
        _print_eval_table([run], None)
        print(f"\nSaved artifacts & 3D replay to: {replay_path}")
        return 0

    if args.cmd == "compare":
        spec = _load_scenario(args.scenario, args)
        runs, comparison = compare_policies(spec, base_dir=args.scenario.parent)
        replay_path = save_run_bundle(runs, args.out, comparison_summary=comparison)
        _print_eval_table(runs, comparison)
        print(f"\nSaved comparison & 3D replay to: {replay_path}")
        return 0

    if args.cmd == "serve":
        from .viewer.server import serve_replay

        serve_replay(
            args.run_dir,
            glb_path=args.glb,
            imagery_path=args.imagery,
            host=args.host,
            port=args.port,
        )
        return 0

    if args.cmd == "service":
        from .viewer.server import serve_service

        serve_service(host=args.host, port=args.port, runs_root=args.runs_dir)
        return 0

    return 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
