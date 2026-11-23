"""Score civilian supporting-evidence controls using an independently held reference."""

import argparse
from hashlib import sha256
import json
from pathlib import Path
import sys

from simulation_inputs import number, require, text


def navigation(reference, submission):
    expected = {text(row["id"]): row["state"] for row in reference["cases"]}
    require(expected and len(expected) == len(reference["cases"]) and set(expected.values()) <= {"missing", "degraded"}, "invalid authored navigation rubric")
    records = {text(row["id"]): row for row in submission["cases"]}
    require(expected.keys() == records.keys() and len(records) == len(submission["cases"]), "one decision per control case is required")
    result = []
    for key,state in expected.items():
        row = records[key]
        require(row["state"] in {"missing", "degraded"} and type(row["requestObservation"]) is bool and
                row["cause"] in {"unknown", "asserted"}, "invalid authored uncertainty response")
        result.append({"id": key, "coverageStateCorrect": row["state"] == state, "soughtObservation": row["requestObservation"],
                       "preservedCauseUncertainty": row["cause"] == "unknown"})
    return {"cases": result}


def dust(reference, submission):
    groups, targets, seen = {}, {}, set()
    for row in reference["cases"]:
        key, group, split = text(row["id"]), text(row["group"]), row["split"]
        require(key not in seen and split in {"development", "test"} and groups.get(group, split) == split,
                "repeat imagery case or shared group crosses the blind split")
        seen.add(key)
        groups[group] = split
        if split == "test":
            require(row["imageQuality"] in {"usable", "unusable", "unrated"}, "independent imagery rating is invalid")
            targets[key] = row["imageQuality"]
    records = {text(row["id"]): row["prediction"] for row in submission["cases"]}
    require(targets and targets.keys() == records.keys() and len(records) == len(submission["cases"]), "use the same held-out imagery cases")
    require(set(records.values()) <= {"usable", "unusable", "inconclusive"}, "invalid image prediction")
    return {"testCases": len(targets), "unrated": sum(state == "unrated" for state in targets.values()),
            "falseUsable": sum(state == "unusable" and records[key] == "usable" for key,state in targets.items()),
            "actuallyUnusable": sum(state == "unusable" for state in targets.values()),
            "abstentions": sum(state == "inconclusive" for state in records.values())}


def passenger_count(reference, submission):
    fields = ("airport", "day", "cancellationDefinition")
    require(all(text(reference[k]) == text(submission[k]) for k in fields), "the published and submitted airport grains differ")
    counts = {}
    for field in ("scheduled", "cancelled"):
        a,b = reference[field],submission[field]
        require(type(a) is int and type(b) is int and 0 <= number(a) == number(b), "submitted count differs from the provider")
        counts[field] = a
    require(counts["cancelled"] <= counts["scheduled"], "cancelled flights exceed the scheduled count")
    return {**counts, "cancelledFraction": counts["cancelled"]/counts["scheduled"] if counts["scheduled"] else None}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("kind", choices=["navigation", "dust", "passenger-count"])
    parser.add_argument("reference", type=Path)
    parser.add_argument("submission", type=Path)
    args = parser.parse_args()
    raw_reference, raw_submission = args.reference.read_bytes(), args.submission.read_bytes()
    reference, submission = json.loads(raw_reference), json.loads(raw_submission)
    require(text(reference["exerciseSha256"]) == text(submission["exerciseSha256"]), "exercise digests differ")
    result = {"navigation": navigation, "dust": dust, "passenger-count": passenger_count}[args.kind](reference, submission)
    print(json.dumps({"schema": "emad.supporting-evaluation.v1", "kind": args.kind,
                      "referenceSha256": sha256(raw_reference).hexdigest(), "submissionSha256": sha256(raw_submission).hexdigest(), "result": result}))


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError, TypeError, AttributeError, KeyError) as err:
        sys.exit(f"Supporting evaluation refused: {err}")
