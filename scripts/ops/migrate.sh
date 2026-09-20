#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
# shellcheck source=lib.sh
. "$SCRIPT_DIR/lib.sh"

command_name="${1:-up}"
case "$command_name" in
    status|up) ;;
    down)
        case "${C4ISR_ENV:-development}" in
            staging|production)
                if [ "${C4ISR_ALLOW_MIGRATION_DOWN:-}" != "1" ]; then
                    echo "refusing migration rollback in staging/production; set C4ISR_ALLOW_MIGRATION_DOWN=1 after approval" >&2
                    exit 1
                fi
                ;;
        esac
        ;;
    *)
        echo "usage: $0 [status|up|down]" >&2
        exit 2
        ;;
esac

goose_bin="${GOOSE_BIN:-goose}"
if [ ! -x "$goose_bin" ] && ! command -v "$goose_bin" >/dev/null 2>&1; then
    echo "goose is required; run task setup or set GOOSE_BIN" >&2
    exit 1
fi

url="$(database_url)"
migrations_dir="${C4ISR_MIGRATIONS_DIR:-$(ops_repo_root)/db/migrations}"
"$goose_bin" -dir "$migrations_dir" postgres "$url" "$command_name"
