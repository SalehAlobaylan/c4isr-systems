package httpx

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

type realtimeCredentialKey struct{}

// ProtectRealtimeCredential runs before tracing/logging. The browser can offer
// an authentication subprotocol; it is consumed, never selected or echoed.
// Legacy query credentials are supported but removed from the instrumented URL.
func ProtectRealtimeCredential(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") && strings.HasSuffix(r.URL.Path, "/realtime") {
			clone := r.Clone(r.Context())
			token := ""
			protocols := []string{}
			for _, header := range r.Header.Values("Sec-WebSocket-Protocol") {
				for _, p := range strings.Split(header, ",") {
					p = strings.TrimSpace(p)
					if strings.HasPrefix(p, "emad.auth.") {
						if value, err := url.PathUnescape(strings.TrimPrefix(p, "emad.auth.")); err == nil {
							token = value
						}
					} else {
						protocols = append(protocols, p)
					}
				}
			}
			clone.Header.Del("Sec-WebSocket-Protocol")
			if len(protocols) > 0 {
				clone.Header.Set("Sec-WebSocket-Protocol", strings.Join(protocols, ", "))
			}
			query := clone.URL.Query()
			if token == "" {
				token = query.Get("access_token")
			}
			query.Del("access_token")
			clone.URL.RawQuery = query.Encode()
			clone.RequestURI = clone.URL.RequestURI()
			clone = clone.WithContext(context.WithValue(clone.Context(), realtimeCredentialKey{}, token))
			r = clone
		}
		next.ServeHTTP(w, r)
	})
}
func RealtimeCredential(r *http.Request) string {
	if token, ok := r.Context().Value(realtimeCredentialKey{}).(string); ok {
		return token
	}
	return r.URL.Query().Get("access_token")
}
