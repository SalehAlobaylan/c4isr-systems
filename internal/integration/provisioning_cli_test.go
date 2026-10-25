package integration

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProvisioningCLIReportsIssuesRotatesAndRevokes(t *testing.T) {
	h := newHarness(t)
	t.Setenv("C4ISR_DATABASE_URL", h.pool.Config().ConnConfig.ConnString())
	t.Setenv("C4ISR_DATABASE_URL_FILE", "")
	t.Setenv("C4ISR_AUTH_TOKENS", "")
	t.Setenv("C4ISR_AUTH_TOKENS_FILE", "")
	taskDir := t.TempDir()
	specPath := filepath.Join(taskDir, "connector.json")
	raw, err := json.Marshal(map[string]any{
		"id": "cli-connector", "kind": "service", "name": "CLI fixture",
		"resource":    map[string]string{"workspaceId": "cli", "environmentId": "world", "partition": "PUBLIC_EVIDENCE"},
		"permissions": []string{"sources.read"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(specPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	invoke := func(args ...string) string {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "go", append([]string{"run", "../../apps/emad-admin"}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("provisioning command failed: %v %s", err, out)
		}
		return string(out)
	}
	if out := invoke("-action", "report"); !strings.Contains(out, "sources resolved=") {
		t.Fatalf("missing ownership report: %s", out)
	}
	secretPath := filepath.Join(taskDir, "first.token")
	out := invoke("-action", "provision", "-spec", specPath, "-out", secretPath)
	credentialID := strings.TrimPrefix(strings.Fields(out)[0], "credential=")
	secret, err := os.ReadFile(secretPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, strings.TrimSpace(string(secret))) {
		t.Fatal("bearer credential was printed")
	}
	if status, _ := scopeRequest(t, h, strings.TrimSpace(string(secret)), "GET", "/api/v1/sources", nil, nil); status != 200 {
		t.Fatalf("CLI credential not usable: %d", status)
	}
	secondPath := filepath.Join(taskDir, "second.token")
	rotated := invoke("-action", "rotate", "-spec", specPath, "-credential", credentialID, "-out", secondPath)
	if status, _ := scopeRequest(t, h, strings.TrimSpace(string(secret)), "GET", "/api/v1/sources", nil, nil); status != 401 {
		t.Fatal("rotation retained the old credential")
	}
	newSecret, err := os.ReadFile(secondPath)
	if err != nil {
		t.Fatal(err)
	}
	if status, _ := scopeRequest(t, h, strings.TrimSpace(string(newSecret)), "GET", "/api/v1/sources", nil, nil); status != 200 {
		t.Fatal("rotation did not issue a usable replacement")
	}
	invoke("-action", "revoke", "-credential", strings.TrimPrefix(strings.Fields(rotated)[0], "credential="))
	if status, _ := scopeRequest(t, h, strings.TrimSpace(string(newSecret)), "GET", "/api/v1/sources", nil, nil); status != 401 {
		t.Fatal("CLI revocation did not take effect")
	}
	invoke("-action", "backfill")
	// Restricted evaluator grants require deliberate local authorization. Ordinary
	// administration never receives truth access by default.
	humanSpec := map[string]any{"id": "cli-human-evaluator", "kind": "human", "operatorId": "operator-01", "resource": map[string]string{"workspaceId": "default", "environmentId": "local", "partition": "EXERCISE_OBSERVATION", "runId": "legacy"}, "permissions": []string{"operational.read"}}
	humanRaw, _ := json.Marshal(humanSpec)
	if err = os.WriteFile(specPath, humanRaw, 0600); err != nil {
		t.Fatal(err)
	}
	invoke("-action", "provision", "-spec", specPath, "-out", filepath.Join(taskDir, "human.token"))
	grant := map[string]any{"id": "cli-human-evaluator", "resource": map[string]string{"workspaceId": "default", "environmentId": "local", "partition": "EXERCISE_TRUTH", "runId": "cli-evaluation"}, "permissions": []string{"operational.read", "sim.truth.read", "sim.recordings.read", "sim.evaluation.read"}}
	grantRaw, _ := json.Marshal(grant)
	if err = os.WriteFile(specPath, grantRaw, 0600); err != nil {
		t.Fatal(err)
	}
	denied := exec.Command("go", "run", "../../apps/emad-admin", "-action", "grant", "-spec", specPath)
	if out, e := denied.CombinedOutput(); e == nil || !strings.Contains(string(out), "forbidden") {
		t.Fatal("truth grant did not require explicit local authorization", e, string(out))
	}
	invoke("-action", "grant", "-allow-truth-grants", "-spec", specPath)
	var granted bool
	if err = h.pool.QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM principal_grants WHERE principal_id='cli-human-evaluator' AND partition='EXERCISE_TRUTH' AND run_id='cli-evaluation' AND 'sim.truth.read'=ANY(permissions))`).Scan(&granted); err != nil || !granted {
		t.Fatal("explicit evaluator grant not persisted", err)
	}
}
