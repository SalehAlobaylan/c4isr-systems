// Package integration exercises the fully assembled server against a real
// PostgreSQL/PostGIS instance started with Testcontainers. These tests are
// skipped unless C4ISR_TEST_INTEGRATION=1 so that plain `go test ./...` stays
// Docker-free.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/SalehAlobaylan/c4isr-systems/internal/app"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/config"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/logging"
)

const integrationEnv = "C4ISR_TEST_INTEGRATION"

type harness struct {
	t      *testing.T
	server *httptest.Server
	pool   *pgxpool.Pool
	app    *app.App
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	if os.Getenv(integrationEnv) != "1" {
		t.Skipf("set %s=1 to run integration tests (Docker required)", integrationEnv)
	}

	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, "imresamu/postgis:17-3.5",
		tcpostgres.WithDatabase("c4isr"),
		tcpostgres.WithUsername("c4isr"),
		tcpostgres.WithPassword("c4isr"),
		testcontainers.WithWaitStrategy(
			// PostGIS images log the readiness message a second time after
			// the init scripts (including CREATE EXTENSION) finish.
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(180*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("start postgis container: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Logf("terminate container: %v", err)
		}
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	pool, err := db.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	applyMigrations(ctx, t, pool)

	cfg := config.Config{
		HTTPAddr:     ":0",
		DatabaseURL:  dsn,
		LogLevel:     "error",
		LogFormat:    "text",
		ScenariosDir: scenariosDir(),
	}
	logger := logging.New(io.Discard, "text", "error")
	application, err := app.New(ctx, cfg, logger)
	if err != nil {
		t.Fatalf("build app: %v", err)
	}
	server := httptest.NewServer(application.Handler())

	h := &harness{t: t, server: server, pool: pool, app: application}
	t.Cleanup(func() {
		server.Close()
		application.Close()
		pool.Close()
	})
	return h
}

func scenariosDir() string {
	dir, err := filepath.Abs("../../scenarios")
	if err != nil {
		panic(err)
	}
	return dir
}

// applyMigrations executes the Up section of each goose migration in order.
func applyMigrations(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	files, err := filepath.Glob("../../db/migrations/*.sql")
	if err != nil {
		t.Fatalf("glob migrations: %v", err)
	}
	sort.Strings(files)
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read migration %s: %v", file, err)
		}
		up := extractUpSection(string(raw))
		if strings.TrimSpace(up) == "" {
			continue
		}
		if _, err := pool.Exec(ctx, up); err != nil {
			t.Fatalf("apply migration %s: %v", filepath.Base(file), err)
		}
	}
}

func extractUpSection(content string) string {
	var lines []string
	inUp := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "-- +goose Up"):
			inUp = true
			continue
		case strings.HasPrefix(trimmed, "-- +goose Down"):
			return strings.Join(lines, "\n")
		case strings.HasPrefix(trimmed, "-- +goose StatementBegin"), strings.HasPrefix(trimmed, "-- +goose StatementEnd"):
			continue
		}
		if inUp {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

func (h *harness) do(method, path string, body any, out any) int {
	h.t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			h.t.Fatalf("marshal request: %v", err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, h.server.URL+path, reader)
	if err != nil {
		h.t.Fatalf("build request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-Operator-ID", "operator-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		h.t.Fatalf("read response: %v", err)
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			h.t.Fatalf("%s %s: decode response %q: %v", method, path, string(raw), err)
		}
	}
	return resp.StatusCode
}

func (h *harness) createSource(id string) {
	h.t.Helper()
	status := h.do(http.MethodPost, "/api/v1/sources", map[string]any{
		"id": id, "name": id, "type": "synthetic",
	}, nil)
	if status != http.StatusCreated {
		h.t.Fatalf("create source: status %d", status)
	}
}

func (h *harness) ingestObservation(trackHint string, lat, lng float64, observedAt time.Time) map[string]any {
	h.t.Helper()
	var out map[string]any
	status := h.do(http.MethodPost, "/api/v1/observations", map[string]any{
		"sourceId":   "simulator-test",
		"type":       "track.observation",
		"observedAt": observedAt.UTC().Format(time.RFC3339Nano),
		"position":   map[string]any{"lat": lat, "lng": lng},
		"trackHint":  trackHint,
	}, &out)
	if status != http.StatusCreated {
		h.t.Fatalf("ingest observation: status %d (%v)", status, out)
	}
	return out
}

func waitFor(t *testing.T, timeout time.Duration, interval time.Duration, description string, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(interval)
	}
	t.Fatalf("timed out waiting for %s", description)
}
