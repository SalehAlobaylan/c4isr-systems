"""Verify and record the independent operator's receiving evidence for F03."""

import argparse
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import re

from environment_readiness_ledger import HASH, ROLES, load, stamp
from receive_environment_release import LibraryAPI, receive


RECEIPT_SCHEMA = "emad.environment-independent-receiving.v1"
RELEASE_ID = re.compile(r"georelease_[a-z0-9]{26}\Z")
DISPLAY_CONTENT = re.compile(r"/environment-displays/(geodisplay_[a-f0-9]{64})/content\Z")


def digest(document):
    payload = json.dumps(document, sort_keys=True, separators=(",", ":"), allow_nan=False)
    return hashlib.sha256(payload.encode()).hexdigest()


def review_pins(candidate, preflight):
    if (candidate["schema"] != "emad.environment-installed-candidate.v1" or
            preflight["schema"] != "emad.staging-release-review-preflight.v1" or preflight["status"] != "PASS" or
            digest(candidate["identity"]) != candidate["installationSha256"] or
            preflight["installationSha256"] != candidate["installationSha256"]):
        raise ValueError("Review preflight and installed candidate must share the same verified installation")
    releases = candidate["releases"]
    if (len(releases) != 3 or {r["role"] for r in releases} != ROLES or
            len({r["id"] for r in releases}) != 3 or len(preflight["releases"]) != 3 or
            candidate["identity"]["releasePins"] != [{k: v for k, v in r.items() if k != "state"} for r in releases]):
        raise ValueError("Review requires the three exact installed release pins")
    verified = {r["id"]: r for r in preflight["releases"]}
    for release in releases:
        if (not RELEASE_ID.fullmatch(release["id"]) or not HASH.fullmatch(release["sha256"]) or
                (verified[release["id"]]["manifestSha256"], verified[release["id"]]["role"]) !=
                (release["sha256"], release["role"])):
            raise ValueError("The verified releases differ from the installed manifest pins")

    riyadh = next(r for r in releases if r["role"] == "riyadh-detailed")
    preflight_riyadh = verified[riyadh["id"]]
    archive_path = f"/environment-releases/{riyadh['id']}/prepared"
    archives, displays = [], {}
    for item in preflight_riyadh["objects"]:
        if not HASH.fullmatch(item["sha256"]) or type(item["bytes"]) is not int or item["bytes"] <= 0:
            raise ValueError("The verified archive and displays require full digests and byte lengths")
        if item["path"] == archive_path:
            archives.append(item)
            continue
        match = DISPLAY_CONTENT.fullmatch(item["path"])
        if not match or match[1] in displays:
            raise ValueError("The review preflight contains an invalid or duplicate display pin")
        displays[match[1]] = (item["sha256"], item["bytes"])
    if len(archives) != 1 or archives[0]["sha256"] != riyadh["preparedSha256"] or not displays:
        raise ValueError("The review preflight must pin the Riyadh archive and all reviewed displays")
    return riyadh, archives[0], displays, preflight_riyadh["manifestLayers"]


def receiver_identity(api, resource):
    session = api.call("GET", "/auth/me")
    current = session["resource"]
    if (current["workspaceId"], current["environmentId"], current["partition"]) != (
            resource["workspace"], resource["environment"], "PUBLIC_EVIDENCE"):
        raise ValueError("Use the receiver credential for the exact public evidence workspace")
    permissions = set(session["permissions"])
    operator = session["operator"]["id"]
    if (session["principal"]["kind"] != "human" or not operator or
            not {"environments.read", "environments.download"} <= permissions or
            {"environments.qualify", "environments.release"} & permissions):
        raise ValueError("Use a separate human receiver credential with download access and no review authority")
    return operator


def verify_walkthrough(walkthrough, release, operator, layers):
    if ((walkthrough["releaseId"], walkthrough["manifestSha256"], walkthrough["receiverOperatorId"]) !=
            (release["id"], release["sha256"], operator) or walkthrough["terrainOpened"] is not True):
        raise ValueError("The receiver's walkthrough must name this manifest, their operator ID and the opened terrain")
    usable = walkthrough["usableLayers"]
    limits = walkthrough["limitations"]
    if (not isinstance(usable, list) or not usable or any(layer not in layers for layer in usable) or
            not isinstance(limits, list) or not limits or
            not all(isinstance(note, str) and note.strip() for note in limits)):
        raise ValueError("Record the usable manifest layers and their limitations in the receiver's own words")


def verify_download(verified, release, archive, displays):
    display_receipts = verified["displays"]
    if (verified["schema"] != "emad.environment-receiver-check.v1" or verified["status"] != "PASS" or
            (verified["releaseId"], verified["manifestSha256"], verified["preparedSha256"], verified["preparedBytes"]) !=
            (release["id"], release["sha256"], archive["sha256"], archive["bytes"]) or
            len(display_receipts) != len(displays) or
            {item["id"]: (item["sha256"], item["bytes"]) for item in display_receipts} != displays):
        raise ValueError("The receiver receipt must match the prepared archive and every verified display")


def record_receiving(api, candidate, preflight, walkthrough, destination):
    release, archive, displays, layers = review_pins(candidate, preflight)
    resource = candidate["identity"]["resource"]
    operator = receiver_identity(api, resource)
    verify_walkthrough(walkthrough, release, operator, layers)
    verified = receive(api, release["id"], release["sha256"], archive["sha256"], destination,
                       [(display_id, pin[0]) for display_id, pin in displays.items()])
    verify_download(verified, release, archive, displays)
    return {"schema": RECEIPT_SCHEMA, "installationSha256": candidate["installationSha256"], "apiOrigin": api.origin,
            "receiverOperatorId": operator, "walkthrough": walkthrough, "download": verified}


def review_history(api, release):
    endpoint = f"/environment-releases/{release['id']}"
    detail = api.call("GET", endpoint)
    history = api.call("GET", endpoint + "/decisions")
    if (detail["state"] != "RELEASED" or detail["manifestSha256"] != release["sha256"] or
            len(history) < 2 or [event["state"] for event in history[-2:]] != ["VERIFIED", "RELEASED"]):
        raise ValueError(f"The {release['role']} release needs recorded verification and release decisions")
    events = history[-2:]
    if (any(event["manifestSha256"] != release["sha256"] or not event.get("humanId") or
            not event.get("note", "").strip() for event in events) or
            stamp(events[0]["occurredAt"]) > stamp(events[1]["occurredAt"])):
        raise ValueError("Each decision must name an authorized actor and bind exact manifest evidence")
    return events


def verify_receiving(api, candidate, preflight, receipt, receipt_file):
    release, archive, displays, layers = review_pins(candidate, preflight)
    if (receipt["schema"] != RECEIPT_SCHEMA or receipt["installationSha256"] != candidate["installationSha256"] or
            receipt["apiOrigin"] != api.origin):
        raise ValueError("The receiver receipt belongs to a different installation or API origin")
    operator = receipt["receiverOperatorId"]
    if not isinstance(operator, str) or not operator.strip():
        raise ValueError("The receiver receipt requires an authenticated operator")
    verify_walkthrough(receipt["walkthrough"], release, operator, layers)
    verified = receipt["download"]
    verify_download(verified, release, archive, displays)
    decisions = {pin["id"]: review_history(api, pin) for pin in candidate["releases"]}
    riyadh_decisions = decisions[release["id"]]
    if operator in {event["humanId"] for event in riyadh_decisions}:
        raise ValueError("Riyadh requires a different human receiver from its release reviewers")
    checked = stamp(verified["checkedAt"])
    if not stamp(riyadh_decisions[-1]["occurredAt"]) <= checked <= datetime.now(timezone.utc):
        raise ValueError("Receiving must be recorded after the Riyadh release decision")

    evidence = {"receiver": receipt, "decisions": decisions}
    check = {"status": "PASS", "installationSha256": candidate["installationSha256"],
             "targets": [{k: pin[k] for k in ("id", "sha256")} for pin in candidate["releases"]],
             "evidenceSha256": digest(evidence), "sources": [receipt_file.name]}
    return {"independentReceiver": check, "f03ReviewEvidence": evidence}


def confirmed_receipt(path, expected_sha256):
    receipt, identity = load(path)
    if identity["sha256"] != expected_sha256.lower():
        raise ValueError("Receipt SHA-256 differs from the digest confirmed by the receiver")
    return receipt


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    actions = parser.add_subparsers(dest="action", required=True)
    capture = actions.add_parser("receive", help="record the walkthrough with the receiver's credential")
    verify = actions.add_parser("verify", help="check the receipt against release decisions with the reviewer's credential")
    for action in (capture, verify):
        for option in ("candidate", "preflight", "token-file", "output"):
            action.add_argument(f"--{option}", type=Path, required=True)
        action.add_argument("--api", required=True)
    capture.add_argument("--walkthrough", type=Path, required=True)
    capture.add_argument("--archive", type=Path, required=True)
    verify.add_argument("--receipt", type=Path, required=True)
    verify.add_argument("--receiver-confirmed-sha256", required=True,
                        help="Exact receipt SHA-256 provided directly by the authenticated receiver")
    verify.add_argument("--checks", type=Path, required=True, help="Existing F09 technical checks")
    args = parser.parse_args()
    try:
        if args.output.exists() or args.action == "receive" and args.output.resolve() == args.archive.resolve():
            raise ValueError("Choose a new evidence path separate from the archive")
        candidate, _ = load(args.candidate)
        preflight, _ = load(args.preflight)
        resource = candidate["identity"]["resource"]
        api = LibraryAPI(args.api, args.token_file, resource["workspace"], resource["environment"])
        if args.action == "receive":
            walkthrough, _ = load(args.walkthrough)
            result = record_receiving(api, candidate, preflight, walkthrough, args.archive)
        else:
            receipt = confirmed_receipt(args.receipt, args.receiver_confirmed_sha256)
            checks, _ = load(args.checks)
            result = {**checks, **verify_receiving(api, candidate, preflight, receipt, args.receipt)}
        with args.output.open("x", encoding="utf-8") as output:
            json.dump(result, output, indent=2, allow_nan=False)
        if args.action == "receive":
            _, identity = load(args.output)
            print(json.dumps({"receipt": str(args.output), "sha256": identity["sha256"]}))
        else:
            print(args.output)
    except (KeyError, TypeError, ValueError, OSError, RuntimeError) as error:
        parser.exit(2, f"{error}\n")


if __name__ == "__main__":
    main()
