package geoenv

import (
	"bytes"
	"cmp"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
)

const supportingSchema = "emad.supporting-source-review.v1"

type supportInput struct {
	Schema      string             `json:"schema"`
	Title       string             `json:"title"`
	Sources     []exerciseSource   `json:"sources"`
	Events      []supportEvent     `json:"events"`
	Fields      []supportField     `json:"fields"`
	Indicators  []supportIndicator `json:"indicators"`
	Comparisons []supportPair      `json:"comparisons"`
	Delivery    *supportDelivery   `json:"delivery,omitempty"`
}

type supportOutput struct {
	Title         string             `json:"title"`
	Cutoff        time.Time          `json:"cutoff"`
	Sources       []exerciseSource   `json:"sources"`
	Events        []supportEvent     `json:"events"`
	UniqueOrigins int                `json:"uniqueOrigins"`
	Fields        []supportField     `json:"fields"`
	Indicators    []supportIndicator `json:"indicators"`
	Comparisons   []supportChange    `json:"comparisons"`
	Delivery      []supportQueue     `json:"delivery"`
}

type supportEvent struct {
	ID          string            `json:"id"`
	SourceID    string            `json:"sourceId"`
	Origin      string            `json:"origin"`
	Kind        string            `json:"kind"`
	Revision    int               `json:"revision"`
	ObservedAt  time.Time         `json:"observedAt"`
	AvailableAt time.Time         `json:"availableAt"`
	EffectiveAt *time.Time        `json:"effectiveAt,omitempty"`
	ExpiresAt   *time.Time        `json:"expiresAt,omitempty"`
	Status      string            `json:"status"`
	Label       string            `json:"label"`
	Area        string            `json:"area"`
	Languages   map[string]string `json:"languages,omitempty"`
	Details     string            `json:"details,omitempty"`
	Value       *float64          `json:"value,omitempty"`
	Unit        string            `json:"unit,omitempty"`
}

func (s Service) reviewSupportingSources(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	cutoff, err := time.Parse(time.RFC3339Nano, query.Get("knownAt"))
	if err != nil {
		httpx.Error(w, apperr.Validation("provide a supporting-evidence cutoff with a UTC offset"))
		return
	}
	s.reviewSimulationSource(w, r, "supporting-source-review", "v1", query.Get("inputSha256"), func(raw []byte) (string, any, error) {
		out, err := replaySupportingSources(raw, cutoff)
		return "review", out, err
	})
}

func ReadSupportingSources(raw []byte, cutoff time.Time, digest string) (any, error) {
	if len(raw) == 0 || len(raw) > simulationReviewMaxBytes || !digestPattern.MatchString(digest) || artifacts.Hash(raw) != digest {
		return nil, apperr.Validation("supporting package differs from its trusted digest or exceeds its input budget")
	}
	out, err := replaySupportingSources(raw, cutoff)
	if err != nil {
		return nil, err
	}
	return map[string]any{"schema": supportingSchema, "inputSha256": digest, "review": out}, nil
}

func replaySupportingSources(raw []byte, cutoff time.Time) (supportOutput, error) {
	var input supportInput
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	out := supportOutput{Cutoff: cutoff.UTC(), Events: []supportEvent{}, Fields: []supportField{}, Indicators: []supportIndicator{}, Comparisons: []supportChange{}, Delivery: []supportQueue{}}
	if err := decoder.Decode(&input); err != nil || decoder.Decode(new(any)) != io.EOF || input.Schema != supportingSchema ||
		cutoff.IsZero() || !regionalLabel(input.Title) || len(input.Sources) == 0 || len(input.Sources) > 64 || len(input.Events) > 500 ||
		len(input.Fields) > 1000 || len(input.Indicators) > 500 || len(input.Comparisons) > 150 ||
		(len(input.Events)+len(input.Fields)+len(input.Indicators) == 0 && input.Delivery == nil) {
		return out, apperr.Validation("expected one bounded supporting evidence package")
	}
	sources := map[string]bool{}
	for _, source := range input.Sources {
		if !keyPattern.MatchString(source.ID) || sources[source.ID] || !digestPattern.MatchString(source.SHA256) ||
			!regionalLabel(source.Edition) || !regionalLabel(source.Attribution) {
			return out, apperr.Validation("invalid supporting source edition, identity or digest")
		}
		sources[source.ID] = true
	}
	out.Title, out.Sources = input.Title, input.Sources
	if err := input.reviewEvents(sources, &out); err != nil {
		return out, err
	}
	if err := input.reviewFields(sources, &out); err != nil {
		return out, err
	}
	if err := input.reviewIndicators(sources, &out); err != nil {
		return out, err
	}
	if input.Delivery != nil {
		rows, err := input.Delivery.review(cutoff, sources)
		out.Delivery = rows
		return out, err
	}
	return out, nil
}

func (input supportInput) reviewEvents(sources map[string]bool, out *supportOutput) error {
	type revision struct {
		source, id string
		n          int
	}
	seen := map[revision]bool{}
	lineage := map[[2]string][2]string{}
	heads := map[[2]string]int{}
	origins := map[string]bool{}
	training := map[string]bool{}
	for _, row := range input.Events {
		id := [2]string{row.SourceID, row.ID}
		key := revision{row.SourceID, row.ID, row.Revision}
		if !keyPattern.MatchString(row.ID) || !keyPattern.MatchString(row.Origin) || !sources[row.SourceID] || seen[key] ||
			!regionalLabel(row.Label) || !regionalLabel(row.Area) || len(row.Details) > 640 || row.Revision < 1 ||
			row.ObservedAt.IsZero() || row.AvailableAt.Before(row.ObservedAt) ||
			(row.ExpiresAt != nil && (row.EffectiveAt == nil || !row.ExpiresAt.After(*row.EffectiveAt))) || !validSupportEvent(row) {
			return apperr.Validation("invalid supporting event, lineage, revision or availability")
		}
		if row.Kind == "navigation-training" {
			if row.Revision != 1 || training[row.ID] {
				return apperr.Validation("authored navigation controls require unique case IDs")
			}
			training[row.ID] = true
		}
		if prior, ok := lineage[id]; ok && prior != [2]string{row.Origin, row.Kind} {
			return apperr.Validation("a publication revision changes its origin or product kind")
		}
		lineage[id] = [2]string{row.Origin, row.Kind}
		seen[key] = true
		if !row.AvailableAt.After(out.Cutoff) {
			heads[id] = max(heads[id], row.Revision)
		}
	}
	for _, row := range input.Events {
		if !row.AvailableAt.After(out.Cutoff) {
			out.Events = append(out.Events, row)
			if heads[[2]string{row.SourceID, row.ID}] == row.Revision {
				origins[row.Origin] = true
			}
		}
	}
	out.UniqueOrigins = len(origins)
	slices.SortStableFunc(out.Events, func(a, b supportEvent) int {
		if order := a.AvailableAt.Compare(b.AvailableAt); order != 0 {
			return order
		}
		if order := cmp.Compare(a.SourceID, b.SourceID); order != 0 {
			return order
		}
		if order := cmp.Compare(a.ID, b.ID); order != 0 {
			return order
		}
		return cmp.Compare(a.Revision, b.Revision)
	})
	return nil
}

func validSupportEvent(row supportEvent) bool {
	if row.Kind == "navigation-training" {
		return row.Area == "Training area" && (row.Status == "degraded" || row.Status == "missing") && len(row.Languages) == 0 && row.Value == nil && row.Unit == ""
	}
	if !slices.Contains([]string{"observed", "estimated", "closed", "possible", "published", "cancelled", "missing", "source-error", "independently-verified"}, row.Status) ||
		!slices.Contains([]string{"earthquake", "thermal-sample", "disaster-bulletin", "catalog-entry", "image-opportunity", "imagery", "public-notice", "regional-outage", "field-verdict", "news-citation"}, row.Kind) {
		return false
	}
	if row.Value != nil && (row.Kind != "earthquake" || !regionalLabel(row.Unit) || *row.Value < -5 || *row.Value > 12) || row.Value == nil && row.Unit != "" {
		return false
	}
	if row.Kind == "image-opportunity" && row.Status != "possible" || row.Kind == "field-verdict" && row.Status != "independently-verified" {
		return false
	}
	if len(row.Languages) > 0 && (row.Kind != "public-notice" || len(row.Languages) > 6) {
		return false
	}
	for code, value := range row.Languages {
		if !keyPattern.MatchString(code) || !regionalLabel(value) {
			return false
		}
	}
	return true
}
