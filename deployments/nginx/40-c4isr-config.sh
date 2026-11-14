#!/bin/sh
set -eu

if [ -n "${C4ISR_UI_TOKEN_FILE:-}" ] && [ -r "${C4ISR_UI_TOKEN_FILE}" ]; then
    C4ISR_API_TOKEN="$(cat "${C4ISR_UI_TOKEN_FILE}")"
    export C4ISR_API_TOKEN
fi

: "${C4ISR_UI_API_BASE_URL:=/api/v1}"
: "${C4ISR_API_TOKEN:=}"
: "${VITE_MAP_STYLE_URL:=}"
: "${C4ISR_UI_LOCAL_BASEMAP_ONLY:=false}"
: "${C4ISR_UI_GEOGRAPHIC_PREVIEW_ENABLED:=true}"
case "$C4ISR_UI_LOCAL_BASEMAP_ONLY" in true|false) ;; *) exit 1 ;; esac
case "$C4ISR_UI_GEOGRAPHIC_PREVIEW_ENABLED" in true|false) ;; *) exit 1 ;; esac
export C4ISR_UI_API_BASE_URL C4ISR_API_TOKEN VITE_MAP_STYLE_URL C4ISR_UI_LOCAL_BASEMAP_ONLY C4ISR_UI_GEOGRAPHIC_PREVIEW_ENABLED

envsubst '${C4ISR_UI_API_BASE_URL} ${C4ISR_API_TOKEN} ${VITE_MAP_STYLE_URL} ${C4ISR_UI_LOCAL_BASEMAP_ONLY} ${C4ISR_UI_GEOGRAPHIC_PREVIEW_ENABLED}' \
    < /usr/share/nginx/html/config.js.template \
    > /usr/share/nginx/html/config.js
