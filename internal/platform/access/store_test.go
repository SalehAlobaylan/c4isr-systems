package access

import (
	"context"
	"testing"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAuthenticationDatabaseFailureIsNotCredentialRejection(t *testing.T) {
	// A closed, lazy pool models unavailable infrastructure without contacting a
	// database. Workers must be able to retry this error instead of exiting as
	// though their credential had been revoked.
	pool, err := pgxpool.New(context.Background(), "postgres://fixture:fixture@127.0.0.1:1/fixture?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	pool.Close()
	store := NewStore(pool)
	if _, err = store.Authenticate(context.Background(), "fixture-token", nil); !apperr.Is(err, apperr.CodeInternal) {
		t.Fatalf("database outage was misclassified: %v", err)
	}
}
