"""Qualify stable idle storage through a completed candidate observation."""

import argparse
import hashlib
import json
import math
from pathlib import Path
import sys

from observe_environment_storage import GAUGES, utc_stamp


def verify(candidate, soak, trend):
    start, end = utc_stamp(soak["startedAt"]), utc_stamp(soak["checkedAt"])
    if (soak["state"] not in {"SOAK_PASS", "PRE_RELEASE_OBSERVATION_PASS"} or (end - start).total_seconds() < 86400
            or soak["elapsedSeconds"] < 86400 or soak["failures"] or soak["sampleGaps"]):
        raise ValueError("The pinned observation must complete before storage qualification")
    identity = candidate["installationSha256"]
    bound = "installationSha256" in soak or "apiGeneration" in soak
    generation = candidate.get("services", {}).get("api-1")
    if bound and (not generation or soak.get("installationSha256") != identity or soak.get("apiGeneration") != generation):
        raise ValueError("Storage and observation API generations differ")
    targets = [{key: release[key] for key in ("id", "sha256")} for release in candidate["releases"]]
    if targets != [{key: release[key] for key in ("id", "sha256")} for release in soak["candidate"]]:
        raise ValueError("Storage and observation release pins differ")

    previous = start
    retained = None
    for sample in trend:
        observed = utc_stamp(sample["at"])
        if (sample["installationSha256"] != identity or sample["status"] != "OK"
                or bound and sample.get("apiStartedAt") != generation["startedAt"]
                or not 0 < (observed - previous).total_seconds() <= 130):
            raise ValueError("Storage samples are missing, invalid or bound to a different candidate")
        previous = observed
        gauges = sample["gauges"]
        if not all(math.isfinite(gauges[key]) and gauges[key] >= 0 for key in GAUGES):
            raise ValueError("Invalid storage gauges")
        if retained is None:
            retained = gauges["retained_asset_bytes"]
        if gauges["retained_asset_bytes"] != retained or any(gauges[key] for key in GAUGES if key != "retained_asset_bytes"):
            raise ValueError("Storage or queue changed during the idle observation")
    if retained is None or retained == 0 or abs((end - previous).total_seconds()) > 130:
        raise ValueError("The storage samples do not cover the observation window")
    return {"status": "PASS", "installationSha256": identity, "targets": targets,
            "sampleCount": len(trend), "retainedBytes": retained}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("candidate", "soak", "trend", "output"):
        parser.add_argument(name, type=Path)
    args = parser.parse_args()
    try:
        candidate = json.loads(args.candidate.read_text(encoding="utf-8"))
        observation = json.loads(args.soak.read_text(encoding="utf-8"))
        raw = args.trend.read_bytes()
        result = verify(candidate, observation, [json.loads(line) for line in raw.splitlines()])
        result["evidenceSha256"] = hashlib.sha256(raw).hexdigest()
        with args.output.open("x", encoding="utf-8") as output:
            json.dump(result, output, indent=2)
    except (OSError, KeyError, TypeError, ValueError) as error:
        print(str(error), file=sys.stderr)
        return 2
    print(json.dumps(result))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
