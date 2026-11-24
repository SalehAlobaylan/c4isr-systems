package coverage

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/areas"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/jackc/pgx/v5"
)

type InventorySource struct {
	SourceID      string               `json:"sourceId"`
	Name          string               `json:"name"`
	Family        string               `json:"family"`
	Support       string               `json:"support"`
	Gate          string               `json:"gate"`
	RetainedCount int                  `json:"retainedCount"`
	EarliestAt    *time.Time           `json:"earliestAt,omitempty"`
	LatestAt      *time.Time           `json:"latestAt,omitempty"`
	Bounds        *publicingest.Bounds `json:"bounds,omitempty"`
	DailyBudget   *int                 `json:"dailyBudget,omitempty"`
	AttemptsToday *int                 `json:"attemptsToday,omitempty"`
	NextAction    string               `json:"nextAction,omitempty"`
}

type InventoryTimeBucket struct {
	Month             string               `json:"month"`
	Label             string               `json:"label"`
	From              time.Time            `json:"from"`
	To                time.Time            `json:"to"`
	Total             int                  `json:"total"`
	ByFamily          map[string]int       `json:"byFamily"`
	Bounds            *publicingest.Bounds `json:"bounds,omitempty"`
	SuggestedAreaName string               `json:"suggestedAreaName,omitempty"`
}

type InventoryFootprint struct {
	Key           string              `json:"key"`
	Name          string              `json:"name"`
	Bounds        publicingest.Bounds `json:"bounds"`
	SourceIDs     []string            `json:"sourceIds"`
	SourceNames   []string            `json:"sourceNames"`
	Families      []string            `json:"families"`
	RetainedCount int                 `json:"retainedCount"`
	EarliestAt    *time.Time          `json:"earliestAt,omitempty"`
	LatestAt      *time.Time          `json:"latestAt,omitempty"`
}

type InventoryResult struct {
	Schema        string                `json:"schema"`
	EvaluatedAt   time.Time             `json:"evaluatedAt"`
	Meaning       string                `json:"meaning"`
	TotalRetained int                   `json:"totalRetained"`
	Sources       []InventorySource     `json:"sources"`
	TimeBuckets   []InventoryTimeBucket `json:"timeBuckets"`
	Footprints    []InventoryFootprint  `json:"footprints"`
}

type sourceStats struct {
	count    int
	earliest *time.Time
	latest   *time.Time
}

func (st *sourceStats) merge(count int, earliest, latest *time.Time) {
	st.count += count
	if earliest != nil && !earliest.IsZero() && (st.earliest == nil || earliest.Before(*st.earliest)) {
		v := earliest.UTC()
		st.earliest = &v
	}
	if latest != nil && !latest.IsZero() && (st.latest == nil || latest.After(*st.latest)) {
		v := latest.UTC()
		st.latest = &v
	}
}

func unionBounds(a, b *publicingest.Bounds) *publicingest.Bounds {
	if a == nil {
		if b == nil {
			return nil
		}
		cp := *b
		return &cp
	}
	if b == nil {
		cp := *a
		return &cp
	}
	return &publicingest.Bounds{
		West:  math.Min(a.West, b.West),
		South: math.Min(a.South, b.South),
		East:  math.Max(a.East, b.East),
		North: math.Max(a.North, b.North),
	}
}

func footprintKey(b publicingest.Bounds) string {
	return fmt.Sprintf("%.2f,%.2f,%.2f,%.2f", b.West, b.South, b.East, b.North)
}

func deriveFootprintName(b publicingest.Bounds, sourceNames []string) string {
	// Extract common human-readable location prefix from source names if present
	// (e.g., "Jeddah surface elevation · ..." or "USGS earthquakes · Saudi region").
	counts := map[string]int{}
	candidates := []string{}
	for _, raw := range sourceNames {
		parts := strings.Split(raw, "·")
		for _, part := range parts {
			clean := strings.TrimSpace(part)
			lower := strings.ToLower(clean)
			switch {
			case strings.Contains(lower, "jeddah"):
				counts["Jeddah corridor"]++
			case strings.Contains(lower, "riyadh"):
				counts["Riyadh region"]++
			case strings.Contains(lower, "saudi"):
				counts["Saudi region"]++
			default:
				if clean != "" && len(clean) <= 32 {
					candidates = append(candidates, clean)
				}
			}
		}
	}
	bestLabel, bestCount := "", 0
	for label, n := range counts {
		if n > bestCount || (n == bestCount && label < bestLabel) {
			bestLabel, bestCount = label, n
		}
	}
	if bestLabel != "" {
		return bestLabel
	}
	spanLng := b.East - b.West
	spanLat := b.North - b.South
	if spanLng >= 180 || spanLat >= 120 {
		return "Broad configured footprint"
	}
	if len(candidates) > 0 && len(sourceNames) == 1 {
		return candidates[0]
	}
	return fmt.Sprintf("%.2f°E–%.2f°E, %.2f°N–%.2f°N", b.West, b.East, b.South, b.North)
}

// Inventory returns a read-only, RLS-scoped summary of what retained data and
// approved regional footprints already exist in this workspace across time and
// space. It performs zero upstream provider requests and never discloses
// unauthorized sources or shared account identifiers.
func (s Service) Inventory(ctx context.Context) (InventoryResult, error) {
	var out InventoryResult
	if e := areas.Require(ctx); e != nil {
		return out, e
	}
	now := time.Now().UTC()
	out = InventoryResult{
		Schema:      "emad.coverage-inventory.v1",
		EvaluatedAt: now,
		Meaning:     "Authorized retained workspace inventory and approved regional footprints. Retained counts never establish global completeness or factual absence.",
		Sources:     []InventorySource{},
		TimeBuckets: []InventoryTimeBucket{},
		Footprints:  []InventoryFootprint{},
	}

	e := db.RepeatableRead(ctx, s.Pool, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT s.id,s.name,s.status,COALESCE(s.metadata,'{}'::jsonb),COALESCE(jsonb_agg(jsonb_build_object('key',p.profile_key,'revision',p.head_revision_id) ORDER BY p.profile_key)FILTER(WHERE p.profile_key IS NOT NULL),'[]'::jsonb) FROM sources s LEFT JOIN retrieval_profiles p ON p.source_id=s.id AND p.scope_id=s.scope_id GROUP BY s.id,s.name,s.status,s.metadata ORDER BY s.id LIMIT 256`)
		if err != nil {
			return err
		}
		type sourceRow struct {
			id, name, status string
			metadata         map[string]any
			profiles         []struct {
				Key      string `json:"key"`
				Revision string `json:"revision"`
			}
		}
		entries := []sourceRow{}
		for rows.Next() {
			var r sourceRow
			var metaRaw, profRaw []byte
			if err = rows.Scan(&r.id, &r.name, &r.status, &metaRaw, &profRaw); err != nil {
				rows.Close()
				return err
			}
			_ = json.Unmarshal(metaRaw, &r.metadata)
			if err = json.Unmarshal(profRaw, &r.profiles); err != nil {
				rows.Close()
				return err
			}
			entries = append(entries, r)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}

		// Per-source daily attempt counts (strictly per-source, never exposing account groups).
		attemptsBySource := map[string]int{}
		today := now.Format("2006-01-02")
		stRows, err := tx.Query(ctx, `SELECT source_id,requests_today FROM source_connector_state WHERE requests_day=$1::date`, today)
		if err != nil {
			return err
		}
		for stRows.Next() {
			var sid string
			var reqs int
			if err = stRows.Scan(&sid, &reqs); err != nil {
				stRows.Close()
				return err
			}
			attemptsBySource[sid] = reqs
		}
		err = stRows.Err()
		stRows.Close()
		if err != nil {
			return err
		}

		// Aggregate retained counts and monthly buckets across authorized tables.
		statsBySource := map[string]*sourceStats{}
		getStats := func(sid string) *sourceStats {
			if st, ok := statsBySource[sid]; ok {
				return st
			}
			st := &sourceStats{}
			statsBySource[sid] = st
			return st
		}

		type bucketAgg struct {
			month    time.Time
			total    int
			byFamily map[string]int
			sources  map[string]bool
		}
		buckets := map[string]*bucketAgg{}
		addBucket := func(month time.Time, family, sourceID string, n int) {
			if n <= 0 || month.IsZero() {
				return
			}
			m := time.Date(month.UTC().Year(), month.UTC().Month(), 1, 0, 0, 0, 0, time.UTC)
			key := m.Format("2006-01")
			b, ok := buckets[key]
			if !ok {
				b = &bucketAgg{month: m, byFamily: map[string]int{}, sources: map[string]bool{}}
				buckets[key] = b
			}
			b.total += n
			b.byFamily[family] += n
			b.sources[sourceID] = true
		}

		// 1. Public events (earthquakes, thermal, events) — requires observations.read
		if access.Allows(ctx, "observations.read") {
			evRows, err := tx.Query(ctx, `
				WITH latest AS (
					SELECT DISTINCT ON (r.source_id, r.provider_record_id)
						r.source_id,
						COALESCE(NULLIF(NULLIF(r.document#>>'{reported,observedAt}',''),'0001-01-01T00:00:00Z')::timestamptz, r.revision_at) AS stamp,
						r.document#>>'{reported,type}' AS kind,
						COALESCE((r.document#>>'{reported,withdrawn}')::boolean, false) AS withdrawn
					FROM public_event_revisions r
					JOIN public_evidence_catalog c ON c.evidence_id=r.evidence_id AND c.scope_id=r.scope_id
					ORDER BY r.source_id, r.provider_record_id, r.revision_at DESC, r.evidence_id DESC
				)
				SELECT source_id,
					CASE WHEN kind='event.earthquake' THEN 'earthquakes' WHEN kind='thermal.detection' THEN 'thermal' ELSE 'events' END AS fam,
					date_trunc('month', stamp AT TIME ZONE 'UTC') AS m,
					count(*)::int, min(stamp), max(stamp)
				FROM latest
				WHERE NOT withdrawn AND stamp IS NOT NULL
				GROUP BY 1,2,3`)
			if err != nil {
				return err
			}
			for evRows.Next() {
				var sid, fam string
				var month, earliest, latest time.Time
				var n int
				if err = evRows.Scan(&sid, &fam, &month, &n, &earliest, &latest); err != nil {
					evRows.Close()
					return err
				}
				getStats(sid).merge(n, &earliest, &latest)
				addBucket(month, fam, sid, n)
			}
			err = evRows.Err()
			evRows.Close()
			if err != nil {
				return err
			}
		}

		// 2. Public context records (weather, bulletins) — requires sources.read
		ctxRows, err := tx.Query(ctx, `
			WITH latest AS (
				SELECT DISTINCT ON (source_id, record_key)
					source_id, kind, valid_from
				FROM public_context_records
				ORDER BY source_id, record_key, revision_at DESC, known_at DESC, evidence_id DESC
			)
			SELECT source_id,
				CASE WHEN kind='REPORT' THEN 'bulletins' ELSE 'weather' END AS fam,
				date_trunc('month', valid_from AT TIME ZONE 'UTC') AS m,
				count(*)::int, min(valid_from), max(valid_from)
			FROM latest
			GROUP BY 1,2,3`)
		if err != nil {
			return err
		}
		for ctxRows.Next() {
			var sid, fam string
			var month, earliest, latest time.Time
			var n int
			if err = ctxRows.Scan(&sid, &fam, &month, &n, &earliest, &latest); err != nil {
				ctxRows.Close()
				return err
			}
			getStats(sid).merge(n, &earliest, &latest)
			addBucket(month, fam, sid, n)
		}
		err = ctxRows.Err()
		ctxRows.Close()
		if err != nil {
			return err
		}

		// 3. Imagery scenes & captures — requires sources.read
		imgRows, err := tx.Query(ctx, `
			SELECT source_id,
				date_trunc('month', acquired_at AT TIME ZONE 'UTC') AS m,
				count(*)::int, min(acquired_at), max(acquired_at)
			FROM imagery_scenes
			GROUP BY 1,2`)
		if err != nil {
			return err
		}
		for imgRows.Next() {
			var sid string
			var month, earliest, latest time.Time
			var n int
			if err = imgRows.Scan(&sid, &month, &n, &earliest, &latest); err != nil {
				imgRows.Close()
				return err
			}
			getStats(sid).merge(n, &earliest, &latest)
			addBucket(month, "imagery", sid, n)
		}
		err = imgRows.Err()
		imgRows.Close()
		if err != nil {
			return err
		}

		// 4. External moving tracks — requires tracks.read
		if access.Allows(ctx, "tracks.read") {
			trkRows, err := tx.Query(ctx, `
				SELECT source_id,
					CASE WHEN kind='vessel' THEN 'vessels' ELSE 'aircraft' END AS fam,
					date_trunc('month', last_seen_at AT TIME ZONE 'UTC') AS m,
					count(*)::int, min(first_seen_at), max(last_seen_at)
				FROM external_track_segments
				GROUP BY 1,2,3`)
			if err != nil {
				return err
			}
			for trkRows.Next() {
				var sid, fam string
				var month, earliest, latest time.Time
				var n int
				if err = trkRows.Scan(&sid, &fam, &month, &n, &earliest, &latest); err != nil {
					trkRows.Close()
					return err
				}
				getStats(sid).merge(n, &earliest, &latest)
				addBucket(month, fam, sid, n)
			}
			err = trkRows.Err()
			trkRows.Close()
			if err != nil {
				return err
			}
		}

		// 5. Imagery analysis findings — requires assessments.read
		if access.Allows(ctx, "assessments.read") {
			fndRows, err := tx.Query(ctx, `
				SELECT source_id,
					date_trunc('month', (input#>>'{scenes,1,acquiredAt}')::timestamptz AT TIME ZONE 'UTC') AS m,
					count(*)::int,
					min((input#>>'{scenes,1,acquiredAt}')::timestamptz),
					max((input#>>'{scenes,1,acquiredAt}')::timestamptz)
				FROM imagery_jobs
				WHERE emad_content_visible('job', id)
					AND status='SUCCEEDED'
					AND input->>'kind'='NDVI_CHANGE'
					AND input#>>'{scenes,1,acquiredAt}' IS NOT NULL
				GROUP BY 1,2`)
			if err != nil {
				return err
			}
			for fndRows.Next() {
				var sid string
				var month, earliest, latest time.Time
				var n int
				if err = fndRows.Scan(&sid, &month, &n, &earliest, &latest); err != nil {
					fndRows.Close()
					return err
				}
				getStats(sid).merge(n, &earliest, &latest)
				addBucket(month, "findings", sid, n)
			}
			err = fndRows.Err()
			fndRows.Close()
			if err != nil {
				return err
			}
		}

		// 6. Reference datasets & geographic regions per source
		refBoundsBySource := map[string]*publicingest.Bounds{}
		refTerrainBySource := map[string]bool{}
		refRows, err := tx.Query(ctx, `
			SELECT source_id, dataset_key, kind, document
			FROM reference_datasets
			WHERE emad_content_visible('reference', id)
				AND COALESCE((document#>>'{rights,expiresAt}')::timestamptz>clock_timestamp(), true)
			LIMIT 512`)
		if err != nil {
			return err
		}
		for refRows.Next() {
			var sid, key, kind string
			var raw []byte
			if err = refRows.Scan(&sid, &key, &kind, &raw); err != nil {
				refRows.Close()
				return err
			}
			var doc struct {
				AOI          *publicingest.Bounds `json:"aoi"`
				RetrievedAt  *time.Time           `json:"retrievedAt"`
				FeatureCount int                  `json:"featureCount"`
			}
			_ = json.Unmarshal(raw, &doc)
			lowerKey := strings.ToLower(key)
			lowerKind := strings.ToLower(kind)
			if lowerKind == "terrain" || lowerKind == "elevation" || lowerKind == "grid" || strings.Contains(lowerKey, "terrain") || strings.Contains(lowerKey, "dem") {
				refTerrainBySource[sid] = true
			}
			if doc.AOI != nil && doc.AOI.Validate() == nil {
				refBoundsBySource[sid] = unionBounds(refBoundsBySource[sid], doc.AOI)
			}
			cnt := 1
			if lowerKind == "vector" && doc.FeatureCount > 0 {
				cnt = doc.FeatureCount
			}
			getStats(sid).merge(cnt, doc.RetrievedAt, doc.RetrievedAt)
		}
		err = refRows.Err()
		refRows.Close()
		if err != nil {
			return err
		}

		regRows, err := tx.Query(ctx, `SELECT source_id, count(*)::int FROM geographic_regions GROUP BY source_id`)
		if err != nil {
			return err
		}
		for regRows.Next() {
			var sid string
			var cnt int
			if err = regRows.Scan(&sid, &cnt); err != nil {
				regRows.Close()
				return err
			}
			getStats(sid).merge(cnt, nil, nil)
		}
		err = regRows.Err()
		regRows.Close()
		if err != nil {
			return err
		}

		// Assemble InventorySource list & footprint groups.
		boundsBySource := map[string]*publicingest.Bounds{}
		nameBySource := map[string]string{}
		fpMap := map[string]*InventoryFootprint{}
		fpFamilies := map[string]map[string]bool{}

		for _, entry := range entries {
			nameBySource[entry.id] = entry.name
			st := statsBySource[entry.id]
			item := InventorySource{
				SourceID: entry.id,
				Name:     entry.name,
				Family:   "unknown",
				Support:  "not_admitted",
				Gate:     "not_admitted",
			}
			if st != nil {
				item.RetainedCount = st.count
				item.EarliestAt = st.earliest
				item.LatestAt = st.latest
				out.TotalRetained += st.count
			}

			category, _ := entry.metadata["category"].(string)
			if len(entry.profiles) == 0 {
				if category == "GEOGRAPHIC_REFERENCE" {
					item.Support = "retained_reference"
					item.Gate = "retained"
					if refTerrainBySource[entry.id] || strings.Contains(strings.ToLower(entry.id), "dem") {
						item.Family = "terrain"
					} else {
						item.Family = "reference"
					}
					item.Bounds = refBoundsBySource[entry.id]
					if item.Bounds == nil {
						if cov := extractCoverageBounds(entry.metadata); len(cov) > 0 {
							b := cov[0]
							item.Bounds = &b
						}
					}
				} else {
					capInfo, gateState, gateAction, hasBinding := s.Ingestion.SourceBinding(ctx, entry.id)
					if hasBinding && capInfo.Family != "" {
						item.Family = capInfo.Family
					}
					activation, _ := entry.metadata["activation"].(map[string]any)
					if gateState == "" {
						gateState, _ = activation["state"].(string)
					}
					if gateAction == "" {
						gateAction, _ = activation["nextAction"].(string)
					}
					if gateState != "" {
						item.Gate = gateState
					}
					item.NextAction = gateAction
				}
			} else {
				item.Support = "admitted"
				item.Gate = "not_configured"
				maxDaily := 0
				for _, p := range entry.profiles {
					policy, pErr := s.Ingestion.AcquisitionPolicy(ctx, entry.id, p.Key)
					if pErr != nil {
						return pErr
					}
					if policy.Gate != "not_configured" {
						item.Gate = policy.Gate
						if policy.Capability.Family != "" {
							item.Family = policy.Capability.Family
						}
					}
					if policy.Admission.Policy.AOI.Validate() == nil {
						b := policy.Admission.Policy.AOI
						item.Bounds = unionBounds(item.Bounds, &b)
					}
					if policy.Admission.Policy.MaxRequestsPerDay > maxDaily {
						maxDaily = policy.Admission.Policy.MaxRequestsPerDay
					}
				}
				if item.Gate == "not_configured" && (item.Family == "" || item.Family == "unknown") {
					item.Support = "not_configured"
				}
				if maxDaily > 0 {
					budget := maxDaily
					attempts := attemptsBySource[entry.id]
					item.DailyBudget = &budget
					item.AttemptsToday = &attempts
				}
			}

			if item.Bounds != nil {
				boundsBySource[entry.id] = item.Bounds
				if item.Support == "admitted" || item.Support == "retained_reference" {
					key := footprintKey(*item.Bounds)
					fp, ok := fpMap[key]
					if !ok {
						fp = &InventoryFootprint{
							Key:         key,
							Bounds:      *item.Bounds,
							SourceIDs:   []string{},
							SourceNames: []string{},
							Families:    []string{},
						}
						fpMap[key] = fp
						fpFamilies[key] = map[string]bool{}
					}
					fp.SourceIDs = append(fp.SourceIDs, item.SourceID)
					fp.SourceNames = append(fp.SourceNames, item.Name)
					fp.RetainedCount += item.RetainedCount
					if item.Family != "" && item.Family != "unknown" && !fpFamilies[key][item.Family] {
						fpFamilies[key][item.Family] = true
						fp.Families = append(fp.Families, item.Family)
					}
					if item.EarliestAt != nil && (fp.EarliestAt == nil || item.EarliestAt.Before(*fp.EarliestAt)) {
						v := *item.EarliestAt
						fp.EarliestAt = &v
					}
					if item.LatestAt != nil && (fp.LatestAt == nil || item.LatestAt.After(*fp.LatestAt)) {
						v := *item.LatestAt
						fp.LatestAt = &v
					}
				}
			}

			out.Sources = append(out.Sources, item)
		}

		// Finalize footprints sorted by retainedCount desc then area desc.
		for _, fp := range fpMap {
			sort.Strings(fp.Families)
			fp.Name = deriveFootprintName(fp.Bounds, fp.SourceNames)
			out.Footprints = append(out.Footprints, *fp)
		}
		sort.Slice(out.Footprints, func(i, j int) bool {
			if out.Footprints[i].RetainedCount != out.Footprints[j].RetainedCount {
				return out.Footprints[i].RetainedCount > out.Footprints[j].RetainedCount
			}
			return out.Footprints[i].Name < out.Footprints[j].Name
		})

		// Finalize monthly time buckets sorted newest first (max 24 months).
		keys := make([]string, 0, len(buckets))
		for k := range buckets {
			keys = append(keys, k)
		}
		sort.Sort(sort.Reverse(sort.StringSlice(keys)))
		if len(keys) > 24 {
			keys = keys[:24]
		}
		for _, k := range keys {
			b := buckets[k]
			from := b.month
			to := from.AddDate(0, 1, 0)
			if to.After(now) {
				to = now
			}
			if !to.After(from) {
				continue
			}
			var combinedBounds *publicingest.Bounds
			names := []string{}
			for sid := range b.sources {
				if sb := boundsBySource[sid]; sb != nil {
					combinedBounds = unionBounds(combinedBounds, sb)
				}
				if nm := nameBySource[sid]; nm != "" {
					names = append(names, nm)
				}
			}
			suggestedName := ""
			if combinedBounds != nil {
				suggestedName = deriveFootprintName(*combinedBounds, names)
			}
			out.TimeBuckets = append(out.TimeBuckets, InventoryTimeBucket{
				Month:             k,
				Label:             from.Format("Jan 2006"),
				From:              from,
				To:                to,
				Total:             b.total,
				ByFamily:          b.byFamily,
				Bounds:            combinedBounds,
				SuggestedAreaName: suggestedName,
			})
		}
		return nil
	})
	if e != nil {
		return InventoryResult{}, e
	}
	return out, nil
}
