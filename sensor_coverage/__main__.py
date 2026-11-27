"""Module entry point: ``python -m sensor_coverage ...``."""
import sys

from .cli import main

if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
