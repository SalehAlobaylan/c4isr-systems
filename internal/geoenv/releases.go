package geoenv

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/environmentprep"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
	"github.com/jackc/pgx/v5"
)

type Member struct {
	Name    string `json:"name"`
	Layer   string `json:"layer"`
	SHA256  string `json:"sha256"`
	Bytes   int64  `json:"bytes"`
	InputID string `json:"inputId,omitempty"`
}
type Manifest struct {
	Schema         string          `json:"schema"`
	InputSHA256    string          `json:"inputSha256"`
	Toolchain      string          `json:"toolchainSha256"`
	ProfileID      string          `json:"profileId"`
	AreaRevisionID string          `json:"areaRevisionId"`
	Area           json.RawMessage `json:"area"`
	Inputs         []Asset         `json:"inputs"`
	Members        []Member        `json:"members"`
	Frame          Frame           `json:"frame"`
	Quality        struct {
		CoordinateErrorM float64         `json:"coordinateErrorM"`
		PhysicalAccuracy string          `json:"physicalAccuracy"`
		Coastal          json.RawMessage `json:"coastal"`
		Observations     struct {
			Radar   json.RawMessage `json:"radar"`
			Weather json.RawMessage `json:"weather"`
		} `json:"observations"`
		Enrichment struct {
			GroundComparison  json.RawMessage `json:"groundComparison"`
			DimensionCoverage json.RawMessage `json:"dimensionCoverage"`
		} `json:"enrichment"`
	} `json:"quality"`
}
type Frame struct {
	Axes        []string       `json:"axes"`
	Origin      []float64      `json:"origin"`
	HeightDatum string         `json:"heightDatum"`
	AxisOrder   string         `json:"axisOrder"`
	Controls    []FrameControl `json:"controls"`
}
type FrameControl struct {
	Longitude        float64   `json:"longitude"`
	Latitude         float64   `json:"latitude"`
	EllipsoidHeightM float64   `json:"ellipsoidHeightM"`
	ENUM             []float64 `json:"enuM"`
}

func validFrame(f Frame) bool {
	if len(f.Origin) != 3 || len(f.Controls) < 3 || len(f.Controls) > 16 || !slices.Equal(f.Axes, []string{"east", "north", "up"}) || f.HeightDatum != "WGS84 ellipsoid" || f.AxisOrder != "longitude, latitude" {
		return false
	}
	if math.Abs(f.Origin[0]) > 180 || math.Abs(f.Origin[1]) > 85 || f.Origin[2] != 0 {
		return false
	}
	origin := environmentprep.Origin{Longitude: f.Origin[0], Latitude: f.Origin[1], Height: f.Origin[2]}
	for _, control := range f.Controls {
		if len(control.ENUM) != 3 || math.Abs(control.Longitude) > 180 || math.Abs(control.Latitude) > 85 || math.Abs(control.EllipsoidHeightM) > 10000 {
			return false
		}
		projected := environmentprep.ENU(origin, control.Longitude, control.Latitude, control.EllipsoidHeightM)
		var squared float64
		for i, value := range projected {
			delta := value - control.ENUM[i]
			squared += delta * delta
		}
		if !(math.Sqrt(squared) <= .001) {
			return false
		}
	}
	return true
}

type EnvironmentRelease struct {
	ID             string          `json:"id"`
	BuildID        string          `json:"buildId"`
	Name           string          `json:"name"`
	State          string          `json:"state"`
	ManifestSHA256 string          `json:"manifestSha256"`
	Manifest       json.RawMessage `json:"manifest,omitempty"`
	BundleID       string          `json:"bundleId"`
	CreatedAt      time.Time       `json:"createdAt"`
	sources        []string
}

var memberName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,100}\.(tif|gpkg|json)$`)
var errPublicationPolicy = errors.New("current source policy prevents publication")

func validateManifest(raw []byte, j Job) (Manifest, error) {
	var m Manifest
	if json.Unmarshal(raw, &m) != nil || m.Schema != "emad.environment-release.v1" || m.InputSHA256 != j.InputSHA256 || m.Toolchain != j.recipe.Toolchain || m.ProfileID != j.recipe.Profile.ID || m.AreaRevisionID != j.recipe.Area.RevisionID || !jsonEqual(m.Area, j.recipe.Area.Geometry) ||
		len(m.Inputs) != len(j.recipe.Inputs) || len(m.Members) < 1 || len(m.Members) > 1023 || m.Quality.CoordinateErrorM < 0 || m.Quality.CoordinateErrorM > 0.001 || m.Quality.PhysicalAccuracy != "UNKNOWN" {
		return m, ErrInvalidInputs
	}
	if !validFrame(m.Frame) {
		return m, ErrInvalidInputs
	}
	inputLayers := make(map[string]string, len(m.Inputs))
	for i, a := range m.Inputs {
		f := j.recipe.Inputs[i]
		x, _ := json.Marshal(a)
		y, _ := json.Marshal(f)
		if !jsonEqual(x, y) {
			return m, ErrInvalidInputs
		}
		inputLayers[a.ID] = a.Representation.Layer
	}
	seen := map[string]bool{}
	layers := map[string]bool{}
	enriched := m.ProfileID == enrichmentProfile
	observation := m.ProfileID == observationProfile
	var total int64
	for _, part := range m.Members {
		if !memberName.MatchString(part.Name) || part.Name == "manifest.json" || seen[part.Name] || !digestPattern.MatchString(part.SHA256) || part.Bytes < 1 || part.Bytes > artifacts.MaxGeographicBytes {
			return m, ErrInvalidInputs
		}
		if (enriched || observation || m.ProfileID == coastalProfile) && (part.InputID == "" || !slices.Contains(j.recipe.Profile.Required, part.Layer) && !slices.Contains(j.recipe.Profile.Optional, part.Layer)) {
			return m, ErrInvalidInputs
		}
		if part.InputID != "" {
			if layer, ok := inputLayers[part.InputID]; !ok || layer != part.Layer {
				return m, ErrInvalidInputs
			}
		}
		seen[part.Name] = true
		layers[part.Layer] = true
		total += part.Bytes
	}
	if m.ProfileID == coastalProfile && (len(m.Members) != 1 || len(m.Quality.Coastal) == 0 || jsonEqual(m.Quality.Coastal, json.RawMessage("null"))) {
		return m, ErrInvalidInputs
	}
	if observation {
		for _, candidate := range []struct {
			layer   string
			quality json.RawMessage
		}{{"radar", m.Quality.Observations.Radar}, {"weather-field", m.Quality.Observations.Weather}} {
			input := slices.ContainsFunc(j.recipe.Inputs, func(a Asset) bool { return a.Representation.Layer == candidate.layer })
			if input != layers[candidate.layer] || input != (len(candidate.quality) > 0 && !jsonEqual(candidate.quality, json.RawMessage("null"))) {
				return m, ErrInvalidInputs
			}
		}
		if !layers["radar"] && !layers["weather-field"] {
			return m, ErrInvalidInputs
		}
	}
	if total > 4<<30 {
		return m, ErrInvalidInputs
	}
	for _, layer := range j.recipe.Profile.Required {
		if !layers[layer] {
			return m, ErrInvalidInputs
		}
	}
	if enriched {
		for _, layer := range j.recipe.Profile.Optional {
			if slices.ContainsFunc(j.recipe.Inputs, func(a Asset) bool { return a.Representation.Layer == layer }) && !layers[layer] {
				return m, ErrInvalidInputs
			}
		}
		var comparisons []json.RawMessage
		if len(m.Quality.Enrichment.DimensionCoverage) == 0 ||
			json.Unmarshal(m.Quality.Enrichment.GroundComparison, &comparisons) != nil || comparisons == nil ||
			layers["ground"] != (len(comparisons) > 0) {
			return m, ErrInvalidInputs
		}
	}
	return m, nil
}

func readSmall(path string, maxBytes int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err == nil && int64(len(raw)) > maxBytes {
		return nil, ErrInvalidInputs
	}
	return raw, err
}

func zipIntegrity(file *os.File, size int64, raw []byte, m Manifest) error {
	z, err := zip.NewReader(file, size)
	if err != nil || len(z.File) != len(m.Members)+1 {
		return ErrInvalidInputs
	}
	expected := map[string]Member{"manifest.json": {Name: "manifest.json", SHA256: artifacts.Hash(raw), Bytes: int64(len(raw))}}
	for _, v := range m.Members {
		expected[v.Name] = v
	}
	for _, f := range z.File {
		v, ok := expected[f.Name]
		if !ok || !f.Mode().IsRegular() || f.UncompressedSize64 != uint64(v.Bytes) {
			return ErrInvalidInputs
		}
		delete(expected, f.Name)
		r, err := f.Open()
		if err != nil {
			return ErrInvalidInputs
		}
		h := sha256.New()
		n, err := io.Copy(h, io.LimitReader(r, v.Bytes+1))
		r.Close()
		if err != nil || n != v.Bytes || hex.EncodeToString(h.Sum(nil)) != v.SHA256 {
			return ErrInvalidInputs
		}
	}
	return nil
}

func (s Service) stage(ctx context.Context, j Job, dir string) error {
	raw, err := readSmall(filepath.Join(dir, "manifest.json"), 8<<20)
	if err != nil {
		return err
	}
	f, err := os.Open(filepath.Join(dir, "prepared.zip"))
	if err != nil {
		return err
	}
	defer f.Close()
	return s.stageArchive(ctx, j, f, raw)
}

func (s Service) stageArchive(ctx context.Context, j Job, f *os.File, raw []byte) error {
	m, err := validateManifest(raw, j)
	if err != nil {
		return err
	}
	stat, err := f.Stat()
	if err != nil {
		return err
	}
	if !stat.Mode().IsRegular() || stat.Size() < 1 || stat.Size() > artifacts.MaxGeographicBytes {
		return ErrInvalidInputs
	}
	if err = zipIntegrity(f, stat.Size(), raw, m); err != nil {
		return err
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return err
	}
	digest := hex.EncodeToString(h.Sum(nil))
	p, r, _ := access.From(ctx)
	if p.ID != j.worker {
		return apperr.Forbidden("worker lease identity changed")
	}
	first := j.recipe.Inputs[0]
	a := Asset{ID: "geoasset_" + artifacts.Hash([]byte(r.ID()+"\x00"+j.ID+"\x00"+digest)), State: "PENDING", MediaType: "application/zip", owner: p.ID, AssetInput: AssetInput{
		RequestKey: j.ID, SourceID: first.SourceID, AdmissionID: first.AdmissionID, SHA256: digest, Bytes: stat.Size(), RetainUntil: first.RetainUntil,
		Representation: Representation{Format: "release-zip", Layer: "environment release", ProductEdition: m.ProfileID, Footprint: m.Area, Semantics: "Prepared reference data; see manifest and quality report"},
	}}
	for _, input := range j.recipe.Inputs {
		if input.RetainUntil.Before(a.RetainUntil) {
			a.RetainUntil = input.RetainUntil
		}
		if expiry := input.Rights.ExpiresAt; expiry != nil && expiry.Before(a.RetainUntil) {
			a.RetainUntil = *expiry
		}
	}
	a.objectKey = objectKey(r.ID(), a)
	if err = db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		if err := lockLease(ctx, tx, j); err != nil {
			return err
		}
		ctx := db.UseTx(ctx, tx)
		if err := s.checkInputsIn(ctx, tx, j.recipe, false); err != nil {
			return fmt.Errorf("%w: %v", errPublicationPolicy, err)
		}
		if prior, err := s.asset(ctx, a.ID); apperr.Is(err, apperr.CodeNotFound) {
			if err = insertAsset(ctx, tx, a, j.sources); err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else if prior.State != "PENDING" || !prior.RetainUntil.After(time.Now()) {
			return apperr.Conflict("prepared output reservation is unavailable")
		}
		// Each worker attempt uses a verified spool for the same immutable key.
		// A stale worker cannot publish even if its upload succeeds.
		tag, err := tx.Exec(ctx, `UPDATE geographic_assets SET upload_until=clock_timestamp()+interval '32 minutes' WHERE id=$1 AND state='PENDING' AND retain_until>clock_timestamp()`, a.ID)
		if err == nil && tag.RowsAffected() != 1 {
			return apperr.Conflict("prepared output reservation expired")
		}
		return err
	}); err != nil {
		return err
	}
	if err = s.Store.PutGeographic(ctx, a.objectKey, f, a.Bytes, a.SHA256, a.MediaType); err != nil {
		return err
	}
	ctx, err = access.Refresh(ctx)
	if err != nil {
		return err
	}
	if err = require(ctx, Execute, true); err != nil {
		return err
	}
	return db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		if err := lockLease(ctx, tx, j); err != nil {
			return err
		}
		ctx := db.UseTx(ctx, tx)
		human, err := s.actor(ctx, j)
		if err != nil {
			return err
		}
		if err = s.checkInputs(human, j.recipe, false); err != nil {
			return fmt.Errorf("%w: %v", errPublicationPolicy, err)
		}
		if err = s.checkInputsIn(ctx, tx, j.recipe, false); err != nil {
			return fmt.Errorf("%w: %v", errPublicationPolicy, err)
		}
		tag, err := tx.Exec(ctx, `UPDATE geographic_assets SET state='READY',upload_until=NULL WHERE id=$1 AND state='PENDING' AND retain_until>clock_timestamp()`, a.ID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return apperr.Conflict("prepared output reservation unavailable")
		}
		rel := EnvironmentRelease{ID: ids.New("georelease"), BuildID: j.ID, BundleID: a.ID, ManifestSHA256: artifacts.Hash(raw), sources: j.sources}
		if _, err = tx.Exec(ctx, `INSERT INTO geographic_environment_releases(id,build_id,required_sources,manifest_bytes,manifest_hash,bundle_id,publication_lease_sha256) VALUES($1,$2,$3,$4,$5,$6,$7)`, rel.ID, j.ID, rel.sources, raw, rel.ManifestSHA256, a.ID, leaseProof(j.worker, j.lease)); err != nil {
			return err
		}
		if err = releaseEvent(ctx, tx, rel, "STAGED", "", "Worker verified the exact prepared bundle bytes."); err != nil {
			return err
		}
		tag, err = tx.Exec(ctx, `UPDATE geographic_environment_builds SET state='SUCCEEDED',lease_id='',worker_id='',lease_until=NULL WHERE id=$1 AND lease_until>clock_timestamp()`, j.ID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return apperr.Conflict("build lease expired during publication")
		}
		return buildEvent(ctx, tx, j, "SUCCEEDED", "")
	})
}

func lockLease(ctx context.Context, tx pgx.Tx, j Job) error {
	var id string
	err := tx.QueryRow(ctx, `SELECT id FROM geographic_environment_builds WHERE id=$1 AND state='RUNNING' AND lease_id=$2 AND worker_id=$3 AND lease_until>clock_timestamp() FOR UPDATE`, j.ID, j.lease, j.worker).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.Conflict("build lease is no longer active")
	}
	return err
}

const releaseSelect = `SELECT r.id,r.build_id,d.document->>'name',r.required_sources,r.manifest_bytes,r.manifest_hash,r.bundle_id,r.created_at,
	(SELECT state FROM geographic_environment_release_events WHERE release_id=r.id AND scope_id=r.scope_id ORDER BY occurred_at DESC,id DESC LIMIT 1)
	FROM geographic_environment_releases r
	JOIN geographic_environment_builds b ON (b.id,b.scope_id)=(r.build_id,r.scope_id)
	JOIN geographic_environment_definitions d ON (d.id,d.scope_id,d.revision)=(b.definition_id,b.scope_id,b.definition_revision) `

func scanRelease(row pgx.Row) (EnvironmentRelease, error) {
	var r EnvironmentRelease
	err := row.Scan(&r.ID, &r.BuildID, &r.Name, &r.sources, &r.Manifest, &r.ManifestSHA256, &r.BundleID, &r.CreatedAt, &r.State)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, apperr.NotFound("environment release", "unavailable")
	}
	if err == nil && artifacts.Hash(r.Manifest) != r.ManifestSHA256 {
		return r, apperr.Conflict("release manifest failed integrity verification")
	}
	return r, err
}
func (s Service) release(ctx context.Context, id string) (EnvironmentRelease, error) {
	return scanRelease(db.Scoped(s.Pool).QueryRow(ctx, releaseSelect+`WHERE r.id=$1`, id))
}
func (s Service) eligibleRelease(ctx context.Context, r EnvironmentRelease) error {
	return s.eligibleReleaseIn(ctx, nil, r)
}
func (s Service) eligibleReleaseIn(ctx context.Context, tx pgx.Tx, r EnvironmentRelease) error {
	j, err := s.job(ctx, r.BuildID)
	if err != nil {
		return err
	}
	a, err := s.asset(ctx, r.BundleID)
	if err != nil {
		return err
	}
	if a.State != "READY" || !a.RetainUntil.After(time.Now()) {
		return apperr.Conflict("release bytes are unavailable")
	}
	return s.checkInputsIn(ctx, tx, j.recipe, false)
}

type ReleaseAction struct {
	ManifestSHA256 string `json:"manifestSha256"`
	Note           string `json:"note"`
}

type ReleaseDecision struct {
	State          string    `json:"state"`
	ManifestSHA256 string    `json:"manifestSha256"`
	HumanID        string    `json:"humanId,omitempty"`
	Note           string    `json:"note"`
	OccurredAt     time.Time `json:"occurredAt"`
}

func (s Service) releaseDecisions(ctx context.Context, id string) ([]ReleaseDecision, error) {
	if err := require(ctx, Read, false); err != nil {
		return nil, err
	}
	if !access.Allows(ctx, Qualify) && !access.Allows(ctx, Release) {
		return nil, apperr.Forbidden("release decision history requires review authority")
	}
	r, err := s.release(ctx, id)
	if err != nil {
		return nil, err
	}
	rows, err := db.Scoped(s.Pool).Query(ctx, `SELECT state,human_id,note,occurred_at FROM geographic_environment_release_events
		WHERE release_id=$1 ORDER BY occurred_at,id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	decisions := []ReleaseDecision{}
	for rows.Next() {
		decision := ReleaseDecision{ManifestSHA256: r.ManifestSHA256}
		if err := rows.Scan(&decision.State, &decision.HumanID, &decision.Note, &decision.OccurredAt); err != nil {
			return nil, err
		}
		decisions = append(decisions, decision)
	}
	return decisions, rows.Err()
}

func (s Service) checkPreparedBytes(ctx context.Context, r EnvironmentRelease) error {
	asset, err := s.asset(ctx, r.BundleID)
	if err != nil {
		return err
	}
	if s.Store == nil || asset.State != "READY" || !asset.RetainUntil.After(time.Now()) {
		return apperr.Conflict("prepared archive is unavailable")
	}
	reader, err := s.Store.OpenGeographic(ctx, asset.objectKey, asset.Bytes, nil)
	if err != nil {
		return apperr.Conflict("prepared archive is unavailable")
	}
	defer reader.Close()
	hash := sha256.New()
	length, err := io.Copy(hash, io.LimitReader(reader, asset.Bytes+1))
	if err != nil || length != asset.Bytes || hex.EncodeToString(hash.Sum(nil)) != asset.SHA256 {
		return apperr.Conflict("prepared archive failed its retained SHA-256 check")
	}
	return nil
}

func (s Service) transition(ctx context.Context, id, action string, in ReleaseAction) (EnvironmentRelease, error) {
	var r EnvironmentRelease
	permission := Release
	if action == "verify" {
		permission = Qualify
	}
	if err := require(ctx, permission, false); err != nil {
		return r, err
	}
	if !digestPattern.MatchString(in.ManifestSHA256) || !boundedText(in.Note, 2000) {
		return r, apperr.Validation("exact manifest digest and review evidence are required")
	}
	// Verify physical bytes before holding the decision lock. Publication stores
	// immutable identities, and the transaction below rechecks the exact release,
	// state and current source policy before recording a new decision.
	if action == "verify" || action == "release" {
		candidate, err := s.release(ctx, id)
		if err != nil {
			return r, err
		}
		if candidate.ManifestSHA256 != in.ManifestSHA256 {
			return r, apperr.Conflict("review does not bind the exact retained manifest")
		}
		if candidate.State == "STAGED" || action == "release" && candidate.State == "VERIFIED" {
			if err := s.eligibleRelease(ctx, candidate); err != nil {
				return r, err
			}
			if err := s.checkPreparedBytes(ctx, candidate); err != nil {
				return r, err
			}
			ctx, err = access.Refresh(ctx)
			if err != nil {
				return r, err
			}
			if err := require(ctx, permission, false); err != nil {
				return r, err
			}
		}
	}
	p, resource, _ := access.From(ctx)
	err := db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		ctx := db.UseTx(ctx, tx)
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, resource.ID()+":georelease:"+id); err != nil {
			return err
		}
		var err error
		r, err = s.release(ctx, id)
		if err != nil {
			return err
		}
		if r.ManifestSHA256 != in.ManifestSHA256 {
			return apperr.Conflict("review does not bind the exact retained manifest")
		}
		next := map[string]string{"verify": "VERIFIED", "release": "RELEASED", "withdraw": "WITHDRAWN"}[action]
		note := strings.TrimSpace(in.Note)
		if r.State == next {
			var actor, recordedNote string
			if err = tx.QueryRow(ctx, `SELECT human_id,note FROM geographic_environment_release_events WHERE release_id=$1 ORDER BY occurred_at DESC,id DESC LIMIT 1`, id).Scan(&actor, &recordedNote); err != nil {
				return err
			}
			if actor != p.HumanID || strings.TrimSpace(recordedNote) != note {
				return apperr.Conflict("a different review decision is already recorded")
			}
			if action != "withdraw" {
				return s.eligibleReleaseIn(ctx, tx, r)
			}
			return nil
		}
		valid := action == "verify" && r.State == "STAGED" || action == "release" && r.State == "VERIFIED" || action == "withdraw" && slices.Contains([]string{"STAGED", "VERIFIED", "RELEASED"}, r.State)
		if !valid {
			return apperr.Conflict("invalid environment release transition")
		}
		if action != "withdraw" {
			if err = s.eligibleReleaseIn(ctx, tx, r); err != nil {
				return err
			}
		}
		if err = releaseEvent(ctx, tx, r, next, p.HumanID, note); err != nil {
			return err
		}
		r.State = next
		return nil
	})
	return r, err
}
func releaseEvent(ctx context.Context, tx pgx.Tx, r EnvironmentRelease, state, human, note string) error {
	if state == "" {
		return fmt.Errorf("missing release event state")
	}
	_, err := tx.Exec(ctx, `INSERT INTO geographic_environment_release_events(id,release_id,required_sources,state,human_id,note) VALUES($1,$2,$3,$4,$5,$6)`, ids.New("georel_event"), r.ID, r.sources, state, human, note)
	return err
}
