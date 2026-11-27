"""Command-line entry point for EMAD Multi-Threat Salvo & Layered Air Defense."""
from __future__ import annotations

import argparse
from pathlib import Path
import sys

from .viewer.server import serve_service


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(prog="python -m salvo_defense.cli", description=__doc__)
    sub = parser.add_subparsers(dest="cmd", required=True)
    for name in ("serve", "service"):
        srv = sub.add_parser(name, help="Run the stateful local Salvo Defense HTTP service & 3D viewer")
        srv.add_argument("--host", default="127.0.0.1")
        srv.add_argument("--port", type=int, default=8767)
        srv.add_argument("--runs-dir", default=None)

    args = parser.parse_args(argv)
    if args.cmd in ("serve", "service"):
        serve_service(host=args.host, port=args.port, runs_root=Path(args.runs_dir) if args.runs_dir else None)
        return 0
    return 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
