"""Inspect a retained geographic source or apply a steward-reviewed revision."""
import argparse
import json
from pathlib import Path
import sys
from urllib.parse import quote, urlencode

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "data"))
from environment_library import read_small
from intake_environment import LibraryAPI


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--api", required=True)
    parser.add_argument("--token-file", required=True, type=Path)
    parser.add_argument("--workspace", required=True)
    parser.add_argument("--environment", required=True)
    parser.add_argument("source_id")
    actions = parser.add_subparsers(dest="action", required=True)
    inspect = actions.add_parser("inspect", help="Read the current policy and one dependency page")
    inspect.add_argument("--after", default="")
    renew = actions.add_parser("renew", help="Submit a steward-reviewed admission document")
    renew.add_argument("reviewed_policy", type=Path)
    args = parser.parse_args()
    api = LibraryAPI(args.api, args.token_file, args.workspace, args.environment)
    path = "/environment-sources/" + quote(args.source_id, safe="")
    after = getattr(args, "after", "")
    current = api.call("GET", path + "/impact" + ("?" + urlencode({"after": after}) if after else ""))
    if args.action == "inspect":
        result = current
    else:
        decision = json.loads(read_small(args.reviewed_policy))
        intended_use = decision.get("intendedUse") if isinstance(decision, dict) else None
        if not isinstance(intended_use, str) or not intended_use.strip():
            parser.error("A reviewed admission with an intended use is required")
        if decision.get("expectedRevisionId") != current["currentPolicy"]["profileRevisionId"]:
            parser.error("The reviewed admission must name the current profile revision from inspect")
        result = api.call("POST", path + "/admissions", decision)
    print(json.dumps(result, indent=2, ensure_ascii=False))


if __name__ == "__main__":
    main()
