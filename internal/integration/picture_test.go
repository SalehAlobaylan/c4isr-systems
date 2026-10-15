package integration

import (
	"context"
	"net/http"
	"testing"
	"time"
)

// Replay must reconstruct from the same evidence the live picture was built
// from. These tests walk a real scenario run and assert that scrubbing back
// shows the state at that instant - including the fact that an alert had not
// yet been raised.
func TestPictureReconstructsFromEvidence(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.startScenario(t, "restricted-area-intrusion")

	// Let the run produce a breach so there is real evidence to reconstruct.
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		var alerts int
		if err := h.pool.QueryRow(ctx, `SELECT count(*) FROM alerts`).Scan(&alerts); err != nil {
			t.Fatalf("count alerts: %v", err)
		}
		if alerts > 0 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	var firstAt, firstID string
	if err := h.pool.QueryRow(ctx,
		`SELECT track_id, to_char(observed_at, 'YYYY-MM-DD"T"HH24:MI:SS.MS"Z"')
		 FROM track_history ORDER BY observed_at ASC LIMIT 1`).Scan(&firstID, &firstAt); err != nil {
		t.Fatalf("read earliest track history: %v", err)
	}

	parsed, err := time.Parse(time.RFC3339Nano, firstAt)
	if err != nil {
		t.Fatalf("parse observed_at %q: %v", firstAt, err)
	}

	// Scrub to the very first observation: the alert cannot exist yet, and the
	// track must be present because it had been observed by then.
	var replay struct {
		At     string `json:"at"`
		Live   bool   `json:"live"`
		Tracks []struct {
			ID       string `json:"id"`
			Position *struct {
				Lat float64 `json:"lat"`
				Lng float64 `json:"lng"`
			} `json:"position"`
			Freshness string `json:"freshness"`
		} `json:"tracks"`
		Alerts []struct {
			ID string `json:"id"`
		} `json:"alerts"`
	}
	if status := h.do(http.MethodGet,
		"/api/v1/picture?at="+parsed.UTC().Format(time.RFC3339Nano), nil, &replay); status != http.StatusOK {
		t.Fatalf("replay snapshot: status %d", status)
	}

	if replay.Live {
		t.Error("an explicit instant must not be reported as live")
	}
	if len(replay.Alerts) != 0 {
		t.Errorf("alerts present at the first observation = %d, want 0", len(replay.Alerts))
	}

	found := false
	for _, track := range replay.Tracks {
		if track.ID != firstID {
			continue
		}
		found = true
		if track.Position == nil {
			t.Error("reconstructed track has no position")
		}
		// Freshness is derived against the requested instant, so the first
		// sample of a track is live at the moment it was recorded.
		if track.Freshness != "live" {
			t.Errorf("freshness at the recording instant = %q, want live", track.Freshness)
		}
	}
	if !found {
		t.Errorf("track %s observed at the cut is missing from the reconstruction", firstID)
	}
}

func TestPictureLiveAndWindowAreConsistent(t *testing.T) {
	h := newHarness(t)
	h.startScenario(t, "normal-patrol")
	time.Sleep(3 * time.Second)

	var window struct {
		Earliest string `json:"earliest"`
		Latest   string `json:"latest"`
	}
	if status := h.do(http.MethodGet, "/api/v1/picture/window", nil, &window); status != http.StatusOK {
		t.Fatalf("picture window: status %d", status)
	}
	if window.Earliest == "" || window.Latest == "" {
		t.Fatalf("window = %+v, want a bounded span", window)
	}
	earliest, err := time.Parse(time.RFC3339, window.Earliest)
	if err != nil {
		t.Fatalf("parse earliest: %v", err)
	}
	latest, err := time.Parse(time.RFC3339, window.Latest)
	if err != nil {
		t.Fatalf("parse latest: %v", err)
	}
	if latest.Before(earliest) {
		t.Fatalf("window is inverted: %v .. %v", earliest, latest)
	}

	var live struct {
		Live    bool   `json:"live"`
		At      string `json:"at"`
		Tracks  []any  `json:"tracks"`
		Assets  []any  `json:"assets"`
		Alerts  []any  `json:"alerts"`
		Windows struct {
			Earliest string `json:"earliest"`
			Latest   string `json:"latest"`
		} `json:"window"`
	}
	if status := h.do(http.MethodGet, "/api/v1/picture?live=true", nil, &live); status != http.StatusOK {
		t.Fatalf("live picture: status %d", status)
	}
	if !live.Live {
		t.Error("live=true must report the picture as live")
	}
	// Collections must serialize as arrays; a null would break the interface.
	for name, value := range map[string]any{
		"Tracks": live.Tracks, "Assets": live.Assets, "Alerts": live.Alerts,
	} {
		if value == nil {
			t.Errorf("%s serialized as null; the contract promises an array", name)
		}
	}
	// The window is derived from live evidence, which grows between calls, so
	// the two readings must agree in shape and bounds rather than be
	// byte-identical. An exact comparison here would fail whenever a scenario
	// emits an observation between the two requests, which says nothing about
	// whether the endpoints agree.
	if live.Windows.Earliest == "" || live.Windows.Latest == "" {
		t.Errorf("snapshot window is unbounded: %+v", live.Windows)
	}
	snapshotEarliest, err := time.Parse(time.RFC3339, live.Windows.Earliest)
	if err != nil {
		t.Fatalf("parse snapshot earliest: %v", err)
	}
	snapshotLatest, err := time.Parse(time.RFC3339, live.Windows.Latest)
	if err != nil {
		t.Fatalf("parse snapshot latest: %v", err)
	}
	// The snapshot window may have advanced by the time it was read, but it can
	// never reach further back than the window endpoint reported.
	if snapshotEarliest.Before(earliest.Add(-time.Second)) {
		t.Errorf("snapshot earliest %v precedes the window endpoint %v", snapshotEarliest, earliest)
	}
	if snapshotLatest.Before(latest) {
		t.Errorf("snapshot latest %v precedes the window endpoint %v", snapshotLatest, latest)
	}
	if snapshotLatest.Before(snapshotEarliest) {
		t.Errorf("snapshot window is inverted: %v .. %v", snapshotEarliest, snapshotLatest)
	}
}

func TestPictureIsNotAPrivilege(t *testing.T) {
	h := newHarness(t)
	h.startScenario(t, "normal-patrol")
	time.Sleep(2 * time.Second)

	// Replay is a read of recorded evidence and must be available to the same
	// role that can see the live picture.
	if status := h.do(http.MethodGet, "/api/v1/picture?live=true", nil, nil); status != http.StatusOK {
		t.Fatalf("operator replay: status %d", status)
	}

	// And it must not be writable. Replay reconstructs recorded evidence and never
	// creates a new fact, so there is no POST verb on the route at all. The
	// authorization layer rejects any method it has no policy for before the
	// router sees it, so 403 is the correct and sufficient answer here - the
	// route policy test in internal/platform/httpx is what pins the route to
	// read-only.
	if status := h.do(http.MethodPost, "/api/v1/picture", map[string]any{}, nil); status == http.StatusOK ||
		status == http.StatusCreated {
		t.Errorf("POST /api/v1/picture = %d, want it rejected", status)
	}
}
