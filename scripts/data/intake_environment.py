"""Stage a reviewed geographic candidate using an explicitly scoped credential."""
import argparse
import hashlib
import json
from pathlib import Path
from urllib.parse import urlsplit

import requests

from environment_library import ASSET_ID, encoded, file_digest, read_small, verify


class LibraryAPI:
    def __init__(self, origin, token_file, workspace, environment):
        address = urlsplit(origin)
        if (address.scheme != "https" and
            (address.scheme != "http" or address.hostname not in {"localhost", "127.0.0.1", "::1"})) or (
                not address.hostname or address.username or address.password or address.query or address.fragment):
            raise ValueError("Use an HTTPS API or a loopback development API")
        token = token_file.read_text(encoding="utf-8").strip()
        if not token or len(token) > 4096:
            raise ValueError("A bounded scoped credential file is required")
        self.origin = origin.rstrip("/") + "/api/v1"
        self.headers = {"Authorization": "Bearer " + token, "X-EMAD-Workspace-ID": workspace,
                        "X-EMAD-Environment-ID": environment, "X-EMAD-Partition": "PUBLIC_EVIDENCE"}

    def call(self, method, path, body=None, source=None):
        with requests.request(method, self.origin + path, headers=self.headers, json=body,
                              data=source, timeout=(10, 300), allow_redirects=False) as response:
            if not 200 <= response.status_code < 300:
                try:
                    reason = response.json()["error"]["message"]
                except (ValueError, KeyError, TypeError):
                    reason = "Unavailable"
                raise RuntimeError(f"{method} {path} returned {response.status_code}: {reason}")
            return response.json()


def intake(api, candidate, bindings):
    recipe = json.loads(read_small(candidate / "recipe.json"))
    if recipe["profile"]["id"] == "reference-enrichment-v1":
        from environment_enrichment import verify as verify_enrichment
        verify_enrichment(candidate / "prepared.zip")
    elif recipe["profile"]["id"] == "reference-observation-v1":
        from environment_observation import verify as verify_observation
        verify_observation(candidate / "prepared.zip")
    elif recipe["profile"]["id"] == "coastal-context-v1":
        from environment_coastal import verify as verify_coastal
        verify_coastal(candidate / "prepared.zip")
    else:
        verify(candidate / "prepared.zip")
    by_id = bindings["assets"]
    if set(by_id) != {asset["id"] for asset in recipe["inputs"]}:
        raise ValueError("Reviewed bindings must account for every exact retained input")
    for asset in recipe["inputs"]:
        identifier = asset["id"]
        if not ASSET_ID.fullmatch(identifier) or file_digest(candidate / "inputs" / identifier, asset["bytes"]) != (asset["sha256"], asset["bytes"]):
            raise ValueError("Candidate input bytes differ from their receipts")
        binding = by_id[identifier]
        if not binding.get("sourceId") or not binding.get("admissionId") or not binding.get("retainUntil"):
            raise ValueError("Every input requires a source, reviewed admission and retention date")
    ids = []
    for asset in recipe["inputs"]:
        identifier = asset["id"]
        binding = by_id[identifier]
        metadata = {key: binding[key] for key in ("sourceId", "admissionId", "retainUntil", "referenceId", "sceneId") if key in binding}
        payload = {**metadata, "sha256": asset["sha256"], "bytes": asset["bytes"],
                   "representation": asset["representation"]}
        payload["requestKey"] = hashlib.sha256(encoded(payload)).hexdigest()
        reserved = api.call("POST", "/environment-assets", payload)
        if reserved["state"] != "READY":
            with (candidate / "inputs" / identifier).open("rb") as stream:
                api.call("PUT", "/environment-assets/" + reserved["id"] + "/content", source=stream)
        ids.append(reserved["id"])
    area = api.call("POST", "/areas/revisions", {"kind": "drawn", "name": bindings["name"],
                                                "geometry": recipe["area"]["geometry"]})
    desired = {"name": bindings["name"], "areaRevisionId": area["revisionId"],
               "profileId": recipe["profile"]["id"], "expectedRevision": 0, "assetIds": sorted(ids)}
    definition = None
    path = "/geographic-environments"
    while path:
        page = api.call("GET", path)
        definition = next((item for item in page["items"] if all(item.get(key) == desired[key]
                           for key in ("name", "areaRevisionId", "profileId", "assetIds"))), None)
        if definition or not page.get("next"):
            break
        path = "/geographic-environments?after=" + page["next"]
    if definition is None:
        definition = api.call("POST", "/geographic-environments", desired)
    check = api.call("GET", "/geographic-environments/" + definition["id"] + "/preflight")
    return {"definitionId": definition["id"], "revision": definition["revision"],
            "eligibility": check["eligible"], "diagnostics": check["diagnostics"]}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("candidate", type=Path)
    parser.add_argument("bindings", type=Path)
    parser.add_argument("--api", required=True)
    parser.add_argument("--token-file", type=Path, required=True)
    parser.add_argument("--workspace", required=True)
    parser.add_argument("--environment", required=True)
    args = parser.parse_args()
    api = LibraryAPI(args.api, args.token_file, args.workspace, args.environment)
    result = intake(api, args.candidate, json.loads(read_small(args.bindings)))
    print(json.dumps(result))
    return 0 if result["eligibility"] else 2


if __name__ == "__main__":
    raise SystemExit(main())
