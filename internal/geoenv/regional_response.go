package geoenv

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
)

const regionalExerciseSchema = "emad.regional-response-review.v1"

type regionalExercise struct {
	Schema     string              `json:"schema"`
	Title      string              `json:"title"`
	Sources    []exerciseSource    `json:"sources"`
	Indicators []regionalIndicator `json:"indicators"`
	Checks     []regionalCheck     `json:"reconciliations"`
	Service    *regionalService    `json:"service,omitempty"`
	Coast      *regionalCoast      `json:"coast,omitempty"`
}

type regionalIndicator struct {
	ID          string    `json:"id"`
	SourceID    string    `json:"sourceId"`
	RegionCode  string    `json:"regionCode"`
	Region      string    `json:"region"`
	Geography   string    `json:"geography"`
	Measure     string    `json:"measure"`
	NativeLabel string    `json:"nativeLabel"`
	Unit        string    `json:"unit"`
	Scope       string    `json:"populationScope"`
	Channel     string    `json:"channel"`
	PeriodFrom  string    `json:"periodFrom"`
	PeriodTo    string    `json:"periodToExclusive"`
	AvailableAt time.Time `json:"availableAt"`
	Value       *int64    `json:"value"`
}

type regionalCheck struct {
	TotalID      string   `json:"totalId"`
	ComponentIDs []string `json:"componentIds"`
}

type regionalReconciliation struct {
	regionalCheck
	ComponentSum *int64 `json:"componentSum"`
	Residual     *int64 `json:"unallocatedResidual"`
}

type regionalResponse struct {
	Title      string                   `json:"title"`
	Cutoff     time.Time                `json:"cutoff"`
	Sources    []exerciseSource         `json:"sources"`
	Indicators []regionalIndicator      `json:"indicators"`
	Checks     []regionalReconciliation `json:"reconciliations"`
	Service    *regionalServiceReview   `json:"service"`
	Coast      *regionalCoastReview     `json:"coast"`
}

func (s Service) reviewRegionalResponse(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	cutoff, err := time.Parse(time.RFC3339Nano, query.Get("knownAt"))
	if err != nil {
		httpx.Error(w, apperr.Validation("provide a regional review cutoff with a UTC offset"))
		return
	}
	var slots *int
	if value := query.Get("staffedSlots"); value != "" {
		count, err := strconv.Atoi(value)
		if err != nil || count < 0 || count > regionalMaxSlots {
			httpx.Error(w, apperr.Validation("provide a staffing assumption between 0 and 128 slots"))
			return
		}
		slots = &count
	}
	s.reviewSimulationSource(w, r, "regional-response-review", "v1", query.Get("inputSha256"),
		func(raw []byte) (string, any, error) {
			out, err := replayRegionalResponse(raw, cutoff, query.Get("manifestSha256"), slots)
			return "review", out, err
		})
}

// ReadRegionalResponse reproduces the Atlas comparison from a trusted local input.
func ReadRegionalResponse(raw []byte, cutoff time.Time, digest, manifest string, slots *int) (any, error) {
	if len(raw) == 0 || len(raw) > simulationReviewMaxBytes || !digestPattern.MatchString(digest) || artifacts.Hash(raw) != digest {
		return nil, apperr.Validation("regional package differs from its trusted digest or exceeds the input budget")
	}
	result, err := replayRegionalResponse(raw, cutoff, manifest, slots)
	if err != nil {
		return nil, err
	}
	return map[string]any{"schema": regionalExerciseSchema, "inputSha256": digest, "review": result}, nil
}

func replayRegionalResponse(raw []byte, cutoff time.Time, manifest string, slots *int) (regionalResponse, error) {
	var exercise regionalExercise
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	out := regionalResponse{Cutoff: cutoff.UTC(), Indicators: []regionalIndicator{}, Checks: []regionalReconciliation{}}
	if err := decoder.Decode(&exercise); err != nil || decoder.Decode(new(any)) != io.EOF ||
		exercise.Schema != regionalExerciseSchema || !regionalLabel(exercise.Title) || cutoff.IsZero() ||
		len(exercise.Sources) == 0 || len(exercise.Sources) > 32 || len(exercise.Indicators) > 250 || len(exercise.Checks) > 50 ||
		(len(exercise.Indicators) == 0 && exercise.Service == nil && exercise.Coast == nil) ||
		(slots != nil && (exercise.Service == nil || *slots < 0 || *slots > regionalMaxSlots)) {
		return out, apperr.Validation("expected one bounded regional response review package")
	}
	sources := map[string]bool{}
	for _, entry := range exercise.Sources {
		if !keyPattern.MatchString(entry.ID) || sources[entry.ID] || !digestPattern.MatchString(entry.SHA256) ||
			!regionalLabel(entry.Edition) || !regionalLabel(entry.Attribution) {
			return out, apperr.Validation("invalid regional source identity, edition, digest or attribution")
		}
		sources[entry.ID] = true
	}
	out.Title, out.Sources = exercise.Title, exercise.Sources
	if err := exercise.reviewIndicators(sources, &out); err != nil {
		return out, err
	}
	if exercise.Service != nil {
		service, err := exercise.Service.review(cutoff, manifest, slots, sources)
		if err != nil {
			return out, err
		}
		out.Service = service
	}
	if exercise.Coast != nil {
		coast, err := exercise.Coast.review(cutoff, manifest, sources)
		if err != nil {
			return out, err
		}
		out.Coast = coast
	}
	return out, nil
}

func regionalLabel(v string) bool { return strings.TrimSpace(v) != "" && len(v) <= 640 }

func (e regionalExercise) reviewIndicators(sources map[string]bool, out *regionalResponse) error {
	indexed := make(map[string]regionalIndicator, len(e.Indicators))
	for _, row := range e.Indicators {
		from, errFrom := time.Parse(time.DateOnly, row.PeriodFrom)
		to, errTo := time.Parse(time.DateOnly, row.PeriodTo)
		if !keyPattern.MatchString(row.ID) || indexed[row.ID].ID != "" || !sources[row.SourceID] ||
			!keyPattern.MatchString(row.RegionCode) || !regionalLabel(row.Region) || !regionalLabel(row.Geography) ||
			!regionalLabel(row.Measure) || !regionalLabel(row.NativeLabel) || !regionalLabel(row.Scope) ||
			!regionalLabel(row.Unit) || !regionalLabel(row.Channel) || errFrom != nil || errTo != nil || !from.Before(to) ||
			row.AvailableAt.Before(to) || (row.Value != nil && (*row.Value < 0 || *row.Value > 1_000_000_000_000)) {
			return apperr.Validation("regional indicator has invalid identity, scope, count or period")
		}
		indexed[row.ID] = row
		if !row.AvailableAt.After(out.Cutoff) {
			out.Indicators = append(out.Indicators, row)
		}
	}
	seen := map[string]bool{}
	for _, check := range e.Checks {
		total := indexed[check.TotalID]
		if total.ID == "" || seen[total.ID] || len(check.ComponentIDs) == 0 || len(check.ComponentIDs) > 50 {
			return apperr.Validation("invalid regional reconciliation")
		}
		seen[total.ID] = true
		complete, visible := true, !total.AvailableAt.After(out.Cutoff)
		count := int64(0)
		components := map[string]bool{total.ID: true}
		for _, id := range check.ComponentIDs {
			part := indexed[id]
			if part.ID == "" || components[id] || part.SourceID != total.SourceID || part.RegionCode != total.RegionCode || part.Region != total.Region ||
				part.Geography != total.Geography || part.Scope != total.Scope || part.Unit != total.Unit || part.Channel != total.Channel ||
				part.PeriodFrom != total.PeriodFrom || part.PeriodTo != total.PeriodTo {
				return apperr.Validation("reconciliation crosses reporting grains or repeats a component")
			}
			components[id] = true
			visible = visible && !part.AvailableAt.After(out.Cutoff)
			if part.Value == nil {
				complete = false
			} else {
				count += *part.Value
			}
		}
		if visible {
			result := regionalReconciliation{regionalCheck: check}
			if complete {
				result.ComponentSum = &count
				if total.Value != nil {
					residual := *total.Value - count
					result.Residual = &residual
				}
			}
			out.Checks = append(out.Checks, result)
		}
	}
	return nil
}
