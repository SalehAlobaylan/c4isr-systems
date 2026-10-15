package tracks

import (
	"context"
	"testing"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/freshness"
)

func freshnessTrackService(t *testing.T, now time.Time, tracks ...Track) *Service {
	t.Helper()
	repo := newFakeRepository()
	for _, track := range tracks {
		repo.tracks[track.ID] = track
	}
	svc := NewServiceWithFreshness(repo, nil, testDispatcher(), freshness.Policy{
		DelayedAfter:     time.Minute,
		StaleAfter:       5 * time.Minute,
		UnavailableAfter: 30 * time.Minute,
	})
	svc.now = func() time.Time { return now }
	return svc
}

func TestGetClassifiesFreshness(t *testing.T) {
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name  string
		track Track
		want  freshness.State
	}{
		{
			name:  "recently seen track is live",
			track: Track{ID: "trk_1", Status: StatusActive, LastSeenAt: now.Add(-20 * time.Second)},
			want:  freshness.StateLive,
		},
		{
			name:  "aging track is delayed",
			track: Track{ID: "trk_2", Status: StatusActive, LastSeenAt: now.Add(-3 * time.Minute)},
			want:  freshness.StateDelayed,
		},
		{
			name:  "quiet track is stale",
			track: Track{ID: "trk_3", Status: StatusActive, LastSeenAt: now.Add(-12 * time.Minute)},
			want:  freshness.StateStale,
		},
		{
			name:  "long quiet track is unavailable",
			track: Track{ID: "trk_4", Status: StatusActive, LastSeenAt: now.Add(-45 * time.Minute)},
			want:  freshness.StateUnavailable,
		},
		{
			name:  "declared lost track is unavailable",
			track: Track{ID: "trk_5", Status: StatusLost, LastSeenAt: now.Add(-20 * time.Second)},
			want:  freshness.StateUnavailable,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := freshnessTrackService(t, now, tc.track)
			got, err := svc.Get(context.Background(), tc.track.ID)
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
	svc := freshnessTrackService(t, now,
		Track{ID: "trk_live", Status: StatusActive, LastSeenAt: now},
		Track{ID: "trk_lost", Status: StatusLost, LastSeenAt: now},
	)

	items, _, err := svc.List(context.Background(), 0, 0)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	got := map[string]freshness.State{}
	for _, item := range items {
		got[item.ID] = item.Freshness
	}
	if got["trk_live"] != freshness.StateLive {
		t.Errorf("trk_live freshness = %q, want %q", got["trk_live"], freshness.StateLive)
	}
	if got["trk_lost"] != freshness.StateUnavailable {
		t.Errorf("trk_lost freshness = %q, want %q", got["trk_lost"], freshness.StateUnavailable)
	}
}

func TestFindByExternalRefClassifiesFreshness(t *testing.T) {
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	svc := freshnessTrackService(t, now, Track{
		ID:          "trk_1",
		ExternalRef: "unknown-01",
		Status:      StatusActive,
		LastSeenAt:  now.Add(-12 * time.Minute),
	})

	got, err := svc.FindByExternalRef(context.Background(), "unknown-01")
	if err != nil {
		t.Fatalf("FindByExternalRef() error = %v", err)
	}
	if got.Freshness != freshness.StateStale {
		t.Fatalf("Freshness = %q, want %q", got.Freshness, freshness.StateStale)
	}
}
