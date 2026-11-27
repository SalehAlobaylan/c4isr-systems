"""CLI for EMAD Monte Carlo Debris, ML Surrogate Training, RL WTA Training & Benchmarks."""
from __future__ import annotations

import argparse
import json
import sys

from .benchmark import run_benchmark_suite
from .server import serve_service
from .surrogate import train_debris_surrogate
from .train_wta import train_ai_wta_policy


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(prog="python -m sim_bench", description=__doc__)
    sub = parser.add_subparsers(dest="cmd", required=True)

    for name in ("serve", "service"):
        srv = sub.add_parser(name, help="Start the local Benchmark & Monte Carlo HTTP service")
        srv.add_argument("--host", default="127.0.0.1")
        srv.add_argument("--port", type=int, default=8768)

    tr_s = sub.add_parser("train-surrogate", help="Train pure-NumPy ML debris footprint surrogate")
    tr_s.add_argument("--samples", type=int, default=320)
    tr_s.add_argument("--epochs", type=int, default=380)

    tr_w = sub.add_parser("train-wta", help="Train observation-only CEM RL WTA policy")
    tr_w.add_argument("--generations", type=int, default=8)
    tr_w.add_argument("--population", type=int, default=12)

    bm = sub.add_parser("benchmark", help="Run frozen multi-biome benchmark leaderboard")
    bm.add_argument("--biomes", nargs="*", default=None)

    args = parser.parse_args(argv)
    if args.cmd in ("serve", "service"):
        serve_service(host=args.host, port=args.port)
        return 0
    if args.cmd == "train-surrogate":
        meta = train_debris_surrogate(n_samples=args.samples, epochs=args.epochs)
        print(json.dumps(meta, indent=2))
        return 0
    if args.cmd == "train-wta":
        meta = train_ai_wta_policy(generations=args.generations, population=args.population)
        print(json.dumps(meta, indent=2))
        return 0
    if args.cmd == "benchmark":
        rep = run_benchmark_suite(env_ids=args.biomes)
        print(json.dumps(rep["leaderboard"], indent=2))
        return 0
    return 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
