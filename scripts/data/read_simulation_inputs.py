"""Read-only command line for prepared civilian simulation inputs."""

import argparse
from hashlib import sha256
import json
from pathlib import Path
import sys

import simulation_inputs as inputs


def parse_args():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)

    def command(name, description):
        sub = commands.add_parser(name, help=description)
        sub.add_argument("input", type=Path, help="local input file")
        sub.add_argument("--sha256", help="trusted SHA-256 of the input file")
        return sub

    command("municipal", "Riyadh 940 workload and retrospective rows").add_argument("--rows", action="store_true")
    roads = command("road-access", "verified access through a reviewed directed graph")
    roads.add_argument("--known-at", required=True, type=inputs.instant)
    roads.add_argument("--origin", required=True)
    command("population", "conservative aggregation of native population cells")
    command("population-raster", "read selected native GeoTIFF cells using reviewed allocations").add_argument(
        "--allocations", required=True, type=Path)
    images = command("image-pair", "select reviewed before and after scenes")
    images.add_argument("--event", required=True, type=inputs.instant)
    images.add_argument("--known-at", required=True, type=inputs.instant)
    images.add_argument("--min-usable", required=True, type=float)
    energy = command("flight-energy", "measure energy in one completed AirLab research log")
    energy.add_argument("--max-gap-seconds", required=True, type=float)
    command("flood-review", "score a held-out civil flood inspection review")
    cems = command("cems-products", "review delivered Copernicus EMS public flood product metadata")
    cems.add_argument("--activation", required=True)
    cems.add_argument("--known-at", required=True, type=inputs.instant)
    outages = command("radar-outages", "summarize historical country-level Internet outage evidence")
    outages.add_argument("--country", required=True, type=str.upper)
    outages.add_argument("--from", required=True, dest="start", type=inputs.instant)
    outages.add_argument("--to-exclusive", required=True, type=inputs.instant)
    ports = command("port-activity", "daily PortWatch activity with a frozen weekday baseline")
    ports.add_argument("--port", required=True)
    ports.add_argument("--start", required=True, type=inputs.day)
    ports.add_argument("--end-exclusive", required=True, type=inputs.day)
    ports.add_argument("--minimum-history", type=int, default=4)
    ports.add_argument("--consecutive", type=int, default=3)
    command("regional-indicators", "check reviewed regional indicators for consistent reporting grain")
    command("notices", "replay civil service notices at an evidence cutoff").add_argument(
        "--known-at", required=True, type=inputs.instant)
    command("delivery-replay", "measure reporting gaps across exercise desks").add_argument(
        "--known-at", required=True, type=inputs.instant)
    command("regional-queue", "compare authored staffing against the same arrivals").add_argument(
        "--known-at", required=True, type=inputs.instant)
    return parser.parse_args()


def consume(args, raw):
    if args.command == "municipal":
        return inputs.municipal(raw, args.rows)
    if args.command == "flight-energy":
        return inputs.airlab_energy(raw, args.max_gap_seconds)
    if args.command == "population-raster":
        return inputs.population_raster(raw, args.allocations.read_bytes())
    document = json.loads(raw)
    handlers = {
        "road-access": lambda: inputs.road_access(document, args.known_at, args.origin),
        "population": lambda: inputs.population_access(document),
        "image-pair": lambda: inputs.paired_images(document, args.event, args.known_at, args.min_usable),
        "flood-review": lambda: inputs.floodnet_scores(document),
        "cems-products": lambda: inputs.cems_products(document, args.activation, args.known_at),
        "radar-outages": lambda: inputs.radar_outages(document, args.country, args.start, args.to_exclusive),
        "port-activity": lambda: inputs.port_activity(
            document, args.port, args.start, args.end_exclusive, args.minimum_history, args.consecutive),
        "regional-indicators": lambda: inputs.regional_indicators(document),
        "notices": lambda: inputs.notices_at(document, args.known_at),
        "delivery-replay": lambda: inputs.delivery_replay(document, args.known_at),
        "regional-queue": lambda: inputs.regional_queue(document, args.known_at),
    }
    return handlers[args.command]()


def main():
    args = parse_args()
    raw = args.input.read_bytes()
    digest = sha256(raw).hexdigest()
    inputs.require(args.sha256 is None or args.sha256.lower() == digest, "input SHA-256 mismatch")
    result = consume(args, raw)
    document = {"schema": "emad.simulation-input-analysis.v1", "operation": args.command,
                "inputSha256": digest, "result": result}
    print(json.dumps(document, sort_keys=True, ensure_ascii=True, allow_nan=False))


if __name__ == "__main__":
    try:
        main()
    except (ImportError, OSError, OverflowError, TypeError, ValueError, KeyError, AttributeError) as exc:
        sys.exit(f"Simulation input refused: {exc}")
