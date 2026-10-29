package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/config"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/reference"
	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestMapFirstProvisionedMapProfileAndUpgradedDefault(t *testing.T) {
	h := newConfiguredHarness(t, func(c *config.Config) {
		c.ProductProfile = config.ProfileMap
		c.ScenariosDir = filepath.Join(t.TempDir(), "no-scenarios")
	})
	directory := t.TempDir()
	t.Setenv("C4ISR_DATABASE_URL", h.pool.Config().ConnConfig.ConnString())
	t.Setenv("C4ISR_DATABASE_URL_FILE", "")
	provision := func(profile string) {
		t.Helper()
		cmd := exec.Command("go", "run", "../../apps/emad-local", "-credentials-dir", directory, "-profile", profile)
		if out, e := cmd.CombinedOutput(); e != nil {
			t.Fatalf("provision %s: %v %s", profile, e, out)
		}
	}
	provision("map")
	if _, e := os.Stat(filepath.Join(directory, "worker-runtime.token")); !os.IsNotExist(e) {
		t.Fatal("map setup required a runtime credential")
	}
	raw, e := os.ReadFile(filepath.Join(directory, "local-map-analyst.token"))
	if e != nil {
		t.Fatal(e)
	}
	status, session := scopeRequest(t, h, strings.TrimSpace(string(raw)), "GET", "/api/v1/auth/me", nil, nil)
	if status != 200 || session["productProfile"] != "map" || session["resource"].(map[string]any)["partition"] != "PUBLIC_EVIDENCE" {
		t.Fatalf("map entry: %d %v", status, session)
	}
	if image := os.Getenv("C4ISR_TEST_DELIVERY_IMAGE"); image != "" {
		connection, parseErr := url.Parse(h.pool.Config().ConnConfig.ConnString())
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		connection.Host = fmt.Sprintf("host.docker.internal:%d", h.pool.Config().ConnConfig.Port)
		container, e := testcontainers.GenericContainer(context.Background(), testcontainers.GenericContainerRequest{Started: true, ContainerRequest: testcontainers.ContainerRequest{
			Image: image, Entrypoint: []string{"/app/bin/c4isr-server"}, ExposedPorts: []string{"8080/tcp"},
			Env:        map[string]string{"C4ISR_ENV": "test", "C4ISR_PRODUCT_PROFILE": "map", "C4ISR_DATABASE_AUTH": "true", "C4ISR_AUTH_REQUIRED": "true", "C4ISR_DATABASE_URL": connection.String(), "C4ISR_HTTP_ADDR": ":8080", "C4ISR_LOG_LEVEL": "warn", "C4ISR_RUNTIME_BUILDS_FILE": "/not-installed/runtime-builds.txt"},
			WaitingFor: wait.ForHTTP("/health").WithPort("8080/tcp").WithStartupTimeout(60 * time.Second),
		}})
		if container != nil {
			defer container.Terminate(context.Background())
		}
		if e != nil {
			t.Fatal("Linux map delivery startup", e)
		}
		origin, e := container.Endpoint(context.Background(), "http")
		if e != nil {
			t.Fatal(e)
		}
		request, e := http.NewRequest("GET", origin+"/api/v1/auth/me", nil)
		if e != nil {
			t.Fatal(e)
		}
		request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(raw)))
		response, e := http.DefaultClient.Do(request)
		if e != nil {
			t.Fatal(e)
		}
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		if response.StatusCode != 200 || !strings.Contains(string(body), `"productProfile":"map"`) {
			t.Fatalf("Linux map delivery auth: %d %s", response.StatusCode, body)
		}
		request, e = http.NewRequest("POST", origin+"/api/v1/analyst-scope/resolve", strings.NewReader(`{"area":{"kind":"world"},"temporal":{"mode":"monitoring","rollingSeconds":86400,"revisionView":"latest","timezone":"Asia/Riyadh"}}`))
		if e != nil {
			t.Fatal(e)
		}
		request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(raw)))
		request.Header.Set("Content-Type", "application/json")
		response, e = http.DefaultClient.Do(request)
		if e != nil {
			t.Fatal(e)
		}
		body, _ = io.ReadAll(response.Body)
		response.Body.Close()
		var resolved map[string]any
		if e = json.Unmarshal(body, &resolved); e != nil || response.StatusCode != 200 || resolved["schema"] != "emad.scope-snapshot.v1" || resolved["scopeHash"] == "" {
			t.Fatalf("Linux scope/timezone contract: %d %s %v", response.StatusCode, body, e)
		}
		t.Log("Linux delivery served a real map session without simulation executables or a runtime descriptor")
	}
	for _, permission := range session["permissions"].([]any) {
		if strings.HasPrefix(permission.(string), "sim.") || permission == "sources.manage" {
			t.Fatal("ordinary map analyst received administrative/exercise permission", permission)
		}
	}
	for _, path := range []string{"/api/v1/simulation-studio/scenes", "/api/v1/scenarios"} {
		if status, _ := scopeRequest(t, h, strings.TrimSpace(string(raw)), "GET", path, nil, nil); status != 403 && status != 404 {
			t.Fatalf("map historical route %s: %d", path, status)
		}
	}
	// Upgrade keeps the older broad identities intact; their saved exercise
	// default cannot override the selected deployment profile.
	provision("historical-full")
	old, e := os.ReadFile(filepath.Join(directory, "local-analyst.token"))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = h.pool.Exec(context.Background(), `UPDATE principal_credentials SET default_scope_id='default/local/EXERCISE_OBSERVATION/legacy' WHERE principal_id='local-analyst'`); e != nil {
		t.Fatal(e)
	}
	status, session = scopeRequest(t, h, strings.TrimSpace(string(old)), "GET", "/api/v1/auth/me", nil, nil)
	if status != 200 || session["resource"].(map[string]any)["partition"] != "PUBLIC_EVIDENCE" {
		t.Fatalf("upgraded saved default: %d %v", status, session)
	}
	for _, r := range session["resources"].([]any) {
		if r.(map[string]any)["partition"] != "PUBLIC_EVIDENCE" {
			t.Fatal("map catalog exposed exercise navigation", r)
		}
	}
	var count int
	if e = h.pool.QueryRow(context.Background(), `SELECT count(*) FROM principal_grants WHERE principal_id='local-analyst' AND partition='EXERCISE_INPUT'`).Scan(&count); e != nil || count != 1 {
		t.Fatal("historical author grant was changed", count, e)
	}
}

func TestMapFirstContentPurposeCatalogHistoryCountsAndRLS(t *testing.T) {
	f := newPublicFixture(t)
	raw, e := os.ReadFile("../../testdata/reference/jeddah-makkah/places.json")
	if e != nil {
		t.Fatal(e)
	}
	var input reference.Input
	if e = json.Unmarshal(raw, &input); e != nil {
		t.Fatal(e)
	}
	input.SourceID = f.source
	status, body := scopeRequest(t, f.h, f.manager, "POST", "/api/v1/reference-datasets", input, nil)
	if status != 201 {
		t.Fatalf("reference setup: %d %v", status, body)
	}
	id := body["id"].(string)
	classify := map[string]any{"kind": "reference", "entityId": id, "purpose": "acceptance", "reason": "Exact retained fixture imported by this isolated acceptance test", "receiptReference": "testdata/reference/jeddah-makkah/places.json", "expectedVersion": 0}
	if status, _ = scopeRequest(t, f.h, f.worker, "POST", "/api/v1/content-purpose", classify, nil); status != 403 {
		t.Fatal("worker classified human-reviewed purpose", status)
	}
	status, body = scopeRequest(t, f.h, f.manager, "POST", "/api/v1/content-purpose", classify, nil)
	if status != 201 || body["version"] != float64(1) {
		t.Fatalf("classification: %d %v", status, body)
	}
	status, body = scopeRequest(t, f.h, f.manager, "POST", "/api/v1/content-purpose", classify, nil)
	if status != 201 || body["version"] != float64(1) {
		t.Fatalf("idempotent receipt: %d %v", status, body)
	}
	status, body = scopeRequest(t, f.h, f.manager, "GET", "/api/v1/reference-datasets", nil, nil)
	if status != 200 || body["total"] != float64(0) {
		t.Fatalf("acceptance catalog pollution: %d %v", status, body)
	}
	status, body = scopeRequest(t, f.h, f.manager, "GET", "/api/v1/reference-datasets?content_view=all", nil, nil)
	if status != 200 || body["total"] != float64(1) {
		t.Fatalf("admin history: %d %v", status, body)
	}
	if status, _ = scopeRequest(t, f.h, f.worker, "GET", "/api/v1/reference-datasets?content_view=all", nil, nil); status != 403 {
		t.Fatal("non-admin expanded default catalog", status)
	}
	// Repeated captures and repeated jobs must not imply new dates/studies.
	for i := 0; i < 4; i++ {
		input := fmt.Sprintf(`{"scene":{"id":"test-scene-%d","acquiredAt":"2026-01-%02dT00:00:00Z"}}`, i, i/2+1)
		if _, e := f.h.pool.Exec(context.Background(), `INSERT INTO imagery_capture_jobs(id,scope_id,source_id,input,created_by,status)VALUES($1,$2,$3,$4,'fixture','SUCCEEDED')`, fmt.Sprintf("purpose-capture-%d", i), f.resource.ID(), f.source, input); e != nil {
			t.Fatal(e)
		}
	}
	for i := 0; i < 2; i++ {
		if _, e := f.h.pool.Exec(context.Background(), `INSERT INTO imagery_jobs(id,scope_id,source_id,kind,status,request_key,request_hash,input,created_by)VALUES($1,$2,$3,'NDVI_CHANGE','SUCCEEDED',$1,'fixture','{"kind":"NDVI_CHANGE","preparationIds":["before","after"],"parameters":{"threshold":0.2},"algorithm":"fixture"}','fixture')`, fmt.Sprintf("purpose-job-%d", i), f.resource.ID(), f.source); e != nil {
			t.Fatal(e)
		}
	}
	status, body = scopeRequest(t, f.h, f.manager, "GET", "/api/v1/platform/readiness", nil, nil)
	if status != 200 {
		t.Fatalf("readiness: %d %v", status, body)
	}
	for _, v := range body["features"].([]any) {
		f := v.(map[string]any)
		if f["id"] == "reference" && f["count"] != float64(0) {
			t.Fatal("test versions inflated working count", f)
		}
	}
	counts := map[string]float64{}
	for _, item := range body["features"].([]any) {
		value := item.(map[string]any)
		counts[value["id"].(string)] = value["count"].(float64)
	}
	if counts["imagery-captures"] != 4 || counts["imagery-acquisition-dates"] != 2 || counts["imagery-studies"] != 1 {
		t.Fatal("capture/date/study counts conflated", counts)
	}
	// Exact history stays readable; purpose metadata cannot change original bytes.
	status, body = scopeRequest(t, f.h, f.manager, "GET", "/api/v1/reference-datasets/"+id, nil, nil)
	if status != 200 || body["sha256"] != input.SHA256 {
		t.Fatalf("original evidence changed: %d %v", status, body)
	}
	classify["purpose"] = "working"
	if status, _ = scopeRequest(t, f.h, f.manager, "POST", "/api/v1/content-purpose", classify, nil); status != 409 {
		t.Fatal("stale classification overwrote history", status)
	}
	classify["expectedVersion"] = 1
	classify["reason"] = "Reviewed source reference is reusable working input; test lineage remains retained"
	if status, _ = scopeRequest(t, f.h, f.manager, "POST", "/api/v1/content-purpose", classify, nil); status != 201 {
		t.Fatal("explicit reference reclassification", status)
	}
	status, body = scopeRequest(t, f.h, f.manager, "GET", "/api/v1/reference-datasets", nil, nil)
	if status != 200 || body["total"] != float64(1) {
		t.Fatalf("working reference absent: %d %v", status, body)
	}
	status, body = scopeRequest(t, f.h, f.manager, "GET", "/api/v1/content-purpose?kind=reference&view=all", nil, nil)
	if status != 200 || body["total"] != float64(1) || body["items"].([]any)[0].(map[string]any)["version"] != float64(2) {
		t.Fatalf("purpose inventory: %d %v", status, body)
	}
	status, body = scopeRequest(t, f.h, f.manager, "GET", "/api/v1/content-purpose?kind=reference&view=all&offset=100", nil, nil)
	if status != 200 || body["total"] != float64(1) || len(body["items"].([]any)) != 0 {
		t.Fatalf("empty purpose page lost total: %d %v", status, body)
	}
	if status, _ = scopeRequest(t, f.h, f.manager, "GET", "/api/v1/content-purpose?kind=assessment&view=all", nil, nil); status != 403 {
		t.Fatal("source management substituted for assessment read", status)
	}
	if e := access.NewStore(f.h.pool).AddGrant(access.Bootstrap(context.Background()), "source-reviewer", access.Grant{Resource: f.resource, Permissions: []string{"assessments.read"}}); e != nil {
		t.Fatal(e)
	}
	// Counter fixtures model published revisions with canonical identity
	// separation; actual review enforcement is tested by the imagery gate.
	for _, name := range []string{"development-publication", "working-publication"} {
		if _, e := f.h.pool.Exec(context.Background(), `INSERT INTO assessments(id,scope_id,subject_type,subject_id,type,conclusion,method,review_source_id)VALUES($1,$2,'source',$3,'fixture','Counter qualification only','OPERATOR',$3)`, name, f.resource.ID(), f.source); e != nil {
			t.Fatal(e)
		}
		if _, e := f.h.pool.Exec(context.Background(), `INSERT INTO assessment_revisions(id,assessment_id,scope_id,source_id,version,status,document)VALUES($1,$2,$3,$4,1,'PUBLISHED','{"status":"PUBLISHED","conclusion":"Counter qualification only"}')`, name+"-revision", name, f.resource.ID(), f.source); e != nil {
			t.Fatal(e)
		}
		if _, e := f.h.pool.Exec(context.Background(), `INSERT INTO assessment_review_heads(assessment_id,scope_id,source_id,latest_id,published_id)VALUES($1,$2,$3,$4,$4)`, name, f.resource.ID(), f.source, name+"-revision"); e != nil {
			t.Fatal(e)
		}
		if _, e := f.h.pool.Exec(context.Background(), `INSERT INTO assessment_review_identities(revision_id,scope_id,source_id,author_human_id,actor_human_id,publisher_human_id)VALUES($1,$2,$3,'fixture-author','fixture-reviewer','fixture-reviewer')`, name+"-revision", f.resource.ID(), f.source); e != nil {
			t.Fatal(e)
		}
	}
	status, body = scopeRequest(t, f.h, f.manager, "GET", "/api/v1/platform/readiness", nil, nil)
	if status != 200 {
		t.Fatalf("published counter: %d %v", status, body)
	}
	for _, item := range body["features"].([]any) {
		v := item.(map[string]any)
		if v["id"] == "working-reviewed-outputs" && v["count"] != float64(0) {
			t.Fatal("unclassified publications counted as working", v)
		}
	}
	purpose := map[string]any{"kind": "assessment", "entityId": "development-publication", "purpose": "development", "reason": "Exact development publication used only for counter qualification", "receiptReference": "isolated-counter-fixture", "expectedVersion": 0}
	if status, _ = scopeRequest(t, f.h, f.manager, "POST", "/api/v1/content-purpose", purpose, nil); status != 201 {
		t.Fatal("classify development publication", status)
	}
	purpose["purpose"] = "working"
	purpose["expectedVersion"] = 1
	if status, _ = scopeRequest(t, f.h, f.manager, "POST", "/api/v1/content-purpose", purpose, nil); status != 409 {
		t.Fatal("test publication promoted into working history", status)
	}
	purpose["entityId"] = "working-publication"
	purpose["expectedVersion"] = 0
	if status, _ = scopeRequest(t, f.h, f.manager, "POST", "/api/v1/content-purpose", purpose, nil); status != 201 {
		t.Fatal("classify working publication", status)
	}
	status, body = scopeRequest(t, f.h, f.manager, "GET", "/api/v1/platform/readiness", nil, nil)
	if status != 200 {
		t.Fatalf("working counter: %d %v", status, body)
	}
	for _, item := range body["features"].([]any) {
		v := item.(map[string]any)
		if v["id"] == "working-reviewed-outputs" && v["count"] != float64(1) {
			t.Fatal("working/development publications conflated", v)
		}
	}
	ctx := scopeContext(t, f.h, f.resource)
	err := db.WithTx(ctx, f.h.pool, func(tx pgx.Tx) error {
		_, e := tx.Exec(ctx, `UPDATE content_purpose_revisions SET purpose='working' WHERE entity_id=$1`, id)
		return e
	})
	if err == nil {
		t.Fatal("purpose history allowed mutation")
	}
	denied := scopeToken(t, f.h, "purpose-denied", f.resource, []string{"operational.read", "sources.read", "sources.manage"}, []string{"not-this-source"})
	status, body = scopeRequest(t, f.h, denied, "GET", "/api/v1/content-purpose?view=all", nil, nil)
	if status != 200 || body["total"] != float64(0) {
		t.Fatalf("source-restricted classification leaked: %d %v", status, body)
	}
	// Pooled settings must revert after an administrator's all-content request.
	read := scopeToken(t, f.h, "purpose-reader", f.resource, []string{"operational.read", "sources.read"}, []string{f.source})
	if status, _ = scopeRequest(t, f.h, read, "GET", "/api/v1/reference-datasets", nil, nil); status != 200 {
		t.Fatal(status)
	}
	other := access.Resource{WorkspaceID: "other-purpose", EnvironmentID: "world", Partition: access.PublicEvidence}
	otherToken := scopeToken(t, f.h, "purpose-other", other, []string{"operational.read", "sources.read", "sources.manage"}, nil)
	status, body = scopeRequest(t, f.h, otherToken, "GET", "/api/v1/content-purpose?view=all", nil, nil)
	if status != 200 || body["total"] != float64(0) {
		t.Fatalf("scope purpose leak: %d %v", status, body)
	}
}
