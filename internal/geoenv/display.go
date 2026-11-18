package geoenv

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

const displayMedia = "model/gltf-binary"
const maxDisplayBytes int64 = 32 << 20

type DisplayDependency struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
}

type DisplayInput struct {
	RequestKey      string              `json:"requestKey"`
	ManifestSHA256  string              `json:"manifestSha256"`
	ToolchainSHA256 string              `json:"toolchainSha256"`
	Recipe          string              `json:"recipe"`
	Inputs          []DisplayDependency `json:"inputs"`
	SHA256          string              `json:"sha256"`
	Bytes           int64               `json:"bytes"`
}

type Display struct {
	DisplayInput
	ID               string          `json:"id"`
	SchemaVersion    int             `json:"schemaVersion"`
	ReleaseID        string          `json:"releaseId"`
	AreaRevisionID   string          `json:"areaRevisionId"`
	State            string          `json:"state"`
	Frame            Frame           `json:"frame"`
	Bounds           [4]float64      `json:"bounds"`
	PhysicalAccuracy string          `json:"physicalAccuracy"`
	AxisOrder        [3]string       `json:"axisOrder"`
	VerticalDatum    string          `json:"verticalDatum"`
	MediaType        string          `json:"mediaType"`
	RetainUntil      time.Time       `json:"retainUntil"`
	Purpose          *DisplayPurpose `json:"purpose,omitempty"`
	assetID, owner   string
}

const displaySelect = `SELECT d.document,d.asset_id,d.owner_id,a.state FROM geographic_environment_displays d
	JOIN geographic_assets a ON (a.id,a.scope_id)=(d.asset_id,d.scope_id) `

func scanDisplay(row pgx.Row) (Display, error) {
	var d Display
	var raw []byte
	var state string
	err := row.Scan(&raw, &d.assetID, &d.owner, &state)
	if errors.Is(err, pgx.ErrNoRows) {
		return d, apperr.NotFound("environment display", "unavailable")
	}
	if err == nil {
		err = json.Unmarshal(raw, &d)
	}
	d.State = state
	return d, err
}

func (s Service) displayParent(ctx context.Context, tx pgx.Tx, id string) (EnvironmentRelease, error) {
	return s.displayParentFor(ctx, tx, id, access.Allows(ctx, Qualify))
}

func (s Service) displayParentFor(ctx context.Context, tx pgx.Tx, id string, staged bool) (EnvironmentRelease, error) {
	if tx != nil {
		resource, _ := access.ResourceFrom(ctx)
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, resource.ID()+":georelease:"+id); err != nil {
			return EnvironmentRelease{}, err
		}
	}
	rel, err := s.release(ctx, id)
	if err != nil {
		return rel, err
	}
	if rel.State != "RELEASED" && !(staged && (rel.State == "STAGED" || rel.State == "VERIFIED")) {
		return rel, apperr.NotFound("environment display", "unavailable")
	}
	return rel, s.eligibleReleaseIn(ctx, tx, rel)
}

func (s Service) reserveDisplay(ctx context.Context, id string, in DisplayInput) (Display, error) {
	if strings.HasPrefix(in.RequestKey, "display-build:") {
		return Display{}, apperr.Validation("request key is reserved for durable display builds")
	}
	d, err := s.reserveDisplayOutput(ctx, id, in)
	return withDisplayPurpose(d), err
}

func (s Service) reserveDisplayOutput(ctx context.Context, id string, in DisplayInput) (Display, error) {
	var d Display
	if err := require(ctx, Qualify, false); err != nil {
		return d, err
	}
	if !keyPattern.MatchString(in.RequestKey) || !digestPattern.MatchString(in.ManifestSHA256) ||
		!digestPattern.MatchString(in.ToolchainSHA256) || !digestPattern.MatchString(in.SHA256) ||
		in.Bytes < 1 || in.Bytes > maxDisplayBytes || displayFormat(in.Recipe).media == "" || len(in.Inputs) > 128 {
		return d, apperr.Validation("bounded display reservation required")
	}
	if s.Store == nil {
		return d, apperr.Conflict("geographic object storage unavailable")
	}
	p, resource, _ := access.From(ctx)
	displayID := displayIdentity(resource.ID(), id, p.ID, in.RequestKey)
	err := db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		ctx := db.UseTx(ctx, tx)
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, resource.ID()+":geoasset:"+p.ID); err != nil {
			return err
		}
		rel, err := s.displayParent(ctx, tx, id)
		if err != nil {
			return err
		}
		if rel.ManifestSHA256 != in.ManifestSHA256 {
			return apperr.Conflict("display inputs do not match the parent release digest")
		}
		var m Manifest
		if err := json.Unmarshal(rel.Manifest, &m); err != nil {
			return err
		}
		expected, available := displayDependencies(m, in.Recipe)
		if !available || !slices.Equal(expected, in.Inputs) {
			return apperr.Conflict("display must name every exact recipe member in manifest order")
		}
		if prior, err := scanDisplay(tx.QueryRow(ctx, displaySelect+`WHERE d.id=$1`, displayID)); err == nil {
			old, _ := json.Marshal(prior.DisplayInput)
			next, _ := json.Marshal(in)
			if !bytes.Equal(old, next) || prior.State == "EXPIRED" || !prior.RetainUntil.After(time.Now()) {
				return apperr.Conflict("display reservation changed or expired")
			}
			d = prior
			return nil
		} else if !apperr.Is(err, apperr.CodeNotFound) {
			return err
		}
		bundle, err := s.asset(ctx, rel.BundleID)
		if err != nil {
			return err
		}
		format := displayFormat(in.Recipe)
		d = Display{DisplayInput: in, ID: displayID, SchemaVersion: 1, ReleaseID: id, AreaRevisionID: m.AreaRevisionID,
			Frame: m.Frame, PhysicalAccuracy: m.Quality.PhysicalAccuracy, MediaType: format.media, State: "PENDING",
			AxisOrder: [3]string{"east", "up", "south"}, VerticalDatum: "WGS84 ellipsoid",
			RetainUntil: bundle.RetainUntil, owner: p.ID, assetID: "geoasset_" + artifacts.Hash([]byte(displayID+"\x00"+in.SHA256))}
		for _, parent := range m.Inputs {
			policy, err := s.policyIn(ctx, tx, parent, true, false)
			if err != nil {
				return err
			}
			if expires := policy.Rights.ExpiresAt; expires != nil && expires.Before(d.RetainUntil) {
				d.RetainUntil = *expires
			}
		}
		d.Bounds, err = s.areaBounds(ctx, m.Area)
		if err != nil {
			return err
		}
		a := Asset{ID: d.assetID, State: d.State, MediaType: format.media, owner: p.ID, AssetInput: AssetInput{
			RequestKey: in.RequestKey, SourceID: bundle.SourceID, AdmissionID: bundle.AdmissionID,
			SHA256: in.SHA256, Bytes: in.Bytes, RetainUntil: d.RetainUntil,
			Representation: Representation{Format: format.format, Layer: format.layer, Footprint: m.Area, Semantics: "Derived geographic reference display"},
		}}
		a.objectKey = objectKey(resource.ID(), a)
		if err := insertAsset(ctx, tx, a, rel.sources); err != nil {
			return err
		}
		raw, err := json.Marshal(d)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO geographic_environment_displays(id,release_id,asset_id,owner_id,request_key,required_sources,document)
			VALUES($1,$2,$3,$4,$5,$6,$7)`, d.ID, id, d.assetID, d.owner, in.RequestKey, rel.sources, raw)
		return err
	})
	return d, err
}

func displayIdentity(scope, release, owner, key string) string {
	return "geodisplay_" + artifacts.Hash([]byte(scope+"\x00"+release+"\x00"+owner+"\x00"+key))
}

func terrainDependencies(m Manifest) ([]DisplayDependency, bool) {
	expected := []DisplayDependency{}
	surface := false
	for _, part := range m.Members {
		if part.Layer == "surface" || part.Layer == "geoid" {
			expected = append(expected, DisplayDependency{part.Name, part.SHA256})
			surface = surface || part.Layer == "surface"
		}
	}
	return expected, surface
}

func (s Service) readyDisplays(ctx context.Context, release, after string) (page[Display], error) {
	out := page[Display]{Items: []Display{}}
	rows, err := db.Scoped(s.Pool).Query(ctx, displaySelect+
		`WHERE d.release_id=$1 AND d.id>$2 AND a.state='READY' AND a.retain_until>clock_timestamp() ORDER BY d.id LIMIT 21`, release, after)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		d, err := scanDisplay(rows)
		if err != nil {
			return out, err
		}
		if len(out.Items) == 20 {
			out.Next = out.Items[19].ID
			break
		}
		out.Items = append(out.Items, d)
	}
	attachDisplayPurpose(out.Items)
	return out, rows.Err()
}

func (s Service) listDisplays(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := require(ctx, Read, false); err != nil {
		httpx.Error(w, err)
		return
	}
	rel, err := s.displayParent(ctx, nil, chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	out, err := s.readyDisplays(ctx, rel.ID, r.URL.Query().Get("after"))
	respond(w, http.StatusOK, out, err)
}

func (s Service) getDisplay(r *http.Request, permission string) (Display, error) {
	ctx := r.Context()
	if err := require(ctx, permission, false); err != nil {
		return Display{}, err
	}
	d, err := scanDisplay(db.Scoped(s.Pool).QueryRow(ctx, displaySelect+`WHERE d.id=$1`, chi.URLParam(r, "id")))
	if err != nil {
		return d, err
	}
	if d.State != "READY" || !d.RetainUntil.After(time.Now()) {
		return d, apperr.NotFound("environment display", "unavailable")
	}
	_, err = s.displayParent(ctx, nil, d.ReleaseID)
	return withDisplayPurpose(d), err
}

func (s Service) displayManifest(w http.ResponseWriter, r *http.Request) {
	d, err := s.getDisplay(r, Read)
	respond(w, http.StatusOK, d, err)
}

func (s Service) displayContent(w http.ResponseWriter, r *http.Request) {
	d, err := s.getDisplay(r, Download)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	a, err := s.asset(r.Context(), d.assetID)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	s.stream(w, r, a, d.ID+displayFormat(d.Recipe).extension)
}

func (s Service) uploadDisplay(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), attemptTimeout)
	defer cancel()
	if err := require(ctx, Qualify, false); err != nil {
		httpx.Error(w, err)
		return
	}
	d, err := scanDisplay(db.Scoped(s.Pool).QueryRow(ctx, displaySelect+`WHERE d.id=$1`, chi.URLParam(r, "id")))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	p, _, _ := access.From(ctx)
	if d.owner != p.ID || strings.HasPrefix(d.RequestKey, "display-build:") {
		httpx.Error(w, apperr.NotFound("environment display", "unavailable"))
		return
	}
	parent, err := s.displayParent(ctx, nil, d.ReleaseID)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	if s.Store == nil || d.State == "EXPIRED" || !d.RetainUntil.After(time.Now()) || r.ContentLength != d.Bytes {
		httpx.Error(w, apperr.Conflict("display upload unavailable or length differs from reservation"))
		return
	}
	if s.UploadSlots != nil {
		select {
		case s.UploadSlots <- struct{}{}:
			defer func() { <-s.UploadSlots }()
		default:
			httpx.Error(w, apperr.Conflict("geographic uploads are at capacity"))
			return
		}
	}
	controller := http.NewResponseController(w)
	if controller.SetReadDeadline(time.Now().Add(2*time.Minute)) == nil {
		defer controller.SetReadDeadline(time.Time{})
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, d.Bytes))
	var manifest Manifest
	if err != nil || int64(len(raw)) != d.Bytes || artifacts.Hash(raw) != d.SHA256 ||
		json.Unmarshal(parent.Manifest, &manifest) != nil || !validDisplayBytes(raw, d, manifest) {
		httpx.Error(w, apperr.Validation("upload must match the reserved display and its recipe"))
		return
	}
	if d.State == "READY" {
		respond(w, http.StatusOK, d, nil)
		return
	}
	a, err := s.asset(ctx, d.assetID)
	if err == nil {
		err = s.publishDisplay(ctx, d, a, raw)
	}
	d.State = "READY"
	respond(w, http.StatusOK, withDisplayPurpose(d), err)
}

func (s Service) publishDisplay(ctx context.Context, d Display, a Asset, raw []byte) error {
	// Guard the reservation against cleanup for the duration of object I/O.
	unlock, err := s.markUpload(ctx, a.ID)
	if err != nil {
		return err
	}
	defer unlock()
	if err := s.Store.PutGeographic(ctx, a.objectKey, bytes.NewReader(raw), a.Bytes, a.SHA256, a.MediaType); err != nil {
		return apperr.Conflict("display upload could not be verified")
	}
	ctx, err = access.Refresh(ctx)
	if err != nil {
		return err
	}
	if err := require(ctx, Qualify, false); err != nil {
		return err
	}
	return db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		ctx := db.UseTx(ctx, tx)
		if _, err := s.displayParent(ctx, tx, d.ReleaseID); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `UPDATE geographic_assets SET state='READY',upload_until=NULL WHERE id=$1 AND state='PENDING' AND retain_until>clock_timestamp()`, a.ID)
		if err == nil && tag.RowsAffected() != 1 {
			return apperr.Conflict("display reservation expired")
		}
		return err
	})
}
