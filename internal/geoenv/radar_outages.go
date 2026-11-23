package geoenv

import (
	"cmp"
	"encoding/json"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
)

var radarCountryCode = regexp.MustCompile("^[A-Z]{2}$")

type outageAnnotation struct {
	ID           string     `json:"id"`
	StartAt      time.Time  `json:"startAt"`
	EndAt        *time.Time `json:"endAt"`
	ProviderType string     `json:"providerType"`
}

type outageAnnotations struct {
	Country        string             `json:"country"`
	From           time.Time          `json:"from"`
	ToExclusive    time.Time          `json:"toExclusive"`
	AvailabilityAt *time.Time         `json:"availabilityAt"`
	Coverage       string             `json:"coverage"`
	Annotations    []outageAnnotation `json:"annotations"`
}

func (s Service) reviewRadarOutages(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	country := query.Get("country")
	from, errFrom := time.Parse(time.RFC3339Nano, query.Get("from"))
	to, errTo := time.Parse(time.RFC3339Nano, query.Get("toExclusive"))
	if !radarCountryCode.MatchString(country) || errFrom != nil || errTo != nil ||
		!from.Before(to) || to.Sub(from) > 366*24*time.Hour || to.After(time.Now()) {
		httpx.Error(w, apperr.Validation("provide a two-letter country code and a historical interval of up to 366 days"))
		return
	}
	s.reviewSimulationSource(w, r, "cloudflare-radar-annotations", "v4", query.Get("inputSha256"),
		func(raw []byte) (string, any, error) {
			report, err := summarizeRadarOutages(raw, country, from, to)
			return "outages", report, err
		})
}

func summarizeRadarOutages(raw []byte, country string, from, to time.Time) (outageAnnotations, error) {
	out := outageAnnotations{Country: country, From: from.UTC(), ToExclusive: to.UTC(),
		Coverage: "annotations-in-supplied-response", Annotations: []outageAnnotation{}}
	var page struct {
		Success bool              `json:"success"`
		Errors  []json.RawMessage `json:"errors"`
		Result  struct {
			Annotations []struct {
				ID        string   `json:"id"`
				EventType string   `json:"eventType"`
				Locations []string `json:"locations"`
				StartDate string   `json:"startDate"`
				EndDate   string   `json:"endDate"`
				Outage    struct {
					Type string `json:"outageType"`
				} `json:"outage"`
			} `json:"annotations"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &page); err != nil || !page.Success || len(page.Errors) > 0 || page.Result.Annotations == nil {
		return out, apperr.Validation("expected a successful Cloudflare Radar annotations response")
	}
	seen := map[string]bool{}
	for _, row := range page.Result.Annotations {
		if row.EventType != "OUTAGE" || !slices.Contains(row.Locations, country) {
			continue
		}
		if !keyPattern.MatchString(row.ID) || seen[row.ID] || strings.TrimSpace(row.Outage.Type) == "" {
			return out, apperr.Validation("Radar annotation has an invalid or duplicate identity or missing type")
		}
		seen[row.ID] = true
		start, err := time.Parse(time.RFC3339Nano, row.StartDate)
		if err != nil {
			return out, apperr.Validation("Radar annotation has an invalid start time")
		}
		item := outageAnnotation{ID: row.ID, StartAt: start.UTC(), ProviderType: row.Outage.Type}
		if row.EndDate != "" {
			end, err := time.Parse(time.RFC3339Nano, row.EndDate)
			if err != nil || end.Before(start) {
				return out, apperr.Validation("Radar annotation has an invalid end time")
			}
			utcEnd := end.UTC()
			item.EndAt = &utcEnd
		}
		if (item.EndAt != nil && !item.EndAt.After(from)) || !start.Before(to) {
			continue
		}
		out.Annotations = append(out.Annotations, item)
	}
	slices.SortFunc(out.Annotations, func(a, b outageAnnotation) int {
		if order := a.StartAt.Compare(b.StartAt); order != 0 {
			return order
		}
		return cmp.Compare(a.ID, b.ID)
	})
	return out, nil
}
