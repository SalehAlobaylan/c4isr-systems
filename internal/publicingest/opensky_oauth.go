package publicingest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const OpenSkyTokenURL = "https://auth.opensky-network.org/auth/realms/opensky-network/protocol/openid-connect/token"

type oauthToken struct {
	value   string
	expires time.Time
}

// Credentials go only to OpenSky's fixed TLS endpoint. Tokens remain in process
// memory, expire proactively, and never enter source metadata or evidence.
func (f *HTTPFetcher) openSkyToken(ctx context.Context, l Location) (string, error) {
	f.oauthMu.Lock()
	defer f.oauthMu.Unlock()
	key := firmsHash([]byte(l.oauthID + "\x00" + l.oauthSecret))
	if token, ok := f.oauthTokens[key]; ok && time.Now().Add(time.Minute).Before(token.expires) {
		return token.value, nil
	}
	fail := func() (string, error) { return "", &RetrievalError{Code: "provider_auth"} }
	form := url.Values{"grant_type": {"client_credentials"}, "client_id": {l.oauthID}, "client_secret": {l.oauthSecret}}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, OpenSkyTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fail()
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "EMAD-public-context/1.0")
	response, err := f.client.Do(request)
	if err != nil {
		return fail()
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return fail()
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, 65537))
	if err != nil || len(raw) > 65536 {
		return fail()
	}
	var token struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		Type        string `json:"token_type"`
	}
	if json.Unmarshal(raw, &token) != nil || len(token.AccessToken) < 16 || len(token.AccessToken) > 8192 || strings.ContainsAny(token.AccessToken, "\r\n") || !strings.EqualFold(token.Type, "Bearer") || token.ExpiresIn < 120 || token.ExpiresIn > 3600 {
		return fail()
	}
	if len(f.oauthTokens) >= 16 {
		f.oauthTokens = map[string]oauthToken{}
	}
	if f.oauthTokens == nil {
		f.oauthTokens = map[string]oauthToken{}
	}
	f.oauthTokens[key] = oauthToken{token.AccessToken, time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)}
	return token.AccessToken, nil
}
