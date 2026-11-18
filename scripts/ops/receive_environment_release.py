"""Discover and verify an exact released environment with receiver credentials."""

import argparse
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import sys
import tempfile
import zipfile

import requests

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "data"))

from check_installed_environment_library import LibraryAPI, download, object_digest, read_range
import environment_kit


def discover(api, release_id, manifest_digest):
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
    return release


def verify_display(api, release_id, manifest_digest, display_id, expected_sha, destination=None):
    path = f"/environment-displays/{display_id}"
    display = api.call("GET", path + "/manifest")
    size = display["bytes"]
    if ((display["id"], display["releaseId"], display["manifestSha256"], display["sha256"], display["state"])
            != (display_id, release_id, manifest_digest, expected_sha, "READY") or not 1 <= size <= 32 << 20):
        raise ValueError("The display manifest differs from the released display pin")
    digest = hashlib.sha256()
    received = 0
    temporary = None
    stream = None
    if destination is not None:
        destination = Path(destination)
        destination.parent.mkdir(parents=True, exist_ok=True)
        temporary = destination.with_suffix(destination.suffix + ".partial")
        stream = temporary.open("wb")
    try:
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
                if stream is not None:
                    stream.write(block)
        if received != size or digest.hexdigest() != expected_sha:
            raise ValueError("The display download failed its length or SHA-256 check")
        if stream is not None:
            stream.flush()
            os.fsync(stream.fileno())
            stream.close()
            stream = None
            os.replace(temporary, destination)
    finally:
        if stream is not None:
            stream.close()
        if temporary is not None:
            temporary.unlink(missing_ok=True)
    return {"id": display_id, "recipe": display["recipe"], "sha256": expected_sha, "bytes": size,
            **({"purpose": display["purpose"]["purpose"],
                "purposeMeaning": display["purpose"].get("purposeMeaning"),
                "limitations": display["purpose"].get("limitations") or []}
               if display.get("purpose") else {})}


def receive(api, release_id, manifest_digest, prepared_digest, output, display_pins=()):
    release = discover(api, release_id, manifest_digest)
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


def receive_kit(api, release_id, manifest_digest, prepared_digest, output, display_pins,
                inventory_sha, purpose, terms, now, sample, feature=None):
    """Publish an offline kit atomically and prove a native read from it."""
    release = discover(api, release_id, manifest_digest)
    if not re.fullmatch(r"[a-f0-9]{64}", inventory_sha or ""):
        raise ValueError("A trusted kit inventory SHA-256 is required")
    if not terms.get("expiresAt"):
        raise ValueError("The issued terms must state an expiry before a kit can be published")
    if not any(grant.get("scope") == "read-native" and not grant.get("revoked")
               for grant in terms.get("grants", [])):
        raise ValueError("The issued terms do not grant native reads")
    if environment_kit._instant(terms["expiresAt"]) <= environment_kit._instant(now):
        raise ValueError("The issued terms expired before this kit was assembled")
    if output.exists():
        checked = environment_kit.verify(output, inventory_sha, now)
        return {"schema": "emad.environment-receiver-check.v1", "status": "PASS",
                "resumed": True, "releaseId": release_id, "kit": checked}
    archive = read_range(api, release_id)
    if prepared_digest is not None and archive != prepared_digest:
        raise ValueError("The prepared bytes differ from the supplied SHA-256")
    trusted = prepared_digest or archive
    output = Path(output)
    output.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix=".kit-download-", dir=output.parent) as location:
        work = Path(location)
        selected = []
        for display_id, sha in display_pins:
            destination = work / "displays" / f"{display_id}.glb"
            selected.append(verify_display(api, release_id, manifest_digest, display_id, sha,
                                           destination))
        result = download(api, release_id, work / "prepared.zip", manifest_digest, trusted)
        document = environment_kit.inventory_document(
            release, selected,
            {"sha256": result["preparedSha256"], "bytes": result["preparedBytes"]},
            purpose=purpose, terms=terms)
        if environment_kit.digest(document) != inventory_sha:
            raise ValueError("The kit inventory differs from the independently supplied SHA-256")
        sources = {item["id"]: work / item["path"] for item in document["displays"]}
        environment_kit.publish(output, document, work / "prepared.zip", sources)
    checked = environment_kit.verify(output, inventory_sha, now)
    sampled = environment_kit.native_read(output, sample[0], sample[1], sample[2],
                                          now=now, inventory_sha=inventory_sha)
    from environment_reader import Reader
    looked_up = Reader(output / "prepared.zip").feature(feature[0], feature[1]) if feature else None
    manifest = release["manifest"]
    return {"schema": "emad.environment-receiver-check.v1", "status": "PASS",
            "resumed": False, "checkedAt": datetime.now(timezone.utc).isoformat(),
            "releaseId": release_id, "manifestSha256": manifest_digest,
            "preparedSha256": result["preparedSha256"], "preparedBytes": result["preparedBytes"],
            "profileId": manifest["profileId"], "areaRevisionId": manifest["areaRevisionId"],
            "toolchainSha256": manifest["toolchainSha256"],
            "physicalAccuracy": manifest["quality"]["physicalAccuracy"],
            "unsupported": manifest["quality"]["unsupported"],
            "inputCount": len(manifest["inputs"]), "displays": checked["displays"],
            "kit": checked, "sample": sampled, "feature": looked_up}


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
    parser.add_argument("--kit", action="store_true",
                        help="Publish an offline kit directory instead of one prepared archive file")
    parser.add_argument("--kit-inventory-sha256",
                        help="Trusted inventory digest obtained through a separate channel")
    parser.add_argument("--kit-purpose", default="native-inspection")
    parser.add_argument("--kit-terms", type=Path, help="Issued terms JSON used for the kit")
    parser.add_argument("--kit-now", help="Evaluation instant; required with issued terms")
    parser.add_argument("--kit-sample", metavar="MEMBER:ROW:COLUMN",
                        help="Native sample that proves the assembled kit opens")
    parser.add_argument("--kit-feature", metavar="LAYER:SOURCE_ID",
                        help="Feature lookup that proves vector reads from the kit")
    args = parser.parse_args()
    if not re.fullmatch(r"georelease_[a-z0-9]{26}", args.release) or any(
        not re.fullmatch(r"[a-f0-9]{64}", item) for item in (args.manifest_sha256, args.prepared_sha256 or args.manifest_sha256)
    ):
        parser.error("Supply the full release and manifest identities")
    if args.kit and (not args.prepared_sha256 or not args.kit_inventory_sha256 or not args.kit_terms
                     or not args.kit_now or not args.kit_sample
                     or not re.fullmatch(r"[a-f0-9]{64}", args.kit_inventory_sha256)):
        parser.error("A kit requires the trusted prepared and inventory digests, terms, an "
                     "evaluation instant and a sample")
    if args.kit and not re.fullmatch(r"[^:\s]+:\d+:\d+", args.kit_sample):
        parser.error("The kit sample must be MEMBER:ROW:COLUMN")
    if args.kit and args.kit_feature and not re.fullmatch(r"[^:\s]+:[^:\s]+", args.kit_feature):
        parser.error("The kit feature must be LAYER:SOURCE_ID")
    display_pins = [item.split("=", 1) for item in args.display]
    if (len(display_pins) > 20 or any(not re.fullmatch(r"geodisplay_[a-f0-9]{64}=[a-f0-9]{64}", item) for item in args.display)
            or len({item[0] for item in display_pins}) != len(display_pins)):
        parser.error("Supply up to 20 distinct display IDs with full SHA-256 values")
    try:
        api = LibraryAPI(args.api, args.token_file, args.workspace, args.environment)
        if args.kit:
            terms = json.loads(args.kit_terms.read_text(encoding="utf-8"))
            member, row, column = args.kit_sample.split(":")
            feature = args.kit_feature.split(":", 1) if args.kit_feature else None
            result = receive_kit(api, args.release, args.manifest_sha256, args.prepared_sha256,
                                 args.output, display_pins, args.kit_inventory_sha256,
                                 args.kit_purpose, terms, args.kit_now,
                                 (member, int(row), int(column)), feature)
        else:
            result = receive(api, args.release, args.manifest_sha256, args.prepared_sha256, args.output, display_pins)
        print(json.dumps(result))
        return 0
    except (KeyError, ValueError, OSError, RuntimeError, zipfile.BadZipFile) as error:
        print(str(error), file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
