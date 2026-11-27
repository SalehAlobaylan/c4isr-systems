"""Command-line interface for the EMAD Sensor Placement & Blind-Spot Optimizer."""
from __future__ import annotations

import argparse
import json
from pathlib import Path
import sys
from typing import Any

from .service import CoverageService


def _print_summary(job: dict[str, Any]) -> None:
    if job.get("status") != "completed":
        print(f"Run {job.get('jobId')} ended with status={job.get('status')}: {job.get('error')}")
        return
    s = job["summary"]
    base = s["baseline"]
    opt = s.get("optimized")
    cmp_ = s.get("comparison") or {}
    print(f"\n=== Sensor Coverage Study: {job['title']} ({job['jobId']}) ===")
    print(f"  Environment: {job['environmentTitle']} | elapsed: {s['elapsedS']}s")
    print(
        f"  Baseline @ {base['aglM']:.0f}m AGL: coverage={base['coverageFraction']*100:.1f}% "
        f"(weighted {base['weightedCoverageFraction']*100:.1f}%) | "
        f"blind={base['blindAreaKm2']:.2f} km² | objective={base['objective']:.4f} | "
        f"min warning={base['minWarningTimeS']}s"
    )
    if opt:
        gain = cmp_.get("objectiveGainPct")
        gain_txt = f"{gain:+.1f}%" if gain is not None else "n/a"
        print(
            f"  Optimized @ {opt['aglM']:.0f}m AGL: coverage={opt['coverageFraction']*100:.1f}% "
            f"(weighted {opt['weightedCoverageFraction']*100:.1f}%) | "
            f"blind={opt['blindAreaKm2']:.2f} km² | objective={opt['objective']:.4f} ({gain_txt}) | "
            f"min warning={opt['minWarningTimeS']}s"
        )
    for r in (opt or base).get("routes") or []:
        print(f"  Route [{r['label']}]: warning={r['warningTimeS']}s | tracked={r['trackedFraction']*100:.1f}%")


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(
        prog="python -m sensor_coverage",
        description="EMAD 3D Sensor Placement & Blind-Spot Optimizer CLI",
    )
    sub = parser.add_subparsers(dest="cmd", required=True)

    p_env = sub.add_parser("environments", help="List discovered verified EMAD environments")
    p_env.add_argument("--json", action="store_true", help="Output raw JSON")

    p_run = sub.add_parser("run", help="Run a deterministic baseline + optimized coverage study")
    p_run.add_argument("--env", default=None, help="Environment ID (default: first discovered)")
    p_run.add_argument("--config", type=Path, default=None, help="Optional JSON draft payload overriding defaults")
    p_run.add_argument("--out", type=Path, default=None, help="Optional output directory for replay.json")
    p_run.add_argument("--no-optimize", action="store_true", help="Evaluate baseline only")
    p_run.add_argument("--extra-sensors", type=int, default=None, help="Add N extra sensors during optimization")
    p_run.add_argument("--visibility-km", type=float, default=None, help="Override EO/IR visibility (km)")

    p_svc = sub.add_parser("service", help="Run the standalone Sensor Coverage HTTP service + 3D viewer")
    p_svc.add_argument("--host", default="127.0.0.1")
    p_svc.add_argument("--port", type=int, default=8766)
    p_svc.add_argument("--runs-dir", type=Path, default=None, help="Directory for persisted coverage runs")

    p_srv = sub.add_parser("serve", help="Serve one static coverage replay bundle locally")
    p_srv.add_argument("run_dir", type=Path, help="Directory containing replay.json")
    p_srv.add_argument("--glb", type=Path, default=None, help="Optional verified EMAD terrain GLB")
    p_srv.add_argument("--imagery", type=Path, default=None, help="Optional Sentinel-2 optical imagery JSON")
    p_srv.add_argument("--host", default="127.0.0.1")
    p_srv.add_argument("--port", type=int, default=8766)

    args = parser.parse_args(argv)

    if args.cmd == "environments":
        svc = CoverageService()
        envs = svc.list_environments()
        if args.json:
            print(json.dumps(envs, indent=2))
        else:
            for e in envs:
                print(
                    f"- {e['id']}: {e['title']} "
                    f"(buildings={e['buildingCount']}, roads={e['roadCount']}, glb={e['hasGlb']})"
                )
        return 0

    if args.cmd == "run":
        runs_root = args.out.resolve() if args.out else None
        svc = CoverageService(runs_root=runs_root) if runs_root else CoverageService()
        envs = svc.list_environments()
        if not envs:
            print("No verified EMAD environments discovered.", file=sys.stderr)
            return 1
        chosen = next((e for e in envs if e["id"] == args.env), envs[0]) if args.env else envs[0]
        payload = json.loads(args.config.read_text(encoding="utf-8")) if args.config else dict(chosen["defaults"])
        if args.env:
            payload["environmentId"] = chosen["id"]
        if args.no_optimize:
            payload.setdefault("optimizer", {})["enabled"] = False
        if args.extra_sensors is not None:
            payload.setdefault("optimizer", {})["extraSensors"] = int(args.extra_sensors)
        if args.visibility_km is not None:
            payload.setdefault("weather", {})["visibilityKm"] = float(args.visibility_km)
        job = svc.start_run(payload, sync=True)
        _print_summary(job)
        print(f"\nSaved replay bundle: {svc.runs_root / job['jobId'] / 'replay.json'}")
        return 0 if job.get("status") == "completed" else 1

    if args.cmd == "service":
        from .viewer.server import serve_service

        serve_service(host=args.host, port=args.port, runs_root=args.runs_dir)
        return 0

    if args.cmd == "serve":
        from .viewer.server import serve_replay

        serve_replay(args.run_dir, glb_path=args.glb, imagery_path=args.imagery, host=args.host, port=args.port)
        return 0

    return 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
