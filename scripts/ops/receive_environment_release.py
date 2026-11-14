"""Discover and verify an exact released environment with receiver credentials."""

import argparse
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import re
import sys
import zipfile

import requests

from check_installed_environment_library import LibraryAPI, download, object_digest, read_range


def verify_display(api, release_id, manifest_digest, display_id, expected_sha):
    path = f"/environment-displays/{display_id}"
    display = api.call("GET", path + "/manifest")
    size = display["bytes"]
    if ((display["id"], display["releaseId"], display["manifestSha256"], display["sha256"], display["state"])
            != (display_id, release_id, manifest_digest, expected_sha, "READY") or not 1 <= size <= 32 << 20):
        raise ValueError("The display manifest differs from the released display pin")
    digest = hashlib.sha256()
    received = 0
    with requests.get(api.origin + path + "/content", headers={**api.headers, "Accept-Encoding": "identity"},
                      timeout=(5, 60), allow_redirects=False, stream=True) as response:
        if (response.status_code != 200 or object_digest(response) != expected_sha or
                response.headers.get("Content-Encoding", "identity") != "identity" or
                response.headers.get("Content-Length") != str(size)):
            raise ValueError("The display download differs from its promised identity or size")
        for block in response.iter_content(256 << 10):
            received += len(block)
            if received > size:
                raise ValueError("The display exceeded its declared size")
            digest.update(block)
    if received != size or digest.hexdigest() != expected_sha:
        raise ValueError("The display download failed its length or SHA-256 check")
    return {"id": display_id, "recipe": display["recipe"], "sha256": expected_sha, "bytes": size}


def receive(api, release_id, manifest_digest, prepared_digest, output, display_pins=()):
    after = ""
    while True:
        page = api.call("GET", "/environment-releases" + ("?after=" + after if after else ""))
        listed = next((item for item in page["items"] if item["id"] == release_id), None)
        if listed:
            break
        next_page = page.get("next")
        if not next_page or next_page <= after:
            raise ValueError("The exact release is unavailable in the receiver's library")
        after = next_page
    release = api.call("GET", "/environment-releases/" + release_id)
    if any(item["state"] != "RELEASED" or item["manifestSha256"] != manifest_digest for item in (listed, release)):
        raise ValueError("The receiver did not discover the exact released manifest")
    archive = read_range(api, release_id)
    if prepared_digest is not None and archive != prepared_digest:
        raise ValueError("The prepared bytes differ from the supplied SHA-256")
    displays = [verify_display(api, release_id, manifest_digest, display_id, sha) for display_id, sha in display_pins]
    result = download(api, release_id, output, manifest_digest, archive)
    manifest = release["manifest"]
    return {"schema": "emad.environment-receiver-check.v1", "status": "PASS", "checkedAt": datetime.now(timezone.utc).isoformat(),
            "releaseId": release_id, "manifestSha256": manifest_digest, "preparedSha256": result["preparedSha256"],
            "preparedBytes": result["preparedBytes"], "profileId": manifest["profileId"], "areaRevisionId": manifest["areaRevisionId"],
            "toolchainSha256": manifest["toolchainSha256"], "physicalAccuracy": manifest["quality"]["physicalAccuracy"],
            "unsupported": manifest["quality"]["unsupported"], "inputCount": len(manifest["inputs"]), "displays": displays}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("output", type=Path)
    parser.add_argument("--api", required=True)
    parser.add_argument("--workspace", required=True)
    parser.add_argument("--environment", required=True)
    parser.add_argument("--token-file", type=Path, required=True)
    parser.add_argument("--release", required=True)
    parser.add_argument("--manifest-sha256", required=True)
    parser.add_argument("--prepared-sha256")
    parser.add_argument("--display", action="append", default=[], metavar="ID=SHA256",
                        help="Verify an independently supplied exact display pin alongside the archive")
    args = parser.parse_args()
    if not re.fullmatch(r"georelease_[a-z0-9]{26}", args.release) or any(
        not re.fullmatch(r"[a-f0-9]{64}", item) for item in (args.manifest_sha256, args.prepared_sha256 or args.manifest_sha256)
    ):
        parser.error("Supply the full release and manifest identities")
    display_pins = [item.split("=", 1) for item in args.display]
    if (len(display_pins) > 20 or any(not re.fullmatch(r"geodisplay_[a-f0-9]{64}=[a-f0-9]{64}", item) for item in args.display)
            or len({item[0] for item in display_pins}) != len(display_pins)):
        parser.error("Supply up to 20 distinct display IDs with full SHA-256 values")
    try:
        api = LibraryAPI(args.api, args.token_file, args.workspace, args.environment)
        result = receive(api, args.release, args.manifest_sha256, args.prepared_sha256, args.output, display_pins)
        print(json.dumps(result))
        return 0
    except (KeyError, ValueError, OSError, RuntimeError, zipfile.BadZipFile) as error:
        print(str(error), file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
