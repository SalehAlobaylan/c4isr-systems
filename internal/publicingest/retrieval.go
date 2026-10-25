package publicingest

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
)

// Locations are trusted server configuration, not source metadata or API input.
// One location binds one admitted source and resource, preventing arbitrary URL
// selection or reuse of a connector's quota by another source.
type Location struct {
	ID                   string          `json:"id"`
	SourceID             string          `json:"sourceId"`
	Resource             access.Resource `json:"resource"`
	URL                  string          `json:"url"`
	Adapter              string          `json:"adapter"`
	AuthTokenEnv         string          `json:"authTokenEnv,omitempty"`
	OAuthClientIDEnv     string          `json:"oauthClientIdEnv,omitempty"`
	OAuthClientSecretEnv string          `json:"oauthClientSecretEnv,omitempty"`
	GateState            string          `json:"gateState,omitempty"`
	GateNextAction       string          `json:"gateNextAction,omitempty"`
	ReferenceOrigins     []string        `json:"referenceOrigins,omitempty"`
	MaxRequestsPerDay    int             `json:"maxRequestsPerDay"`
	token                string
	oauthID, oauthSecret string
}
type Registry map[string]Location

func LoadRegistry(path string) (Registry, error) {
	if path == "" {
		return Registry{}, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.New("cannot read public connector locations")
	}
	defer file.Close()
	if info, err := file.Stat(); err != nil || info.Size() > 1024*1024 {
		return nil, errors.New("public connector configuration exceeds size limit")
	}
	decoder := json.NewDecoder(io.LimitReader(file, 1024*1024))
	decoder.DisallowUnknownFields()
	var entries []Location
	if err = decoder.Decode(&entries); err != nil {
		return nil, errors.New("invalid public connector locations")
	}
	if err = decoder.Decode(new(any)); err != io.EOF {
		return nil, errors.New("trailing public connector configuration")
	}
	return NewRegistry(entries)
}
func NewRegistry(entries []Location) (Registry, error) {
	if len(entries) > 1000 {
		return nil, errors.New("too many public connector locations")
	}
	registry := Registry{}
	bindings := map[string]bool{}
	for _, entry := range entries {
		if err := entry.Resource.Validate(); err != nil || entry.Resource.Partition != access.PublicEvidence {
			return nil, errors.New("connector locations require public resources")
		}
		if _, err := normalizeIdentity(entry.ID, 160); err != nil {
			return nil, errors.New("invalid connector location identity")
		}
		if _, err := normalizeIdentity(entry.SourceID, 512); err != nil {
			return nil, errors.New("invalid connector source identity")
		}
		if err := publicReference(entry.URL); err != nil && !(entry.Adapter == AISAdapter && entry.URL == AISURL) {
			return nil, errors.New("connector locations require credential-free HTTPS endpoints")
		}
		u, _ := url.Parse(entry.URL)
		if u.Port() != "" && u.Port() != "443" {
			return nil, errors.New("connector locations require HTTPS port 443")
		}
		if (entry.Adapter != "emad-json-page-v1" && entry.Adapter != USGSAdapter && entry.Adapter != FIRMSAdapter && entry.Adapter != STACAdapter && entry.Adapter != EarthSearchAdapter && entry.Adapter != OpenSkyAdapter && entry.Adapter != AISAdapter && entry.Adapter != METARAdapter && !isGDACS(entry.Adapter)) || entry.MaxRequestsPerDay < 1 || entry.MaxRequestsPerDay > 10000 {
			return nil, errors.New("unsupported connector adapter or quota")
		}
		if (entry.Adapter == METARAdapter && entry.URL != METARURL) || (entry.Adapter == GDACSAdapter && entry.URL != GDACSURL) || ((entry.Adapter == METARAdapter || entry.Adapter == GDACSAdapter) && entry.AuthTokenEnv != "") {
			return nil, errors.New("context adapters require fixed credential-free provider endpoints")
		}
		if entry.Adapter == GDACSRSSAdapter && (entry.URL != GDACSRSSURL || entry.AuthTokenEnv != "") {
			return nil, errors.New("GDACS RSS requires its fixed credential-free endpoint")
		}
		if entry.Adapter == OpenSkyAdapter && (entry.URL != OpenSkyURL || entry.AuthTokenEnv != "") {
			return nil, errors.New("OpenSky requires its fixed bounded endpoint and native OAuth or anonymous access")
		}
		if entry.GateState != "" && entry.GateState != "NEEDS_CREDENTIALS" && entry.GateState != "NEEDS_PERMISSION" {
			return nil, errors.New("unknown provider prerequisite state")
		}
		if len(entry.GateNextAction) > 500 || strings.ContainsAny(entry.GateNextAction, "\r\n") {
			return nil, errors.New("invalid provider prerequisite action")
		}
		if entry.GateState == "" && (entry.OAuthClientIDEnv != "" || entry.OAuthClientSecretEnv != "") {
			if entry.Adapter != OpenSkyAdapter || entry.OAuthClientIDEnv == "" || entry.OAuthClientSecretEnv == "" {
				return nil, errors.New("OAuth credential binding requires the native OpenSky adapter")
			}
			entry.oauthID, entry.oauthSecret = os.Getenv(entry.OAuthClientIDEnv), os.Getenv(entry.OAuthClientSecretEnv)
			if len(entry.oauthID) < 1 || len(entry.oauthID) > 512 || len(entry.oauthSecret) < 8 || len(entry.oauthSecret) > 4096 || strings.ContainsAny(entry.oauthID+entry.oauthSecret, "\r\n") {
				return nil, errors.New("OpenSky OAuth credentials are missing or invalid")
			}
		}
		if entry.Adapter == AISAdapter && (entry.URL != AISURL || entry.AuthTokenEnv == "") {
			return nil, errors.New("AISStream requires its fixed WSS endpoint and server key")
		}
		if entry.Adapter == STACAdapter && (entry.URL != STACURL || entry.AuthTokenEnv != "") {
			return nil, errors.New("CDSE STAC requires its fixed credential-free endpoint")
		}
		if entry.Adapter == EarthSearchAdapter && (entry.URL != EarthSearchURL || entry.AuthTokenEnv != "") {
			return nil, errors.New("Earth Search requires its fixed credential-free Collection 1 endpoint")
		}
		if entry.Adapter == USGSAdapter && entry.URL != USGSQueryURL {
			return nil, errors.New("USGS adapter requires the vetted USGS query endpoint")
		}
		if entry.Adapter == FIRMSAdapter && (entry.URL != FIRMSAreaURL || entry.AuthTokenEnv == "") {
			return nil, errors.New("FIRMS adapter requires its vetted endpoint and a server credential environment variable")
		}
		for _, origin := range entry.ReferenceOrigins {
			if err := publicReference(origin); err != nil {
				return nil, errors.New("invalid artifact reference origin")
			}
			v, _ := url.Parse(origin)
			if v.Path != "" && v.Path != "/" {
				return nil, errors.New("reference origins must not include paths")
			}
		}
		if entry.GateState == "" && entry.AuthTokenEnv != "" {
			entry.token = os.Getenv(entry.AuthTokenEnv)
			if entry.token == "" || strings.ContainsAny(entry.token, "\r\n") {
				return nil, errors.New("connector credential environment variable is missing or invalid")
			}
		}
		if entry.GateState == "" && entry.Adapter == FIRMSAdapter && !firmsKey.MatchString(entry.token) {
			return nil, errors.New("invalid FIRMS map key format")
		}
		key := entry.Resource.ID() + "/" + entry.SourceID
		if _, ok := registry[entry.ID]; ok || bindings[key] {
			return nil, errors.New("duplicate connector location or source binding")
		}
		bindings[key] = true
		registry[entry.ID] = entry
	}
	return registry, nil
}

type FetchRequest struct {
	Admission  Admission
	Checkpoint string
	Now        time.Time
	From, To   time.Time
}
type Retrieval struct {
	CreditsRemaining   *int64 `json:"creditsRemaining,omitempty"`
	Discontinuous      bool   `json:"discontinuous,omitempty"`
	CompressionEnabled bool   `json:"compressionEnabled,omitempty"`
	CoverageMeaning    string `json:"coverageMeaning,omitempty"`
	ResponseHash       string `json:"responseHash,omitempty"`
	SelectionExcluded  int    `json:"selectionExcluded,omitempty"`
	LocationID         string `json:"locationId"`
	HTTPStatus         int    `json:"httpStatus"`
	ETag               string `json:"etag,omitempty"`
	LastModified       string `json:"lastModified,omitempty"`
}
type Fetcher interface {
	Fetch(context.Context, Location, FetchRequest) (Page, Retrieval, error)
}
type RetrievalError struct {
	Code    string
	RetryAt time.Time
}

func (e *RetrievalError) Error() string { return e.Code }

type HTTPFetcher struct {
	client      *http.Client
	aisSlots    chan struct{}
	oauthMu     sync.Mutex
	oauthTokens map[string]oauthToken
}

func NewHTTPFetcher() *HTTPFetcher {
	transport := &http.Transport{Proxy: nil, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}, ResponseHeaderTimeout: 10 * time.Second, TLSHandshakeTimeout: 5 * time.Second, MaxResponseHeaderBytes: 32 * 1024, DisableCompression: true, DisableKeepAlives: true}
	transport.DialContext = safeDial
	return &HTTPFetcher{aisSlots: make(chan struct{}, 1), client: &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("provider redirect denied") }}}
}

var blockedNetworks = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"), netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"), netip.MustParsePrefix("198.18.0.0/15"), netip.MustParsePrefix("198.51.100.0/24"), netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001:db8::/32"), netip.MustParsePrefix("64:ff9b::/96"), netip.MustParsePrefix("64:ff9b:1::/48"), netip.MustParsePrefix("2002::/16"), netip.MustParsePrefix("2001::/32"),
}

func publicAddress(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, prefix := range blockedNetworks {
		if prefix.Contains(ip) {
			return false
		}
	}
	return true
}
func safeDial(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, errors.New("provider address denied")
	}
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil || len(ips) == 0 {
		return nil, errors.New("provider DNS unavailable")
	}
	for _, ip := range ips {
		if !publicAddress(ip) {
			return nil, errors.New("provider address denied")
		}
	}
	// Dial the validated address itself, so DNS cannot change between checking
	// and connecting. TLS still authenticates the original URL hostname.
	return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
}

func safeHeader(v string) string {
	if len(v) > 256 || strings.ContainsAny(v, "\r\n") {
		return ""
	}
	return v
}
func (f *HTTPFetcher) Fetch(ctx context.Context, location Location, in FetchRequest) (Page, Retrieval, error) {
	meta := Retrieval{LocationID: location.ID}
	var page Page
	if location.GateState != "" {
		return page, meta, &RetrievalError{Code: "provider_auth"}
	}
	if location.Adapter == AISAdapter {
		return f.fetchAIS(ctx, location, in)
	}
	u, err := url.Parse(location.URL)
	if err != nil {
		return page, meta, &RetrievalError{Code: "invalid_location"}
	}
	p := in.Admission.Policy
	query := u.Query()
	query.Set("checkpoint", in.Checkpoint)
	query.Set("limit", strconv.Itoa(p.MaxRecordsPerPage))
	query.Set("bbox", strings.Join([]string{strconv.FormatFloat(p.AOI.West, 'f', -1, 64), strconv.FormatFloat(p.AOI.South, 'f', -1, 64), strconv.FormatFloat(p.AOI.East, 'f', -1, 64), strconv.FormatFloat(p.AOI.North, 'f', -1, 64)}, ","))
	query.Set("from", in.From.Format(time.RFC3339Nano))
	query.Set("to", in.To.Format(time.RFC3339Nano))
	u.RawQuery = query.Encode()
	requestURL := u.String()
	if location.Adapter == EarthSearchAdapter {
		requestURL, err = earthSearchURL(location, in)
		if err != nil {
			return page, meta, err
		}
	}
	if location.Adapter == OpenSkyAdapter {
		requestURL, err = openskyURL(location, in)
		if err != nil {
			return page, meta, err
		}
	}
	if location.Adapter == STACAdapter {
		requestURL, err = stacURL(location, in)
		if err != nil {
			return page, meta, err
		}
	}
	if location.Adapter == USGSAdapter {
		requestURL, err = usgsURL(location, in)
		if err != nil {
			return page, meta, err
		}
	}
	if location.Adapter == FIRMSAdapter {
		requestURL, err = firmsURL(location, in)
		if err != nil {
			return page, meta, err
		}
	}
	if location.Adapter == METARAdapter {
		requestURL = metarURL(in)
	}
	if location.Adapter == GDACSAdapter {
		requestURL = gdacsURL(in)
	}
	if location.Adapter == GDACSRSSAdapter {
		requestURL = GDACSRSSURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return page, meta, &RetrievalError{Code: "invalid_location"}
	}
	if location.Adapter == FIRMSAdapter {
		req.Header.Set("Accept", "text/csv")
	} else if location.Adapter == GDACSRSSAdapter {
		req.Header.Set("Accept", "application/xml, text/xml;q=0.9")
	} else {
		req.Header.Set("Accept", "application/json")
	}
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("User-Agent", "EMAD-public-context/1.0")
	if location.token != "" && location.Adapter != FIRMSAdapter {
		req.Header.Set("Authorization", "Bearer "+location.token)
	}
	privateToken := ""
	if location.Adapter == OpenSkyAdapter && location.oauthID != "" {
		privateToken, err = f.openSkyToken(ctx, location)
		if err != nil {
			return page, meta, err
		}
		req.Header.Set("Authorization", "Bearer "+privateToken)
	}
	response, err := f.client.Do(req)
	if err != nil {
		return page, meta, &RetrievalError{Code: "provider_unavailable"}
	}
	defer response.Body.Close()
	meta.HTTPStatus = response.StatusCode
	if location.Adapter == OpenSkyAdapter {
		if value, err := strconv.ParseInt(response.Header.Get("X-Rate-Limit-Remaining"), 10, 64); err == nil && value >= 0 && value <= 1_000_000_000 {
			meta.CreditsRemaining = &value
		}
	}
	meta.ETag = safeHeader(response.Header.Get("ETag"))
	meta.LastModified = safeHeader(response.Header.Get("Last-Modified"))
	if response.StatusCode == http.StatusNoContent && location.Adapter == METARAdapter {
		page, err = DecodeMETAR([]byte(`[]`), in)
		return page, meta, err
	}
	if response.StatusCode == http.StatusNoContent && location.Adapter == USGSAdapter {
		page, err = DecodeUSGS([]byte(`{"type":"FeatureCollection","metadata":{"count":0,"status":200},"features":[]}`), in)
		return page, meta, err
	}
	if response.StatusCode != http.StatusOK {
		if response.StatusCode == 401 && privateToken != "" {
			f.oauthMu.Lock()
			delete(f.oauthTokens, firmsHash([]byte(location.oauthID+"\x00"+location.oauthSecret)))
			f.oauthMu.Unlock()
		}
		retry := time.Time{}
		value := response.Header.Get("Retry-After")
		if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 && seconds <= 30*86400 {
			retry = in.Now.Add(time.Duration(seconds) * time.Second)
		} else if at, err := http.ParseTime(value); err == nil && at.After(in.Now) {
			retry = at
		}
		if location.Adapter == OpenSkyAdapter && response.StatusCode == http.StatusTooManyRequests {
			if seconds, err := strconv.Atoi(response.Header.Get("X-Rate-Limit-Retry-After-Seconds")); err == nil && seconds >= 0 && seconds <= 30*86400 {
				if at := in.Now.Add(time.Duration(seconds) * time.Second); at.After(retry) {
					retry = at
				}
			}
		}
		code := "provider_unavailable"
		if response.StatusCode == 401 || response.StatusCode == 403 {
			code = "provider_auth"
		}
		if response.StatusCode == 429 {
			code = "provider_rate_limit"
		}
		return page, meta, &RetrievalError{Code: code, RetryAt: retry}
	}
	if encoding := response.Header.Get("Content-Encoding"); encoding != "" && encoding != "identity" {
		return page, meta, &RetrievalError{Code: "unsupported_encoding"}
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, int64(p.MaxResponseBytes)+1))
	if err != nil {
		return page, meta, &RetrievalError{Code: "provider_unavailable"}
	}
	if len(raw) > p.MaxResponseBytes {
		return page, meta, &RetrievalError{Code: "response_too_large"}
	}
	if privateToken != "" && (strings.Contains(string(raw), privateToken) || strings.Contains(string(raw), location.oauthSecret)) {
		return page, meta, &RetrievalError{Code: "provider_auth"}
	}
	meta.ResponseHash = firmsHash(raw)
	if location.Adapter == METARAdapter || isGDACS(location.Adapter) {
		meta.Discontinuous = true
		meta.CoverageMeaning = "Bounded returned station samples or provider bulletins; selection window is not guaranteed spatial/temporal reception or hazard coverage."
		if location.Adapter == METARAdapter {
			page, err = DecodeMETAR(raw, in)
		} else if location.Adapter == GDACSRSSAdapter {
			page, err = DecodeGDACSRSS(raw, in)
		} else {
			page, err = DecodeGDACS(raw, in)
		}
		meta.SelectionExcluded = page.ExcludedRecords
		return page, meta, err
	}
	if location.Adapter == OpenSkyAdapter {
		meta.Discontinuous = true
		meta.CoverageMeaning = "One current bounded snapshot; lookback filters source timestamps and does not provide historical or complete reception coverage."
		page, err = DecodeOpenSky(raw, in)
		meta.SelectionExcluded = page.ExcludedRecords
		return page, meta, err
	}
	if location.Adapter == STACAdapter {
		page, err = DecodeSTAC(raw, in)
		meta.SelectionExcluded = page.ExcludedRecords
		return page, meta, err
	}
	if location.Adapter == EarthSearchAdapter {
		page, err = DecodeEarthSearch(raw, in)
		meta.SelectionExcluded = page.ExcludedRecords
		return page, meta, err
	}
	if location.Adapter == FIRMSAdapter {
		page, err = DecodeFIRMS(raw, in)
		meta.SelectionExcluded = page.ExcludedRecords
		return page, meta, err
	}
	if location.Adapter == USGSAdapter {
		page, err = DecodeUSGS(raw, in)
		return page, meta, err
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&page); err != nil {
		return page, meta, &RetrievalError{Code: "invalid_response"}
	}
	if err = decoder.Decode(new(any)); err != io.EOF {
		return page, meta, &RetrievalError{Code: "invalid_response"}
	}
	return page, meta, nil
}

func permittedReference(location Location, raw string) bool {
	if location.Adapter == AISAdapter && raw == AISReference {
		return true
	}
	if raw == "" {
		return true
	}
	if publicReference(raw) != nil {
		return false
	}
	u, _ := url.Parse(raw)
	base, _ := url.Parse(location.URL)
	if strings.EqualFold(u.Host, base.Host) {
		return true
	}
	for _, origin := range location.ReferenceOrigins {
		v, _ := url.Parse(origin)
		if strings.EqualFold(u.Host, v.Host) {
			return true
		}
	}
	return false // References are not fetched automatically, including artifacts.
}
