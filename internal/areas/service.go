package areas

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/jsoncanon"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Selection struct {
	RevisionID       string          `json:"revisionId,omitempty"`
	Kind             string          `json:"kind,omitempty"`
	Name             string          `json:"name,omitempty"`
	RegionID         string          `json:"regionId,omitempty"`
	SourceID         string          `json:"sourceId,omitempty"`
	LogicalKey       string          `json:"logicalKey,omitempty"`
	ReleaseID        string          `json:"releaseId,omitempty"`
	Geometry         json.RawMessage `json:"geometry,omitempty"`
	ParentRevisionID string          `json:"parentRevisionId,omitempty"`
}
type Geography struct {
	SourceID       string `json:"sourceId"`
	LogicalKey     string `json:"logicalKey"`
	RegionID       string `json:"regionId"`
	EditionID      string `json:"editionId"`
	ReleaseID      string `json:"releaseId,omitempty"`
	ParentRegionID string `json:"parentRegionId,omitempty"`
	Attribution    string `json:"attribution"`
	EditionSHA256  string `json:"editionSha256"`
	Meaning        string `json:"meaning"`
}
type Revision struct {
	Schema               string          `json:"schema"`
	NormalizationVersion string          `json:"normalizationVersion"`
	AreaID               string          `json:"areaId"`
	RevisionID           string          `json:"revisionId"`
	Resource             access.Resource `json:"resource"`
	Name                 string          `json:"name"`
	Kind                 string          `json:"kind"`
	Geometry             json.RawMessage `json:"geometry,omitempty"`
	OriginalGeometry     json.RawMessage `json:"originalGeometry,omitempty"`
	GeometrySHA256       string          `json:"geometrySha256,omitempty"`
	Normalized           bool            `json:"normalized"`
	Components           []Component     `json:"components"`
	AreaM2               float64         `json:"areaM2"`
	Display              Display         `json:"display"`
	Geography            *Geography      `json:"geography,omitempty"`
	ParentRevisionID     string          `json:"parentRevisionId,omitempty"`
	Relationship         string          `json:"relationship,omitempty"`
	RequiredSources      []string        `json:"requiredSources"`
	Persistent           bool            `json:"persistent"`
}
type Service struct{ Pool *pgxpool.Pool }

func Require(ctx context.Context) error {
	if e := access.Require(ctx, "sources.read"); e != nil {
		return e
	}
	r, e := access.ResourceFrom(ctx)
	if e != nil {
		return e
	}
	if r.Partition != access.PublicEvidence {
		return apperr.Forbidden("analyst areas require PUBLIC_EVIDENCE")
	}
	return nil
}
func (s Service) Get(ctx context.Context, id string) (Revision, error) {
	var out Revision
	if e := Require(ctx); e != nil {
		return out, e
	}
	var raw []byte
	var canonical, original []byte
	e := db.Scoped(s.Pool).QueryRow(ctx, `SELECT document,canonical_geometry,original_geometry FROM area_revisions WHERE id=$1`, id).Scan(&raw, &canonical, &original)
	if errors.Is(e, pgx.ErrNoRows) {
		return out, apperr.NotFound("area revision", "unavailable")
	}
	if e != nil {
		return out, e
	}
	e = json.Unmarshal(raw, &out)
	out.Geometry = canonical
	out.OriginalGeometry = original
	if len(canonical) > 0 && Hash(canonical) != out.GeometrySHA256 {
		return out, apperr.Conflict("retained area integrity mismatch")
	}
	out.Persistent = true
	return out, e
}
func (s Service) Resolve(ctx context.Context, in Selection) (Revision, error) {
	var out Revision
	if e := Require(ctx); e != nil {
		return out, e
	}
	bad := func() (Revision, error) {
		return out, apperr.Validation("choose one exact area revision, one geographic identity, world, or an explicit WGS84 area; conflicting selections are forbidden")
	}
	if len(in.Name) > 200 || len(in.SourceID) > 512 || len(in.RegionID) > 256 || len(in.LogicalKey) > 160 || len(in.ReleaseID) > 160 || len(in.RevisionID) > 160 || len(in.ParentRevisionID) > 160 {
		return bad()
	}
	if in.RevisionID != "" {
		if in.Kind != "" || in.Name != "" || in.RegionID != "" || in.SourceID != "" || in.LogicalKey != "" || in.ReleaseID != "" || len(in.Geometry) > 0 || in.ParentRevisionID != "" {
			return bad()
		}
		return s.Get(ctx, in.RevisionID)
	}
	_, resource, _ := access.From(ctx)
	out = Revision{Schema: "emad.area-revision.v1", NormalizationVersion: "wgs84-seam-v1", Resource: resource, Kind: in.Kind, Name: strings.TrimSpace(in.Name), Components: []Component{}, RequiredSources: []string{}, Display: Display{"supported", 85, "World is a discovery scope. Select an area before bounded processing."}, ParentRevisionID: in.ParentRevisionID}
	if out.Kind == "" {
		if in.RegionID != "" || in.LogicalKey != "" {
			out.Kind = "administrative"
		} else if len(in.Geometry) > 0 {
			out.Kind = "drawn"
		} else {
			return bad()
		}
	}
	if out.Kind == "world" {
		if in.RegionID != "" || in.SourceID != "" || in.LogicalKey != "" || in.ReleaseID != "" || len(in.Geometry) > 0 || in.ParentRevisionID != "" {
			return bad()
		}
		out.Name = "World · configured evidence"
	} else if in.RegionID != "" || in.LogicalKey != "" {
		if out.Kind != "administrative" && out.Kind != "named" {
			return bad()
		}
		if len(in.Geometry) > 0 || in.RegionID != "" && in.LogicalKey != "" || in.LogicalKey != "" && (in.SourceID == "" || in.ReleaseID == "") {
			return bad()
		}
		var raw, props []byte
		g := Geography{}
		var kind, name, parent string
		e := db.Scoped(s.Pool).QueryRow(ctx, `SELECT g.id,g.source_id,g.region_key,g.edition_id,g.parent_key,g.kind,g.name,ST_AsGeoJSON(g.geometry,17),e.document FROM geographic_regions g JOIN geographic_editions e ON e.id=g.edition_id WHERE ($1::text='' OR g.id=$1) AND ($2::text='' OR g.source_id=$2) AND ($3::text='' OR g.region_key=$3) AND ($4::text='' OR EXISTS(SELECT 1 FROM geographic_release_editions re WHERE re.release_id=$4 AND re.edition_id=g.edition_id))`, in.RegionID, in.SourceID, in.LogicalKey, in.ReleaseID).Scan(&g.RegionID, &g.SourceID, &g.LogicalKey, &g.EditionID, &parent, &kind, &name, &raw, &props)
		if errors.Is(e, pgx.ErrNoRows) {
			return out, apperr.NotFound("geographic area", "unavailable")
		}
		if e != nil {
			return out, e
		}
		var edition struct {
			SHA256 string `json:"sha256"`
			Rights struct {
				Attribution string `json:"attribution"`
			} `json:"rights"`
		}
		if e = json.Unmarshal(props, &edition); e != nil {
			return out, e
		}
		g.ReleaseID = in.ReleaseID
		g.Attribution = edition.Rights.Attribution
		g.EditionSHA256 = edition.SHA256
		g.Meaning = "Retained source boundary interpretation; selection does not establish jurisdiction or source coverage."
		if g.ReleaseID == "" {
			var releases []string
			e = db.Scoped(s.Pool).QueryRow(ctx, `SELECT COALESCE(array_agg(release_id ORDER BY release_id),ARRAY[]::text[]) FROM geographic_release_editions WHERE edition_id=$1`, g.EditionID).Scan(&releases)
			if e != nil {
				return out, e
			}
			if len(releases) > 1 {
				return out, apperr.Validation("geographic edition belongs to multiple releases; select an explicit release to preserve its parent hierarchy")
			}
			if len(releases) == 1 {
				g.ReleaseID = releases[0]
			}
		}
		if parent != "" {
			e = db.Scoped(s.Pool).QueryRow(ctx, `SELECT p.id FROM geographic_regions p WHERE p.source_id=$1 AND p.region_key=$2 AND (($3::text<>'' AND EXISTS(SELECT 1 FROM geographic_release_editions re WHERE re.release_id=$3 AND re.edition_id=p.edition_id)) OR ($3='' AND p.edition_id=$4)) ORDER BY p.id LIMIT 1`, g.SourceID, parent, g.ReleaseID, g.EditionID).Scan(&g.ParentRegionID)
			if e != nil && !errors.Is(e, pgx.ErrNoRows) {
				return out, e
			}
		}
		out.Name = name
		out.Geography = &g
		out.RequiredSources = []string{g.SourceID}
		in.Geometry = raw
		if kind == "place" {
			out.Kind = "point"
			out.Geometry = raw
			out.OriginalGeometry = raw
			out.GeometrySHA256 = Hash(raw)
			out.Display.Message = "Place point is a navigation target. Draw an area or deliberately create a labeled buffer before processing."
		}
	} else if in.SourceID != "" || in.ReleaseID != "" || out.Kind == "administrative" || out.Kind == "point" {
		return bad()
	}
	if out.Kind != "world" && out.Kind != "point" {
		if out.Kind != "administrative" && out.Kind != "named" && out.Kind != "drawn" && out.Kind != "viewport" && out.Kind != "composite" {
			return bad()
		}
		var e error
		out.Geometry, out.Components, out.AreaM2, out.Display, out.Normalized, e = normalize(ctx, s.Pool, in.Geometry)
		if e != nil {
			return out, e
		}
		out.OriginalGeometry = append(json.RawMessage(nil), in.Geometry...)
		out.GeometrySHA256 = Hash(out.Geometry)
	}
	if out.Name == "" {
		out.Name = map[string]string{"drawn": "Drawn study area", "viewport": "Explicit map view", "named": "Named study area", "composite": "Composite study area"}[out.Kind]
	}
	if in.ParentRevisionID != "" {
		parent, e := s.Get(ctx, in.ParentRevisionID)
		if e != nil {
			return out, e
		}
		if out.Kind == "point" || out.Kind == "world" || parent.Kind == "point" {
			return bad()
		}
		if parent.Kind != "world" {
			var covered bool
			if e = db.Scoped(s.Pool).QueryRow(ctx, `SELECT ST_Covers(ST_SetSRID(ST_GeomFromGeoJSON($1),4326),ST_SetSRID(ST_GeomFromGeoJSON($2),4326))`, string(parent.Geometry), string(out.Geometry)).Scan(&covered); e != nil {
				return out, e
			}
			if !covered {
				return out, apperr.Validation("subset must be covered by the exact retained parent area, including its holes and components")
			}
		}
		out.Relationship = "subset"
		for _, source := range parent.RequiredSources {
			if !contains(out.RequiredSources, source) {
				out.RequiredSources = append(out.RequiredSources, source)
			}
		}
		sort.Strings(out.RequiredSources)
	}
	raw, e := jsoncanon.Marshal(out)
	if e != nil {
		return out, e
	}
	h := Hash(raw)
	out.RevisionID = "arev_" + h
	logical := resource.ID() + "/" + out.Kind + "/" + out.GeometrySHA256
	if out.Geography != nil {
		logical = resource.ID() + "/" + out.Geography.SourceID + "/" + out.Geography.LogicalKey
	}
	out.AreaID = "area_" + Hash([]byte(logical))[:32]
	return out, nil
}
func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}

func SelectionFor(a Revision) Selection {
	if a.Persistent {
		return Selection{RevisionID: a.RevisionID}
	}
	in := Selection{Kind: a.Kind, Name: a.Name, ParentRevisionID: a.ParentRevisionID}
	if a.Geography != nil {
		in.RegionID = a.Geography.RegionID
		in.ReleaseID = a.Geography.ReleaseID
		in.SourceID = a.Geography.SourceID
	} else if a.Kind != "world" {
		in.Geometry = a.OriginalGeometry
	}
	return in
}
func (s Service) Persist(ctx context.Context, tx pgx.Tx, in Selection) (Revision, error) {
	out, e := s.Resolve(db.UseTx(ctx, tx), in)
	if e != nil {
		return out, e
	}
	if out.Kind == "point" {
		return out, apperr.Validation("a place point is not a processing area")
	}
	out.Persistent = true
	raw, _ := json.Marshal(out)
	sources, _ := json.Marshal(out.RequiredSources)
	var geometry any
	if len(out.Geometry) > 0 {
		geometry = string(out.Geometry)
	}
	_, e = tx.Exec(ctx, `INSERT INTO area_revisions(id,area_id,document,geometry,required_sources,canonical_geometry,original_geometry) VALUES($1,$2,$3,CASE WHEN $4::text IS NULL THEN NULL ELSE ST_SetSRID(ST_GeomFromGeoJSON($4),4326) END,$5,$6,$7) ON CONFLICT(id,scope_id)DO NOTHING`, out.RevisionID, out.AreaID, raw, geometry, sources, []byte(out.Geometry), []byte(out.OriginalGeometry))
	return out, e
}
func (s Service) Mount(r chi.Router) {
	r.Post("/areas/resolve", func(w http.ResponseWriter, r *http.Request) {
		var in Selection
		if !Decode(w, r, &in) {
			return
		}
		v, e := s.Resolve(r.Context(), in)
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, v)
	})
	r.Get("/areas/revisions/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.Get(r.Context(), chi.URLParam(r, "id"))
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, v)
	})
	r.Post("/areas/revisions", func(w http.ResponseWriter, r *http.Request) {
		var in Selection
		if !Decode(w, r, &in) {
			return
		}
		var v Revision
		e := db.WithTx(r.Context(), s.Pool, func(tx pgx.Tx) error { var e error; v, e = s.Persist(r.Context(), tx, in); return e })
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 201, v)
	})
}
func Decode(w http.ResponseWriter, r *http.Request, out any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 3<<20))
	d.DisallowUnknownFields()
	if d.Decode(out) != nil || d.Decode(new(any)) != io.EOF {
		httpx.Error(w, apperr.Validation("invalid or ambiguous scope document"))
		return false
	}
	return true
}
