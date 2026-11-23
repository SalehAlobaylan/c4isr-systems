"""Issue a civilian place reference from EMAD, or verify one offline."""

import argparse
from datetime import datetime, timezone
import json
from pathlib import Path

from prepare_desert_maps import digest, encode, read_handoff


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    issue = commands.add_parser("issue", help="Obtain an exact authorized place from EMAD")
    issue.add_argument("--release", required=True)
    issue.add_argument("--feature", required=True)
    issue.add_argument("--expires-at", required=True)
    issue.add_argument("--profile")
    issue.add_argument("--output", required=True, type=Path)
    verify = commands.add_parser("verify", help="Verify a received package with its trusted digest")
    verify.add_argument("package", type=Path)
    verify.add_argument("--sha256", required=True)
    args = parser.parse_args()
    if args.command == "verify":
        feature = read_handoff(args.package.read_bytes(), args.sha256, datetime.now(timezone.utc))
        print(json.dumps(feature, ensure_ascii=True))
        return
    from install_local import API
    result = API(profile=args.profile).call("POST", "/geography/place-handoffs", {
        "releaseId": args.release, "featureId": args.feature, "expiresAt": args.expires_at,
    })
    raw = encode(result) + b"\n"
    sha = digest(raw)
    read_handoff(raw, sha, datetime.now(timezone.utc))
    with args.output.open("xb") as output:
        output.write(raw)
    print(json.dumps({"sha256": sha}))


if __name__ == "__main__":
    main()
