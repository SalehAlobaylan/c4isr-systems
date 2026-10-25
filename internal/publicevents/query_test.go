package publicevents

import (
	"net/url"
	"testing"
	"time"
)

func TestPublicHistoryFiltersRejectAmbiguousAndInvalidSelections(t *testing.T) {
	now := time.Now().UTC()
	for _, raw := range []string{"view=unknown", "view=known_at", "as_of=2020-01-01T00:00:00Z", "view=known_at&as_of=invalid", "revisions=uncertain", "quality=confirmed", "bbox=0,0,180,90", "bbox=NaN,0,1,1", "observed_from=2025-01-02T00:00:00Z&observed_to=2025-01-01T00:00:00Z", "source_id=a&source_id=b", "offset=10001", "limit=201", "unexpected=1"} {
		q, _ := url.ParseQuery(raw)
		if _, err := ParseFilters(q, now); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
}
func TestPublicHistoryFiltersPreserveUTCAndMeaning(t *testing.T) {
	q, _ := url.ParseQuery("view=known_at&as_of=2025-01-01T03:00:00%2B03:00&revisions=all&bbox=39,21,40,22&quality=nominal&phenomenon=thermal.detection&limit=25&offset=25")
	f, err := ParseFilters(q, time.Now())
	if err != nil || f.AsOf.Hour() != 0 || f.AsOf.Location() != time.UTC || f.Limit != 25 || f.Bounds.West != 39 || f.Revisions != "all" {
		t.Fatalf("filters %+v %v", f, err)
	}
	if f.Meaning().Mode != "known_at" || f.Meaning().KnownTimeBasis == "" {
		t.Fatal("historical semantics not explicit")
	}
}
