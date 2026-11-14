"""Check the scoped operating state of the three pinned environment releases."""

import argparse
from datetime import datetime, timezone
import json
import math
from pathlib import Path
import re
import sys

import requests

from check_installed_environment_library import LibraryAPI, read_range

MAX_CLOCK_DRIFT_SECONDS = 10
GAUGE = re.compile(r"c4isr_environment_[a-z0-9_]+\Z")


def sample_metrics(api):
    with requests.get(api.origin + "/environment-operations/metrics", headers=api.headers, timeout=(5, 15), allow_redirects=False) as response:
        response.raise_for_status()
        values = {}
        for line in response.text.splitlines():
            if not line or line.startswith("#"):
                continue
            key, raw = line.split()
            value = float(raw)
            if not GAUGE.fullmatch(key) or key in values or not math.isfinite(value) or value < 0:
                raise ValueError("Unexpected or invalid environment metric")
            values[key] = value
        return {key.removeprefix("c4isr_environment_"): value for key, value in values.items()}


def commission(candidate, reader, operator, *, allow_baseline=False):
    scope = reader.call("GET", "/platform/readiness")
    server_time = datetime.fromisoformat(scope["checkedAt"])
    drift = abs((datetime.now(timezone.utc) - server_time).total_seconds())
    metrics = sample_metrics(operator)
    releases = []
    for target in candidate["releases"]:
        release = reader.call("GET", "/environment-releases/" + target["id"])
        if (release["id"] != target["id"] or release["manifestSha256"] != target["sha256"]
                or release["manifest"]["toolchainSha256"] != target["toolchain"]):
            raise ValueError("Installed release differs from the commissioning pin")
        prepared = read_range(reader, release["id"])
        if prepared != target.get("preparedSha256", prepared):
            raise ValueError("Installed archive differs from the commissioning pin")
        releases.append({"id": release["id"], "manifestSha256": target["sha256"], "preparedSha256": prepared,
                         "state": release["state"], "toolchainSha256": target["toolchain"]})
    workers = [item for item in scope["workers"] if item["feature"] == "environment-library"]
    ready = [worker for worker in workers if worker["state"] == "READY"]
    feature = "AVAILABLE" if ready else "UNAVAILABLE"
    artifact_state = next((item["state"] for item in scope["features"] if item["id"] == "artifact-storage"), "UNAVAILABLE")
    expected_toolchain = candidate["identity"]["toolchain"]
    matching_toolchain = any(worker.get("toolchainSha256") == expected_toolchain for worker in ready)
    migrations = candidate["identity"]["migrations"]
    current = (58 in migrations and matching_toolchain and metrics.get("fresh_worker_scratch_samples", 0) > 0
               and metrics.get("object_read_sampled") == 1 and metrics.get("object_read_ok") == 1)
    baseline = allow_baseline and 58 not in migrations and metrics.get("object_read_ok", 1) == 1
    healthy = (drift <= MAX_CLOCK_DRIFT_SECONDS and bool(ready) and bool(releases)
               and metrics["oldest_due_job_seconds"] <= 60 and metrics["expired_leases"] == 0
               and (current or baseline))
    return {"schema": "emad.environment-operations-commission.v1", "checkedAt": datetime.now(timezone.utc).isoformat(),
            "installationSha256": candidate["installationSha256"], "migrations": migrations,
            "clockDriftSeconds": round(drift, 3), "readyWorkerIdentities": len(ready),
            "workerToolchainMatched": matching_toolchain, "legacyBaseline": baseline,
            "readinessState": feature, "artifactReadinessState": artifact_state, "metrics": metrics, "releases": releases,
            "status": "PASS" if healthy else "OPEN"}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("candidate", "reader", "metrics_token", "output"):
        parser.add_argument(name, type=Path)
    parser.add_argument("--api", required=True)
    parser.add_argument("--allow-baseline", action="store_true", help="allow pre-58 installations with exact prepared-package range reads")
    args = parser.parse_args()
    try:
        candidate = json.loads(args.candidate.read_text(encoding="utf-8"))
        resource = candidate["identity"]["resource"]
        reader = LibraryAPI(args.api, args.reader, resource["workspace"], resource["environment"])
        operator = LibraryAPI(args.api, args.metrics_token, resource["workspace"], resource["environment"])
        result = commission(candidate, reader, operator, allow_baseline=args.allow_baseline)
        with args.output.open("x", encoding="utf-8") as output:
            json.dump(result, output, indent=2, allow_nan=False)
        print(json.dumps({key: result[key] for key in ("status", "readinessState", "readyWorkerIdentities", "clockDriftSeconds")}))
        return 0 if result["status"] == "PASS" else 2
    except (OSError, KeyError, ValueError, RuntimeError, requests.RequestException) as error:
        print(str(error), file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
