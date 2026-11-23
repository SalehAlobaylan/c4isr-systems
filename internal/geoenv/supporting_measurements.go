package geoenv

import (
	"slices"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
)

type supportField struct {
	ID            string     `json:"id"`
	SourceID      string     `json:"sourceId"`
	Kind          string     `json:"kind"`
	Lane          string     `json:"lane"`
	Variable      string     `json:"variable"`
	Area          string     `json:"area"`
	Support       string     `json:"support"`
	NativeRef     string     `json:"nativeRef"`
	Method        string     `json:"method"`
	Reference     string     `json:"reference"`
	Status        string     `json:"status"`
	Units         string     `json:"units"`
	Value         *float64   `json:"value"`
	Samples       *int       `json:"samples"`
	Coverage      *float64   `json:"coverage"`
	ValidFrom     time.Time  `json:"validFrom"`
	ValidTo       *time.Time `json:"validTo,omitempty"`
	InitializedAt *time.Time `json:"initializedAt,omitempty"`
	AvailableAt   time.Time  `json:"availableAt"`
}

type supportIndicator struct {
	ID          string    `json:"id"`
	SourceID    string    `json:"sourceId"`
	Upstream    string    `json:"upstream"`
	Area        string    `json:"area"`
	Level       string    `json:"level"`
	Origin      string    `json:"origin,omitempty"`
	Asylum      string    `json:"asylum,omitempty"`
	Kind        string    `json:"kind"`
	Definition  string    `json:"definition"`
	Method      string    `json:"method"`
	Boundary    string    `json:"boundary"`
	PeriodGrain string    `json:"periodGrain"`
	From        string    `json:"from"`
	To          string    `json:"toExclusive"`
	Units       string    `json:"units"`
	Value       *float64  `json:"value"`
	Status      string    `json:"status"`
	AvailableAt time.Time `json:"availableAt"`
}

type supportPair struct {
	Before string `json:"before"`
	After  string `json:"after"`
}

type supportChange struct {
	supportPair
	Status     string   `json:"status"`
	Difference *float64 `json:"difference"`
}

func supportAvailable(status string, value *float64) bool {
	if status == "available" {
		return value != nil && *value >= -1e12 && *value <= 1e12
	}
	return value == nil && slices.Contains([]string{"no-coverage", "quality-rejected", "source-error", "unverified"}, status)
}

func (input supportInput) reviewFields(sources map[string]bool, out *supportOutput) error {
	ids := map[string]bool{}
	type sampleKey struct{ source, variable, native, valid, run, method string }
	samples := map[sampleKey]bool{}
	for _, row := range input.Fields {
		forecast := row.Kind == "forecast-run" || row.Kind == "station-forecast" || row.Kind == "area-advisory"
		retrospective := slices.Contains([]string{"historical-estimate", "reanalysis", "dust-model", "marine-model", "along-track", "regional-column"}, row.Kind)
		if !keyPattern.MatchString(row.ID) || ids[row.ID] || !sources[row.SourceID] || !regionalLabel(row.Area) ||
			!regionalLabel(row.Variable) || !regionalLabel(row.Support) || !regionalLabel(row.Reference) || !regionalLabel(row.NativeRef) || !regionalLabel(row.Units) ||
			!slices.Contains([]string{"native", "nearest", "interpolated", "area-summary", "point-model", "reported"}, row.Method) ||
			(!forecast && !retrospective && row.Kind != "station-observation") || (row.Lane != "decision" && row.Lane != "debrief") ||
			(retrospective && row.Lane != "debrief") || (!supportAvailable(row.Status, row.Value) && !((row.Kind == "area-advisory" || row.Kind == "station-forecast") && row.Status == "reported" && row.Value == nil)) || row.ValidFrom.IsZero() || row.AvailableAt.IsZero() ||
			(row.ValidTo != nil && !row.ValidFrom.Before(*row.ValidTo)) || (!forecast && row.AvailableAt.Before(row.ValidFrom)) ||
			(forecast && (row.InitializedAt == nil || row.InitializedAt.IsZero() || row.InitializedAt.After(row.ValidFrom) || row.InitializedAt.After(row.AvailableAt))) ||
			(row.Samples != nil && (*row.Samples < 0 || *row.Samples > 1000000000)) || (row.Coverage != nil && (*row.Coverage < 0 || *row.Coverage > 1)) ||
			(row.Kind == "regional-column" && row.Status == "available" && (row.Samples == nil || *row.Samples == 0 || row.Coverage == nil || *row.Coverage == 0)) {
			return apperr.Validation("invalid supporting measurement, quality, source times or reference")
		}
		run := ""
		if row.InitializedAt != nil {
			run = row.InitializedAt.UTC().Format(time.RFC3339Nano)
		}
		key := sampleKey{row.SourceID, row.Variable, row.NativeRef, row.ValidFrom.UTC().Format(time.RFC3339Nano), run, row.Method}
		if samples[key] {
			return apperr.Validation("duplicate native supporting observation")
		}
		ids[row.ID], samples[key] = true, true
		if !row.AvailableAt.After(out.Cutoff) {
			out.Fields = append(out.Fields, row)
		}
	}
	return nil
}

func (input supportInput) reviewIndicators(sources map[string]bool, out *supportOutput) error {
	indexed := map[string]supportIndicator{}
	for _, row := range input.Indicators {
		from, errFrom := time.Parse(time.DateOnly, row.From)
		to, errTo := time.Parse(time.DateOnly, row.To)
		if !keyPattern.MatchString(row.ID) || indexed[row.ID].ID != "" || !sources[row.SourceID] ||
			!regionalLabel(row.Upstream) || !regionalLabel(row.Area) || !regionalLabel(row.Level) || !regionalLabel(row.Definition) ||
			!regionalLabel(row.Method) || !regionalLabel(row.Boundary) || !regionalLabel(row.PeriodGrain) || !regionalLabel(row.Units) ||
			!slices.Contains([]string{"stock", "flow", "rate", "assessment"}, row.Kind) || (row.Asylum != "" && !keyPattern.MatchString(row.Asylum)) ||
			(row.Origin != "" && !keyPattern.MatchString(row.Origin)) || !supportAvailable(row.Status, row.Value) || (row.Value != nil && *row.Value < 0) ||
			errFrom != nil || errTo != nil || !from.Before(to) || row.AvailableAt.Before(to) {
			return apperr.Validation("invalid supporting indicator identity, coverage or reporting grain")
		}
		indexed[row.ID] = row
		if !row.AvailableAt.After(out.Cutoff) {
			out.Indicators = append(out.Indicators, row)
		}
	}
	fields := map[string]supportField{}
	for _, row := range input.Fields {
		fields[row.ID] = row
	}
	seen := map[supportPair]bool{}
	for _, pair := range input.Comparisons {
		if pair.Before == pair.After || seen[pair] {
			return apperr.Validation("repeated or identical comparison entries")
		}
		seen[pair] = true
		a, b := indexed[pair.Before], indexed[pair.After]
		f, g := fields[pair.Before], fields[pair.After]
		if (a.ID != "" && f.ID != "") || (b.ID != "" && g.ID != "") {
			return apperr.Validation("comparison identity is ambiguous")
		}
		switch {
		case a.ID != "" && b.ID != "":
			if a.AvailableAt.After(out.Cutoff) || b.AvailableAt.After(out.Cutoff) {
				continue
			}
			same := a.Area == b.Area && a.Level == b.Level && a.Origin == b.Origin && a.Asylum == b.Asylum && a.Kind == b.Kind &&
				a.Definition == b.Definition && a.Method == b.Method && a.Boundary == b.Boundary && a.PeriodGrain == b.PeriodGrain && a.Units == b.Units && a.To <= b.From
			out.Comparisons = append(out.Comparisons, supportDifference(pair, same, a.Value, b.Value))
		case f.ID != "" && g.ID != "":
			if f.AvailableAt.After(out.Cutoff) || g.AvailableAt.After(out.Cutoff) {
				continue
			}
			same := f.Variable == g.Variable && f.Area == g.Area && f.Units == g.Units && f.Support == g.Support && f.Reference == g.Reference
			out.Comparisons = append(out.Comparisons, supportDifference(pair, same, f.Value, g.Value))
		default:
			return apperr.Validation("comparison requires two records of the same family")
		}
	}
	return nil
}

func supportDifference(pair supportPair, compatible bool, a, b *float64) supportChange {
	result := supportChange{supportPair: pair, Status: "comparable"}
	switch {
	case !compatible:
		result.Status = "scope-changed"
	case a == nil || b == nil:
		result.Status = "unavailable"
	default:
		delta := *b - *a
		result.Difference = &delta
	}
	return result
}
