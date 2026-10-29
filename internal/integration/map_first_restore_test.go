package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/app"
	"github.com/SalehAlobaylan/c4isr-systems/internal/investigations"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/config"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/logging"
)

func TestMapFirstPairedRecoveryRetainsCasesApprovalJobsAndPrivateBytes(t *testing.T) {
	original := newS3Fixture(t)
	f := newPublicFixture(t, app.WithArtifactStore(original))
	grantCase(t, f)
	reviewer := caseReviewer(t, f)
	c := caseDecode(t, caseCall(t, f, f.manager, "POST", "/api/v1/investigations", caseInput(), 201))
	shareCase(t, f, c)
	in := deliveryInput(c)
	job := caseCall(t, f, f.manager, "POST", "/api/v1/investigations/"+c.ID+"/delivery-jobs", in, 202)
	id := job["outputId"].(string)
	svc := investigations.Service{Pool: f.h.pool, Store: original}
	auth := deliveryAuth(t, f, f.manager)
	if did, e := svc.WorkDelivery(auth, strings.Repeat("e", 64)); !did || e != nil {
		t.Fatal(did, e)
	}
	output, html, e := svc.ReadOutput(auth, id, false)
	if e != nil {
		t.Fatal(e)
	}
	review := investigations.ReviewInput{Action: "REVIEW_REQUESTED", ExpectedRevisionID: c.Revision.ID, ReviewerHumanID: "independent-case-human", OutputID: id, Reason: "Inspect exact private bytes before isolated paired recovery"}
	event := caseCall(t, f, f.manager, "POST", "/api/v1/investigations/"+c.ID+"/review", review, 201)
	caseCall(t, f, reviewer, "POST", "/api/v1/investigations/"+c.ID+"/review", investigations.ReviewInput{Action: "APPROVED", ExpectedRevisionID: c.Revision.ID, ExpectedEventID: event["id"].(string), Reason: "Synthetic independent identity inspected this exact retained rendering"}, 201)
	beforeBundle := rawDelivery(t, f, f.manager, "/api/v1/outputs/"+id+"/bundle", 200)
	// Source service credentials do not become case members after restore.
	caseCall(t, f, f.worker, "GET", "/api/v1/investigations/"+c.ID, nil, 403)
	pg := f.h.postgresContainerID
	python := filepath.Join("..", "..", ".venv-recovery", "Scripts", "python.exe")
	if _, err := os.Stat(python); err != nil {
		python = "python3"
	}
	inventoryCode := `import sys,json;sys.path.insert(0,'../../scripts/ops');from recovery_inventory import inventory;print(json.dumps(inventory(sys.argv[1],sys.argv[2]),sort_keys=True))`
	readInventory := func(database string) []byte {
		t.Helper()
		raw, e := exec.Command(python, "-c", inventoryCode, pg, database).Output()
		if e != nil {
			t.Fatal("private recovery inventory failed", e)
		}
		return bytes.TrimSpace(raw)
	}
	before := readInventory("c4isr")
	dump, e := exec.Command("docker", "exec", pg, "pg_dump", "-U", "c4isr", "-d", "c4isr", "--format=custom", "--no-owner").Output()
	if e != nil {
		t.Fatal("dump", e)
	}
	if !bytes.Equal(before, readInventory("c4isr")) {
		t.Fatal("writers changed paired recovery snapshot")
	}
	if raw, e := exec.Command("docker", "exec", pg, "createdb", "-U", "c4isr", "--template=template0", "qualification_restore").CombinedOutput(); e != nil {
		t.Fatal("isolated database", e, string(raw))
	}
	command := exec.Command("docker", "exec", "-i", pg, "pg_restore", "-U", "c4isr", "-d", "qualification_restore", "--no-owner", "--exit-on-error")
	command.Stdin = bytes.NewReader(dump)
	if raw, e := command.CombinedOutput(); e != nil {
		t.Fatal("restore", e, string(raw))
	}
	after := readInventory("qualification_restore")
	if !bytes.Equal(before, after) {
		var a, b map[string]json.RawMessage
		_ = json.Unmarshal(before, &a)
		_ = json.Unmarshal(after, &b)
		for key, value := range a {
			if !bytes.Equal(value, b[key]) {
				t.Log("inventory section differs:", key)
			}
		}
		t.Fatal("restored inventory differs, including ACLs/review identity/jobs/leases/intents")
	}
	restoredObjects := newS3Fixture(t)
	var key string
	if e = f.h.pool.QueryRow(context.Background(), `SELECT object_key FROM composite_outputs WHERE id=$1`, id).Scan(&key); e != nil {
		t.Fatal(e)
	}
	raw, e := original.Get(context.Background(), key, int64(len(html)))
	if e != nil || !bytes.Equal(raw, html) {
		t.Fatal("paired bytes", e)
	}
	if e = restoredObjects.Put(context.Background(), key, raw, output.MediaType); e != nil {
		t.Fatal(e)
	}
	pc := f.h.pool.Config().Copy()
	pc.ConnConfig.Database = "qualification_restore"
	pool, e := db.Open(context.Background(), pc.ConnConfig.ConnString())
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	// Current binary, original credentials and independent object store; no
	// reprovisioning or grants are applied after the database restoration.
	cfg := config.Config{DatabaseURL: pc.ConnConfig.ConnString(), Environment: config.EnvironmentTest, ProductProfile: "map", AuthRequired: true, AuthTokens: map[string]string{integrationToken: "operator-01"}, ScenariosDir: scenariosDir()}
	application, e := app.New(context.Background(), cfg, logging.New(io.Discard, "text", "error"), app.WithArtifactStore(restoredObjects))
	if e != nil {
		t.Fatal(e)
	}
	defer application.Close()
	server := httptest.NewServer(application.Handler())
	defer server.Close()
	restored := *f
	restored.h = &harness{t: t, server: server, pool: pool, app: application}
	got := caseCall(t, &restored, f.manager, "GET", "/api/v1/investigations/"+c.ID, nil, 200)
	if got["id"] != c.ID {
		t.Fatal("case identity")
	}
	afterBundle := rawDelivery(t, &restored, f.manager, "/api/v1/outputs/"+id+"/bundle", 200)
	if !bytes.Equal(beforeBundle, afterBundle) {
		t.Fatal("paired recovery changed exact approval bundle")
	}
	caseCall(t, &restored, f.worker, "GET", "/api/v1/investigations/"+c.ID, nil, 403)
	state := caseCall(t, &restored, f.manager, "GET", "/api/v1/delivery-jobs/"+job["id"].(string), nil, 200)
	if state["state"] != "SUCCEEDED" || state["attempt"] != float64(1) {
		t.Fatal("attempt history changed", state)
	}
	// Same original intent resolves without another output or render attempt.
	recovered := caseCall(t, &restored, f.manager, "POST", "/api/v1/investigations/"+c.ID+"/delivery-jobs", in, 202)
	if recovered["id"] != job["id"] {
		t.Fatal("lost intent after recovery")
	}
	if dir := os.Getenv("C4ISR_TEST_EVIDENCE_DIR"); dir != "" {
		var inventory map[string]any
		_ = json.Unmarshal(after, &inventory)
		report := map[string]any{"schema": "emad.map-first-paired-recovery-sample.v1", "status": "PASS", "recordedAt": time.Now().UTC(), "tablesVerified": len(inventory["tables"].(map[string]any)), "briefSha256": output.RenderSHA256, "approvalBundleUnchanged": true, "canonicalIdentityAndDeniedAccess": true, "originalIntentAndAttemptPreserved": true, "privateStores": 2, "databaseDumpRestored": true, "scope": "Synthetic populated case/review/delivery regression; not full installed geography/imagery/environment dependency replay or actual rollback binary qualification"}
		data, _ := json.MarshalIndent(report, "", "  ")
		if e = os.WriteFile(filepath.Join(dir, "paired-recovery.json"), data, 0600); e != nil {
			t.Fatal(e)
		}
	}
}
