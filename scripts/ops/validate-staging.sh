#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)
secret_dir=$(mktemp -d "${TMPDIR:-/tmp}/c4isr-staging-secrets.XXXXXX")
cleanup() {
    rm -f -- "$secret_dir/postgres_password" "$secret_dir/database_url" \
        "$secret_dir/auth_tokens" "$secret_dir/ui_token" "$secret_dir/grafana_admin_password"
    rmdir -- "$secret_dir"
}
trap cleanup EXIT INT TERM

# Git Bash otherwise rewrites container paths such as /bin/promtool into host
# paths. Pass native host mount paths explicitly and preserve container paths.
docker_root="$ROOT"
docker_secret_dir="$secret_dir"
case "$(uname -s)" in
    MINGW*|MSYS*|CYGWIN*)
        docker_root=$(cygpath -m "$ROOT")
        docker_secret_dir=$(cygpath -m "$secret_dir")
        export MSYS_NO_PATHCONV=1
        ;;
esac

printf 'postgres-password\n' > "$secret_dir/postgres_password"
printf 'postgres://c4isr:postgres-password@postgres:5432/c4isr?sslmode=disable\n' > "$secret_dir/database_url"
printf 'operator-01=staging-token\n' > "$secret_dir/auth_tokens"
printf 'staging-token\n' > "$secret_dir/ui_token"
printf 'grafana-password\n' > "$secret_dir/grafana_admin_password"
chmod 600 "$secret_dir"/*

STAGING_SECRET_DIR="$docker_secret_dir" \
ALERTMANAGER_WEBHOOK_URL="http://alert-sink.invalid/alerts" \
docker compose \
    --env-file "$docker_root/deployments/staging/.env.staging.example" \
    -f "$docker_root/deployments/staging/docker-compose.yml" \
    config --quiet

docker run --rm \
    --entrypoint /bin/promtool \
    -v "$docker_root/deployments/observability:/etc/prometheus:ro" \
    prom/prometheus:v3.5.0 check config /etc/prometheus/prometheus.yml
docker run --rm \
    --entrypoint /bin/promtool \
    -v "$docker_root/deployments/observability:/etc/prometheus:ro" \
    prom/prometheus:v3.5.0 check rules /etc/prometheus/alerts.yml
docker run --rm \
    -v "$docker_root/deployments/observability/alloy.alloy:/etc/alloy/config.alloy:ro" \
    grafana/alloy:v1.19.0 fmt /etc/alloy/config.alloy >/dev/null
docker run --rm \
    -v "$docker_root/deployments/observability/loki-config.yml:/etc/loki/config.yml:ro" \
    grafana/loki:3.7.0 -config.file=/etc/loki/config.yml -verify-config=true

echo "staging Compose configuration is valid"
