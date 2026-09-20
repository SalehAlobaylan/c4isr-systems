#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
# shellcheck source=lib.sh
. "$SCRIPT_DIR/lib.sh"

backup_path="${1:-}"
if [ -z "$backup_path" ] || [ ! -r "$backup_path" ]; then
    echo "usage: $0 BACKUP.dump" >&2
    exit 2
fi
command -v pg_restore >/dev/null 2>&1 || { echo "pg_restore is required" >&2; exit 1; }
command -v psql >/dev/null 2>&1 || { echo "psql is required" >&2; exit 1; }

admin_url="$(admin_database_url)"
restore_db="c4isr_restore_test_$(date -u +%Y%m%d%H%M%S)_$$"
case "$restore_db" in
    *[!a-zA-Z0-9_]*|'') echo "invalid generated restore database name" >&2; exit 1 ;;
esac

case "$admin_url" in
    postgres://*|postgresql://*) ;;
    *) echo "C4ISR_DATABASE_ADMIN_URL must be a PostgreSQL URI" >&2; exit 1 ;;
esac
base_url="${admin_url%%\?*}"
query=""
if [ "$base_url" != "$admin_url" ]; then
    query="?${admin_url#*\?}"
fi
target_url="${base_url%/*}/$restore_db$query"

cleanup() {
    psql "$admin_url" -v ON_ERROR_STOP=1 \
        -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '$restore_db' AND pid <> pg_backend_pid();" \
        >/dev/null 2>&1 || true
    psql "$admin_url" -v ON_ERROR_STOP=1 \
        -c "DROP DATABASE IF EXISTS \"$restore_db\";" \
        >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

psql "$admin_url" -v ON_ERROR_STOP=1 \
    -c "CREATE DATABASE \"$restore_db\";"
pg_restore --dbname="$target_url" --no-owner --no-privileges --exit-on-error "$backup_path"

check="$(psql "$target_url" -At -v ON_ERROR_STOP=1 -c \
    "SELECT (postgis_version() IS NOT NULL) AND (to_regclass('public.goose_db_version') IS NOT NULL) AND (to_regclass('public.sources') IS NOT NULL);")"
if [ "$check" != "t" ]; then
    echo "restore verification failed: PostGIS, goose metadata, or sources table missing" >&2
    exit 1
fi

printf 'restore verification passed: %s -> %s\n' "$backup_path" "$restore_db"
