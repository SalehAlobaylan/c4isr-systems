#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
# shellcheck source=lib.sh
. "$SCRIPT_DIR/lib.sh"

url="$(database_url)"
backup_path="${1:-${C4ISR_BACKUP_DIR:-backups}/c4isr-$(date -u +%Y%m%dT%H%M%SZ).dump}"
umask 077
mkdir -p -- "$(dirname -- "$backup_path")"

tmp_path="${backup_path}.tmp.$$"
checksum_path="${backup_path}.sha256"
cleanup() {
    rm -f -- "$tmp_path"
}
trap cleanup EXIT INT TERM

command -v pg_dump >/dev/null 2>&1 || {
    echo "pg_dump is required" >&2
    exit 1
}

pg_dump \
    --format=custom \
    --no-owner \
    --no-privileges \
    --file="$tmp_path" \
    "$url"

if [ ! -s "$tmp_path" ]; then
    echo "pg_dump produced an empty backup" >&2
    exit 1
fi

mv -- "$tmp_path" "$backup_path"
checksum_file "$backup_path" > "$checksum_path"
chmod 600 "$backup_path" "$checksum_path"
printf 'backup=%s\nchecksum=%s\n' "$backup_path" "$checksum_path"
