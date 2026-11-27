"""CLI entrypoint for the unified EMAD Simulation Compute Hub (``python -m sim_core``)."""
from __future__ import annotations

import argparse
from pathlib import Path

from .hub import serve_hub


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(
        prog="python -m sim_core",
        description="EMAD Unified Simulation Compute Hub (Sim + Coverage + Salvo + Bench in one shared-cache process).",
    )
    sub = parser.add_subparsers(dest="cmd", required=True)
    svc = sub.add_parser("service", help="Start the unified loopback compute hub")
    svc.add_argument("--host", default="127.0.0.1", help="Bind host (default: 127.0.0.1)")
    svc.add_argument("--port", type=int, default=8765, help="Bind port (default: 8765)")
    svc.add_argument("--runs-base", default=None, help="Override base directory for run artifacts")
    svc.add_argument("--max-biomes", type=int, default=None, help="LRU cap on heavy loaded biomes in RAM")
    svc.add_argument("--standalone", action="store_true", help="Disable worker-secret requirement for local standalone dev")

    args = parser.parse_args(argv)
    if args.cmd == "service":
        serve_hub(
            host=args.host,
            port=args.port,
            runs_base=Path(args.runs_base) if args.runs_base else None,
            max_loaded_biomes=args.max_biomes,
            worker_secret="" if args.standalone else None,
        )
        return 0
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
