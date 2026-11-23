package geoenv

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/go-chi/chi/v5"
)

func respond[T any](w http.ResponseWriter, status int, value T, err error) {
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, status, value)
}
func request[I, O any](w http.ResponseWriter, r *http.Request, status int, fn func(context.Context, I) (O, error)) {
	var in I
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.Error(w, err)
		return
	}
	out, err := fn(r.Context(), in)
	respond(w, status, out, err)
}

type page[T any] struct {
	Items []T    `json:"items"`
	Next  string `json:"next,omitempty"`
}

func (s Service) Mount(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Cache-Control", "no-store")
				if s.ReadOnly && r.Method != http.MethodGet && !strings.HasSuffix(r.URL.Path, "/withdraw") &&
					!strings.HasSuffix(r.URL.Path, "/suitability") && !strings.HasSuffix(r.URL.Path, "/municipal-workload") &&
					!strings.HasSuffix(r.URL.Path, "/port-activity") && !strings.HasSuffix(r.URL.Path, "/flood-mapping") &&
					!strings.HasSuffix(r.URL.Path, "/radar-outages") && !strings.HasSuffix(r.URL.Path, "/civilian-access-replay") &&
					!strings.HasSuffix(r.URL.Path, "/civilian-aerial-review") && !strings.HasSuffix(r.URL.Path, "/regional-response-review") &&
					!strings.HasSuffix(r.URL.Path, "/supporting-source-review") {
					httpx.Error(w, apperr.Conflict("new environment work is disabled"))
					return
				}
				next.ServeHTTP(w, r)
			})
		})
		r.Get("/environment-profiles", s.listProfiles)
		s.mountWorker(r)
		r.Get("/environment-operations/metrics", s.environmentMetrics)
		r.Get("/environment-operations/storage", s.storageStatus)
		r.Get("/environment-sources/{id}/impact", s.sourcePolicyImpact)
		r.Get("/geographic-environments", s.listDefinitions)
		r.Post("/geographic-environments", func(w http.ResponseWriter, r *http.Request) {
			request(w, r, 201, func(ctx context.Context, in DefinitionInput) (Definition, error) {
				return s.saveDefinition(ctx, "", in)
			})
		})
		r.Get("/geographic-environments/{id}", func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			if err := require(ctx, Read, false); err != nil {
				httpx.Error(w, err)
				return
			}
			d, err := s.definition(ctx, chi.URLParam(r, "id"), 0)
			respond(w, 200, d, err)
		})
		r.Put("/geographic-environments/{id}", func(w http.ResponseWriter, r *http.Request) {
			request(w, r, 200, func(ctx context.Context, in DefinitionInput) (Definition, error) {
				return s.saveDefinition(ctx, chi.URLParam(r, "id"), in)
			})
		})
		for _, endpoint := range []string{"eligibility", "preflight"} {
			r.Get("/geographic-environments/{id}/"+endpoint, func(w http.ResponseWriter, r *http.Request) {
				ctx := r.Context()
				if err := require(ctx, Read, false); err != nil {
					httpx.Error(w, err)
					return
				}
				d, err := s.definition(ctx, chi.URLParam(r, "id"), 0)
				if err != nil {
					httpx.Error(w, err)
					return
				}
				_, result, err := s.preflight(ctx, d)
				respond(w, 200, result, err)
			})
		}
		r.Post("/geographic-environments/{id}/builds", func(w http.ResponseWriter, r *http.Request) {
			request(w, r, 202, func(ctx context.Context, in BuildInput) (Job, error) { return s.submit(ctx, chi.URLParam(r, "id"), in) })
		})
		r.Get("/environment-builds/{id}", func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			if err := require(ctx, Read, false); err != nil {
				httpx.Error(w, err)
				return
			}
			j, err := s.visibleJob(ctx, chi.URLParam(r, "id"))
			respond(w, 200, j, err)
		})
		r.Get("/environment-builds/{id}/attempts", s.attempts)
		for _, action := range []string{"cancel", "retry"} {
			r.Post("/environment-builds/{id}/"+action, func(w http.ResponseWriter, r *http.Request) {
				j, err := s.changeJob(r.Context(), chi.URLParam(r, "id"), action)
				respond(w, 200, j, err)
			})
		}
		r.Get("/environment-assets", s.listAssets)
		r.Post("/environment-sources/{id}/admissions", func(w http.ResponseWriter, r *http.Request) {
			request(w, r, 201, func(ctx context.Context, in RetainedAdmission) (publicingest.Admission, error) {
				return s.admitRetained(ctx, chi.URLParam(r, "id"), in)
			})
		})
		r.Post("/environment-assets", func(w http.ResponseWriter, r *http.Request) { request(w, r, 201, s.reserve) })
		r.Put("/environment-assets/{id}/content", func(w http.ResponseWriter, r *http.Request) {
			if s.UploadSlots != nil {
				select {
				case s.UploadSlots <- struct{}{}:
					defer func() { <-s.UploadSlots }()
				default:
					httpx.Error(w, apperr.Conflict("geographic uploads are at capacity"))
					return
				}
			}
			ctx, cancel := context.WithTimeout(r.Context(), 30*time.Minute)
			defer cancel()
			controller := http.NewResponseController(w)
			if deadline, ok := ctx.Deadline(); ok && controller.SetReadDeadline(deadline) == nil {
				defer controller.SetReadDeadline(time.Time{})
			}
			a, err := s.upload(ctx, chi.URLParam(r, "id"), r.Body, r.ContentLength)
			respond(w, 200, a, err)
		})
		r.Get("/environment-assets/{id}/content", s.assetContent)
		r.Get("/environment-releases", s.listReleases)
		r.Get("/environment-releases/{id}", s.releaseDetail)
		r.Get("/environment-releases/{id}/manifest", s.releaseManifest)
		r.Get("/environment-releases/{id}/lineage", s.releaseLineage)
		r.Get("/environment-releases/{id}/decisions", func(w http.ResponseWriter, r *http.Request) {
			v, err := s.releaseDecisions(r.Context(), chi.URLParam(r, "id"))
			respond(w, 200, v, err)
		})
		r.Get("/environment-releases/{id}/quality", s.releaseQuality)
		r.Post("/environment-releases/{id}/municipal-workload", s.reviewMunicipalQ1)
		r.Post("/environment-releases/{id}/port-activity", s.reviewPortActivity)
		r.Post("/environment-releases/{id}/flood-mapping", s.reviewFloodProducts)
		r.Post("/environment-releases/{id}/radar-outages", s.reviewRadarOutages)
		r.Post("/environment-releases/{id}/civilian-access-replay", s.reviewCivilianAccess)
		r.Post("/environment-releases/{id}/civilian-aerial-review", s.reviewCivilianAerial)
		r.Post("/environment-releases/{id}/regional-response-review", s.reviewRegionalResponse)
		r.Post("/environment-releases/{id}/supporting-source-review", s.reviewSupportingSources)
		r.Get("/environment-releases/{id}/quality-report", s.structuredReleaseQuality)
		r.Get("/environment-releases/{id}/stac", s.stac)
		r.Get("/environment-releases/{id}/prepared", s.prepared)
		r.Get("/environment-releases/{id}/rebuild", s.rebuild)
		r.Get("/environment-releases/{id}/displays", s.listDisplays)
		s.assessmentRoutes(r)
		r.Post("/environment-releases/{id}/viewing-packages", func(w http.ResponseWriter, r *http.Request) {
			request(w, r, http.StatusCreated, func(ctx context.Context, in ViewingPackageInput) (ViewingPackage, error) {
				return s.issueViewingPackage(ctx, chi.URLParam(r, "id"), in)
			})
		})
		r.Post("/environment-releases/{id}/display-builds", func(w http.ResponseWriter, r *http.Request) {
			request(w, r, http.StatusAccepted, func(ctx context.Context, in DisplayBuildInput) (Job, error) {
				j, err := s.submitDisplayBuild(ctx, chi.URLParam(r, "id"), in)
				if err == nil {
					w.Header().Set("Location", "/api/v1/environment-builds/"+j.ID)
				}
				return j, err
			})
		})
		r.Post("/environment-display-builds/{id}/cancel", func(w http.ResponseWriter, r *http.Request) {
			j, err := s.changeBuild(r.Context(), chi.URLParam(r, "id"), "cancel", true)
			respond(w, http.StatusOK, j, err)
		})
		r.Post("/environment-releases/{id}/displays", func(w http.ResponseWriter, r *http.Request) {
			request(w, r, http.StatusCreated, func(ctx context.Context, in DisplayInput) (Display, error) {
				return s.reserveDisplay(ctx, chi.URLParam(r, "id"), in)
			})
		})
		r.Get("/environment-displays/{id}/manifest", s.displayManifest)
		r.Get("/environment-displays/{id}/content", s.displayContent)
		r.Put("/environment-displays/{id}/content", s.uploadDisplay)
		for _, action := range []string{"verify", "release", "withdraw"} {
			r.Post("/environment-releases/{id}/"+action, func(w http.ResponseWriter, r *http.Request) {
				request(w, r, 200, func(ctx context.Context, in ReleaseAction) (EnvironmentRelease, error) {
					return s.transition(ctx, chi.URLParam(r, "id"), action, in)
				})
			})
		}
	})
}

func (s Service) listDefinitions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := require(ctx, Read, false); err != nil {
		httpx.Error(w, err)
		return
	}
	out := page[Definition]{Items: []Definition{}}
	rows, err := db.Scoped(s.Pool).Query(ctx, `SELECT document FROM (
		SELECT DISTINCT ON(id) id,profile_id,document FROM geographic_environment_definitions
		WHERE id>$1 ORDER BY id,revision DESC
	) latest WHERE ($2='' OR profile_id=$2) ORDER BY id LIMIT 51`, r.URL.Query().Get("after"), r.URL.Query().Get("profile"))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var d Definition
		var raw []byte
		if err = rows.Scan(&raw); err == nil {
			err = json.Unmarshal(raw, &d)
		}
		if err != nil {
			httpx.Error(w, err)
			return
		}
		if len(out.Items) == 50 {
			out.Next = out.Items[49].ID
			break
		}
		out.Items = append(out.Items, d)
	}
	respond(w, 200, out, rows.Err())
}
func (s Service) listAssets(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := require(ctx, Read, false); err != nil {
		httpx.Error(w, err)
		return
	}
	q := r.URL.Query()
	source := q.Get("source")
	if source != "" {
		if err := access.RequireSource(ctx, source); err != nil {
			httpx.Error(w, err)
			return
		}
	}
	out, err := s.visibleAssets(ctx, q.Get("after"), q.Get("layer"), source, 50)
	respond(w, 200, out, err)
}

func (s Service) visibleAssets(ctx context.Context, after, layer, source string, limit int) (page[Asset], error) {
	out := page[Asset]{Items: []Asset{}}
	for len(out.Items) <= limit {
		rows, err := db.Scoped(s.Pool).Query(ctx, assetSelect()+`WHERE id>$1 AND state='READY' AND retain_until>clock_timestamp() AND document#>>'{representation,format}' NOT IN ('release-zip','display-glb','display-layer-json','display-pmtiles') AND ($2='' OR document#>>'{representation,layer}'=$2) AND ($3='' OR source_id=$3) ORDER BY id LIMIT 100`, after, layer, source)
		if err != nil {
			return out, err
		}
		batch := []Asset{}
		for rows.Next() {
			a, e := scanAsset(rows)
			if e != nil {
				rows.Close()
				return out, e
			}
			batch = append(batch, a)
		}
		rows.Close()
		if err = rows.Err(); err != nil {
			return out, err
		}
		for _, a := range batch {
			after = a.ID
			if _, err := s.policy(ctx, a, false, false); err == nil {
				out.Items = append(out.Items, a)
			}
			if len(out.Items) > limit {
				break
			}
		}
		if len(batch) < 100 {
			break
		}
	}
	if len(out.Items) > limit {
		out.Items = out.Items[:limit]
		out.Next = out.Items[limit-1].ID
	}
	return out, nil
}

type attemptEvent struct {
	Attempt int       `json:"attempt"`
	State   string    `json:"state"`
	Code    string    `json:"code"`
	At      time.Time `json:"at"`
}

func (s Service) attempts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := require(ctx, Read, false); err != nil {
		httpx.Error(w, err)
		return
	}
	id := chi.URLParam(r, "id")
	if _, err := s.visibleJob(ctx, id); err != nil {
		httpx.Error(w, err)
		return
	}
	rows, err := db.Scoped(s.Pool).Query(ctx, `SELECT attempt,state,code,occurred_at FROM geographic_environment_build_events WHERE build_id=$1 ORDER BY occurred_at,id`, id)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	defer rows.Close()
	out := []attemptEvent{}
	for rows.Next() {
		var v attemptEvent
		if err = rows.Scan(&v.Attempt, &v.State, &v.Code, &v.At); err != nil {
			httpx.Error(w, err)
			return
		}
		out = append(out, v)
	}
	respond(w, 200, out, rows.Err())
}

func (s Service) visibleRelease(ctx context.Context, r EnvironmentRelease) error {
	if r.State != "RELEASED" && !access.Allows(ctx, Qualify) && !access.Allows(ctx, Release) {
		return apperr.NotFound("environment release", "unavailable")
	}
	return s.eligibleRelease(ctx, r)
}
func (s Service) listReleases(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := require(ctx, Read, false); err != nil {
		httpx.Error(w, err)
		return
	}
	out := page[EnvironmentRelease]{Items: []EnvironmentRelease{}}
	after := r.URL.Query().Get("after")
	source := r.URL.Query().Get("source")
	if source != "" {
		if err := access.RequireSource(ctx, source); err != nil {
			httpx.Error(w, err)
			return
		}
	}
	for len(out.Items) < 21 {
		rows, err := db.Scoped(s.Pool).Query(ctx, releaseSelect+`WHERE r.id>$1 AND ($2='' OR r.build_id=$2) AND ($3::text='' OR r.required_sources @> jsonb_build_array($3::text)) ORDER BY r.id LIMIT 21`, after, r.URL.Query().Get("build"), source)
		if err != nil {
			httpx.Error(w, err)
			return
		}
		batch := []EnvironmentRelease{}
		for rows.Next() {
			rel, e := scanRelease(rows)
			if e != nil {
				rows.Close()
				httpx.Error(w, e)
				return
			}
			batch = append(batch, rel)
		}
		rows.Close()
		if err = rows.Err(); err != nil {
			httpx.Error(w, err)
			return
		}
		for _, rel := range batch {
			after = rel.ID
			if err := s.visibleRelease(ctx, rel); err == nil {
				rel.Manifest = nil // list metadata stays bounded
				out.Items = append(out.Items, rel)
			}
			if len(out.Items) == 21 {
				break
			}
		}
		if len(batch) < 21 {
			break
		}
	}
	if len(out.Items) == 21 {
		out.Items = out.Items[:20]
		out.Next = out.Items[19].ID
	}
	respond(w, 200, out, nil)
}
func (s Service) getVisible(r *http.Request, perm string) (EnvironmentRelease, error) {
	ctx := r.Context()
	if err := require(ctx, perm, false); err != nil {
		return EnvironmentRelease{}, err
	}
	rel, err := s.release(ctx, chi.URLParam(r, "id"))
	if err == nil {
		err = s.visibleRelease(ctx, rel)
	}
	return rel, err
}
func (s Service) releaseDetail(w http.ResponseWriter, r *http.Request) {
	rel, err := s.getVisible(r, Read)
	respond(w, 200, rel, err)
}
func (s Service) releaseManifest(w http.ResponseWriter, r *http.Request) {
	rel, err := s.getVisible(r, Read)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("ETag", `"`+rel.ManifestSHA256+`"`)
	_, _ = w.Write(rel.Manifest)
}
func (s Service) releaseQuality(w http.ResponseWriter, r *http.Request) {
	rel, err := s.getVisible(r, Read)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	var m struct {
		Quality json.RawMessage `json:"quality"`
	}
	err = json.Unmarshal(rel.Manifest, &m)
	respond(w, 200, m.Quality, err)
}
func (s Service) stac(w http.ResponseWriter, r *http.Request) {
	rel, err := s.getVisible(r, Read)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	if rel.State != "RELEASED" {
		httpx.Error(w, apperr.NotFound("released environment", "unavailable"))
		return
	}
	var m Manifest
	if err = json.Unmarshal(rel.Manifest, &m); err != nil {
		httpx.Error(w, err)
		return
	}
	bounds, err := s.areaBounds(r.Context(), m.Area)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	base := "/api/v1/environment-releases/" + rel.ID
	assets := map[string]any{"prepared": map[string]any{"href": base + "/prepared", "type": "application/zip", "roles": []string{"data"}}, "manifest": map[string]any{"href": base + "/manifest", "type": "application/json", "roles": []string{"metadata"}}}
	displays, err := s.readyDisplays(r.Context(), rel.ID, "")
	if err != nil {
		httpx.Error(w, err)
		return
	}
	for _, display := range displays.Items {
		assets[display.ID] = map[string]any{
			"href": "/api/v1/environment-displays/" + display.ID + "/content",
			"type": display.MediaType, "roles": []string{"overview"}, "title": "Visual reference " + displayFormat(display.Recipe).layer,
		}
	}
	respond(w, 200, map[string]any{"type": "Feature", "stac_version": "1.1.0", "id": rel.ID, "geometry": m.Area, "bbox": bounds,
		"properties": map[string]any{"datetime": rel.CreatedAt, "emad:datetimeMeaning": "Package creation time; see manifest for per-source data times", "emad:profile": m.ProfileID},
		"links":      []map[string]string{{"rel": "self", "href": base + "/stac"}},
		"assets":     assets}, nil)
}

func (s Service) areaBounds(ctx context.Context, area json.RawMessage) ([4]float64, error) {
	var bounds [4]float64
	err := db.Scoped(s.Pool).QueryRow(ctx, `SELECT ST_XMin(g),ST_YMin(g),ST_XMax(g),ST_YMax(g) FROM (SELECT ST_GeomFromGeoJSON($1) g) x`, string(area)).Scan(&bounds[0], &bounds[1], &bounds[2], &bounds[3])
	return bounds, err
}

func parseRange(value string, size int64) (*artifacts.ByteRange, error) {
	if value == "" {
		return nil, nil
	}
	bad := fmt.Errorf("only one satisfiable byte range is supported")
	rest, ok := strings.CutPrefix(value, "bytes=")
	if !ok || strings.Contains(rest, ",") {
		return nil, bad
	}
	left, right, ok := strings.Cut(rest, "-")
	if !ok {
		return nil, bad
	}
	start, end := int64(0), size-1
	var err error
	if left == "" {
		length, err := strconv.ParseInt(right, 10, 64)
		if err != nil || length <= 0 {
			return nil, bad
		}
		start = max(0, size-length)
	} else {
		start, err = strconv.ParseInt(left, 10, 64)
		if err != nil || start < 0 || start >= size {
			return nil, bad
		}
		if right != "" {
			end, err = strconv.ParseInt(right, 10, 64)
			if err != nil || end < start {
				return nil, bad
			}
			end = min(size-1, end)
		}
	}
	return &artifacts.ByteRange{Start: start, End: end}, nil
}
func (s Service) stream(w http.ResponseWriter, r *http.Request, a Asset, filename string) {
	if s.Store == nil || a.State != "READY" {
		httpx.Error(w, apperr.Conflict("geographic object unavailable"))
		return
	}
	tag := `"` + a.SHA256 + `"`
	requestedRange := r.Header.Get("Range")
	if validator := r.Header.Get("If-Range"); validator != "" && validator != tag {
		requestedRange = ""
	}
	part, err := parseRange(requestedRange, a.Bytes)
	if err != nil {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", a.Bytes))
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return
	}
	in, err := s.Store.OpenGeographic(r.Context(), a.objectKey, a.Bytes, part)
	if err != nil {
		httpx.Error(w, apperr.Conflict("geographic object unavailable"))
		return
	}
	defer in.Close()
	w.Header().Set("Content-Type", a.MediaType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("ETag", tag)
	w.Header().Set("Accept-Ranges", "bytes")
	length, status := a.Bytes, 200
	if part != nil {
		length, status = part.End-part.Start+1, 206
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", part.Start, part.End, a.Bytes))
	}
	w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
	w.WriteHeader(status)
	if _, err := io.CopyN(w, in, length); err != nil {
		panic(http.ErrAbortHandler)
	}
}
func (s Service) assetContent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := require(ctx, Download, false); err != nil {
		httpx.Error(w, err)
		return
	}
	a, err := s.asset(ctx, chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	if !originalFormat(a.Representation.Format) {
		httpx.Error(w, apperr.NotFound("source object", "unavailable"))
		return
	}
	if _, err = s.policy(ctx, a, false, true); err != nil {
		httpx.Error(w, err)
		return
	}
	s.stream(w, r, a, a.ID)
}
func (s Service) prepared(w http.ResponseWriter, r *http.Request) {
	rel, err := s.getVisible(r, Download)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	if rel.State != "RELEASED" && rel.State != "STAGED" && rel.State != "VERIFIED" {
		httpx.Error(w, apperr.Conflict("release is no longer available for download"))
		return
	}
	a, err := s.asset(r.Context(), rel.BundleID)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	s.stream(w, r, a, rel.ID+".zip")
}
func (s Service) rebuild(w http.ResponseWriter, r *http.Request) {
	rel, err := s.getVisible(r, Download)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	if rel.State != "RELEASED" {
		httpx.Error(w, apperr.Conflict("rebuild bundle requires a released environment"))
		return
	}
	ctx := r.Context()
	if s.Store == nil {
		httpx.Error(w, apperr.Conflict("geographic object storage unavailable"))
		return
	}
	j, err := s.job(ctx, rel.BuildID)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	if err = s.checkInputs(ctx, j.recipe, true); err != nil {
		httpx.Error(w, err)
		return
	}
	recipe, err := json.Marshal(struct {
		Recipe
		InputSHA256 string `json:"inputSha256"`
	}{j.recipe, j.InputSHA256})
	if err != nil {
		httpx.Error(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+rel.ID+`-rebuild.zip"`)
	if err := s.writeRebuild(ctx, w, recipe, rel.Manifest, j.recipe.Inputs); err != nil {
		// A closed HTTP stream tells the client that the ZIP is incomplete.
		panic(http.ErrAbortHandler)
	}
}

func (s Service) writeRebuild(ctx context.Context, dst io.Writer, recipe, manifest []byte, inputs []Asset) error {
	z := zip.NewWriter(dst)
	for _, entry := range []struct {
		name string
		raw  []byte
	}{{"recipe.json", recipe}, {"manifest.json", manifest}} {
		out, err := z.Create(entry.name)
		if err != nil {
			return err
		}
		if _, err = out.Write(entry.raw); err != nil {
			return err
		}
	}
	written := map[string]bool{}
	for _, input := range inputs {
		if written[input.SHA256] {
			continue
		}
		fresh, err := access.Refresh(ctx)
		if err != nil {
			return err
		}
		if err = require(fresh, Download, false); err != nil {
			return err
		}
		a, err := s.asset(fresh, input.ID)
		if err != nil {
			return err
		}
		if a.State != "READY" || a.SHA256 != input.SHA256 || a.AdmissionID != input.AdmissionID {
			return apperr.Conflict("frozen input bytes are unavailable")
		}
		if _, err = s.policy(fresh, a, true, true); err != nil {
			return err
		}
		in, err := s.Store.OpenGeographic(ctx, a.objectKey, a.Bytes, nil)
		if err != nil {
			return err
		}
		out, err := z.CreateHeader(&zip.FileHeader{Name: "inputs/" + a.SHA256, Method: zip.Store})
		if err != nil {
			in.Close()
			return err
		}
		h := sha256.New()
		n, err := io.Copy(io.MultiWriter(out, h), io.LimitReader(in, a.Bytes+1))
		in.Close()
		if err != nil {
			return err
		}
		if n != a.Bytes || hex.EncodeToString(h.Sum(nil)) != a.SHA256 {
			return apperr.Conflict("frozen input failed integrity verification")
		}
		written[a.SHA256] = true
	}
	return z.Close()
}
