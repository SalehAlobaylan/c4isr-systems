"""Score an offline civilian drifter search exercise using withheld observations."""

import argparse
import csv
import hashlib
import json
import math
from pathlib import Path
from statistics import median
import sys

from simulation_inputs import instant, number, require, text, utc


def compile_drifters(capture, layout):
    """Prepare separate exercise conditions and blind cells from a pinned NOAA QC extract."""
    doc = json.loads(layout.read_bytes())
    raw = capture.read_bytes()
    digest = hashlib.sha256(raw).hexdigest()
    require(digest == doc["captureSha256"], "NOAA extract differs from its reviewed digest")
    reader = csv.DictReader(raw.decode("utf-8-sig").splitlines())
    require(reader.fieldnames == ["ID", "time", "latitude", "longitude", "ve", "vn", "sst"], "unexpected NOAA extract schema")
    units = next(reader)
    require([units[k] for k in reader.fieldnames] == ["", "UTC", "degrees_north", "degrees_east", "m/s", "m/s", "Kelvin"],
            "unexpected NOAA extract units")
    entries = {}
    for row in reader:
        identity, at = row["ID"], instant(row["time"])
        lat, lon = number(float(row["latitude"])), number(float(row["longitude"]))
        require(identity.isdigit() and (identity, at) not in entries and -90 <= lat <= 90 and -180 <= lon <= 180,
                "invalid or repeated NOAA observation")
        entries[identity, at] = (lat, lon, row)
    require(entries, "no observations in the retained NOAA extract")

    cells = []
    for item in doc["cells"]:
        west, south, east, north = map(number, item["bounds"])
        require(-180 <= west < east <= 180 and -90 <= south < north <= 90, "invalid search grid bounds")
        identity = text(item["id"])
        for earlier in cells:
            w, s, e, n = earlier["bounds"]
            require(earlier["id"] != identity and (east <= w or e <= west or north <= s or n <= south),
                    "search grid cells overlap or repeat")
        area = 6371.0088**2 * math.radians(east-west) * (math.sin(math.radians(north))-math.sin(math.radians(south)))
        cells.append({"id": identity, "areaKm2": round(area, 6), "bounds": (west, south, east, north)})

    def cell_at(lat, lon):
        options = [item["id"] for item in cells if item["bounds"][0] <= lon < item["bounds"][2] and item["bounds"][1] <= lat < item["bounds"][3]]
        require(len(options) == 1, "selected NOAA position lies outside the grid or in overlapping cells")
        return options[0]

    exercise = {"sourceSha256": digest, "source": "NOAA AOML Global Drifter Program hourly QC, interpolated",
                "cells": [{"id": c["id"], "areaKm2": c["areaKm2"]} for c in cells], "cases": []}
    answers = {"cases": []}
    groups = {}
    for case in doc["cases"]:
        identity, group = case["providerId"], text(case["observationGroup"])
        split, start, end = case["split"], instant(case["from"]), instant(case["cutoff"])
        condition_at, blind_from = instant(case["conditionAt"]), instant(case["blindFrom"])
        require(split in {"test", "development"} and (identity not in groups or groups[identity] == (group, split)) and
                start <= condition_at < blind_from < end, "case risks exposing its blind observations or crosses observation groups")
        groups[identity] = (group, split)
        lat, lon, row = entries[identity, condition_at]
        current_e, current_n, temp = [number(float(row[field])) for field in ("ve", "vn", "sst")]
        require(abs(current_e) < 10 and abs(current_n) < 10 and 260 < temp < 330, "NOAA sea conditions are missing or outside the exercise range")
        exercise["cases"].append({"id": case["id"], "observationGroup": group, "split": split,
                                  "from": utc(start), "cutoff": utc(end),
                                  "conditions": [{"cell": cell_at(lat, lon), "observedAt": utc(condition_at),
                                                  "kind": "interpolated", "source": "NOAA AOML hourly QC, earlier sample",
                                                  "eastwardMps": current_e, "northwardMps": current_n, "sstKelvin": temp}]})
        if split == "test":
            blind = [{"at": utc(at), "cell": cell_at(lat, lon)} for (program, at), (lat, lon, _) in sorted(entries.items())
                     if program == identity and blind_from <= at < end]
            require(blind, "held-out case has no retained NOAA fixes")
            answers["cases"].append({"id": case["id"], "observations": blind})
    participant = json.dumps(exercise, sort_keys=True, separators=(",", ":"), allow_nan=False).encode() + b"\n"
    answers["exerciseSha256"] = hashlib.sha256(participant).hexdigest()
    return participant, answers


def evaluate_search(exercise, reference, submissions):
    """Use only preregistered held-out observations to verify a claimed sighting."""
    cells = {text(item["id"]): number(item["areaKm2"]) for item in exercise["cells"]}
    require(len(cells) == len(exercise["cells"]) and cells and all(0 < v <= 10000 for v in cells.values()),
            "unique positive-area search cells are required")
    groups, cases = {}, {}
    for case in exercise["cases"]:
        key, group = text(case["id"]), text(case["observationGroup"])
        split = case["split"]
        start, until = instant(case["from"]), instant(case["cutoff"])
        require(key not in cases and split in {"development", "test"} and
                (group not in groups or groups[group] == split) and 0 < (until-start).total_seconds() <= 31 * 86400,
                "invalid duration or overlap across development and blind observation groups")
        groups[group] = split
        cases[key] = case
    held_out = {key: case for key, case in cases.items() if case["split"] == "test"}
    require(held_out, "no held-out civilian search cases")

    for case in cases.values():
        conditions = case["conditions"]
        require(conditions, "state which sea conditions were available for the exercise window")
        for item in conditions:
            at = instant(item["observedAt"])
            require(item["cell"] in cells and instant(case["from"]) <= at < instant(case["cutoff"]) and
                    item["kind"] in {"observed", "interpolated", "authored", "unknown"},
                    "sea condition has a different location, time or evidence status")
            text(item["source"])
    truth = {}
    for item in reference["cases"]:
        key = item["id"]
        require(key in held_out and key not in truth, "reference repeats or leaves the registered blind split")
        case = held_out[key]
        signed = set()
        for o in item["observations"]:
            at = instant(o["at"])
            point = (at, o["cell"])
            require(instant(case["from"]) <= at < instant(case["cutoff"]) and o["cell"] in cells and point not in signed,
                    "reference sighting lies outside the reviewed window or repeats")
            signed.add(point)
        truth[key] = signed
    require(truth.keys() == held_out.keys(), "each held-out case needs an independent reference")

    results = []
    seen = set()
    for run in submissions["cases"]:
        key = run["id"]
        require(key in truth and key not in seen, "submission repeats or leaves the held-out split")
        seen.add(key)
        case = held_out[key]
        start, until = instant(case["from"]), instant(case["cutoff"])
        searched = set()
        covered_at = {}
        for visit in run["effort"]:
            at, cell = instant(visit["completedAt"]), visit["cell"]
            require(cell in cells and start <= at < until, "effort falls outside its reviewed cell or cutoff")
            searched.add(cell)
            covered_at[cell] = min(at, covered_at.get(cell, at))
        verified, rejected = [], 0
        for claim in run["sightings"]:
            at, cell = instant(claim["at"]), claim["cell"]
            require(start <= at < until and cell in cells, "claim outside the reviewed case")
            if (at, cell) in truth[key] and cell in covered_at and covered_at[cell] <= at:
                verified.append((at-start).total_seconds())
            else:
                rejected += 1
        elapsed = min(verified) if verified else None
        results.append({"id": key, "searchedAreaKm2": sum(cells[cell] for cell in sorted(searched)),
                        "firstVerifiedSightingSeconds": elapsed, "unverifiedClaims": rejected,
                        "outcome": "verified" if elapsed is not None else "unresolved"})
    require(seen == set(held_out), "submission must include unresolved cases in the held-out denominator")
    verified_times = [r["firstVerifiedSightingSeconds"] for r in results if r["outcome"] == "verified"]
    return {"heldOutCases": len(held_out), "verifiedCases": len(verified_times),
            "unresolvedCases": len(held_out)-len(verified_times),
            "medianVerifiedSightingSeconds": median(verified_times) if verified_times else None,
            "cases": sorted(results, key=lambda row: row["id"])}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    if len(sys.argv) > 1 and sys.argv[1] == "prepare-noaa":
        parser.add_argument("capture", type=Path)
        parser.add_argument("layout", type=Path)
        parser.add_argument("--audience", choices=["participant", "evaluator"], required=True)
        args = parser.parse_args(sys.argv[2:])
        participant, answers = compile_drifters(args.capture, args.layout)
        if args.audience == "participant":
            sys.stdout.buffer.write(participant)
        else:
            print(json.dumps(answers, sort_keys=True))
        return
    for name in ("exercise", "reference", "submissions"):
        parser.add_argument(name, type=Path)
    args = parser.parse_args()
    raw = [file.read_bytes() for file in (args.exercise, args.reference, args.submissions)]
    digests = [hashlib.sha256(data).hexdigest() for data in raw]
    exercise, reference, submissions = map(json.loads, raw)
    require(reference["exerciseSha256"] == submissions["exerciseSha256"] == digests[0], "coastal exercise pin differs")
    result = evaluate_search(exercise, reference, submissions)
    print(json.dumps({"schema": "emad.civilian-coastal-search-score.v1", "exerciseSha256": digests[0],
                      "referenceSha256": digests[1], "submissionsSha256": digests[2], "result": result}, sort_keys=True))


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError, TypeError, AttributeError, KeyError, StopIteration) as err:
        sys.exit(f"Coastal search input refused: {err}")
