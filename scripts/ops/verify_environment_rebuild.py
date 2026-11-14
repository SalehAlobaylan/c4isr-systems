"""Verify a pinned rebuild and expose the output only after its checks pass."""
import argparse
from contextlib import contextmanager
import hashlib
from importlib.metadata import version
import json
import math
import os
from pathlib import Path
import shutil
import sys
from tempfile import TemporaryDirectory
import zipfile

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "data"))
import environment_library as base
from environment_library import ASSET_ID, HASH, MAX_OBJECT, archive_entries, archive_json, file_digest

EGM2008_GRID = "4191d471eefebf24091b56dbc604353cb3b8cf8cc70e448bb9ae56a272bef17a"


def check_coordinates(package, manifest):
    import rasterio
    from pyproj import Transformer

    frame = manifest["frame"]
    lon, lat, height = frame["origin"]
    transform = Transformer.from_pipeline(
        "+proj=pipeline +step +proj=unitconvert +xy_in=deg +xy_out=rad "
        "+step +proj=cart +ellps=WGS84 +step +proj=topocentric +ellps=WGS84 "
        f"+lon_0={lon} +lat_0={lat} +h_0={height}")
    controls = frame["controls"]
    if len(controls) < 3:
        raise ValueError("Prepared frame lacks coordinate controls")
    errors = [math.dist(control["enuM"], transform.transform(control["longitude"], control["latitude"],
                        control["ellipsoidHeightM"], errcheck=True)) for control in controls]
    if any(not math.isfinite(error) or error > .001 for error in errors):
        raise ValueError("Portable ENU control disagreement exceeds 1 mm")
    by_id = {item["id"]: item for item in manifest["inputs"]}
    checked = 0
    with zipfile.ZipFile(package) as source, TemporaryDirectory(prefix="emad-raster-registration-") as scratch:
        for member in manifest["members"]:
            if not member["name"].endswith(".tif") or not (registration := member.get("details", {}).get("registration")):
                continue
            if member["layer"] == "geoid":
                grid = by_id[member["inputId"]]
                if grid["sha256"] != EGM2008_GRID or grid["representation"]["verticalDatum"] != "EGM2008":
                    raise ValueError("Prepared geoid does not identify the pinned EGM2008 grid")
            target = Path(scratch) / "member.tif"
            with source.open(member["name"]) as raster_bytes, target.open("xb") as temp:
                shutil.copyfileobj(raster_bytes, temp, 256 << 10)
            with rasterio.open(target) as raster:
                if registration not in {"Area", "Point"} or raster.tags().get("AREA_OR_POINT", "Area") != registration:
                    raise ValueError("Prepared raster registration differs from the declared native grid")
            target.unlink()
            checked += 1
    return {"enuControls": len(controls), "maxControlErrorM": max(errors), "rasterRegistrations": checked}


@contextmanager
def processor_for(profile):
    if profile in base.PROFILE_LAYERS:
        yield base.verify, base.rebuild, base.toolchain
    elif profile == "coastal-context-v1":
        import environment_coastal as coastal
        yield coastal.verify, coastal.rebuild, coastal.toolchain
    elif profile == "reference-enrichment-v1":
        import environment_enrichment as enrichment
        with enrichment.legacy_contract():
            yield enrichment.verify, base.rebuild, enrichment.toolchain
    elif profile == "reference-observation-v1":
        import environment_observation as observation
        yield observation.verify, observation.rebuild, observation.toolchain
    else:
        raise ValueError(f"No retained processor for profile {profile}")


def check_bundle(source, expected_manifest):
    entries = archive_entries(source)
    raw, manifest = archive_json(source, entries, "manifest.json")
    if hashlib.sha256(raw).hexdigest() != expected_manifest:
        raise ValueError("Rebuild bundle does not contain the selected manifest")
    _, recipe = archive_json(source, entries, "recipe.json")
    if (recipe["schema"] != "emad.environment-recipe.v1" or manifest["schema"] != base.SCHEMA
            or recipe["profile"]["id"] != manifest["profileId"] or recipe["toolchainSha256"] != manifest["toolchainSha256"]
            or recipe["inputSha256"] != manifest["inputSha256"] or recipe["area"]["revisionId"] != manifest["areaRevisionId"]
            or recipe["area"]["geometry"] != manifest["area"] or recipe["inputs"] != manifest["inputs"]):
        raise ValueError("The rebuild recipe differs from the pinned manifest")
    if not 1 <= len(recipe["inputs"]) <= 64:
        raise ValueError("Rebuild input count exceeds the supported bounds")
    required = {"manifest.json", "recipe.json"}
    identities = set()
    for item in recipe["inputs"]:
        asset, digest, size = item["id"], item["sha256"], item["bytes"]
        name = "inputs/" + digest
        if not ASSET_ID.fullmatch(asset) or asset in identities or not HASH.fullmatch(digest) or not 1 <= size <= MAX_OBJECT:
            raise ValueError("The rebuild recipe contains an invalid input identity")
        identities.add(asset)
        if name not in entries:
            raise ValueError(f"Missing retained input {asset} ({digest})")
        if entries[name].file_size != size:
            raise ValueError(f"Retained input length differs for {asset} ({digest})")
        if name not in required:
            with source.open(entries[name]) as stream:
                if base.hash_stream(stream, size) != (digest, size):
                    raise ValueError(f"Retained input hash differs for {asset} ({digest})")
        required.add(name)
    if set(entries) != required:
        raise ValueError("Unexpected member in the retained input archive")
    return manifest


def verify_prepared(package, expected_manifest, expected_archive=None):
    if not HASH.fullmatch(expected_manifest) or expected_archive is not None and not HASH.fullmatch(expected_archive):
        raise ValueError("Expected digests must be full lowercase SHA-256 values")
    archive_sha, size = file_digest(package, MAX_OBJECT)
    if expected_archive and archive_sha != expected_archive:
        raise ValueError("Prepared archive differs from its trusted SHA-256")
    with zipfile.ZipFile(package) as source:
        raw, manifest = archive_json(source, archive_entries(source), "manifest.json")
    if hashlib.sha256(raw).hexdigest() != expected_manifest:
        raise ValueError("Prepared manifest differs from its trusted SHA-256")
    with processor_for(manifest["profileId"]) as (verify, _, _):
        result = verify(package, expected_manifest)
    return {**result, "schema": "emad.environment-portable-check.v1", "preparedSha256": archive_sha,
            "preparedBytes": size, "toolchainSha256": manifest["toolchainSha256"], "inputs": manifest["inputs"],
            "byteIdentityVerified": expected_archive is not None, "coordinateChecks": check_coordinates(package, manifest)}


def reproduce(bundle, output, expected_manifest, expected_archive=None):
    if not HASH.fullmatch(expected_manifest) or expected_archive is not None and not HASH.fullmatch(expected_archive):
        raise ValueError("Expected digests must be full lowercase SHA-256 values")
    if output.exists():
        raise FileExistsError("Rebuild output already exists")
    with zipfile.ZipFile(bundle) as source:
        metadata = check_bundle(source, expected_manifest)

    # The pinned processor writes to a private directory. Publish the verified
    # file with an atomic, no-replace link on the destination filesystem.
    with TemporaryDirectory(dir=output.parent, prefix=".environment-rebuild-") as directory:
        pending = Path(directory) / "prepared.zip"
        with processor_for(metadata["profileId"]) as (_, rebuild, toolchain):
            actual_toolchain = toolchain()
            if actual_toolchain != metadata["toolchainSha256"]:
                raise ValueError(f"Preparation toolchain differs. Expected {metadata['toolchainSha256']}, found {actual_toolchain}")
            result = rebuild(bundle, pending)
        archive_digest, size = file_digest(pending, MAX_OBJECT)
        if expected_archive is not None and (archive_digest != expected_archive or result["manifestSha256"] != expected_manifest):
            raise ValueError("Rebuilt package does not match the pinned physical archive and manifest")
        os.link(pending, output)
    return {
        **result,
        "schema": "emad.environment-rebuild-check.v1",
        "sourceManifestSha256": expected_manifest,
        "toolchainSha256": metadata["toolchainSha256"],
        "preparedSha256": archive_digest,
        "preparedBytes": size,
        "byteIdentical": True if expected_archive is not None else None,
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    if sys.argv[1:2] == ["toolchain"]:
        parser.add_argument("profile")
        parser.add_argument("--image-id", required=True, help="Immutable preparation image ID from the container runtime")
        args = parser.parse_args(sys.argv[2:])
        if not args.image_id.startswith("sha256:") or not HASH.fullmatch(args.image_id[7:]):
            parser.error("Supply an immutable SHA-256 image ID")
        try:
            with processor_for(args.profile) as (_, _, toolchain):
                report = {"schema": "emad.environment-processor.v1", "profileId": args.profile,
                          "declaredImageId": args.image_id, "toolchainSha256": toolchain(), "releaseSchema": base.SCHEMA,
                          "readerSha256": file_digest(Path(__file__))[0], "python": sys.version.split()[0],
                          "packages": {name: version(name) for name in ("fiona", "numpy", "osmium", "pyproj", "rasterio", "shapely")}}
            print(json.dumps(report))
        except (OSError, ValueError) as exc:
            parser.exit(2, f"{exc}\n")
        return 0
    if sys.argv[1:2] == ["verify"]:
        parser.add_argument("prepared", type=Path)
        parser.add_argument("--manifest-sha256", required=True)
        parser.add_argument("--prepared-sha256")
        args = parser.parse_args(sys.argv[2:])
        try:
            print(json.dumps(verify_prepared(args.prepared, args.manifest_sha256, args.prepared_sha256)))
        except (KeyError, OSError, TypeError, ValueError, zipfile.BadZipFile) as exc:
            parser.exit(2, f"{exc}\n")
        return 0
    parser.add_argument("bundle", type=Path)
    parser.add_argument("output", type=Path)
    parser.add_argument("--manifest-sha256", required=True)
    parser.add_argument("--prepared-sha256", help="Require exact archive and manifest identity")
    args = parser.parse_args()
    try:
        result = reproduce(args.bundle, args.output, args.manifest_sha256, args.prepared_sha256)
    except (KeyError, OSError, TypeError, ValueError, zipfile.BadZipFile) as exc:
        print(str(exc), file=sys.stderr)
        return 2
    print(json.dumps(result))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
