package assets

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/freshness"
)

func discardDispatcher() *events.Dispatcher {
	return events.NewDispatcher(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

type freshnessRepository struct {
	assets map[string]Asset
}

func (f *freshnessRepository) Create(_ context.Context, asset Asset) (Asset, error) {
	if f.assets == nil {
		f.assets = map[string]Asset{}
	}
	f.assets[asset.ID] = asset
	return asset, nil
}

func (f *freshnessRepository) Get(_ context.Context, id string) (Asset, error) {
	asset, ok := f.assets[id]
	if !ok {
		return Asset{}, apperr.NotFound("asset", id)
	}
	return asset, nil
}

func (f *freshnessRepository) Exists(_ context.Context, id string) (bool, error) {
	_, ok := f.assets[id]
	return ok, nil
}

func (f *freshnessRepository) List(_ context.Context, limit, offset int) ([]Asset, int, error) {
	items := make([]Asset, 0, len(f.assets))
	for _, asset := range f.assets {
		items = append(items, asset)
	}
	return items, len(items), nil
}

func (f *freshnessRepository) UpdateStatus(_ context.Context, id string, status Status) (Asset, error) {
	asset, ok := f.assets[id]
	if !ok {
		return Asset{}, apperr.NotFound("asset", id)
	}
	asset.Status = status
	f.assets[id] = asset
	return asset, nil
}

func freshnessService(t *testing.T, now time.Time, assets ...Asset) (*Service, *freshnessRepository) {
	t.Helper()
	repo := &freshnessRepository{assets: map[string]Asset{}}
	for _, asset := range assets {
		repo.assets[asset.ID] = asset
	}
	svc := NewServiceWithFreshness(repo, discardDispatcher(), freshness.Policy{
		DelayedAfter:     time.Minute,
		StaleAfter:       5 * time.Minute,
		UnavailableAfter: 30 * time.Minute,
	})
	svc.now = func() time.Time { return now }
	return svc, repo
}

func TestGetClassifiesFreshness(t *testing.T) {
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	thirtySecondsAgo := now.Add(-30 * time.Second)
	fourMinutesAgo := now.Add(-4 * time.Minute)
	twentyMinutesAgo := now.Add(-20 * time.Minute)

	cases := []struct {
		name  string
		asset Asset
		want  freshness.State
	}{
		{
			name:  "recently seen asset is live",
			asset: Asset{ID: "ast_1", Status: StatusAvailable, ConnectionState: "connected", LastSeenAt: &thirtySecondsAgo},
			want:  freshness.StateLive,
		},
		{
			name:  "asset past the delayed threshold is delayed",
			asset: Asset{ID: "ast_5", Status: StatusAvailable, ConnectionState: "connected", LastSeenAt: &fourMinutesAgo},
			want:  freshness.StateDelayed,
		},
		{
			name:  "asset that stopped reporting is stale",
			asset: Asset{ID: "ast_2", Status: StatusAvailable, ConnectionState: "connected", LastSeenAt: &twentyMinutesAgo},
			want:  freshness.StateStale,
		},
		{
			name:  "declared offline beats a recent report",
			asset: Asset{ID: "ast_3", Status: StatusOffline, ConnectionState: "connected", LastSeenAt: &thirtySecondsAgo},
			want:  freshness.StateUnavailable,
		},
		{
			name:  "registered but never reporting has no data",
			asset: Asset{ID: "ast_4", Status: StatusAvailable, ConnectionState: "unknown"},
			want:  freshness.StateUnknown,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := freshnessService(t, now, tc.asset)
			got, err := svc.Get(context.Background(), tc.asset.ID)
			if err != nil {
				t.Fatalf("Get() error = %v", err)
			}
			if got.Freshness != tc.want {
				t.Fatalf("Freshness = %q, want %q", got.Freshness, tc.want)
			}
		})
	}
}

func TestListClassifiesFreshness(t *testing.T) {
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	old := now.Add(-90 * time.Minute)
	svc, _ := freshnessService(t, now,
		Asset{ID: "ast_live", Status: StatusAvailable, ConnectionState: "connected", LastSeenAt: &now},
		Asset{ID: "ast_old", Status: StatusAvailable, ConnectionState: "connected", LastSeenAt: &old},
	)

	items, _, err := svc.List(context.Background(), 0, 0)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	got := map[string]freshness.State{}
	for _, item := range items {
		got[item.ID] = item.Freshness
	}
	if got["ast_live"] != freshness.StateLive {
		t.Errorf("ast_live freshness = %q, want %q", got["ast_live"], freshness.StateLive)
	}
	if got["ast_old"] != freshness.StateUnavailable {
		t.Errorf("ast_old freshness = %q, want %q", got["ast_old"], freshness.StateUnavailable)
	}
}

func TestFreshnessIsNotPersisted(t *testing.T) {
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	longAgo := now.Add(-time.Hour)
	svc, repo := freshnessService(t, now, Asset{ID: "ast_1", Status: StatusAvailable, ConnectionState: "connected", LastSeenAt: &longAgo})

	read, err := svc.Get(context.Background(), "ast_1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if read.Freshness == "" {
		t.Fatal("Get() must classify freshness")
	}
	if stored := repo.assets["ast_1"]; stored.Freshness != "" {
		t.Fatalf("freshness was written back to the repository: %q", stored.Freshness)
	}
}

func TestUpdateStatusClassifiesFreshness(t *testing.T) {
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	svc, _ := freshnessService(t, now, Asset{ID: "ast_1", Status: StatusAvailable, ConnectionState: "connected"})

	updated, err := svc.UpdateStatus(context.Background(), "ast_1", StatusOffline)
	if err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
	if updated.Freshness != freshness.StateUnavailable {
		t.Fatalf("Freshness = %q, want %q", updated.Freshness, freshness.StateUnavailable)
	}
}

func TestNewServiceAppliesDefaultPolicy(t *testing.T) {
	svc := NewService(&freshnessRepository{assets: map[string]Asset{}}, discardDispatcher())
	if svc.fresh != freshness.DefaultPolicy().Normalized() {
		t.Fatalf("default policy = %+v, want %+v", svc.fresh, freshness.DefaultPolicy().Normalized())
	}
}
