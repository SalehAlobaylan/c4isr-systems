"""Score blinded civil access exercise results against an independent case file."""

import argparse
from collections import Counter
from hashlib import sha256
import json
from pathlib import Path
from statistics import median
import sys

from simulation_inputs import instant, require, text, utc


def time_to_verified_quantiles(records):
    """Return median and 90th-percentile clearance time with right censoring."""
    by_time = {}
    for seconds, occurred in records:
        group = by_time.setdefault(seconds, Counter())
        group["events" if occurred else "censored"] += 1
    risk = len(records)
    survival = 1.0
    quantiles = {"p50Seconds": None, "p90Seconds": None}
    for seconds, group in sorted(by_time.items()):
        survival *= 1 - group["events"] / risk
        for metric, threshold in (("p50Seconds", .5), ("p90Seconds", .9)):
            if quantiles[metric] is None and 1 - survival + 1e-12 >= threshold:
                quantiles[metric] = seconds
        risk -= sum(group.values())
    return quantiles


def score_access(truth_raw, runs_raw):
    truth, document = json.loads(truth_raw), json.loads(runs_raw)
    require(truth["schema"] == "emad.civilian-access-answers.v1" and
            document["schema"] == "emad.civilian-access-submissions.v1", "unexpected evaluation data format")
    expected = text(truth["exerciseSha256"])
    require(len(expected) == 64 and all(c in "0123456789abcdef" for c in expected) and
            document["exerciseSha256"] == expected, "submissions identify a different frozen exercise")
    truth_digest = sha256(truth_raw).hexdigest()
    cases = {}
    for case in truth["cases"]:
        key = text(case["id"])
        require(key not in cases, "answer file repeats a case")
        aware, end = instant(case["awarenessAt"]), instant(case["cutoffAt"])
        verified = instant(case["verifiedAvailableAt"]) if case.get("verifiedAvailableAt") else None
        visits, complete = case["expectedVisits"], case["verdictCoverageComplete"]
        require(aware < end and type(complete) is bool and type(visits) is int and visits >= 0,
                "invalid case interval, coverage or allowed visits")
        require(verified is None or aware <= verified <= end, "verified access falls outside the case")
        cases[key] = aware, end, verified, visits, complete
    require(cases, "answer file contains no cases")

    results = {}
    for run in document["runs"]:
        key = text(run["id"])
        choices = run["cases"]
        require(key not in results and {record["caseId"] for record in choices} == set(cases) and
                len(choices) == len(cases), "each run must cover exactly the same blinded cases")
        counts = Counter()
        clearance = []
        operator_seconds = []
        case_results = []
        for record in choices:
            identity = record["caseId"]
            aware, end, verified, visits, complete = cases[identity]
            dispatches = [instant(at) for at in record["dispatchedAt"]]
            reopenings = [instant(at) for at in record["declaredOpenAt"]]
            require(all(aware <= at <= end for at in dispatches + reopenings), "decision falls outside the exercise window")
            require(reopenings == sorted(set(reopenings)), "reopening decisions must be unique and ordered")
            elapsed = record.get("operatorSeconds")
            if elapsed is not None:
                require(type(elapsed) is int and elapsed >= 0, "invalid operator duration")
                operator_seconds.append(elapsed)

            premature = 0
            indeterminate = 0
            valid = None
            for at in reopenings:
                if verified is not None and at >= verified:
                    valid = at if valid is None else valid
                elif complete:
                    premature += 1
                else:
                    indeterminate += 1
            counts["reopeningDecisions"] += len(reopenings)
            counts["prematureReopenings"] += premature
            counts["unscorableReopenings"] += indeterminate
            counts["excessAssignments"] += max(0, len(dispatches) - visits)
            outcome = "unqualified"
            if complete:
                counts["qualifiedCases"] += 1
                seconds = ((valid if valid is not None else end) - aware).total_seconds()
                clearance.append((seconds, valid is not None))
                counts["unresolvedAtCutoff"] += valid is None
                outcome = "unresolved" if valid is None else "verified-reopening"
            case_results.append({
                "caseId": identity, "outcome": outcome,
                "verifiedReopeningAt": utc(valid) if valid is not None else None,
                "expectedVisits": visits, "assignments": len(dispatches),
                "prematureReopenings": premature, "unscorableReopenings": indeterminate,
            })

        results[key] = {
            "caseCount": len(cases), "qualifiedCases": counts["qualifiedCases"],
            "unresolvedAtCutoff": counts["unresolvedAtCutoff"],
            "reopeningDecisions": counts["reopeningDecisions"],
            "prematureReopenings": counts["prematureReopenings"],
            "unscorableReopenings": counts["unscorableReopenings"],
            "excessAssignments": counts["excessAssignments"],
            "timeToVerifiedReopening": time_to_verified_quantiles(clearance),
            "operatorSecondsMedian": median(operator_seconds) if operator_seconds else None,
            "operatorSecondsSampleCount": len(operator_seconds),
            "cases": case_results,
        }
    require(results, "submissions contain no runs")
    return {"schema": "emad.civilian-access-evaluation.v1", "exerciseSha256": expected,
            "answerFileSha256": truth_digest, "submissionsSha256": sha256(runs_raw).hexdigest(), "runs": results}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("exercise", type=Path)
    parser.add_argument("answers", type=Path)
    parser.add_argument("submissions", type=Path)
    args = parser.parse_args()
    result = score_access(args.answers.read_bytes(), args.submissions.read_bytes())
    require(sha256(args.exercise.read_bytes()).hexdigest() == result["exerciseSha256"],
            "exercise bytes differ from the evaluator's pin")
    print(json.dumps(result, sort_keys=True, allow_nan=False))


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError, TypeError, KeyError) as exc:
        sys.exit(f"Access evaluation refused: {exc}")
