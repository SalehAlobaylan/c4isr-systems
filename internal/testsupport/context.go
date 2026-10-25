// Package testsupport supplies explicit authority for existing domain fixtures.
// Negative authorization tests construct their own restricted principals.
package testsupport

import (
	"context"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
)

func Context() context.Context {
	r := access.LegacyResource()
	wildcard := r
	wildcard.RunID = "*"
	p := access.Principal{ID: "fixture-controller", Kind: "human", Grants: []access.Grant{{Resource: wildcard, Permissions: access.Permissions}}}
	ctx, err := access.Bind(context.Background(), p, r, nil)
	if err != nil {
		panic(err)
	}
	return ctx
}
