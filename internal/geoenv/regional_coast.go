package geoenv

import (
	"cmp"
	"slices"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
)

type regionalCoast struct {
	Port                   string                   `json:"port"`
	PortWatchID            string                   `json:"portWatchId"`
	Inventory              *regionalPortInventory   `json:"inventory,omitempty"`
	RetainedManifestSHA256 string                   `json:"retainedManifestSha256,omitempty"`
	Depth                  *regionalDepth           `json:"depth,omitempty"`
	Coverage               []regionalReception      `json:"coverage"`
	Notices                []regionalMaritimeNotice `json:"notices"`
}

type regionalPortInventory struct {
	SourceID   string `json:"sourceId"`
	Identifier string `json:"identifier"`
	Locality   string `json:"locality"`
	Match      string `json:"match"`
}

type regionalDepth struct {
	GridSourceID      string                `json:"gridSourceId"`
	ShorelineSourceID string                `json:"shorelineSourceId"`
	Horizontal        string                `json:"horizontalReference"`
	Vertical          string                `json:"verticalReference"`
	Samples           []regionalDepthSample `json:"samples"`
}

type regionalDepthSample struct {
	Cell       string   `json:"cell"`
	State      string   `json:"state"`
	ElevationM *float64 `json:"elevationM"`
	SourceType string   `json:"sourceType"`
}

type regionalReception struct {
	SourceID        string     `json:"sourceId"`
	Day             string     `json:"day"`
	AvailableAt     time.Time  `json:"availableAt"`
	WindowMinutes   int        `json:"windowMinutes"`
	UsableMinutes   int        `json:"usableMinutes"`
	UniqueReports   *int       `json:"uniqueReports"`
	Duplicates      *int       `json:"duplicates"`
	PortCalls       *int       `json:"portWatchCalls"`
	PortSourceID    string     `json:"portWatchSourceId,omitempty"`
	PortAvailableAt *time.Time `json:"portWatchAvailableAt,omitempty"`
}

type regionalMaritimeNotice struct {
	ID              string     `json:"id"`
	Publisher       string     `json:"publisher"`
	SourceID        string     `json:"sourceId"`
	Area            string     `json:"warningArea"`
	Revision        int        `json:"revision"`
	Kind            string     `json:"kind"`
	Status          string     `json:"status"`
	IssuedAt        time.Time  `json:"issuedAt"`
	AvailableAt     time.Time  `json:"availableAt"`
	EffectiveAt     time.Time  `json:"effectiveAt"`
	ExpiresAt       *time.Time `json:"expiresAt"`
	Summary         string     `json:"summary"`
	ServiceImpact   string     `json:"serviceImpact"`
	ImpactSourceID  string     `json:"impactSourceId,omitempty"`
	IssuerReference string     `json:"issuerReference"`
}

type regionalCoastReview struct {
	Port        string                   `json:"port"`
	PortWatchID string                   `json:"portWatchId"`
	Inventory   *regionalPortInventory   `json:"inventory"`
	Depth       *regionalDepth           `json:"depth"`
	Coverage    []regionalReception      `json:"coverage"`
	Notices     []regionalMaritimeNotice `json:"notices"`
}

func (coast regionalCoast) review(cutoff time.Time, manifest string, sources map[string]bool) (*regionalCoastReview, error) {
	needsPort := coast.Inventory != nil || coast.Depth != nil || len(coast.Coverage) > 0
	if !regionalLabel(coast.Port) || ((coast.PortWatchID != "" || needsPort) && coast.PortWatchID != "port24" && coast.PortWatchID != "port2074") ||
		len(coast.Coverage) > 180 || len(coast.Notices) > 160 {
		return nil, apperr.Validation("invalid regional civil coastal context")
	}
	if coast.RetainedManifestSHA256 != "" && (!digestPattern.MatchString(coast.RetainedManifestSHA256) || coast.RetainedManifestSHA256 != manifest) {
		return nil, apperr.Conflict("the coastal comparison needs its pinned environment manifest")
	}
	if p := coast.Inventory; p != nil && (!sources[p.SourceID] || !regionalLabel(p.Identifier) || !regionalLabel(p.Locality) ||
		!slices.Contains([]string{"matched-locality", "ambiguous", "unmatched"}, p.Match)) {
		return nil, apperr.Validation("invalid reviewed port inventory crosswalk")
	}
	if d := coast.Depth; d != nil {
		if !sources[d.GridSourceID] || !sources[d.ShorelineSourceID] || !regionalLabel(d.Horizontal) || !regionalLabel(d.Vertical) ||
			len(d.Samples) == 0 || len(d.Samples) > 32 {
			return nil, apperr.Validation("invalid coastal grid references or sample budget")
		}
		seen := map[string]bool{}
		for _, sample := range d.Samples {
			if !keyPattern.MatchString(sample.Cell) || seen[sample.Cell] || !regionalLabel(sample.SourceType) ||
				!slices.Contains([]string{"land", "offshore", "nodata"}, sample.State) ||
				(sample.State == "nodata") != (sample.ElevationM == nil) ||
				(sample.ElevationM != nil && (*sample.ElevationM < -12000 || *sample.ElevationM > 9000 ||
					(sample.State == "offshore" && *sample.ElevationM >= 0))) {
				return nil, apperr.Validation("coastal sample has invalid cell identity, elevation or coverage")
			}
			seen[sample.Cell] = true
		}
	}
	out := &regionalCoastReview{Port: coast.Port, PortWatchID: coast.PortWatchID, Inventory: coast.Inventory,
		Depth: coast.Depth, Coverage: []regionalReception{}, Notices: []regionalMaritimeNotice{}}
	days := map[[2]string]bool{}
	for _, row := range coast.Coverage {
		at, err := time.Parse(time.DateOnly, row.Day)
		key := [2]string{row.Day, row.SourceID}
		if err != nil || row.AvailableAt.Before(at.AddDate(0, 0, 1)) || days[key] || !sources[row.SourceID] ||
			row.WindowMinutes < 1 || row.WindowMinutes > 1440 || row.UsableMinutes < 0 || row.UsableMinutes > row.WindowMinutes ||
			(row.UsableMinutes == 0 && (row.UniqueReports != nil || row.Duplicates != nil)) ||
			(row.UniqueReports != nil && *row.UniqueReports < 0) || (row.Duplicates != nil && *row.Duplicates < 0) ||
			(row.PortCalls != nil && (*row.PortCalls < 0 || !sources[row.PortSourceID] || row.PortSourceID == row.SourceID)) ||
			(row.PortAvailableAt != nil && (row.PortCalls == nil || row.PortAvailableAt.Before(at.AddDate(0, 0, 1)))) {
			return nil, apperr.Validation("invalid completed-day coastal reception coverage")
		}
		days[key] = true
		if row.AvailableAt.After(cutoff) {
			continue
		}
		if row.PortAvailableAt != nil && row.PortAvailableAt.After(cutoff) {
			row.PortCalls, row.PortSourceID, row.PortAvailableAt = nil, "", nil
		}
		out.Coverage = append(out.Coverage, row)
	}
	slices.SortFunc(out.Coverage, func(a, b regionalReception) int {
		if order := cmp.Compare(a.Day, b.Day); order != 0 {
			return order
		}
		return cmp.Compare(a.SourceID, b.SourceID)
	})
	var err error
	out.Notices, err = reviewedMaritimeNotices(coast.Notices, cutoff, sources)
	return out, err
}

func reviewedMaritimeNotices(rows []regionalMaritimeNotice, cutoff time.Time, sources map[string]bool) ([]regionalMaritimeNotice, error) {
	type revision struct {
		publisher, id string
		number        int
	}
	visible := map[[2]string]regionalMaritimeNotice{}
	seen := map[revision]bool{}
	for _, row := range rows {
		key := revision{row.Publisher, row.ID, row.Revision}
		if !keyPattern.MatchString(row.ID) || !regionalLabel(row.Publisher) || !sources[row.SourceID] || seen[key] || row.Revision < 1 ||
			!regionalLabel(row.Area) || !regionalLabel(row.IssuerReference) || !regionalLabel(row.Summary) ||
			!slices.Contains([]string{"civil-service-notice", "historical-debrief"}, row.Kind) ||
			!slices.Contains([]string{"active", "cancelled"}, row.Status) ||
			!slices.Contains([]string{"unknown", "reported", "independently-verified"}, row.ServiceImpact) || row.IssuedAt.IsZero() ||
			(row.ServiceImpact == "independently-verified" && (!sources[row.ImpactSourceID] || row.ImpactSourceID == row.SourceID)) ||
			row.AvailableAt.Before(row.IssuedAt) || row.EffectiveAt.IsZero() || (row.ExpiresAt != nil && !row.ExpiresAt.After(row.EffectiveAt)) {
			return nil, apperr.Validation("invalid coastal issuer notice, revision or service-impact evidence")
		}
		seen[key] = true
		if row.AvailableAt.After(cutoff) || row.EffectiveAt.After(cutoff) {
			continue
		}
		identity := [2]string{row.Publisher, row.ID}
		if row.Revision > visible[identity].Revision {
			visible[identity] = row
		}
	}
	out := []regionalMaritimeNotice{}
	for _, row := range visible {
		if row.Status != "cancelled" && row.ExpiresAt != nil && !row.ExpiresAt.After(cutoff) {
			row.Status = "expired"
		}
		out = append(out, row)
	}
	slices.SortFunc(out, func(a, b regionalMaritimeNotice) int {
		if order := a.AvailableAt.Compare(b.AvailableAt); order != 0 {
			return order
		}
		if order := cmp.Compare(a.Publisher, b.Publisher); order != 0 {
			return order
		}
		return cmp.Compare(a.ID, b.ID)
	})
	return out, nil
}
