#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)
compose="docker compose --env-file $ROOT/deployments/staging/.env.staging -f $ROOT/deployments/staging/docker-compose.yml"
backup_path="${1:-${C4ISR_BACKUP_DIR:-backups}/c4isr-staging-$(date -u +%Y%m%dT%H%M%SZ).dump}"
mkdir -p -- "$(dirname -- "$backup_path")"
tmp_path="${backup_path}.tmp.$$"
checksum_path="${backup_path}.sha256"
cleanup() { rm -f -- "$tmp_path"; }
trap cleanup EXIT INT TERM

umask 077
# The password is read inside the container; it never crosses the shell or
# appears in process arguments on the backup host.
$compose exec -T postgres sh -c \
    'PGPASSWORD="$(cat /run/secrets/postgres_password)" pg_dump -U c4isr -d c4isr --format=custom --no-owner --no-privileges' \
    > "$tmp_path"

if [ ! -s "$tmp_path" ]; then
    echo "staging pg_dump produced an empty backup" >&2
    exit 1
fi

mv -- "$tmp_path" "$backup_path"
if command -v sha256sum >/dev/null 2>&1; then
    sha256sum -- "$backup_path" > "$checksum_path"
else
    shasum -a 256 -- "$backup_path" > "$checksum_path"
fi
chmod 600 "$backup_path" "$checksum_path"
printf 'backup=%s\nchecksum=%s\n' "$backup_path" "$checksum_path"
