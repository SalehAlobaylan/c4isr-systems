package contextpackage

import (
	"context"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"testing"
)

func TestExportRefreshCannotMovePinnedContentsAcrossResources(t *testing.T) {
	a := access.Resource{WorkspaceID: "export-a", EnvironmentID: "world", Partition: access.PublicEvidence}
	b := a
	b.WorkspaceID = "export-b"
	p := access.Principal{ID: "approver", Kind: "human", Grants: []access.Grant{{Resource: a, Permissions: []string{"operational.read", "context.export"}}, {Resource: b, Permissions: []string{"operational.read", "context.export"}}}}
	original, err := access.Bind(context.Background(), p, a, nil)
	if err != nil {
		t.Fatal(err)
	}
	changed := access.WithRefresh(original, func(c context.Context) (context.Context, error) { return access.Bind(c, p, b, nil) })
	if _, err := refreshExport(changed); !apperr.Is(err, apperr.CodeConflict) {
		t.Fatal("pinned export followed a changed default workspace", err)
	}
	revoked := access.WithRefresh(original, func(context.Context) (context.Context, error) { return nil, apperr.Unauthorized("credential revoked") })
	if _, err := refreshExport(revoked); !apperr.Is(err, apperr.CodeUnauthorized) {
		t.Fatal("revoked approver remained authorized", err)
	}
	if _, err := refreshExport(original); err != nil {
		t.Fatal("unchanged approver rejected", err)
	}
}
