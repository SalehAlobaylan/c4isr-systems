#!/bin/sh
set -eu

if [ -n "${C4ISR_UI_TOKEN_FILE:-}" ] && [ -r "${C4ISR_UI_TOKEN_FILE}" ]; then
    C4ISR_API_TOKEN="$(cat "${C4ISR_UI_TOKEN_FILE}")"
    export C4ISR_API_TOKEN
fi

: "${C4ISR_UI_API_BASE_URL:=/api/v1}"
: "${C4ISR_API_TOKEN:=}"
: "${VITE_MAP_STYLE_URL:=}"
export C4ISR_UI_API_BASE_URL C4ISR_API_TOKEN VITE_MAP_STYLE_URL

envsubst '${C4ISR_UI_API_BASE_URL} ${C4ISR_API_TOKEN} ${VITE_MAP_STYLE_URL}' \
    < /usr/share/nginx/html/config.js.template \
    > /usr/share/nginx/html/config.js
