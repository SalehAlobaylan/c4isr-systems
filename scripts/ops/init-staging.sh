#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)
secret_dir="${STAGING_SECRET_DIR:-$ROOT/deployments/staging/.secrets}"
env_file="$ROOT/deployments/staging/.env.staging"

umask 077
mkdir -p -- "$secret_dir"

secret_count=$(find "$secret_dir" -maxdepth 1 -type f | wc -l | awk '{$1=$1};1')
if [ "$secret_count" -gt 0 ] && [ "$secret_count" -ne 5 ]; then
    echo "staging secret directory is incomplete; repair it manually before continuing: $secret_dir" >&2
    exit 1
fi

if [ "$secret_count" -eq 5 ]; then
    if [ "${FORCE:-}" != "1" ]; then
        echo "staging secrets already exist at $secret_dir; use FORCE=1 to rotate the UI/operator/Grafana secrets" >&2
        exit 1
    fi
    password=$(cat "$secret_dir/postgres_password")
    database_url=$(cat "$secret_dir/database_url")
else
    if [ -n "${FORCE:-}" ] && [ "$FORCE" = "1" ]; then
        echo "cannot force-rotate an uninitialized staging secret directory" >&2
        exit 1
    fi
    if command -v openssl >/dev/null 2>&1; then
        password=$(openssl rand -hex 24)
    else
        password=$(od -An -N24 -tx1 /dev/urandom | tr -d ' \n')
    fi
    database_url="postgres://c4isr:${password}@postgres:5432/c4isr?sslmode=disable"
fi

if command -v openssl >/dev/null 2>&1; then
    token=$(openssl rand -hex 32)
    grafana_password=$(openssl rand -hex 24)
else
    token=$(od -An -N32 -tx1 /dev/urandom | tr -d ' \n')
    grafana_password=$(od -An -N24 -tx1 /dev/urandom | tr -d ' \n')
fi

printf '%s\n' "$password" > "$secret_dir/postgres_password"
printf '%s\n' "$database_url" > "$secret_dir/database_url"
printf 'operator-01=%s\n' "$token" > "$secret_dir/auth_tokens"
printf '%s\n' "$token" > "$secret_dir/ui_token"
printf '%s\n' "$grafana_password" > "$secret_dir/grafana_admin_password"
chmod 600 "$secret_dir"/*

if [ ! -e "$env_file" ]; then
    cp "$ROOT/deployments/staging/.env.staging.example" "$env_file"
fi

printf 'staging initialized\nsecrets=%s\nenv=%s\n' "$secret_dir" "$env_file"
