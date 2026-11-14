package publicingest

import (
	"encoding/json"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/geo"
)

const EONETAdapter = "nasa-eonet-floods-v1"
const EONETURL = "https://eonet.gsfc.nasa.gov/api/v3/events"
const EONETDataset = "eonet-v3-floods"

var eonetID = regexp.MustCompile(`^EONET_[0-9]{1,12}$`)
var gdacsEventID = regexp.MustCompile(`^[0-9]{1,15}$`)

type EventGeometry struct {
	Date        time.Time       `json:"date"`
	Type        string          `json:"type"`
	Coordinates json.RawMessage `json:"coordinates"`
}

type EventCitation struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

type EventHistory struct {
	ProviderID string          `json:"providerId"`
	Category   string          `json:"category"`
	ClosedAt   *time.Time      `json:"closedAt"`
	Geometries []EventGeometry `json:"geometries"`
	Citations  []EventCitation `json:"citations"`
}

func eonetURL(in FetchRequest) string {
	b := in.Admission.Policy.AOI
	coordinates := []string{}
	for _, v := range []float64{b.West, b.North, b.East, b.South} {
		coordinates = append(coordinates, strconv.FormatFloat(v, 'f', -1, 64))
	}
	return EONETURL + "?" + url.Values{"category": {"floods"}, "status": {"all"}, "bbox": {strings.Join(coordinates, ",")},
		"start": {in.From.UTC().Format(time.DateOnly)}, "end": {in.To.UTC().Format(time.DateOnly)}, "limit": {strconv.Itoa(in.Admission.Policy.MaxRecordsPerPage)}}.Encode()
}

func geometryPosition(g EventGeometry, bounds Bounds) (*geo.Point, bool) {
	point := func(v []float64) (*geo.Point, bool) {
		if len(v) != 2 {
			return nil, false
		}
		p := geo.Point{Lng: v[0], Lat: v[1]}
		return &p, p.Valid()
	}
	switch g.Type {
	case "Point":
		var v []float64
		if json.Unmarshal(g.Coordinates, &v) != nil {
			return nil, false
		}
		p, ok := point(v)
		if ok && !within(bounds, *p) {
			return nil, true
		}
		return p, ok
	case "Polygon", "MultiPolygon":
		var polygons [][][][]float64
		if g.Type == "Polygon" {
			var rings [][][]float64
			if json.Unmarshal(g.Coordinates, &rings) != nil {
				return nil, false
			}
			polygons = append(polygons, rings)
		} else if json.Unmarshal(g.Coordinates, &polygons) != nil {
			return nil, false
		}
		var anchor *geo.Point
		count := 0
		for _, rings := range polygons {
			if len(rings) == 0 {
				return nil, false
			}
			for j, ring := range rings {
				if len(ring) < 4 {
					return nil, false
				}
				first, ok := point(ring[0])
				if !ok {
					return nil, false
				}
				last, ok := point(ring[len(ring)-1])
				if !ok || *first != *last {
					return nil, false
				}
				for _, coord := range ring {
					count++
					p, ok := point(coord)
					if !ok || count > 512 {
						return nil, false
					}
					if anchor == nil && j == 0 && within(bounds, *p) {
						anchor = p
					}
				}
			}
		}
		return anchor, count >= 4
	}
	return nil, false
}

func DecodeEONET(raw []byte, in FetchRequest) (Page, error) {
	fail := func() (Page, error) { return Page{}, &RetrievalError{Code: "invalid_response"} }
	var response struct{ Events []json.RawMessage }
	p := in.Admission.Policy
	if len(raw) > p.MaxResponseBytes || json.Unmarshal(raw, &response) != nil || response.Events == nil || len(response.Events) > p.MaxRecordsPerPage {
		return fail()
	}
	page := Page{Records: []Record{}, CoverageFrom: in.From, CoverageTo: in.To}
	if len(response.Events) == p.MaxRecordsPerPage {
		page.PartialReason = "provider_result_limit"
	}
	seen := map[string]bool{}
	for _, original := range response.Events {
		var event struct {
			ID, Title, Description, Link string
			Closed                       *time.Time
			Categories                   []struct{ ID string }
			Sources                      []EventCitation
			Geometry                     []EventGeometry
		}
		if json.Unmarshal(original, &event) != nil || !eonetID.MatchString(event.ID) || seen[event.ID] || event.Link != EONETURL+"/"+event.ID ||
			len(event.Geometry) < 1 || len(event.Geometry) > 64 || len(event.Sources) < 1 || len(event.Sources) > 7 ||
			len(event.Title) > 1000 || strings.TrimSpace(event.Title) == "" || len(original) > 128<<10 {
			return fail()
		}
		seen[event.ID] = true
		flood := false
		for _, c := range event.Categories {
			flood = flood || c.ID == "floods"
		}
		if !flood {
			return fail()
		}
		var position *geo.Point
		var first, last time.Time
		matched := false
		for _, g := range event.Geometry {
			anchor, ok := geometryPosition(g, p.AOI)
			if !ok || g.Date.IsZero() || g.Date.After(in.Now) {
				return fail()
			}
			if anchor == nil {
				continue
			}
			matched = matched || !g.Date.Before(in.From) && !g.Date.After(in.To)
			if first.IsZero() || g.Date.Before(first) {
				first = g.Date
			}
			if last.IsZero() || g.Date.After(last) {
				last, position = g.Date, anchor
			}
		}
		if !matched || position == nil {
			// The provider may return geometries which cross the selection
			// without an in-bounds vertex. Keep that query visibly partial.
			for _, g := range event.Geometry {
				if g.Type != "Point" && !g.Date.Before(in.From) && !g.Date.After(in.To) && page.PartialReason == "" {
					page.PartialReason = "reported_geometry_anchor_unavailable"
				}
			}
			page.ExcludedRecords++
			continue
		}
		if event.Closed != nil && (event.Closed.Before(last) || event.Closed.After(in.Now)) {
			return fail()
		}
		lineage := []string{"eonet:" + event.ID}
		exact := map[string]bool{}
		for _, citation := range event.Sources {
			u, err := url.Parse(citation.URL)
			if len(citation.ID) > 64 || strings.TrimSpace(citation.ID) == "" || len(citation.URL) > 2048 || err != nil ||
				u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
				return fail()
			}
			if citation.ID != "GDACS" {
				if publicReference(citation.URL) != nil {
					return fail()
				}
				continue
			}
			q, parseErr := url.ParseQuery(u.RawQuery)
			if u.ForceQuery || parseErr != nil {
				return fail()
			}
			if u.Host == "www.gdacs.org" && u.Path == "/report.aspx" && len(q) == 2 && len(q["eventtype"]) == 1 && len(q["eventid"]) == 1 && q.Get("eventtype") == "FL" && gdacsEventID.MatchString(q.Get("eventid")) {
				exact["gdacs:FL:"+q.Get("eventid")] = true
			} else if publicReference(citation.URL) != nil {
				return fail()
			}
		}
		var keys []string
		for key := range exact {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		lineage = append(lineage, keys...)
		claim := event.Description
		if strings.TrimSpace(claim) == "" {
			claim = event.Title
		}
		context := ContextRecord{Kind: "REPORT", Title: event.Title, Position: *position, From: first, To: last,
			LocationBasis:    "Provider point or reported polygon boundary vertex; not a confirmed affected-area boundary",
			TimeBasis:        "First and latest admitted provider geometry dates; closure is a separate lifecycle state",
			ExtractionMethod: EONETAdapter, Uncertainty: "Curated provider report with unknown spatial and temporal accuracy",
			Report: &SourceReport{Claim: claim, CitedSource: "NASA EONET", CitedSourceID: event.ID, DocumentURL: event.Link,
				CitedFrom: first.Format(time.RFC3339), CitedTo: last.Format(time.RFC3339), LineageKeys: lineage,
				CorroborationMeaning: "An exact GDACS citation identifies shared lineage, not independent corroboration.",
				EventHistory:         &EventHistory{ProviderID: event.ID, Category: "floods", ClosedAt: event.Closed, Geometries: event.Geometry, Citations: event.Sources}}}
		if context.Validate(in.Now) != nil {
			return fail()
		}
		page.Records = append(page.Records, Record{ProviderRecordID: event.ID, ObservedAt: &last, ReferenceURL: event.Link, OriginalRecord: original, Context: &context})
	}
	return page, nil
}
