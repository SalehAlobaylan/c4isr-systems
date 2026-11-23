"""Capture and rehearse a paired staging backup without restarting its readers."""

import argparse
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tarfile
import tempfile
import time
from urllib.parse import urlsplit
from uuid import uuid4

import requests

from capture_environment_candidate import container
from intake_environment import LibraryAPI
from paired_backup import client, digest, wait
from recovery_inventory import inventory, sql, verify_required

ROOT = Path(__file__).resolve().parents[2]
HEX = re.compile(r"[0-9a-f]{64}\Z")
SCHEMA = "emad.staging-environment-backup.v1"
EPHEMERAL_READ_TABLES = ("analyst_read_cursors", "analyst_read_snapshots")
RUNTIME_TABLES = {"platform_worker_status", *EPHEMERAL_READ_TABLES}
POLICY_TABLES = {"principals", "operators", "principal_grants", "principal_credentials", "sources", "source_admissions",
                 "retrieval_profiles", "retrieval_profile_revisions", "retrieval_execution_grants",
                 "geographic_environment_release_events"}

# Run with the installed API's existing object-store credentials. Return only
# bounded tar members over stdout. The host never needs a port on the source store.
READ_BUCKET = r'''
import boto3, hashlib, io, json, os, pathlib, sys, tarfile
from botocore.config import Config
def secret(name):
    return pathlib.Path(os.environ[name + "_FILE"]).read_text().strip()
s3 = boto3.client("s3", endpoint_url=os.environ["C4ISR_ARTIFACT_S3_ENDPOINT"],
    region_name=os.environ["C4ISR_ARTIFACT_S3_REGION"], config=Config(s3={"addressing_style":"path"}),
    aws_access_key_id=secret("C4ISR_ARTIFACT_S3_ACCESS_KEY"), aws_secret_access_key=secret("C4ISR_ARTIFACT_S3_SECRET_KEY"))
bucket = os.environ["C4ISR_ARTIFACT_S3_BUCKET"]
def listing():
    return sorted((x["Key"], x["Size"], x["ETag"]) for page in s3.get_paginator("list_objects_v2").paginate(Bucket=bucket)
                  for x in page.get("Contents", []))
before = listing()
if len(before)>20000 or sum(row[1] for row in before)>32<<30:
    raise ValueError("Installed bucket exceeds rehearsal budget")
with tarfile.open(fileobj=sys.stdout.buffer, mode="w|") as stream:
    for key, size, etag in before:
        if not size:
            raise ValueError("Unexpected empty stored object")
        response=s3.get_object(Bucket=bucket, Key=key, IfMatch=etag)
        if response["ContentLength"]!=size:
            raise ValueError("Stored object size changed")
        record=json.dumps({"key":key,"bytes":size,"mediaType":response.get("ContentType","application/octet-stream")}).encode()
        identity=hashlib.sha256(key.encode()).hexdigest()
        info=tarfile.TarInfo(identity+".json"); info.size=len(record); stream.addfile(info,io.BytesIO(record))
        info=tarfile.TarInfo(identity); info.size=size
        with response["Body"] as body:
            stream.addfile(info,body)
    if listing()!=before:
        raise ValueError("Object inventory changed during capture")
'''


def now():
    return datetime.now(timezone.utc)


def run(*args, **kwargs):
    return subprocess.run([str(x) for x in args], cwd=ROOT, check=True, timeout=300, **kwargs)


def save(path, value):
    with path.open("x", encoding="utf-8") as output:
        json.dump(value, output, indent=2)


def load(path):
    return json.loads(path.read_text(encoding="utf-8-sig"))


def receipt(path):
    return {"file": path.name, "sha256": digest(path), "bytes": path.stat().st_size}


def comparable(snapshot):
    # Heartbeats and temporary read sessions change independently of durable
    # records. Their table definitions and access rules are still compared.
    return {**snapshot, "tables": {k: v for k, v in snapshot["tables"].items() if k not in RUNTIME_TABLES}}


def capture_bucket(api, dest, omitted):
    dest.mkdir()
    objects = []
    with subprocess.Popen(["docker", "exec", api, "python", "-c", READ_BUCKET], stdout=subprocess.PIPE) as process:
        try:
            with tarfile.open(fileobj=process.stdout, mode="r|") as source:
                iterator = iter(source)
                for meta in iterator:
                    if not meta.isfile() or not meta.name.endswith(".json") or not 1 <= meta.size <= 8192:
                        raise ValueError("Invalid object receipt in staging snapshot")
                    record = json.load(source.extractfile(meta))
                    identity = hashlib.sha256(record["key"].encode()).hexdigest()
                    item = next(iterator, None)
                    if (meta.name != identity + ".json" or item is None or not item.isfile() or
                            item.name != identity or item.size != record["bytes"] or not 1 <= item.size <= 1 << 30):
                        raise ValueError("Invalid object identity in staging snapshot")
                    if record["key"] in omitted:
                        continue
                    path = dest / identity
                    hasher = hashlib.sha256()
                    with path.open("xb") as output, source.extractfile(item) as body:
                        while chunk := body.read(1 << 20):
                            output.write(chunk)
                            hasher.update(chunk)
                    if path.stat().st_size != item.size:
                        raise ValueError("Truncated object in staging snapshot")
                    objects.append({**record, "file": "objects/" + identity, "sha256": hasher.hexdigest()})
            if process.wait(timeout=45):
                raise RuntimeError("Source object store did not complete its snapshot")
        except BaseException:
            process.kill()
            raise
    return objects


def verify_navigation(directory, objects):
    indexed = {item["key"]: item for item in objects}
    parents = set()
    for key, item in indexed.items():
        match = re.fullmatch(r"installation/sources/([0-9a-f]{64})/receipts/([0-9a-f]{64})\.json", key)
        if not match:
            continue
        parent, digest_id = match.groups()
        if digest_id != item["sha256"]:
            raise ValueError("Retained navigation receipt identity changed")
        record = load(directory / item["file"])
        if not record.get("archived") or not record.get("chunks"):
            continue
        hasher = hashlib.sha256()
        length = 0
        for chunk in record["chunks"]:
            if not chunk["key"].startswith(f"installation/sources/{parent}/"):
                raise ValueError("Navigation chunk belongs to a different parent")
            stored = indexed[chunk["key"]]
            if any(stored[field] != chunk[field] for field in ("sha256", "bytes")):
                raise ValueError("Retained navigation chunk identity changed")
            with (directory / stored["file"]).open("rb") as source:
                while block := source.read(1 << 20):
                    hasher.update(block)
                    length += len(block)
        if record["sha256"] != parent or hasher.hexdigest() != parent or record["bytes"] != length:
            raise ValueError("Retained navigation parent identity changed")
        parents.add(parent)
    return sorted(parents)


def capture(candidate, directory):
    from recovery_inventory import restore_policy

    started = now()
    pg, api = (candidate["services"][key]["name"] for key in ("postgres-1", "api-1"))
    def installed_services():
        observed = {}
        for key, expected in candidate["services"].items():
            actual = container(expected["name"])
            if any(actual[field] != expected[field] for field in ("image", "configurationSha256")):
                raise ValueError(f"Installed {key} differs from candidate")
            observed[key] = actual
        return observed

    services = installed_services()
    before = inventory(pg)
    omitted, _ = restore_policy(before, started)
    dump = directory / "database.dump"
    with dump.open("xb") as stream:
        exclusions = [f"--exclude-table-data=public.{table}" for table in EPHEMERAL_READ_TABLES]
        run("docker", "exec", pg, "pg_dump", "-Fc", "--no-owner", *exclusions, "-U", "c4isr", "-d", "c4isr", stdout=stream)
    objects = capture_bucket(api, directory / "objects", omitted)
    after = inventory(pg)
    if installed_services() != services:
        raise ValueError("An installed service restarted during the paired capture")
    if comparable(before) != comparable(after):
        changed = sorted(key for key in before["tables"] if key not in RUNTIME_TABLES and before["tables"][key] != after["tables"].get(key))
        raise ValueError("Snapshot changed during capture. Changed tables: " + ", ".join(changed))
    verify_required(after, objects, now())
    nav = verify_navigation(directory, objects)
    if not nav:
        raise ValueError("The paired snapshot contains no verified retained navigation parents")
    for path, data in ((directory / "inventory.json", after), (directory / "candidate.json", candidate)):
        save(path, data)
    save(directory / "manifest.json", {"schema": SCHEMA, "startedAt": started.isoformat(), "closedAt": now().isoformat(),
         "cutoff": started.isoformat(), "database": receipt(dump), "inventory": receipt(directory / "inventory.json"),
         "candidate": receipt(directory / "candidate.json"), "runtimeTablesExcludedFromComparison": sorted(RUNTIME_TABLES),
         "ephemeralReadTablesOmitted": list(EPHEMERAL_READ_TABLES),
         "navigationParents": nav, "objects": objects})
    return len(objects)


def validated_backup(directory, expected=None):
    if expected and (not HEX.fullmatch(expected) or digest(directory / "manifest.json") != expected):
        raise ValueError("Backup manifest differs from its trusted SHA-256")
    manifest = load(directory / "manifest.json")
    if manifest["schema"] != SCHEMA:
        raise ValueError("Unsupported staging backup format")
    if not 1 <= len(manifest["objects"]) <= 20000 or sum(item["bytes"] for item in manifest["objects"]) > 32 << 30:
        raise ValueError("Backup exceeds the rehearsal object budget")
    for label, name in (("database", "database.dump"), ("inventory", "inventory.json"), ("candidate", "candidate.json")):
        path = directory / name
        if not path.resolve().is_relative_to(directory) or manifest[label] != receipt(path):
            raise ValueError(f"The {label} backup member differs from its receipt")
    keys = set()
    for item in manifest["objects"]:
        identity = hashlib.sha256(item["key"].encode()).hexdigest()
        if item["key"] in keys or item["file"] != "objects/" + identity:
            raise ValueError("Duplicate or invalid backup object")
        keys.add(item["key"])
        path = directory / item["file"]
        if (not path.resolve().is_relative_to(directory) or not 1 <= item["bytes"] <= 1 << 30
                or path.stat().st_size != item["bytes"] or digest(path) != item["sha256"]):
            raise ValueError("Backup object differs from its receipt")
    return manifest


def serve_probe(api, release):
    path = "/environment-releases/" + release["id"]
    detail = api.call("GET", path)
    if detail["manifestSha256"] != release["sha256"]:
        raise ValueError("Restored API returned a different release manifest")
    with requests.get(api.origin + path + "/manifest", headers=api.headers, timeout=15, allow_redirects=False) as response:
        response.raise_for_status()
        if hashlib.sha256(response.content).hexdigest() != release["sha256"]:
            raise ValueError("Restored manifest bytes differ from their trusted digest")
    assets = {path + "/prepared": release.get("preparedSha256")}
    after = ""
    while True:
        page = api.call("GET", path + "/displays" + ("?after=" + after if after else ""))
        for display in page["items"]:
            assets["/environment-displays/" + display["id"] + "/content"] = display["sha256"]
        after = page.get("next")
        if not after:
            break
    proof = []
    for endpoint, expected in assets.items():
        with requests.get(api.origin + endpoint, headers=api.headers, stream=True, timeout=(5, 60), allow_redirects=False) as response:
            response.raise_for_status()
            tag = response.headers.get("ETag", "").strip('"')
            length = int(response.headers.get("Content-Length", 0))
            hasher = hashlib.sha256()
            total = 0
            for block in response.iter_content(1 << 20):
                total += len(block)
                if total > length:
                    raise ValueError("Restored read exceeded its declared length")
                hasher.update(block)
        if not HEX.fullmatch(tag) or hasher.hexdigest() != tag or expected and tag != expected or not 0 < total == length:
            raise ValueError("Restored object differs from the API identity")
        proof.append({"path": endpoint, "sha256": tag, "bytes": total})
    for suffix in ("/manifest", "/prepared"):
        with requests.get(api.origin + path + suffix, timeout=10, allow_redirects=False) as response:
            if response.status_code not in (401, 403, 404):
                raise ValueError("Unauthenticated restored data is visible")
    return {"id": release["id"], "manifestSha256": release["sha256"], "objects": proof}


def restore(directory, settings, token, report, previous_api=None, expected=None, exercise=None):
    from recovery_inventory import restore_policy

    started, timer = now(), time.monotonic()
    manifest = validated_backup(directory, expected)
    candidate = load(directory / "candidate.json")
    pg = candidate["services"]["postgres-1"]["name"]
    # Compare policy-filtered objects against the live original immediately
    # before populating the clone. Denied or missing keys remain absent.
    captured = load(directory / "inventory.json")
    current = inventory(pg)
    def assert_policy(snapshot):
        if any(snapshot["tables"].get(table) != captured["tables"].get(table) for table in POLICY_TABLES):
            raise ValueError("Live source or access policy changed. Take a new snapshot before serving the clone")
    assert_policy(current)
    capture_omit, _ = restore_policy(captured, now())
    live_omit, live_required = restore_policy(current, now())
    current_keys = {row["key"] for row in current["objects"]}
    captured_keys = {row["key"] for row in captured["objects"]}
    removed = captured_keys - current_keys
    omitted = capture_omit | live_omit | removed
    usable = [item for item in manifest["objects"] if item["key"] not in omitted]
    scoped_before = {item["key"] for item in captured["objects"] if item["required"]}
    live_subset = {k: v for k, v in live_required.items() if k in scoped_before}
    remaining = {item["key"]: item for item in usable}
    for key, item in live_subset.items():
        if key not in remaining or any(remaining[key][field] != item[field] for field in ("sha256", "bytes")):
            raise ValueError("Current required object is absent or changed in this recovery set")
    if verify_navigation(directory, usable) != manifest["navigationParents"]:
        raise ValueError("Retained navigation parents differ from the paired capture")
    if previous_api is not None and (not re.fullmatch(r"sha256:[0-9a-f]{64}", previous_api) or previous_api == candidate["apiImage"]):
        raise ValueError("Supply the immutable image ID of a different, previously installed API")

    project = "emad-paired-" + uuid4().hex[:10]
    postgres, database = project + "-postgres-1", "paired_rehearsal"
    # Settings are supplied by the operator. The dump and evidence never carry
    # database, object-store or reviewer credentials in plaintext.
    settings = settings.resolve()
    source_settings = settings.read_text(encoding="utf-8-sig")
    values = dict(re.findall(r"^([A-Z_][A-Z_0-9]*)=(.*)$", source_settings, re.M))
    secret_dir = Path(values["STAGING_SECRET_DIR"].strip("'\""))
    if not secret_dir.is_absolute():
        raise ValueError("Use an absolute STAGING_SECRET_DIR for the restore rehearsal")
    bucket = values["EMAD_BUCKET"].strip("'\"")
    endpoint = "http://127.0.0.1:8498"
    resource = candidate["identity"]["resource"]
    with tempfile.TemporaryDirectory(dir=report.parent, prefix="paired-config-") as tmp:
        config = Path(tmp)
        url = (secret_dir / "database_url").read_text().strip()
        address = urlsplit(url)
        if address.scheme not in ("postgresql", "postgres") or address.hostname != "postgres" or address.path != "/c4isr":
            raise ValueError("The recovery stack requires a Compose-local PostgreSQL connection")
        cloned_url, replaced = re.subn(r"/c4isr(?=\?|$)", "/" + database, url)
        if replaced != 1:
            raise ValueError("Cannot bind a separate rehearsal database")
        (config / "database_url").write_text(cloned_url)
        overlay = config / "override.yml"
        overlay.write_text(f'''services:
  postgres:
    image: {candidate['services']['postgres-1']['image']}
  api:
    image: {candidate['apiImage']}
    ports: !override ["127.0.0.1:8498:8080"]
  garage:
    image: {candidate['services']['garage-1']['image']}
    ports: ["127.0.0.1:8499:3900"]
secrets:
  database_url:
    file: '{(config/'database_url').as_posix()}'
''', encoding="utf-8")
        compose = ("docker", "compose", "--env-file", settings, "-p", project, "-f", "deployments/staging/docker-compose.yml",
                   "-f", "deployments/staging/delivery.yml", "-f", overlay)
        try:
            run(*compose, "up", "-d", "--no-build", "postgres", "garage")
            wait(lambda: run("docker", "exec", postgres, "pg_isready", "-h", "127.0.0.1", "-U", "c4isr",
                             stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode == 0)
            sql(postgres, "c4isr", "CREATE ROLE emad_data NOLOGIN NOSUPERUSER NOBYPASSRLS")
            run("docker", "exec", postgres, "createdb", "-U", "c4isr", "-T", "template0", database)
            with (directory / "database.dump").open("rb") as dump:
                run("docker", "exec", "-i", postgres, "pg_restore", "-U", "c4isr", "-d", database, "--no-owner", "--exit-on-error", stdin=dump)
            restored = inventory(postgres, database)
            if comparable(restored) != comparable(captured):
                raise ValueError("Restored inventory or ACLs differ from the paired database")
            omitted_reads = manifest.get("ephemeralReadTablesOmitted")
            if omitted_reads is not None and (omitted_reads != list(EPHEMERAL_READ_TABLES) or
                                              any(restored["tables"][table]["rows"] for table in EPHEMERAL_READ_TABLES)):
                raise ValueError("Restored temporary read tables must be empty")
            if omitted & scoped_before:
                # Stale copied authorizations must not be opened even inside an
                # isolated API when eligible retained dependencies changed.
                raise ValueError("Live policy withdrew captured dependencies. Take a fresh paired snapshot before serving the clone")
            s3 = client({"region": "garage", "accessKey": (secret_dir / "object_access_key").read_text().strip(),
                         "secretKey": (secret_dir / "object_secret_key").read_text().strip()}, "http://127.0.0.1:8499")
            wait(lambda: s3.head_bucket(Bucket=bucket))
            for item in usable:
                with (directory / item["file"]).open("rb") as stream:
                    s3.put_object(Bucket=bucket, Key=item["key"], Body=stream, ContentType=item["mediaType"])
                response = s3.get_object(Bucket=bucket, Key=item["key"])
                with response["Body"] as body:
                    if response["ContentLength"] != item["bytes"] or hashlib.file_digest(body, "sha256").hexdigest() != item["sha256"]:
                        raise ValueError("Restored object failed the exact readback check")
            assert_policy(inventory(pg))
            run(*compose, "up", "-d", "--no-deps", "--no-build", "api")
            wait(lambda: requests.get(endpoint + "/health", timeout=3).ok)
            api = LibraryAPI(endpoint, token, resource["workspace"], resource["environment"])
            rows = {item["entity_id"]: item for item in captured["objects"] if item["required"] and item["family"] == "geographic"}
            pins = []
            for release in candidate["releases"]:
                asset_id = api.call("GET", "/environment-releases/" + release["id"])["bundleId"]
                pins.append({**release, "preparedSha256": rows[asset_id]["sha256"]})
            releases = [serve_probe(api, release) for release in pins]
            forward = None
            if previous_api:
                rollback = config / "previous.yml"
                rollback.write_text(f"services:\n  api:\n    image: {previous_api}\n", encoding="utf-8")
                try:
                    run(*compose, "-f", rollback, "up", "-d", "--force-recreate", "--no-deps", "--no-build", "api")
                    wait(lambda: requests.get(endpoint + "/health", timeout=3).ok, seconds=30)
                    if container(project + "-api-1")["image"] != previous_api:
                        raise ValueError("Previous API image identity changed")
                    previous = [serve_probe(api, release) for release in pins]
                    if previous != releases:
                        raise ValueError("Previous API returned different object identities")
                    forward = {"previousImage": previous_api, "previousReads": "PASS"}
                except (KeyError, OSError, RuntimeError, ValueError, subprocess.SubprocessError, requests.RequestException) as error:
                    forward = {"previousImage": previous_api, "previousReads": "FAIL", "reason": type(error).__name__}
                run(*compose, "up", "-d", "--force-recreate", "--no-deps", "--no-build", "api")
                wait(lambda: requests.get(endpoint + "/health", timeout=3).ok)
                if container(project + "-api-1")["image"] != candidate["apiImage"] or [serve_probe(api, r) for r in pins] != releases:
                    raise ValueError("Forward recovery did not reproduce the current API reads")
                forward.update(currentImage=candidate["apiImage"], forwardReads="PASS", migrationAction="none")
            if exercise is not None:
                exercise(api, postgres, database, candidate, config, compose)
            assert_policy(inventory(pg))
            result = {"schema": "emad.staging-environment-restore.v1", "status": "PASS", "installationSha256": candidate["installationSha256"],
                      "targets": [{"id": r["id"], "sha256": r["sha256"]} for r in candidate["releases"]],
                      "startedAt": started.isoformat(), "elapsedSeconds": round(time.monotonic() - timer, 3),
                      "cutoff": manifest["cutoff"], "backupManifestSha256": digest(directory / "manifest.json"),
                      "tables": len(comparable(restored)["tables"]), "securityDefinitions": restored["securityDefinitions"],
                      "restoredObjects": len(usable), "restoredBytes": sum(item["bytes"] for item in usable),
                      "navigationParents": manifest["navigationParents"], "policyOmittedKeys": len(omitted), "releases": releases,
                      "previousApiRehearsal": forward}
        finally:
            run(*compose, "down", "--volumes")
    save(report, result)
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    actions = parser.add_subparsers(dest="action", required=True)
    collect = actions.add_parser("capture")
    collect.add_argument("candidate", type=Path)
    collect.add_argument("directory", type=Path)
    inspect = actions.add_parser("verify")
    inspect.add_argument("directory", type=Path)
    inspect.add_argument("--manifest-sha256")
    rehearse = actions.add_parser("restore")
    rehearse.add_argument("directory", type=Path)
    rehearse.add_argument("settings", type=Path)
    rehearse.add_argument("reviewer_token", type=Path)
    rehearse.add_argument("report", type=Path)
    rehearse.add_argument("--previous-api-image")
    rehearse.add_argument("--manifest-sha256")
    args = parser.parse_args()
    try:
        directory = args.directory.resolve()
        if not directory.is_relative_to(ROOT / "private"):
            raise ValueError("Use a path in the repository's private evidence directory")
        if args.action == "capture":
            directory.mkdir(parents=True, exist_ok=False)
            count = capture(load(args.candidate), directory)
        elif args.action == "verify":
            count = len(validated_backup(directory, args.manifest_sha256)["objects"])
        else:
            if args.report.exists() or not args.report.resolve().is_relative_to(ROOT / "private"):
                raise ValueError("Use a new private report path")
            result = restore(directory, args.settings, args.reviewer_token, args.report.resolve(), args.previous_api_image, args.manifest_sha256)
            count = result["restoredObjects"]
        print(json.dumps({"status": "PASS", "objects": count, "manifestSha256": digest(directory / "manifest.json")}))
    except (ValueError, KeyError, OSError, RuntimeError, subprocess.SubprocessError, tarfile.TarError) as error:
        parser.exit(2, f"{error}\n")


if __name__ == "__main__":
    main()
