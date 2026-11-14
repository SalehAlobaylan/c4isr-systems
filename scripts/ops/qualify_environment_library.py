"""Measure installed release availability and metadata latency on a pinned candidate."""
import argparse
from concurrent.futures import ThreadPoolExecutor
from datetime import datetime, timezone
import json
import math
from pathlib import Path
import re
import subprocess
import sys
import time

import requests

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "data"))
from intake_environment import LibraryAPI


def record(path, data):
    temporary = path.with_suffix(path.suffix + ".partial")
    temporary.write_text(json.dumps(data, indent=2) + "\n", encoding="utf-8")
    temporary.replace(path)


def observation_status(path, now):
    report = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(report, dict) or report.get("schema") != "emad.environment-soak.v1":
        raise ValueError("Expected an environment soak report")
    if report.get("state") not in {"RUNNING", "SOAK_PASS", "PRE_RELEASE_OBSERVATION_PASS", "GATE_UNMET"}:
        raise ValueError("Unknown environment soak state")
    checked = datetime.fromisoformat(report["checkedAt"])
    limit = float(report["sampleGapLimitSeconds"])
    if checked.tzinfo is None or not math.isfinite(limit) or limit <= 0:
        raise ValueError("Expected a dated observation and a positive sample gap limit")
    age = (now - checked).total_seconds()
    if age < 0:
        raise ValueError("The observation timestamp is in the future")
    report["lastSampleAgeSeconds"] = round(age, 3)
    if report["state"] == "RUNNING" and age > limit:
        report["state"] = "INTERRUPTED"
    return report


def observe(api, target, states):
    start = time.monotonic()
    try:
        with requests.get(api.origin + "/environment-releases/" + target["id"], headers=api.headers,
                          timeout=(3, 5), allow_redirects=False) as response:
            release = response.json() if response.status_code == 200 else {}
        if not isinstance(release, dict):
            return time.monotonic() - start, None
        state = release.get("state")
        ok = state in states and release.get("manifestSha256") == target["sha256"]
        return time.monotonic() - start, state if ok else None
    except (OSError, ValueError, TypeError):
        return time.monotonic() - start, None


def soak(api, targets, duration, cadence, output, states, installation=None, installation_sha256=None):
    if installation is not None:
        from capture_environment_candidate import container
    binding = {} if installation_sha256 is None else {
        "installationSha256": installation_sha256,
        "apiGeneration": installation,
    }
    started = datetime.now(timezone.utc)
    start = time.monotonic()
    previous_elapsed, previous_wall_elapsed = 0, 0
    samples, failures, gaps, max_gap = [], 0, 0, 0
    gap_limit = 2 * cadence + 10
    latest = {}
    with ThreadPoolExecutor(max_workers=5) as pool:
        while True:
            if installation is not None:
                try:
                    same_installation = container(installation["name"]) == installation
                except (OSError, ValueError, RuntimeError, subprocess.SubprocessError):
                    same_installation = False
                if not same_installation:
                    result = json.loads(output.read_text(encoding="utf-8")) if output.exists() else {
                        "schema": "emad.environment-soak.v1", "startedAt": started.isoformat(),
                        "checkedAt": datetime.now(timezone.utc).isoformat(), "sampleGapLimitSeconds": gap_limit,
                        "elapsedSeconds": 0, "candidate": targets, "readerCount": 5, "requests": 0, "failures": 0,
                        "metadataP95Seconds": 0, "maxSampleGapSeconds": 0, "sampleGaps": 0, "lastObservedStates": {},
                    }
                    result.update(binding, state="GATE_UNMET", interruptedAt=datetime.now(timezone.utc).isoformat(),
                                  interruption="Installed API generation changed or could not be verified")
                    record(output, result)
                    return result
            check = [targets[(len(samples) + index) % len(targets)] for index in range(5)]
            for target, (elapsed, observed) in zip(check, pool.map(lambda target: observe(api, target, states), check)):
                samples.append(elapsed)
                failures += observed is None
                latest[target["id"]] = observed
            checked = datetime.now(timezone.utc)
            elapsed = time.monotonic() - start
            wall_elapsed = (checked - started).total_seconds()
            gap = max(elapsed - previous_elapsed, wall_elapsed - previous_wall_elapsed)
            max_gap = max(max_gap, gap)
            gaps += gap > gap_limit
            previous_elapsed, previous_wall_elapsed = elapsed, wall_elapsed
            p95 = sorted(samples)[math.ceil(.95 * len(samples)) - 1]
            observed_seconds = min(elapsed, wall_elapsed)
            complete = observed_seconds >= duration
            state = "RUNNING"
            if complete:
                within_budgets = observed_seconds >= 86400 and failures == 0 and gaps == 0 and p95 <= 1
                released = all(latest.get(target["id"]) == "RELEASED" for target in targets)
                state = ("SOAK_PASS" if released else "PRE_RELEASE_OBSERVATION_PASS") if within_budgets else "GATE_UNMET"
            result = {**binding, "schema": "emad.environment-soak.v1", "state": state,
                      "startedAt": started.isoformat(), "checkedAt": checked.isoformat(),
                      "elapsedSeconds": round(observed_seconds, 3), "candidate": targets, "readerCount": 5,
                      "requests": len(samples), "failures": failures, "metadataP95Seconds": round(p95, 4),
                      "sampleGapLimitSeconds": gap_limit, "maxSampleGapSeconds": round(max_gap, 3), "sampleGaps": gaps,
                      "allowedStates": sorted(states), "lastObservedStates": latest,
                      "scope": "Pinned release detail reads. Worker restart, queue, volume, restore and operator gates use separate evidence."}
            record(output, result)
            if complete:
                return result
            time.sleep(min(cadence, duration - observed_seconds))


def main(argv=None):
    argv = sys.argv[1:] if argv is None else argv
    if argv[:1] == ["status"]:
        parser = argparse.ArgumentParser(prog=f"{Path(sys.argv[0]).name} status",
                                         description="Check the latest sample without altering the report.")
        parser.add_argument("report", type=Path)
        args = parser.parse_args(argv[1:])
        try:
            report = observation_status(args.report, datetime.now(timezone.utc))
        except (KeyError, OSError, TypeError, ValueError) as exc:
            parser.error(str(exc))
        print(json.dumps(report))
        return 0 if report["state"] in {"RUNNING", "SOAK_PASS", "PRE_RELEASE_OBSERVATION_PASS"} else 2
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("targets", type=Path, help="Three pinned release records with id and sha256")
    parser.add_argument("output", type=Path)
    parser.add_argument("--api", required=True)
    parser.add_argument("--token-file", type=Path, required=True)
    parser.add_argument("--workspace", required=True)
    parser.add_argument("--environment", required=True)
    parser.add_argument("--seconds", type=int, default=86400)
    parser.add_argument("--cadence", type=int, default=30)
    parser.add_argument("--allow-staged", action="store_true", help="Observe a frozen candidate during human review")
    parser.add_argument("--installed-candidate", type=Path, help="Fail the observation if the pinned API generation changes")
    args = parser.parse_args(argv)
    targets = json.loads(args.targets.read_text(encoding="utf-8"))
    if (not isinstance(targets, list) or len(targets) != 3 or
            any(not isinstance(item, dict) or not re.fullmatch(r"georelease_[a-z0-9]{26}", str(item.get("id", "")))
                or not re.fullmatch(r"[0-9a-f]{64}", str(item.get("sha256", ""))) for item in targets) or
            len({item["id"] for item in targets}) != 3):
        parser.error("Supply the three exact installed release identities")
    if args.seconds < 1 or not 5 <= args.cadence <= 60 or args.output.exists():
        parser.error("Choose a fresh output, positive duration and 5–60 second cadence")
    api = LibraryAPI(args.api, args.token_file, args.workspace, args.environment)
    states = {"STAGED", "VERIFIED", "RELEASED"} if args.allow_staged else {"RELEASED"}
    if not all(observe(api, target, states)[1] for target in targets):
        parser.error("Every target must be visible in an allowed state and match the pinned digest")
    installation = installation_sha256 = None
    if args.installed_candidate:
        candidate = json.loads(args.installed_candidate.read_text(encoding="utf-8"))
        pins = [{key: row[key] for key in ("id", "sha256")} for row in candidate["releases"]]
        if pins != [{key: row[key] for key in ("id", "sha256")} for row in targets]:
            parser.error("Observation targets differ from the installed candidate")
        installation = candidate["services"]["api-1"]
        installation_sha256 = candidate["installationSha256"]
        if not re.fullmatch(r"[a-f0-9]{64}", installation_sha256):
            parser.error("Supply the exact installed candidate SHA-256")
    report = soak(api, targets, args.seconds, args.cadence, args.output, states, installation, installation_sha256)
    print(json.dumps(report))
    return 0 if report["state"] in {"SOAK_PASS", "PRE_RELEASE_OBSERVATION_PASS"} else 2


if __name__ == "__main__":
    raise SystemExit(main())
