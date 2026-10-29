package reference

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/areas"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/jsoncanon"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

type ReleaseInput struct {
	SourceID     string   `json:"sourceId"`
	Key          string   `json:"key"`
	Version      string   `json:"version"`
	EditionIDs   []string `json:"editionIds"`
	ExpectedHead string   `json:"expectedHead"`
	Activate     bool     `json:"activate"`
}
type Release struct {
	ReleaseInput
	ID         string              `json:"id"`
	SHA256     string              `json:"sha256"`
	ActorID    string              `json:"actorId"`
	RecordedAt time.Time           `json:"recordedAt"`
	Editions   []map[string]string `json:"editions"`
}

func (s *Service) Release(ctx context.Context, in ReleaseInput) (Release, error) {
	var out Release
	if e := require(ctx, "sources.manage"); e != nil {
		return out, e
	}
	p, r, _ := access.From(ctx)
	if p.Kind != "human" {
		return out, apperr.Forbidden("geography release requires a registered human")
	}
	if e := access.RequireSource(ctx, in.SourceID); e != nil {
		return out, e
	}
	if !geographicKey.MatchString(in.Key) || !geographicKey.MatchString(in.Version) || len(in.EditionIDs) < 1 || len(in.EditionIDs) > 256 {
		return out, apperr.Validation("release needs a source, version and 1–256 exact editions")
	}
	sort.Strings(in.EditionIDs)
	for i, id := range in.EditionIDs {
		if i > 0 && id == in.EditionIDs[i-1] {
			return out, apperr.Validation("duplicate release edition")
		}
	}
	specification := struct {
		Source, Key, Version string
		Editions             []string
	}{in.SourceID, in.Key, in.Version, in.EditionIDs}
	spec, _ := json.Marshal(specification)
	hash := Hash(append([]byte(r.ID()+"/"), spec...))
	out = Release{ReleaseInput: in, ID: "georel_" + hash, SHA256: hash, ActorID: p.ID, RecordedAt: time.Now().UTC(), Editions: []map[string]string{}}
	e := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if _, e := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, r.ID()+"/geography/"+in.SourceID); e != nil {
			return e
		}
		for _, id := range in.EditionIDs {
			var sha, version string
			e := tx.QueryRow(ctx, `SELECT document->>'sha256',document->>'version' FROM geographic_editions WHERE id=$1 AND source_id=$2`, id, in.SourceID).Scan(&sha, &version)
			if errors.Is(e, pgx.ErrNoRows) {
				return apperr.NotFound("release edition", "unavailable")
			}
			if e != nil {
				return e
			}
			out.Editions = append(out.Editions, map[string]string{"id": id, "sha256": sha, "version": version})
		}
		var invalid bool
		if e := tx.QueryRow(ctx, `WITH members AS(SELECT * FROM geographic_regions WHERE source_id=$1 AND edition_id=ANY($2::text[])) SELECT EXISTS(SELECT region_key FROM members GROUP BY region_key HAVING count(*)>1) OR EXISTS(SELECT 1 FROM members c WHERE c.parent_key<>'' AND NOT EXISTS(SELECT 1 FROM members p WHERE p.region_key=c.parent_key))`, in.SourceID, in.EditionIDs).Scan(&invalid); e != nil {
			return e
		}
		if invalid {
			return apperr.Validation("release has duplicate logical identities or unresolved parents; stage every required edition before activation")
		}
		if e := tx.QueryRow(ctx, `WITH RECURSIVE members AS(SELECT region_key,parent_key FROM geographic_regions WHERE source_id=$1 AND edition_id=ANY($2::text[])),paths AS(SELECT region_key,parent_key,ARRAY[region_key] trail,false cycle FROM members UNION ALL SELECT p.region_key,m.parent_key,p.trail||m.region_key,m.region_key=ANY(p.trail) FROM paths p JOIN members m ON m.region_key=p.parent_key WHERE NOT p.cycle AND cardinality(p.trail)<64) SELECT EXISTS(SELECT 1 FROM paths WHERE cycle OR cardinality(trail)=64 AND parent_key<>'')`, in.SourceID, in.EditionIDs).Scan(&invalid); e != nil {
			return e
		}
		if invalid {
			return apperr.Validation("release hierarchy contains a cycle or exceeds 63 parent levels")
		}
		var old []byte
		e := tx.QueryRow(ctx, `SELECT document FROM geographic_releases WHERE source_id=$1 AND release_key=$2 AND version=$3`, in.SourceID, in.Key, in.Version).Scan(&old)
		if e == nil {
			var v Release
			if e = json.Unmarshal(old, &v); e != nil {
				return e
			}
			if v.SHA256 != hash {
				return apperr.Conflict("geographic release version is immutable")
			}
			out = v
		} else if !errors.Is(e, pgx.ErrNoRows) {
			return e
		} else {
			encoded, _ := json.Marshal(out)
			if _, e = tx.Exec(ctx, `INSERT INTO geographic_releases(id,source_id,release_key,version,document)VALUES($1,$2,$3,$4,$5)`, out.ID, in.SourceID, in.Key, in.Version, encoded); e != nil {
				return e
			}
			for _, id := range in.EditionIDs {
				if _, e = tx.Exec(ctx, `INSERT INTO geographic_release_editions(release_id,edition_id,source_id)VALUES($1,$2,$3)`, out.ID, id, in.SourceID); e != nil {
					return e
				}
			}
		}
		if in.Activate {
			var head string
			e := tx.QueryRow(ctx, `SELECT release_id FROM geographic_release_heads WHERE source_id=$1`, in.SourceID).Scan(&head)
			if e != nil && !errors.Is(e, pgx.ErrNoRows) {
				return e
			}
			if head != out.ID && head != in.ExpectedHead {
				return apperr.Conflict("geographic release head changed; review the new head")
			}
			_, e = tx.Exec(ctx, `INSERT INTO geographic_release_heads(source_id,release_id)VALUES($1,$2) ON CONFLICT(scope_id,source_id)DO UPDATE SET release_id=EXCLUDED.release_id`, in.SourceID, out.ID)
			if e != nil {
				return e
			}
		}
		return nil
	})
	return out, e
}

type ProfileInput struct {
	Key        string            `json:"key"`
	Version    string            `json:"version"`
	Name       string            `json:"name"`
	Definition string            `json:"definition"`
	Members    []areas.Selection `json:"members"`
}
type RegionProfile struct {
	ID              string           `json:"id"`
	SHA256          string           `json:"sha256"`
	Key             string           `json:"key"`
	Version         string           `json:"version"`
	Name            string           `json:"name"`
	Definition      string           `json:"definition"`
	Members         []areas.Revision `json:"members"`
	RequiredSources []string         `json:"requiredSources"`
}

func (s *Service) Profile(ctx context.Context, in ProfileInput) (RegionProfile, error) {
	var out RegionProfile
	if e := require(ctx, "sources.manage"); e != nil {
		return out, e
	}
	p, resource, _ := access.From(ctx)
	if p.Kind != "human" {
		return out, apperr.Forbidden("region profile requires a human")
	}
	if !geographicKey.MatchString(in.Key) || !geographicKey.MatchString(in.Version) || len(in.Name) < 1 || len(in.Name) > 200 || len(in.Definition) < 10 || len(in.Definition) > 4000 || len(in.Members) < 1 || len(in.Members) > 128 {
		return out, apperr.Validation("versioned region profile definition and bounded exact members required")
	}
	out = RegionProfile{Key: in.Key, Version: in.Version, Name: in.Name, Definition: in.Definition, Members: []areas.Revision{}, RequiredSources: []string{}}
	e := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		for _, selection := range in.Members {
			a, e := (areas.Service{Pool: s.pool}).Persist(ctx, tx, selection)
			if e != nil {
				return e
			}
			if a.Kind == "world" || a.Kind == "point" || a.Geography != nil && a.Geography.ReleaseID == "" {
				return apperr.Validation("profile needs exact released administrative members or explicitly labeled drawn study areas")
			}
			out.Members = append(out.Members, a)
			for _, source := range a.RequiredSources {
				found := false
				for _, v := range out.RequiredSources {
					found = found || v == source
				}
				if !found {
					out.RequiredSources = append(out.RequiredSources, source)
				}
			}
		}
		sort.Strings(out.RequiredSources)
		raw, e := jsoncanon.Marshal(out)
		if e != nil {
			return e
		}
		out.SHA256 = Hash(append([]byte(resource.ID()+"/"), raw...))
		out.ID = "regprof_" + out.SHA256
		raw, _ = json.Marshal(out)
		sources, _ := json.Marshal(out.RequiredSources)
		if _, e := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, resource.ID()+"/profile/"+in.Key); e != nil {
			return e
		}
		var old string
		e = tx.QueryRow(ctx, `SELECT id FROM region_profile_revisions WHERE profile_key=$1 AND version=$2`, in.Key, in.Version).Scan(&old)
		if e == nil && old != out.ID {
			return apperr.Conflict("region profile version is immutable")
		}
		if e != nil && !errors.Is(e, pgx.ErrNoRows) {
			return e
		}
		_, e = tx.Exec(ctx, `INSERT INTO region_profile_revisions(id,profile_key,version,document,required_sources,manifest_bytes)VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(id,scope_id)DO NOTHING`, out.ID, in.Key, in.Version, raw, sources, raw)
		return e
	})
	return out, e
}
func (h *Handler) mountReleases(r chi.Router) {
	r.Get("/geography/releases", func(w http.ResponseWriter, r *http.Request) {
		if e := require(r.Context(), "sources.read"); e != nil {
			httpx.Error(w, e)
			return
		}
		rows, e := db.Scoped(h.s.pool).Query(r.Context(), `SELECT source_id,release_id FROM geographic_release_heads ORDER BY source_id LIMIT 256`)
		if e != nil {
			httpx.Error(w, e)
			return
		}
		defer rows.Close()
		items := []map[string]string{}
		for rows.Next() {
			var source, id string
			if e = rows.Scan(&source, &id); e != nil {
				httpx.Error(w, e)
				return
			}
			items = append(items, map[string]string{"sourceId": source, "releaseId": id})
		}
		if e = rows.Err(); e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, map[string]any{"items": items})
	})
	r.Post("/geography/releases", func(w http.ResponseWriter, r *http.Request) {
		var in ReleaseInput
		if !areas.Decode(w, r, &in) {
			return
		}
		v, e := h.s.Release(r.Context(), in)
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 201, v)
	})
	r.Get("/geography/releases/{id}", func(w http.ResponseWriter, r *http.Request) {
		if e := require(r.Context(), "sources.read"); e != nil {
			httpx.Error(w, e)
			return
		}
		var raw []byte
		e := db.Scoped(h.s.pool).QueryRow(r.Context(), `SELECT document FROM geographic_releases WHERE id=$1`, chi.URLParam(r, "id")).Scan(&raw)
		if errors.Is(e, pgx.ErrNoRows) {
			e = apperr.NotFound("geographic release", "unavailable")
		}
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, json.RawMessage(raw))
	})
	r.Post("/geography/profiles", func(w http.ResponseWriter, r *http.Request) {
		var in ProfileInput
		if !areas.Decode(w, r, &in) {
			return
		}
		v, e := h.s.Profile(r.Context(), in)
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 201, v)
	})
	r.Get("/geography/profiles/{id}", func(w http.ResponseWriter, r *http.Request) {
		if e := require(r.Context(), "sources.read"); e != nil {
			httpx.Error(w, e)
			return
		}
		var raw []byte
		e := db.Scoped(h.s.pool).QueryRow(r.Context(), `SELECT manifest_bytes FROM region_profile_revisions WHERE id=$1`, chi.URLParam(r, "id")).Scan(&raw)
		if errors.Is(e, pgx.ErrNoRows) {
			e = apperr.NotFound("region profile", "unavailable")
		}
		if e != nil {
			httpx.Error(w, e)
			return
		}
		httpx.JSON(w, 200, json.RawMessage(raw))
	})
}
