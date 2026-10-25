package integration

import (
	"context"
	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/modelassets"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"testing"
	"time"
)

func seedAssetBrowser(t *testing.T, h *harness, store artifacts.Store) {
	t.Helper()
	identities := access.NewStore(h.pool)
	admin := access.Bootstrap(context.Background())
	r := access.Resource{WorkspaceID: "default", EnvironmentID: "local", Partition: access.ExerciseInput}
	if e := identities.AddGrant(admin, "human:operator-01", access.Grant{Resource: r, Permissions: []string{"operational.read", "sim.scenarios.author", "audit.read"}}); e != nil {
		t.Fatal(e)
	}
	token := scopeToken(t, h, "browser-asset-worker", r, []string{"operational.read", "sim.assets.import"}, nil)
	bound := workerContext(t, h, token)
	validator, e := modelassets.NewNodeValidator("../../apps/emad-asset-validator/validate.mjs")
	if e != nil {
		t.Fatal(e)
	}
	svc := modelassets.New(h.pool, store)
	work, cancel := context.WithCancel(bound)
	done := make(chan struct{})
	t.Cleanup(func() { cancel(); <-done })
	go func() {
		defer close(done)
		for {
			select {
			case <-work.Done():
				return
			case <-time.After(250 * time.Millisecond):
				if _, e := svc.WorkOne(work, validator, artifacts.Hash([]byte("browser-asset-fixture-worker"))); e != nil && work.Err() == nil {
					t.Errorf("browser asset worker: %v", e)
					return
				}
			}
		}
	}()
}
