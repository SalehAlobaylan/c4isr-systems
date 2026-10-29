package reference

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// Navigation editions have their own bounded admission. They never broaden a
// source connector's retrieval policy or replace immutable reference artifacts.
type GeographyInput struct {
	Staged   bool                `json:"staged,omitempty"`
	SourceID string              `json:"sourceId"`
	Label    string              `json:"label"`
	Version  string              `json:"version"`
	Rights   publicingest.Rights `json:"rights"`
	Receipt  json.RawMessage     `json:"receipt"`
	SHA256   string              `json:"sha256"`
	Artifact []byte              `json:"artifact"`
}
type GeographyEdition struct {
	GeographyInput
	ID         string    `json:"id"`
	AdmittedBy string    `json:"admittedBy"`
	AdmittedAt time.Time `json:"admittedAt"`
	Features   int       `json:"features"`
}

var geographicKey = regexp.MustCompile(`^[a-zA-Z0-9_.:-]{1,160}$`)

func (s *Service) AdmitGeography(ctx context.Context, in GeographyInput) (GeographyEdition, bool, error) {
	var out GeographyEdition
	if e := require(ctx, "sources.manage"); e != nil {
		return out, false, e
	}
	p, r, _ := access.From(ctx)
	if p.Kind != "human" {
		return out, false, apperr.Forbidden("geography admission requires a registered human")
	}
	if e := access.RequireSource(ctx, in.SourceID); e != nil {
		return out, false, e
	}
	if len(in.Artifact) < 1 || len(in.Artifact) > 8<<20 || Hash(in.Artifact) != in.SHA256 || len(in.Label) < 1 || len(in.Label) > 200 || !geographicKey.MatchString(in.Version) || !in.Rights.RetainRaw || !in.Rights.AllowAnalysis || in.Rights.ID == "" || in.Rights.Attribution == "" || in.Rights.TermsURL == "" || len(in.Receipt) < 2 || len(in.Receipt) > 32768 {
		return out, false, apperr.Validation("invalid geography identity, receipt, rights or retained bytes")
	}
	terms, errURL := url.Parse(in.Rights.TermsURL)
	if errURL != nil || terms.Scheme != "https" || terms.Host == "" || terms.User != nil || in.Rights.LegalBasis == "" || in.Rights.ExpiresAt != nil && !in.Rights.ExpiresAt.After(time.Now()) {
		return out, false, apperr.Validation("invalid or expired geography rights")
	}
	var collection Collection
	if json.Unmarshal(in.Artifact, &collection) != nil || collection.Type != "FeatureCollection" || len(collection.CRS) > 0 || len(collection.Features) < 1 || len(collection.Features) > 5000 {
		return out, false, apperr.Validation("geography requires 1–5000 retained features")
	}
	spec, _ := json.Marshal(in)
	key := Hash(append([]byte(r.ID()+"/"), spec...))
	id := "geoedition_" + key[:32]
	raw := in.Artifact
	in.Artifact = nil
	out = GeographyEdition{in, id, p.ID, time.Now().UTC(), len(collection.Features)}
	duplicate := false
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if _, e := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, r.ID()+"/geography"); e != nil {
			return e
		}
		var old []byte
		e := tx.QueryRow(ctx, `SELECT document FROM geographic_editions WHERE id=$1`, id).Scan(&old)
		if e == nil {
			duplicate = true
			return json.Unmarshal(old, &out)
		}
		if !errors.Is(e, pgx.ErrNoRows) {
			return e
		}
		var sourceExists bool
		if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM sources WHERE id=$1)`, in.SourceID).Scan(&sourceExists); e != nil {
			return e
		}
		if !sourceExists {
			return apperr.NotFound("geography source", "unavailable")
		}
		encoded, _ := json.Marshal(out)
		if _, e = tx.Exec(ctx, `INSERT INTO geographic_editions(id,source_id,document,artifact,spec_hash) VALUES($1,$2,$3,$4,$5)`, id, in.SourceID, encoded, raw, key); e != nil {
			return e
		}
		for _, f := range collection.Features {
			prop := f.Properties
			get := func(k string) string { v, _ := prop[k].(string); return v }
			k, parent, kind, name := get("key"), get("parentKey"), get("kind"), get("name")
			if !geographicKey.MatchString(k) || (parent != "" && !geographicKey.MatchString(parent)) || name == "" || len(name) > 200 || !strings.Contains("|country|region|governorate|place|study|", "|"+kind+"|") {
				return apperr.Validation("invalid geographic feature identity or hierarchy")
			}
			if len(f.CRS) > 0 {
				return apperr.Validation("embedded feature CRS is forbidden")
			}
			var g map[string]any
			if json.Unmarshal(f.Geometry, &g) != nil || g["crs"] != nil || (g["type"] != "Polygon" && g["type"] != "MultiPolygon" && g["type"] != "Point") {
				return apperr.Validation("geography requires explicit WGS84 polygons or place points")
			}
			if (kind == "place") != (g["type"] == "Point") {
				return apperr.Validation("place locations require points; country, region and study scopes require polygons")
			}
			var valid bool
			if e = tx.QueryRow(ctx, `WITH g AS(SELECT ST_SetSRID(ST_GeomFromGeoJSON($1),4326) g) SELECT ST_IsValid(g) AND NOT ST_IsEmpty(g) AND ST_NDims(g)=2 AND ST_NPoints(g)<=100000 AND ST_XMin(g)>=-180 AND ST_XMax(g)<=180 AND ST_YMin(g)>=-90 AND ST_YMax(g)<=90 FROM g`, string(f.Geometry)).Scan(&valid); e != nil {
				return apperr.Validation("invalid geographic geometry")
			}
			if !valid {
				return apperr.Validation("geographic geometry exceeds WGS84, validity or size bounds")
			}
			attrs, _ := json.Marshal(prop)
			aliases := get("aliases")
			if len(aliases) > 2000 {
				return apperr.Validation("geographic aliases exceed budget")
			}
			_, e = tx.Exec(ctx, `INSERT INTO geographic_regions(id,edition_id,source_id,region_key,parent_key,kind,name,aliases,geometry,properties) VALUES($1,$2,$3,$4,$5,$6,$7,$8,ST_SetSRID(ST_GeomFromGeoJSON($9),4326),$10)`, id+":"+k, id, in.SourceID, k, parent, kind, name, aliases, string(f.Geometry), attrs)
			if e != nil {
				return e
			}
		}
		return nil
	})
	return out, duplicate, err
}

const regionSelect = `SELECT g.id,ST_AsGeoJSON(g.geometry,17),g.properties||jsonb_build_object('regionKey',g.region_key,'parentKey',g.parent_key,'kind',g.kind,'name',g.name,'releaseId',(SELECT h.release_id FROM geographic_release_heads h WHERE h.source_id=g.source_id AND EXISTS(SELECT 1 FROM geographic_release_editions re WHERE re.release_id=h.release_id AND re.edition_id=g.edition_id)),'editionId',g.edition_id,'sourceId',g.source_id,'version',e.document->>'version','rights',e.document->'rights','sha256',e.document->>'sha256','admittedAt',e.document->>'admittedAt','admittedBy',e.document->>'admittedBy','attribution',e.document#>>'{rights,attribution}','receipt',e.document->'receipt','bounds',jsonb_build_array(ST_XMin(g.geometry),ST_YMin(g.geometry),ST_XMax(g.geometry),ST_YMax(g.geometry))),count(*)OVER() FROM geographic_regions g JOIN geographic_editions e ON e.id=g.edition_id `

// GeographyDiscoveryCTE selects complete per-source release heads for display discovery.
const GeographyDiscoveryCTE = latestRegions

const latestRegions = `WITH chosen AS(SELECT DISTINCT ON(g.source_id,g.region_key) g.id FROM geographic_regions g JOIN geographic_editions e ON e.id=g.edition_id LEFT JOIN geographic_release_heads h ON h.source_id=g.source_id AND h.scope_id=g.scope_id WHERE (h.release_id IS NULL AND NOT COALESCE((e.document->>'staged')::boolean,false)) OR EXISTS(SELECT 1 FROM geographic_release_editions re WHERE re.release_id=h.release_id AND re.edition_id=g.edition_id) ORDER BY g.source_id,g.region_key,(e.document->>'admittedAt')::timestamptz DESC,g.id DESC) `

func (s *Service) Geography(ctx context.Context, q, parent, kind, id string, offset int, selectors ...string) (Viewport, error) {
	out := Viewport{Type: "FeatureCollection", Features: []Feature{}}
	if e := require(ctx, "sources.read"); e != nil {
		return out, e
	}
	if kind != "" && !strings.Contains("|country|region|governorate|place|study|", "|"+kind+"|") {
		return out, apperr.Validation("invalid geographic kind")
	}
	if len(q) > 160 || len(parent) > 160 || len(id) > 256 || offset < 0 || offset > 100000 {
		return out, apperr.Validation("invalid geographic selection")
	}
	source, release, edition := "", "", ""
	if len(selectors) > 0 {
		source = selectors[0]
	}
	if len(selectors) > 1 {
		release = selectors[1]
	}
	if len(selectors) > 2 {
		edition = selectors[2]
	}
	if len(source) > 512 || len(release) > 160 || len(edition) > 160 || id != "" && (q != "" || parent != "" || kind != "" || offset != 0) || release != "" && edition != "" {
		return out, apperr.Validation("conflicting geographic discovery and exact selection")
	}
	// Exact retained IDs never pass through latest discovery. An explicit
	// edition/release likewise keeps its own complete source-qualified hierarchy.
	prefix, join := latestRegions, `JOIN chosen c ON c.id=g.id `
	if id != "" || release != "" || edition != "" {
		prefix = ""
		join = ""
	}
	rows, e := db.Scoped(s.pool).Query(ctx, prefix+regionSelect+join+`WHERE ($1='' OR strpos(emad_geographic_search(g.name||' '||g.aliases),emad_geographic_search($1))>0) AND ($2='' OR g.parent_key=$2) AND ($3='' OR g.kind=$3) AND ($4='' OR g.id=$4) AND ($6::text='' OR g.source_id=$6) AND ($7::text='' OR EXISTS(SELECT 1 FROM geographic_release_editions re WHERE re.release_id=$7 AND re.edition_id=g.edition_id)) AND ($8::text='' OR g.edition_id=$8) ORDER BY g.kind,g.name,g.id LIMIT 300 OFFSET $5`, q, parent, kind, id, offset, source, release, edition)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		f := Feature{Type: "Feature"}
		var props, geometry []byte
		if e = rows.Scan(&f.ID, &geometry, &props, &out.Matched); e != nil {
			return out, e
		}
		f.Geometry = geometry
		if e = json.Unmarshal(props, &f.Properties); e != nil {
			return out, e
		}
		if release != "" {
			f.Properties["releaseId"] = release
		}
		out.Features = append(out.Features, f)
	}
	out.Offset = offset
	out.Truncated = out.Matched > offset+len(out.Features)
	if out.Truncated {
		next := offset + len(out.Features)
		out.NextOffset = &next
	}
	return out, rows.Err()
}

func (h *Handler) mountGeography(r chi.Router) {
	h.mountAvailability(r)
	h.mountReleases(r)
	r.Post("/geography/editions", func(w http.ResponseWriter, r *http.Request) {
		var in GeographyInput
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 12<<20))
		dec.DisallowUnknownFields()
		if dec.Decode(&in) != nil || dec.Decode(new(any)) != io.EOF {
			httpx.Error(w, apperr.Validation("invalid geography admission"))
			return
		}
		v, duplicate, e := h.s.AdmitGeography(r.Context(), in)
		if e != nil {
			httpx.Error(w, e)
			return
		}
		status := 201
		if duplicate {
			status = 200
		}
		httpx.JSON(w, status, v)
	})
	r.Get("/geography/regions", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		for key, values := range q {
			if !map[string]bool{"q": true, "parent": true, "kind": true, "id": true, "offset": true, "source_id": true, "release_id": true, "edition_id": true}[key] || len(values) != 1 {
				httpx.Error(w, apperr.Validation("invalid or ambiguous geography query"))
				return
			}
		}
		offset := 0
		if raw := q.Get("offset"); raw != "" {
			var err error
			offset, err = strconv.Atoi(raw)
			if err != nil {
				httpx.Error(w, apperr.Validation("invalid geography offset"))
				return
			}
		}
		v, e := h.s.Geography(r.Context(), q.Get("q"), q.Get("parent"), q.Get("kind"), q.Get("id"), offset, q.Get("source_id"), q.Get("release_id"), q.Get("edition_id"))
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, v)
	})
}
