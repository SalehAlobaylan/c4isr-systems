"""Prepare and publish scoped display derivatives for one exact enriched release."""
import argparse
import json
from pathlib import Path
import subprocess
import sys
from tempfile import TemporaryDirectory

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "data"))
from environment_library import HASH, file_digest
from intake_environment import LibraryAPI

PROCESSOR = Path(__file__).resolve().parents[1] / "data" / "prepare_enrichment_display.py"
OBSERVATION_PROCESSOR = PROCESSOR.with_name("prepare_observation_display.py")
LAYERS = ("surface", "ground", "land-cover", "ground-delta", "imagery", "roads", "buildings", "water", "radar")


def publish(api, release_id, package, manifest_digest, layers, scene_metadata):
    if not HASH.fullmatch(manifest_digest) or len(set(layers)) != len(layers) or any(layer not in LAYERS for layer in layers):
        raise ValueError("Choose a pinned manifest and distinct supported display layers")
    parent = api.call("GET", "/environment-releases/" + release_id)
    if parent["manifestSha256"] != manifest_digest or parent["state"] not in ("STAGED", "VERIFIED", "RELEASED"):
        raise ValueError("The requested release no longer matches the pinned manifest")
    profile = parent["manifest"]["profileId"]
    if profile == "reference-observation-v1":
        if set(layers) - {"surface", "radar", "imagery"}:
            raise ValueError("Unsupported display for an observation release")
        processor = OBSERVATION_PROCESSOR
    elif profile == "reference-enrichment-v1":
        if "radar" in layers:
            raise ValueError("Radar requires a reference observation release")
        processor = PROCESSOR
    else:
        raise ValueError("The publisher requires a supported versioned release profile")
    with TemporaryDirectory(prefix="emad-enrichment-publication-") as scratch:
        prepared = []
        for layer in layers:
            target = Path(scratch) / (layer + (".glb" if layer in ("surface", "ground") else ".json"))
            request = Path(scratch) / (layer + "-request.json")
            command = [sys.executable, "-I", str(processor), str(package), layer, str(target), str(request),
                       "--manifest-sha256", manifest_digest]
            if layer == "imagery":
                if scene_metadata is None:
                    raise ValueError("The imagery display needs the exact retained Earth Search STAC item")
                command.extend(("--scene-metadata", str(scene_metadata)))
            subprocess.run(command, capture_output=True, check=True, timeout=120)
            intent = json.loads(request.read_bytes())
            if file_digest(target, intent["bytes"]) != (intent["sha256"], intent["bytes"]):
                raise ValueError("Generated display no longer matches its publication request")
            prepared.append((layer, target, intent))
        result = []
        for layer, target, intent in prepared:
            display = api.call("POST", f"/environment-releases/{release_id}/displays", intent)
            if display["state"] != "READY":
                with target.open("rb") as stream:
                    display = api.call("PUT", f"/environment-displays/{display['id']}/content", source=stream)
            if display["state"] != "READY" or display["sha256"] != intent["sha256"]:
                raise ValueError("Display did not publish with its pinned content hash")
            result.append({"layer": layer, **{key: display[key] for key in ("id", "sha256", "bytes", "recipe")}})
        return result


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("release")
    parser.add_argument("package", type=Path)
    parser.add_argument("--manifest-sha256", required=True)
    parser.add_argument("--layers", nargs="+", choices=LAYERS, required=True)
    parser.add_argument("--scene-metadata", type=Path)
    parser.add_argument("--api", required=True)
    parser.add_argument("--token-file", type=Path, required=True)
    parser.add_argument("--workspace", required=True)
    parser.add_argument("--environment", required=True)
    args = parser.parse_args()
    try:
        api = LibraryAPI(args.api, args.token_file, args.workspace, args.environment)
        print(json.dumps(publish(api, args.release, args.package, args.manifest_sha256, args.layers, args.scene_metadata)))
    except subprocess.CalledProcessError as exc:
        parser.exit(2, (exc.stderr or b"Display generation failed.").decode("utf-8", "replace") + "\n")
    except (ValueError, KeyError, OSError, RuntimeError, subprocess.TimeoutExpired) as exc:
        parser.exit(2, f"{exc}\n")
