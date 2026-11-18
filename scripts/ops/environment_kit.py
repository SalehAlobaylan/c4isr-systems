"""Assemble and verify an offline environment kit.

A kit is a directory with an immutable inventory, the verified prepared archive
and any selected display bytes. It opens without a viewer, database or network.
The inventory digest is supplied through a separate trusted channel; it is never
taken from the kit itself.
"""
import argparse
import datetime as dt
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "data"))

SCHEMA = "emad.environment-kit-inventory.v1"
VERIFICATION_SCHEMA = "emad.environment-kit-verification.v1"
READER_CONTRACT = "emad.environment-reader.v1"
SUPPORTED_CONTRACTS = {READER_CONTRACT}
OPERATIONS = {"read-native"}


class KitError(ValueError):
    def __init__(self, code, detail):
        super().__init__(detail)
        self.code = code


def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":"), allow_nan=False)


def digest(value):
    return hashlib.sha256(canonical(value).encode()).hexdigest()


def file_digest(path):
    sha = hashlib.sha256()
    with Path(path).open("rb") as stream:
        for block in iter(lambda: stream.read(1 << 20), b""):
            sha.update(block)
    return sha.hexdigest()


def _safe(root, relative):
    path = (root / relative).resolve()
    root = root.resolve()
    if root != path and root not in path.parents:
        raise KitError("kit-corrupt", "Kit inventory names a path outside the kit")
    return path


def _instant(value):
    try:
        return dt.datetime.fromisoformat(value.replace("Z", "+00:00")).timestamp()
    except (AttributeError, TypeError, ValueError) as error:
        raise KitError("kit-expiry-unknown",
                       "The evaluation instant or stated expiry is not a recognized instant") from error


def inventory_document(release, displays, prepared, *, purpose, terms, operation="read-native",
                       contract=READER_CONTRACT):
    """Build the deterministic document whose SHA-256 is the trusted kit pin."""
    if operation not in OPERATIONS:
        raise KitError("kit-schema-unsupported", f"Unsupported kit operation: {operation}")
    if contract not in SUPPORTED_CONTRACTS:
        raise KitError("kit-schema-unsupported", f"Unsupported reader contract: {contract}")
    manifest = release["manifest"]
    if purpose not in ("native-inspection", "native-analysis"):
        raise KitError("kit-schema-unsupported", f"Unsupported kit purpose: {purpose}")
    display_list = []
    for item in sorted(displays, key=lambda value: value["id"]):
        entry = {
            "id": item["id"], "recipe": item["recipe"], "sha256": item["sha256"],
            "bytes": int(item["bytes"]),
            "path": item.get("path") or f"displays/{item['id']}.glb",
        }
        # Purpose metadata is additive: an older display without a known
        # descriptor keeps the original inventory shape.
        if item.get("purpose"):
            entry["purpose"] = item["purpose"]
            entry["purposeMeaning"] = item.get("purposeMeaning")
            entry["limitations"] = list(item.get("limitations") or [])
        display_list.append(entry)
    return {
        "schema": SCHEMA,
        "operation": operation,
        "purpose": purpose,
        "release": {"id": release.get("id") or release["ID"],
                    "manifestSha256": release.get("manifestSha256") or release["ManifestSHA256"],
                    "areaRevisionId": manifest["areaRevisionId"],
                    "profileId": manifest["profileId"],
                    "toolchainSha256": manifest.get("toolchainSha256") or manifest.get("toolchain")},
        "reader": {"contract": contract, "compatibility": "native-read-v1"},
        "prepared": {"path": "prepared.zip", "sha256": prepared["sha256"],
                     "bytes": int(prepared["bytes"])},
        "members": [{"name": item["name"], "layer": item["layer"],
                     "component": item.get("component"), "sha256": item["sha256"],
                     "bytes": int(item["bytes"])} for item in manifest["members"]],
        "displays": display_list,
        "terms": terms,
    }


def publish(output, document, prepared_source, display_sources=()):
    """Publish a complete kit atomically on the same filesystem.

    Every file is written and hashed in a private sibling directory. The final
    rename only happens after all files and the inventory exist, so an
    interrupted write never leaves a directory that looks like a kit.
    """
    output = Path(output)
    if output.exists():
        raise KitError("kit-output-exists", "A kit already exists at the output path")
    parent = output.parent
    parent.mkdir(parents=True, exist_ok=True)
    temporary = Path(tempfile.mkdtemp(prefix=f".{output.name}.partial-", dir=parent))
    try:
        shutil.copyfile(prepared_source, temporary / "prepared.zip")
        if file_digest(temporary / "prepared.zip") != document["prepared"]["sha256"]:
            raise KitError("kit-corrupt", "Prepared archive differs from the kit inventory")
        sources = dict(display_sources)
        for item in document["displays"]:
            if item["id"] not in sources:
                raise KitError("kit-display-missing",
                               f"Selected display {item['id']} is not available")
            destination = temporary / item["path"]
            destination.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(sources[item["id"]], destination)
            if file_digest(destination) != item["sha256"] or destination.stat().st_size != item["bytes"]:
                raise KitError("kit-corrupt", f"Display {item['id']} differs from the kit inventory")
        raw = canonical(document).encode()
        (temporary / "inventory.json").write_bytes(raw)
        for path in temporary.rglob("*"):
            if path.is_file():
                try:
                    with path.open("rb+") as stream:
                        os.fsync(stream.fileno())
                except OSError:
                    # Some Windows filesystems reject fsync on copied files;
                    # the atomic directory rename is still the publish boundary.
                    pass
        os.replace(temporary, output)
        try:
            descriptor = os.open(parent, os.O_RDONLY)
            try:
                os.fsync(descriptor)
            finally:
                os.close(descriptor)
        except OSError:
            pass
        return output
    except BaseException:
        shutil.rmtree(temporary, ignore_errors=True)
        raise


def _verify_archive_isolated(package, manifest_sha, prepared_sha):
    """Run the profile verifier in its own process.

    The legacy enrichment verifier temporarily overrides module bindings.
    Keeping each verification in a fresh process means two different profiles
    can be verified concurrently without sharing that state.
    """
    script = Path(__file__).with_name("verify_environment_rebuild.py")
    command = [sys.executable, str(script), "verify", str(package),
               "--manifest-sha256", manifest_sha]
    if prepared_sha:
        command += ["--prepared-sha256", prepared_sha]
    try:
        completed = subprocess.run(command, capture_output=True, encoding="utf-8",
                                   errors="replace", timeout=600, check=False)
    except (OSError, subprocess.TimeoutExpired) as error:
        raise KitError("kit-corrupt", "Prepared archive verification could not run") from error
    if completed.returncode != 0:
        detail = (completed.stderr or completed.stdout).strip()[:2000]
        raise KitError("kit-corrupt", detail or "Prepared archive verification failed")
    if len(completed.stdout) > 16 << 20:
        raise KitError("kit-corrupt", "Prepared archive verification exceeded its output bound")
    try:
        return json.loads(completed.stdout)
    except json.JSONDecodeError as error:
        raise KitError("kit-corrupt", "Prepared archive verification returned invalid output") from error


def verify(output, inventory_sha=None, now=None, check_archive=True):
    """Verify a kit before any local read is served."""
    root = Path(output)
    if not root.is_dir():
        raise KitError("kit-missing", "The kit directory is unavailable")
    inventory_path = root / "inventory.json"
    if not inventory_path.is_file():
        raise KitError("kit-interrupted", "The kit has no published inventory")
    raw = inventory_path.read_bytes()
    try:
        document = json.loads(raw)
    except json.JSONDecodeError as error:
        raise KitError("kit-corrupt", "The kit inventory is not valid JSON") from error
    actual = hashlib.sha256(raw).hexdigest()
    if inventory_sha is not None and actual != inventory_sha:
        raise KitError("kit-identity-mismatch",
                       "The kit inventory differs from the independently supplied SHA-256")
    if document.get("schema") != SCHEMA:
        raise KitError("kit-schema-unsupported", "The kit inventory schema is not supported")
    if document.get("reader", {}).get("contract") not in SUPPORTED_CONTRACTS:
        raise KitError("kit-schema-unsupported", "The kit names an unsupported reader contract")
    if document.get("operation") not in OPERATIONS:
        raise KitError("kit-schema-unsupported", "The kit names an unsupported operation")
    terms = document.get("terms") or {}
    expires = terms.get("expiresAt")
    if expires:
        if now is None:
            raise KitError("kit-expiry-unknown",
                           "The kit states an expiry; supply the evaluation instant")
        if _instant(expires) <= _instant(now):
            raise KitError("kit-expired", "The kit terms expired before this verification")
        if not any(grant.get("scope") == document["operation"] and not grant.get("revoked")
                   for grant in terms.get("grants", [])):
            raise KitError("kit-terms-missing", "The kit terms do not grant this operation")
    prepared_path = _safe(root, document["prepared"]["path"])
    if not prepared_path.is_file():
        raise KitError("kit-corrupt", "The kit prepared archive is missing")
    if prepared_path.stat().st_size != document["prepared"]["bytes"]:
        raise KitError("kit-corrupt", "The kit prepared archive length changed")
    if file_digest(prepared_path) != document["prepared"]["sha256"]:
        raise KitError("kit-corrupt", "The kit prepared archive digest changed")
    displays = []
    for item in document.get("displays", []):
        path = _safe(root, item["path"])
        if not path.is_file():
            raise KitError("kit-display-missing", f"Selected display {item['id']} is missing")
        if path.stat().st_size != item["bytes"] or file_digest(path) != item["sha256"]:
            raise KitError("kit-corrupt", f"Selected display {item['id']} changed")
        displays.append({"id": item["id"], "recipe": item["recipe"],
                         "sha256": item["sha256"], "bytes": item["bytes"]})
        if item.get("purpose"):
            displays[-1]["purpose"] = item["purpose"]
            displays[-1]["purposeMeaning"] = item.get("purposeMeaning")
            displays[-1]["limitations"] = item.get("limitations")
    check = None
    if check_archive:
        check = _verify_archive_isolated(prepared_path, document["release"]["manifestSha256"],
                                         document["prepared"]["sha256"])
    reader = None
    if check:
        from environment_reader import Reader
        reader = Reader(prepared_path, document["release"]["manifestSha256"]).inventory()
    return {"schema": VERIFICATION_SCHEMA, "status": "VERIFIED_KIT",
            "inventorySha256": actual, "release": document["release"],
            "purpose": document["purpose"], "operation": document["operation"],
            "reader": document["reader"], "prepared": document["prepared"],
            "displays": displays, "members": document["members"],
            "terms": terms, "archive": check,
            "inventory": {"profileId": reader["profileId"], "layers": reader["layers"]}
            if reader else None}


def native_read(output, member, row, column, unit="metre", inventory_sha=None, now=None):
    """Read after expiry and identity checks, using the same P03 reader as the CLI."""
    verify(output, inventory_sha, now, check_archive=False)
    from environment_reader import Reader
    return Reader(Path(output) / "prepared.zip").sample(member, row, column, unit)


def native_point(output, layer, longitude, latitude, unit="metre", datum=None, component=None,
                 inventory_sha=None, now=None):
    verify(output, inventory_sha, now, check_archive=False)
    from environment_reader import Reader
    return Reader(Path(output) / "prepared.zip").point(layer, longitude, latitude, unit,
                                                      datum, component)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="action", required=True)
    check = sub.add_parser("verify")
    check.add_argument("kit", type=Path)
    check.add_argument("--inventory-sha256", required=True)
    check.add_argument("--now")
    check.add_argument("--skip-archive-check", action="store_true")
    sample = sub.add_parser("sample")
    sample.add_argument("kit", type=Path)
    sample.add_argument("--inventory-sha256", required=True)
    sample.add_argument("--now")
    sample.add_argument("--member", required=True)
    sample.add_argument("--row", type=int, required=True)
    sample.add_argument("--column", type=int, required=True)
    sample.add_argument("--unit", default="metre")
    args = parser.parse_args()
    try:
        if args.action == "verify":
            result = verify(args.kit, args.inventory_sha256, args.now,
                            not args.skip_archive_check)
        else:
            result = native_read(args.kit, args.member, args.row, args.column, args.unit,
                                 args.inventory_sha256, args.now)
        print(json.dumps(result, sort_keys=True, indent=2))
        return 0
    except (KitError, OSError, ValueError) as error:
        print(str(error), file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
