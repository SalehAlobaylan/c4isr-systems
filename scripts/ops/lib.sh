#!/bin/sh
set -eu

ops_repo_root() {
    CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd
}

read_secret_env() {
    key="$1"
    value="$(printenv "$key" 2>/dev/null || true)"
    file="$(printenv "${key}_FILE" 2>/dev/null || true)"
    if [ -n "$value" ] && [ -n "$file" ]; then
        echo "$key and ${key}_FILE cannot both be set" >&2
        return 1
    fi
    if [ -n "$file" ]; then
        if [ ! -r "$file" ]; then
            echo "${key}_FILE is not readable: $file" >&2
            return 1
        fi
        value="$(cat -- "$file")"
    fi
    value="$(printf '%s' "$value" | awk '{$1=$1};1')"
    if [ -z "$value" ]; then
        echo "$key or ${key}_FILE is required" >&2
        return 1
    fi
    printf '%s' "$value"
}

database_url() {
    read_secret_env C4ISR_DATABASE_URL
}

admin_database_url() {
    if [ -n "${C4ISR_DATABASE_ADMIN_URL_FILE:-}" ] || [ -n "${C4ISR_DATABASE_ADMIN_URL:-}" ]; then
        read_secret_env C4ISR_DATABASE_ADMIN_URL
        return
    fi
    database_url
}

checksum_file() {
    file="$1"
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum -- "$file"
    else
        shasum -a 256 -- "$file"
    fi
}
