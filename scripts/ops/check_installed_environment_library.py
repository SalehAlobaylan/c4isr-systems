"""Verify the exact staged packages produced by the installed worker."""
import argparse
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import sys
from tempfile import NamedTemporaryFile
import time

import requests

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "data"))
from environment_library import MAX_OBJECT
from verify_environment_rebuild import verify_prepared
from intake_environment import LibraryAPI


def object_digest(response):
    tag = response.headers.get("ETag", "")
    if not re.fullmatch(r'"[0-9a-f]{64}"', tag):
        raise RuntimeError("Prepared package did not advertise its immutable SHA-256")
    return tag[1:-1]


def read_range(api, release_id):
    with requests.get(api.origin + f"/environment-releases/{release_id}/prepared",
                      headers={**api.headers, "Range": "bytes=0-3"}, timeout=(5, 30),
                      stream=True, allow_redirects=False) as response:
        if (response.status_code != 206 or not response.headers.get("Content-Range", "").startswith("bytes 0-3/")
                or response.raw.read(5) != b"PK\x03\x04"):
            raise RuntimeError("Authorized prepared package range failed")
        return object_digest(response)


def download(api, release_id, output, manifest_digest, range_digest):
    if not re.fullmatch(r"[a-f0-9]{64}", range_digest):
        raise ValueError("A trusted prepared SHA-256 is required")
    digest = hashlib.sha256()
    total = 0
    temporary = None
    size = None
    url = api.origin + f"/environment-releases/{release_id}/prepared"
    try:
        with NamedTemporaryFile(dir=output.parent, suffix=".partial", delete=False) as stream:
            temporary = Path(stream.name)
            for attempt in range(3):
                headers = {**api.headers, "Accept-Encoding": "identity"}
                if total:
                    headers.update(Range=f"bytes={total}-", **{"If-Range": f'"{range_digest}"'})
                try:
                    with requests.get(url, headers=headers, timeout=(10, 120), stream=True, allow_redirects=False) as response:
                        if response.status_code not in ({206} if total else {200}):
                            raise RuntimeError(f"Prepared package request failed: HTTP {response.status_code}")
                        if object_digest(response) != range_digest or response.headers.get("Content-Encoding", "identity") != "identity":
                            raise RuntimeError("Prepared package identity or encoding changed during transfer")
                        length = int(response.headers.get("Content-Length", "0"))
                        if total:
                            if size is None or response.headers.get("Content-Range") != f"bytes {total}-{size - 1}/{size}" or length != size - total:
                                raise RuntimeError("Prepared package range does not continue the verified representation")
                        elif not 1 <= length <= MAX_OBJECT or size is not None and length != size:
                            raise RuntimeError("Prepared package length differs from its advertised bounds")
                        else:
                            size = length
                        for block in response.iter_content(256 << 10):
                            total += len(block)
                            if total > size:
                                raise RuntimeError("Installed package exceeded its advertised length")
                            digest.update(block)
                            stream.write(block)
                except (requests.ConnectionError, requests.Timeout, requests.exceptions.ChunkedEncodingError) as error:
                    if attempt == 2:
                        raise RuntimeError("Prepared package transfer did not recover") from error
                if total == size:
                    break
        if total != size or digest.hexdigest() != range_digest:
            raise RuntimeError("Installed package did not match its advertised length and SHA-256")
        check = verify_prepared(temporary, manifest_digest, range_digest)
        # Publish verified bytes without clobbering any existing file.
        os.link(temporary, output)
    finally:
        if temporary is not None:
            temporary.unlink(missing_ok=True)
    return {"preparedBytes": total, "preparedSha256": digest.hexdigest(), "offlineVerification": check}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("commission", type=Path)
    parser.add_argument("output", type=Path)
    parser.add_argument("--api", required=True)
    parser.add_argument("--reviewer-token", type=Path, required=True)
    parser.add_argument("--timeout", type=int, default=1500)
    args = parser.parse_args()
    commission = json.loads(args.commission.read_text(encoding="utf-8"))
    if (commission.get("schema") != "emad.environment-commission.v1" or not commission.get("areas")
            or any(not area.get("build") for area in commission["areas"].values()) or args.timeout < 1):
        parser.error("Supply a completed commissioning record with queued builds and a positive timeout")
    api = LibraryAPI(args.api, args.reviewer_token, commission["workspace"], commission["environment"])
    args.output.mkdir(parents=True, exist_ok=True)
    end = time.monotonic() + args.timeout
    result = {"schema": "emad.environment-installed-check.v1", "workspace": commission["workspace"],
              "environment": commission["environment"], "areas": {}}
    while len(result["areas"]) < len(commission["areas"]) and time.monotonic() < end:
        for key, area in commission["areas"].items():
            if key in result["areas"]:
                continue
            build = api.call("GET", f"/environment-builds/{area['build']['id']}")
            if build["state"] in {"FAILED", "CANCELED"}:
                raise RuntimeError(f"{key} ended in {build['state']}: {build.get('failureCode', '')}")
            if build["state"] != "SUCCEEDED":
                continue
            rows = api.call("GET", f"/environment-releases?build={build['id']}")["items"]
            if len(rows) != 1:
                raise RuntimeError(f"{key} did not retain exactly one release")
            item = rows[0]
            release = api.call("GET", f"/environment-releases/{item['id']}")
            if release["manifestSha256"] != item["manifestSha256"]:
                raise RuntimeError(f"{key} has different manifest identities in release list and detail")
            range_digest = read_range(api, item["id"])
            file = args.output / f"{key}-prepared.zip"
            checked = download(api, item["id"], file, item["manifestSha256"], range_digest)
            attempts = api.call("GET", f"/environment-builds/{build['id']}/attempts")
            result["areas"][key] = {"name": area["bindings"]["name"], "build": build, "attempts": attempts,
                                     "releaseId": item["id"], "state": item["state"],
                                     "manifestSha256": item["manifestSha256"], **checked}
            (args.output / f"{key}-release.json").write_text(json.dumps(release, indent=2), encoding="utf-8")
            print(f"{key}: {item['state']} {item['manifestSha256']}", flush=True)
        if len(result["areas"]) < len(commission["areas"]):
            time.sleep(5)
    result["checkedAt"] = datetime.now(timezone.utc).isoformat()
    result["state"] = "PACKAGES_VERIFIED" if len(result["areas"]) == len(commission["areas"]) else "INCOMPLETE"
    (args.output / "installed-verification.json").write_text(json.dumps(result, indent=2), encoding="utf-8")
    return 0 if result["state"] == "PACKAGES_VERIFIED" else 2


if __name__ == "__main__":
    raise SystemExit(main())
