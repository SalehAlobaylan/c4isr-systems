#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)
secret_dir="${STAGING_SECRET_DIR:-$ROOT/deployments/staging/.secrets}"
api_url="${C4ISR_STAGING_API_URL:-http://127.0.0.1:8080}"
ui_url="${C4ISR_STAGING_UI_URL:-http://127.0.0.1:8081}"
prometheus_url="${C4ISR_STAGING_PROMETHEUS_URL:-http://127.0.0.1:9090}"
alertmanager_url="${C4ISR_STAGING_ALERTMANAGER_URL:-http://127.0.0.1:9093}"
loki_url="${C4ISR_STAGING_LOKI_URL:-http://127.0.0.1:3100}"
alloy_url="${C4ISR_STAGING_ALLOY_URL:-http://127.0.0.1:12345}"
grafana_url="${C4ISR_STAGING_GRAFANA_URL:-http://127.0.0.1:3001}"

command -v curl >/dev/null 2>&1 || {
    echo "curl is required" >&2
    exit 1
}

timeout_seconds="${C4ISR_SMOKE_TIMEOUT_SECONDS:-10}"
max_attempts="${C4ISR_SMOKE_ATTEMPTS:-30}"
retry_seconds="${C4ISR_SMOKE_RETRY_SECONDS:-2}"

get() {
    curl --fail --silent --show-error --max-time "$timeout_seconds" "$1"
}

wait_for() {
    label="$1"
    url="$2"
    attempt=1
    while [ "$attempt" -le "$max_attempts" ]; do
        if curl --fail --silent --max-time "$timeout_seconds" "$url" >/dev/null; then
            printf '%s: ready\n' "$label"
            return 0
        fi
        if [ "$attempt" -lt "$max_attempts" ]; then
            sleep "$retry_seconds"
        fi
        attempt=$((attempt + 1))
    done
    echo "$label did not become ready: $url" >&2
    return 1
}

if [ ! -r "$secret_dir/ui_token" ]; then
    echo "staging UI token is missing: $secret_dir/ui_token (run task staging:init)" >&2
    exit 1
fi
token=$(cat "$secret_dir/ui_token")
if [ -z "$token" ]; then
    echo "staging UI token is empty" >&2
    exit 1
fi

wait_for "api health" "$api_url/health"
health=$(get "$api_url/health")
printf '%s' "$health" | grep -q '"status":"ok"' || {
    echo "API health is not OK: $health" >&2
    exit 1
}
printf 'api health: ok\n'

metrics=$(get "$api_url/metrics")
printf '%s\n' "$metrics" | grep -q '^c4isr_database_up 1$' || {
    echo "database readiness metric is not up" >&2
    exit 1
}
printf '%s\n' "$metrics" | grep -q '^c4isr_http_requests_total' || {
    echo "HTTP metrics are not exposed" >&2
    exit 1
}
printf 'api metrics: ok\n'

identity=$(curl --fail --silent --show-error --max-time "$timeout_seconds" \
    -H "Authorization: Bearer $token" "$api_url/api/v1/auth/me")
printf '%s' "$identity" | grep -q 'operator-01' || {
    echo "operator authentication failed" >&2
    exit 1
}
printf 'operator authentication: ok\n'

wait_for "operator UI" "$ui_url/"
get "$ui_url/" | grep -q '<div id="root"' || {
    echo "operator UI did not serve its application shell" >&2
    exit 1
}
printf 'operator UI: ok\n'

wait_for "prometheus" "$prometheus_url/-/ready"
get "$prometheus_url/api/v1/status/config" >/dev/null
printf 'prometheus: ok\n'

wait_for "alertmanager" "$alertmanager_url/-/ready"
printf 'alertmanager: ok\n'

wait_for "loki" "$loki_url/ready"
printf 'loki: ok\n'

wait_for "alloy" "$alloy_url/-/ready"
printf 'alloy: ok\n'

wait_for "grafana" "$grafana_url/api/health"
printf 'grafana: ok\n'

printf 'staging smoke checks passed\n'
