package integration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/app"
	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/investigations"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/config"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/SalehAlobaylan/c4isr-systems/internal/reference"
)

// Explicit opt-in host for actual observers. Credentials stay below private/;
// no participant outcome is inferred from this host successfully starting.
func TestMapFirstObservedCandidateHost(t *testing.T) {
	directory := os.Getenv("C4ISR_OBSERVED_HANDOFF_DIR")
	if directory == "" {
		t.Skip("requires an explicitly selected private observer directory")
	}
	absolute, e := filepath.Abs(directory)
	if e != nil {
		t.Fatal(e)
	}
	private, e := filepath.Abs("../../private")
	if e != nil {
		t.Fatal(e)
	}
	relative, e := filepath.Rel(private, absolute)
	if e != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		t.Fatal("observer credentials must remain under the ignored private directory")
	}
	if e = os.MkdirAll(absolute, 0700); e != nil {
		t.Fatal(e)
	}
	dist, e := filepath.Abs("../../apps/operator-ui/dist")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(dist, "index.html")); e != nil {
		t.Fatal("build the production UI before hosting an observed candidate")
	}
	storage := newS3Fixture(t)
	h := newConfiguredHarness(t, func(c *config.Config) { c.ProductProfile = config.ProfileMap }, app.WithArtifactStore(storage))
	credentials := filepath.Join(absolute, "credentials")
	command := exec.Command("go", "run", "../../apps/emad-local", "-credentials-dir", credentials, "-profile", "map")
	command.Env = append(os.Environ(), "C4ISR_DATABASE_URL="+h.pool.Config().ConnConfig.ConnString(), "C4ISR_DATABASE_URL_FILE=")
	if raw, e := command.CombinedOutput(); e != nil {
		t.Fatal("isolated participant provisioning failed", e, string(raw))
	}
	readToken := func(name string) string {
		raw, e := os.ReadFile(filepath.Join(credentials, name+".token"))
		if e != nil {
			t.Fatal(e)
		}
		return strings.TrimSpace(string(raw))
	}
	resource := access.Resource{WorkspaceID: "default", EnvironmentID: "local", Partition: access.PublicEvidence}
	store := access.NewStore(h.pool)
	bind := func(token string) context.Context {
		session, e := store.Authenticate(context.Background(), token, nil)
		if e != nil {
			t.Fatal(e)
		}
		ctx, e := store.BindSession(context.Background(), session, resource)
		if e != nil {
			t.Fatal(e)
		}
		return ctx
	}
	admin := bind(readToken("local-source-admin"))
	status, source := scopeRequest(t, h, readToken("local-source-admin"), "POST", "/api/v1/sources", map[string]any{"id": "observed-navigation", "name": "Acceptance: source-backed Natural Earth navigation", "type": "external"}, nil)
	if status != 201 {
		t.Fatal("observed navigation source", status)
	}
	sourceID := source["id"].(string)
	imported := 0
	if path := os.Getenv("C4ISR_TEST_GEOGRAPHY_PREPARED"); path != "" {
		raw, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		var plan struct {
			Version string                     `json:"version"`
			Chunks  []reference.GeographyInput `json:"chunks"`
		}
		if e = json.Unmarshal(raw, &plan); e != nil {
			t.Fatal(e)
		}
		svc := reference.NewService(h.pool)
		ids := []string{}
		for _, input := range plan.Chunks {
			input.SourceID = sourceID
			input.Staged = true
			input.Rights = publicingest.Rights{ID: "naturalearth-public-domain-observation-v1", LegalBasis: "Natural Earth public-domain source archive retained for this isolated acceptance session", TermsURL: "https://www.naturalearthdata.com/about/terms-of-use/", Attribution: "Made with Natural Earth; source-backed navigation only, not surveyed accuracy", RetainRaw: true, AllowAnalysis: true}
			edition, _, e := svc.AdmitGeography(admin, input)
			if e != nil {
				t.Fatal("source-backed participant navigation", e)
			}
			ids = append(ids, edition.ID)
		}
		if _, e := svc.Release(admin, reference.ReleaseInput{SourceID: sourceID, Key: "observed-navigation", Version: plan.Version, EditionIDs: ids, Activate: true}); e != nil {
			t.Fatal(e)
		}
		imported = len(ids)
	}
	restrictedHuman := "observed-restricted-human"
	if _, e = h.pool.Exec(context.Background(), `INSERT INTO operators(id,name,role)VALUES($1,'Restricted acceptance reader','operator')`, restrictedHuman); e != nil {
		t.Fatal(e)
	}
	authority := access.Bootstrap(context.Background())
	if e = store.CreateHuman(authority, "observed-restricted-reader", restrictedHuman, access.Grant{Resource: resource, Permissions: []string{"operational.read", "sources.read", "investigations.read"}, SourceIDs: []string{sourceID}}); e != nil {
		t.Fatal(e)
	}
	_, token, e := store.IssueCredential(authority, "observed-restricted-reader", resource, time.Now().Add(2*time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(credentials, "observed-restricted-reader.token"), []byte(token), 0600); e != nil {
		t.Fatal(e)
	}
	files := http.FileServer(http.Dir(dist))
	roleTokens := map[string]string{"analyst": readToken("local-map-analyst"), "reviewer": readToken("local-map-reviewer"), "sourceAdministrator": readToken("local-source-admin"), "restrictedReader": token}
	capabilities := map[string]string{}
	privateLinks := map[string]string{}
	for role := range roleTokens {
		var secret [32]byte
		if _, e = rand.Read(secret[:]); e != nil {
			t.Fatal(e)
		}
		capability := hex.EncodeToString(secret[:])
		capabilities[capability] = role
		privateLinks[role] = capability
	}
	web := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/__candidate/connect/") {
			capability := strings.TrimPrefix(r.URL.Path, "/__candidate/connect/")
			if _, ok := capabilities[capability]; !ok {
				http.NotFound(w, r)
				return
			}
			http.SetCookie(w, &http.Cookie{Name: "emad-observed-role", Value: capability, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: 3600})
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Referrer-Policy", "no-referrer")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<script>sessionStorage.removeItem('emad.session-token');location.replace('/')</script>`))
			return
		}
		if r.URL.Path == "/config.js" {
			w.Header().Set("Content-Type", "application/javascript")
			w.Header().Set("Cache-Control", "no-store")
			role := "analyst"
			if cookie, e := r.Cookie("emad-observed-role"); e == nil {
				if selected, ok := capabilities[cookie.Value]; ok {
					role = selected
				}
			}
			configuration, _ := json.Marshal(map[string]any{"apiBaseUrl": "/api/v1", "apiToken": roleTokens[role], "localAutoEntry": true})
			_, _ = w.Write(append(append([]byte("window.__C4ISR_CONFIG__="), configuration...), ';'))
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/health" || r.URL.Path == "/metrics" {
			h.app.Handler().ServeHTTP(w, r)
			return
		}
		if _, e := os.Stat(filepath.Join(dist, filepath.FromSlash(strings.TrimPrefix(r.URL.Path, "/")))); e != nil {
			http.ServeFile(w, r, filepath.Join(dist, "index.html"))
			return
		}
		files.ServeHTTP(w, r)
	}))
	defer web.Close()
	accessHTML := `<!doctype html><meta charset="utf-8"><title>Local candidate access</title><h1>Local candidate access</h1><p>Open each role in a separate browser profile. This private file contains temporary access links; keep it out of observation evidence.</p>`
	for _, role := range []string{"analyst", "reviewer", "sourceAdministrator", "restrictedReader"} {
		accessHTML += `<p><a rel="noreferrer" href="` + web.URL + `/__candidate/connect/` + privateLinks[role] + `">Open ` + role + `</a></p>`
	}
	if e = os.WriteFile(filepath.Join(absolute, "LOCAL_ACCESS.html"), []byte(accessHTML), 0600); e != nil {
		t.Fatal(e)
	}
	binary, e := os.ReadFile(os.Args[0])
	if e != nil {
		t.Fatal(e)
	}
	build := artifacts.Hash(binary)
	handoff := map[string]any{"schema": "emad.observed-candidate-host.v1", "url": web.URL, "credentialsDirectory": credentials, "expiresAt": time.Now().Add(time.Hour).UTC(), "geographicEditions": imported, "workerBuild": build, "resource": resource, "roles": map[string]string{"analyst": "local-map-analyst", "reviewer": "local-map-reviewer", "sourceAdministrator": "local-source-admin", "restrictedReader": "observed-restricted-reader"}, "canonicalHumans": map[string]string{"analyst": "operator-01", "sourceAdministrator": "operator-01", "reviewer": "local-map-review-human", "restrictedReader": restrictedHuman}, "meaning": "Isolated formative observation host. No actual user outcome or full release qualification is implied. No live-provider profile is automatically activated."}
	raw, _ := json.MarshalIndent(handoff, "", "  ")
	if e = os.WriteFile(filepath.Join(absolute, "handoff.json"), raw, 0600); e != nil {
		t.Fatal(e)
	}
	analyst := bind(readToken("local-map-analyst"))
	delivery := investigations.Service{Pool: h.pool, Store: storage}
	end := time.Now().Add(time.Hour)
	for time.Now().Before(end) {
		if _, e := os.Stat(filepath.Join(absolute, "stop")); e == nil {
			return
		}
		work, cancel := context.WithTimeout(analyst, 120*time.Second)
		_, _ = delivery.WorkDelivery(work, build)
		cancel()
		time.Sleep(time.Second)
	}
}
