"""Sample scoped queue and storage gauges until the installed observation ends."""

import argparse
from datetime import datetime, timezone
import json
import math
from pathlib import Path
import subprocess
import time

import requests

from capture_environment_candidate import container
from intake_environment import LibraryAPI


def utc_stamp(value):
    result = datetime.fromisoformat(value)
    if result.tzinfo is None:
        raise ValueError("Evidence timestamps require a timezone")
    return result.astimezone(timezone.utc)

GAUGES = (
    "due_jobs", "active_leases", "expired_leases", "failed_jobs", "oldest_due_job_seconds",
    "pending_asset_bytes", "retained_asset_bytes", "assets_awaiting_purge",
)


def observe(args):
    candidate = json.loads(args.candidate.read_text(encoding="utf-8"))
    installed_api = candidate["services"]["api-1"]
    client = LibraryAPI(args.api, args.token_file, args.workspace, args.environment)
    with args.output.open("x", encoding="utf-8", buffering=1) as stream:
        while True:
            now = datetime.now(timezone.utc)
            sample = {"at": now.isoformat(), "installationSha256": candidate["installationSha256"],
                      "apiStartedAt": installed_api["startedAt"]}
            try:
                running = container(installed_api["name"])
                if running != installed_api:
                    raise ValueError("The installed API generation changed")
                with requests.get(client.origin + "/environment-operations/metrics", headers=client.headers,
                                  timeout=(5, 10), allow_redirects=False) as response:
                    response.raise_for_status()
                    values = {name.removeprefix("c4isr_environment_"): float(value) for name, value in
                              (line.split() for line in response.text.splitlines() if line.startswith("c4isr_environment_"))}
                if not set(GAUGES).issubset(values) or any(not math.isfinite(value) or value < 0 for value in values.values()):
                    raise ValueError("The metrics sample is invalid")
                sample.update(status="OK", gauges=values)
            except (OSError, RuntimeError, ValueError, subprocess.SubprocessError, requests.RequestException) as error:
                sample.update(status="UNAVAILABLE", error=type(error).__name__)
            stream.write(json.dumps(sample, allow_nan=False) + "\n")
            if now >= args.until:
                return
            time.sleep(min(args.cadence, max(0, (args.until - now).total_seconds())))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("candidate", "token_file", "output"):
        parser.add_argument(name, type=Path)
    parser.add_argument("--api", required=True)
    parser.add_argument("--workspace", required=True)
    parser.add_argument("--environment", required=True)
    parser.add_argument("--until", type=utc_stamp, required=True)
    parser.add_argument("--cadence", type=int, default=60)
    args = parser.parse_args()
    seconds = (args.until - datetime.now(timezone.utc)).total_seconds()
    if not 0 < seconds <= 90000 or not 10 <= args.cadence <= 120:
        parser.error("Use a bounded interval and a sample cadence of 10–120 seconds")
    observe(args)


if __name__ == "__main__":
    main()
