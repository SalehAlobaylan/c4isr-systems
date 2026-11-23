"""Reproduce held-out measures for civilian imagery review and completed research recordings."""

import argparse
from collections import defaultdict
import csv
from hashlib import sha256
import json
from math import ceil
from pathlib import Path
from statistics import median
import sys
import zipfile

from simulation_inputs import airlab_energy, instant, number, require, text


def sha(data):
    return sha256(data).hexdigest()


def pinned_file(base, spec, byte_limit):
    path = (base / spec["path"]).resolve()
    require(path.is_relative_to(base.resolve()), "evaluation asset escapes its input directory")
    require(path.stat().st_size <= byte_limit, "evaluation asset exceeds its byte budget")
    raw = path.read_bytes()
    require(sha(raw) == spec["sha256"], "evaluation asset digest mismatch")
    return raw


def airlab_definition(archive, parameters):
    """Pin the public members and split whole recordings within test type."""
    cohorts = defaultdict(list)
    with parameters.open(encoding="utf-8-sig", newline="") as stream, zipfile.ZipFile(archive) as recorded:
        rows = list(csv.DictReader(stream))
        identities = {row["flight"] for row in rows}
        expected = {f"flights/{flight}.csv" for flight in identities}
        actual = [member for member in recorded.namelist() if member.endswith(".csv")]
        require(len(identities) == len(rows) == len(actual) and set(actual) == expected,
                "recordings and parameter index disagree")
        for row in rows:
            flight, route = row["flight"], row["route"]
            require(flight.isdigit() and route in {"A1", "A2", "A3", "H", "R1", "R2", "R3", "R4", "R5", "R6", "R7"},
                    "unknown AirLab recording type or identity")
            test_type = "hover" if route == "H" else "ancillary-ground" if route.startswith("A") else "study-flight"
            member = f"flights/{flight}.csv"
            require(recorded.getinfo(member).file_size <= 8_000_000, "completed survey log exceeds its byte budget")
            cohorts[test_type].append({"flight": flight, "member": member, "sha256": sha(recorded.read(member)),
                                       "platform": "DJI Matrice 100 research aircraft", "testType": test_type})
    flights = []
    for cohort in sorted(cohorts):
        entries = sorted(cohorts[cohort], key=lambda row: int(row["flight"]))
        flights.extend({**row, "split": "test" if index % 5 == 0 else "train"} for index, row in enumerate(entries))
    return {"flights": flights, "parameterIndexSha256": sha(parameters.read_bytes())}


def completed_energy(archive, definition, max_gap):
    groups = defaultdict(lambda: {"train": [], "test": []})
    seen = set()
    with zipfile.ZipFile(archive) as record:
        for entry in definition["flights"]:
            member = entry["member"]
            require(member not in seen and entry["split"] in {"train", "test"} and member.endswith(".csv"),
                    "repeat recording or invalid split")
            seen.add(member)
            info = record.getinfo(member)
            require(info.file_size <= 8_000_000, "completed survey log exceeds its byte budget")
            raw = record.read(info)
            require(sha(raw) == entry["sha256"], "flight log digest mismatch")
            require(Path(member).stem == entry["flight"], "ZIP member and flight identity disagree")
            value = airlab_energy(raw, max_gap, member_flight=entry["flight"])
            require(value["flight"] == entry["flight"], "flight identifier disagrees with the manifest")
            group = (text(entry["platform"]), text(entry["testType"]))
            groups[group][entry["split"]].append(value)
    result = []
    for (platform, test_type), group in sorted(groups.items()):
        train, test = group["train"], group["test"]
        require(len({row["flight"] for row in train + test}) == len(train + test),
                "a completed flight repeats across benchmark partitions")
        elapsed = sum(row["includedSeconds"] for row in train)
        baseline = sum(row["measuredWh"] or 0 for row in train) * 3600 / elapsed if elapsed else None
        errors = []
        comparisons = []
        for row in test:
            if baseline is None or row["measuredWh"] is None:
                continue
            expected = baseline * row["includedSeconds"] / 3600
            error = abs(expected - row["measuredWh"])
            errors.append(error)
            comparisons.append({**row, "baselineWh": expected, "absoluteErrorWh": error})
        errors.sort()
        result.append({"platform": platform, "testType": test_type, "trainingFlights": len(train),
                       "heldOutFlights": len(test), "scoredFlights": len(errors),
                       "medianAbsoluteErrorWh": median(errors) if errors else None,
                       "p90AbsoluteErrorWh": errors[ceil(.9 * len(errors)) - 1] if errors else None,
                       "excludedHeldOutSeconds": sum(row["excludedSeconds"] for row in test),
                       "recordings": comparisons})
    require(result, "no completed research recordings")
    return {"scope": "retrospective-completed-recordings", "maxGapSeconds": max_gap,
            "measurement": "signed net watt-hours from recorded current and voltage", "groups": result}


def flood_review(manifest, submissions, base):
    import numpy as np
    from rasterio.io import MemoryFile

    flood_class = manifest["floodedRoadClass"]
    ignored = manifest["ignoreClass"]
    allowed = set(manifest["classes"])
    require(type(flood_class) is int and flood_class in allowed and ignored not in allowed,
            "expected the reviewed native integer class table")
    split_keys, seen, cases = {}, set(), {}
    for case in manifest["cases"]:
        identity = text(case["id"])
        split = case["split"]
        require(identity not in seen and split in {"train", "validation", "test"}, "duplicate case or invalid split")
        seen.add(identity)
        for kind, key in (("site", text(case["collectionGroup"])), ("image", text(case["imageSha256"])),
                          ("near-duplicate group", text(case["visualGroup"]))):
            index = kind, key
            require(index not in split_keys or split_keys[index] == split, "spatial or visual group crosses partitions")
            split_keys[index] = split
        if split == "test":
            cases[identity] = case
    require(cases, "no held-out test cases")

    def mask(spec):
        raw = pinned_file(base, spec, 8_000_000)
        with MemoryFile(raw) as file, file.open() as raster:
            require(raster.count == 1 and raster.width * raster.height <= 20_000_000,
                    "expected a bounded, native single-band mask")
            values = raster.read(1)
            require(np.issubdtype(values.dtype, np.integer), "class mask uses noninteger values")
            require(np.isin(values, list(allowed | {ignored})).all(), "unknown class in mask")
            return values

    results = {}
    for run in submissions["runs"]:
        identity = text(run["id"])
        selected = run["cases"]
        require(identity not in results and {item["id"] for item in selected} == set(cases) and
                len(selected) == len(cases), "comparison runs must cover the same held-out cases")
        intersection = union = predicted = false_clear = flooded = abstentions = 0
        durations = []
        for item in selected:
            truth = cases[item["id"]]
            decision = item["verdict"]
            require(decision in {"clear", "flooded", "inconclusive"} and type(truth["floodedRoad"]) is bool,
                    "unknown review verdict")
            durations.append(number(item["reviewSeconds"]))
            require(durations[-1] >= 0, "review duration cannot be negative")
            flooded += truth["floodedRoad"]
            false_clear += truth["floodedRoad"] and decision == "clear"
            abstentions += decision == "inconclusive"
            if item.get("prediction"):
                actual = mask(truth["mask"])
                estimate = mask(item["prediction"])
                require(actual.shape == estimate.shape, "prediction and held-out mask are not aligned")
                pixels = actual != ignored
                match, proposal = (actual == flood_class) & pixels, (estimate == flood_class) & pixels
                intersection += int((match & proposal).sum())
                union += int((match | proposal).sum())
                predicted += 1
        results[identity] = {"cases": len(cases), "predictedMasks": predicted,
                             "floodedRoadIoU": intersection / union if union else None,
                             "falseClear": false_clear, "actuallyFlooded": flooded,
                             "inconclusive": abstentions, "medianReviewSeconds": median(durations)}
    require(results, "no held-out review submissions")
    return {"runs": results, "splitCheck": "reviewed-site-and-visual-groups-disjoint"}


def score_handoff(answers, run):
    require(run["reviewSha256"] == answers["reviewSha256"], "handoff submission differs from the pinned case")
    deadline = instant(answers["cutoffAt"])
    cue_at = instant(answers["firstCueAt"])
    require(cue_at <= deadline, "handoff answer interval is reversed")
    events = {}
    for item in run["events"]:
        at = instant(item["at"])
        kind = item["kind"]
        require(kind in {"acknowledged", "paused", "accepted-follow-up"} and cue_at <= at <= deadline and kind not in events,
                "invalid or repeated handoff action")
        events[kind] = item
    followup = events.get("accepted-follow-up")
    acknowledged = "acknowledged" in events
    followed = followup is not None and followup.get("owner") == answers["requiredOwner"]
    return {"acknowledged": acknowledged, "paused": "paused" in events, "handoffAccepted": followed,
            "unresolvedFollowUps": 0 if followed else 1}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)
    pin = sub.add_parser("airlab-definition")
    pin.add_argument("archive", type=Path)
    pin.add_argument("parameters", type=Path)
    energy = sub.add_parser("energy")
    energy.add_argument("archive", type=Path)
    energy.add_argument("definition", type=Path)
    energy.add_argument("--gap-seconds", type=float, required=True)
    for action in ("flood", "handoff"):
        mode = sub.add_parser(action)
        mode.add_argument("definition", type=Path)
        mode.add_argument("submissions", type=Path)
    args = parser.parse_args()
    if args.command == "airlab-definition":
        print(json.dumps(airlab_definition(args.archive, args.parameters), sort_keys=True, allow_nan=False))
        return
    inputs = args.definition.read_bytes()
    definition = json.loads(inputs)
    receipt = {"schema": "emad.civilian-aerial-evaluation.v1", "definitionSha256": sha(inputs)}
    if args.command == "energy":
        receipt["result"] = completed_energy(args.archive, definition, args.gap_seconds)
    else:
        raw = args.submissions.read_bytes()
        submissions = json.loads(raw)
        receipt["submissionsSha256"] = sha(raw)
        receipt["result"] = flood_review(definition, submissions, args.definition.parent) if args.command == "flood" else score_handoff(definition, submissions)
    print(json.dumps(receipt, sort_keys=True, allow_nan=False))


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError, TypeError, KeyError, AttributeError, ImportError) as err:
        sys.exit(f"Aerial evaluation refused: {err}")
