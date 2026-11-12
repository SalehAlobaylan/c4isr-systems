"""Register reviewed retained sources and queue private environment builds."""
import argparse
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import sys

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "data"))
from environment_library import encoded, read_small
from intake_environment import LibraryAPI, intake
from regional_geography import navigation_rights


def read(path):
    return json.loads(read_small(path))


def source(api, key, name):
    listed = api.call("GET", "/sources?limit=100")["items"]
    prior = next((item for item in listed if item["name"] == name), None)
    if prior:
        if prior["type"] != "external" or prior["status"] != "active":
            raise ValueError(f"Existing source differs from the reviewed intake for {name}")
        return prior["id"]
    return api.call("POST", "/sources", {"id": key, "name": name, "type": "external"})["id"]


def family(layer):
    return "osm" if layer in {"roads", "buildings", "water"} else layer


def register_area(api, root, area, rules):
    recipe = read(root / f"{area['key']}-candidate" / "recipe.json")
    bindings = {}
    sources = {}
    for asset in recipe["inputs"]:
        key = family(asset["representation"]["layer"])
        if key not in sources:
            rule = rules[key]
            label = f"{area['name']} · {rule['dataset']}"
            source_id = source(api, f"environment-{area['key']}-{key}", label)
            profile = next((item for item in api.call("GET", f"/sources/{source_id}/profiles")["items"]
                            if item["key"] == "geographic-retained"), None)
            provider = area.get("providers", {}).get(key, rule["upstreamProvider"])
            reviewed = {**rule, "upstreamProvider": provider, "enabled": True, "area": area["area"],
                        "coverageDescription": f"{area['name']} retained inputs for private reference preparation. Exact bytes are identified in the recipe."}
            if profile:
                prior = profile["admission"]
                comparable = {k: v for k, v in reviewed.items() if k != "area"}
                if (any(prior.get(k) != v for k, v in comparable.items())
                        or prior["policy"]["aoi"] != reviewed["area"]):
                    raise ValueError(f"Existing admission differs from reviewed source policy for {label}")
                admission = prior
                revision = profile["id"]
            else:
                admission = api.call("POST", f"/environment-sources/{source_id}/admissions", reviewed)
                revision = admission["profileRevisionId"]
            sources[key] = {"sourceId": source_id, "profileRevisionId": revision, "admissionId": admission["id"]}
        binding = sources[key]
        bindings[asset["id"]] = {"sourceId": binding["sourceId"], "admissionId": binding["admissionId"],
                                  "retainUntil": area["retainUntil"]}
    return {"name": area["name"], "assets": bindings}, list(sources.values())


def install_navigation(api, prepared):
    source_id = source(api, "environment-navigation-natural-earth", "Environment navigation · Natural Earth")
    prior = next((head for head in api.call("GET", "/geography/releases")["items"] if head["sourceId"] == source_id), None)
    editions = []
    for part in prepared["chunks"]:
        edition = api.call("POST", "/geography/editions", {key: value for key, value in part.items() if key != "featureCount"} |
                           {"sourceId": source_id, "staged": True, "rights": navigation_rights()})
        editions.append(edition["id"])
    profile = prepared["profile"]
    release = api.call("POST", "/geography/releases", {"sourceId": source_id, "key": profile["key"],
                       "version": prepared["version"], "editionIds": editions,
                       "expectedHead": prior["releaseId"] if prior else "", "activate": True})
    members = [{"kind": "administrative", "sourceId": source_id, "logicalKey": "country:" + code,
                "releaseId": release["id"]} for code in sorted(profile["countries"])]
    members.extend({"kind": "named", "sourceId": source_id, "logicalKey": item["key"],
                    "releaseId": release["id"]} for item in profile.get("studyAreas", []))
    receipt = api.call("POST", "/geography/profiles", {"key": profile["key"], "version": prepared["version"],
                       "name": profile["name"], "definition": profile["definition"], "members": members})
    return {"releaseId": release["id"], "profileId": receipt["id"], "countryCount": len(profile["countries"])}


def write(path, value):
    temporary = path.with_suffix(path.suffix + ".partial")
    temporary.write_bytes(encoded(value) + b"\n")
    temporary.replace(path)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=["register", "stage"])
    parser.add_argument("--review", type=Path, required=True)
    parser.add_argument("--candidate-root", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--api", required=True)
    parser.add_argument("--token-file", type=Path, required=True)
    parser.add_argument("--workspace", required=True)
    parser.add_argument("--environment", required=True)
    parser.add_argument("--worker-principal")
    args = parser.parse_args()
    if args.action == "stage" and not args.worker_principal:
        parser.error("The exact source-scoped worker identity is required")
    review = read(args.review)
    if review["schema"] != "emad.environment-source-review.v1":
        parser.error("Expected a reviewed source policy document")
    api = LibraryAPI(args.api, args.token_file, args.workspace, args.environment)
    identity = api.call("GET", "/auth/me")
    if identity["resource"]["workspaceId"] != args.workspace or identity["resource"]["environmentId"] != args.environment:
        parser.error("The API resolved a different resource from the selected private scope")
    result = {"schema": "emad.environment-commission.v1", "recordedAt": datetime.now(timezone.utc).isoformat(),
              "workspace": args.workspace, "environment": args.environment, "areas": {}}
    args.output.parent.mkdir(parents=True, exist_ok=True)
    for area in review["areas"]:
        binding, sources = register_area(api, args.candidate_root, area, review["sources"])
        entry = {"bindings": binding, "sources": sources}
        result["areas"][area["key"]] = entry
        write(args.output, result)
        if args.action == "stage":
            for item in sources:
                api.call("POST", f"/sources/{item['sourceId']}/profiles/geographic-retained/execution",
                         {"profileRevisionId": item["profileRevisionId"], "principalId": args.worker_principal, "enabled": True})
            submitted = intake(api, args.candidate_root / f"{area['key']}-candidate", binding)
            if not submitted["eligibility"]:
                raise RuntimeError(f"{area['name']} failed preflight: {submitted['diagnostics']}")
            definition = submitted["definitionId"]
            request_key = hashlib.sha256(encoded([definition, submitted["revision"]])).hexdigest()
            entry["build"] = api.call("POST", f"/geographic-environments/{definition}/builds",
                                      {"requestKey": request_key, "revision": submitted["revision"]})
            write(args.output, result)
            print(f"Queued {area['name']}: {entry['build']['id']}", flush=True)
    if args.action == "stage":
        result["navigation"] = install_navigation(api, read(args.candidate_root / "regional-navigation.json"))
        write(args.output, result)
    print(f"Commissioning record: {args.output}")


if __name__ == "__main__":
    main()
