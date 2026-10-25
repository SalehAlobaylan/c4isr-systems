package httpx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRealtimeCredentialIsRemovedBeforeInstrumentation(t *testing.T) {
	for _, protocol := range []bool{false, true} {
		req := httptest.NewRequest("GET", "/api/v1/realtime?topics=track.updated&access_token=secret-sentinel", nil)
		req.Header.Set("Upgrade", "websocket")
		if protocol {
			req.Header.Set("Sec-WebSocket-Protocol", "emad.realtime.v2, emad.auth.secret-sentinel")
		}
		ProtectRealtimeCredential(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if RealtimeCredential(r) != "secret-sentinel" {
				t.Fatal("credential not available to authenticator")
			}
			if strings.Contains(r.URL.String()+r.RequestURI+r.Header.Get("Sec-WebSocket-Protocol"), "secret-sentinel") {
				t.Fatal("credential remained in instrumented request")
			}
			if r.URL.Query().Get("topics") != "track.updated" {
				t.Fatal("selection lost")
			}
		})).ServeHTTP(httptest.NewRecorder(), req)
	}
}
