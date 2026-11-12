// Package geoenv prepares engine-independent geographic reference releases.
package geoenv

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	Read     = "environments.read"
	Manage   = "environments.manage"
	Build    = "environments.build"
	Execute  = "environments.execute"
	Qualify  = "environments.qualify"
	Release  = "environments.release"
	Download = "environments.download"
)

type Service struct {
	Pool        *pgxpool.Pool
	Store       artifacts.GeographicStore
	Toolchain   string
	Parallel    int
	Spool       string
	ReadOnly    bool
	UploadSlots chan struct{}
}

var digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
var keyPattern = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,128}$`)

func require(ctx context.Context, permission string, service bool) error {
	p, r, ok := access.From(ctx)
	if !ok || r.Partition != access.PublicEvidence {
		return apperr.Forbidden("geographic environments require public evidence scope")
	}
	if service && p.Kind != "service" || !service && (p.Kind != "human" || p.HumanID == "") {
		return apperr.Forbidden("operation requires the appropriate registered identity")
	}
	return access.Require(ctx, permission)
}

type Profile struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Required []string `json:"requiredLayers"`
	Optional []string `json:"optionalLayers,omitempty"`
}

var profiles = []Profile{
	{"overview-v1", "Regional overview", []string{"countries", "subdivisions", "places", "coasts"}, nil},
	{"reference-surface-v1", "Reference surface", []string{"surface"}, []string{"geoid"}},
	{"reference-context-v1", "Reference context", []string{"surface", "imagery", "roads", "buildings", "water"}, []string{"geoid"}},
}

func profile(id string) (Profile, bool) {
	for _, p := range profiles {
		if p.ID == id {
			return p, true
		}
	}
	return Profile{}, false
}

// A representation records what the provider says about the retained bytes.
// UNKNOWN is distinct from zero uncertainty or an empty measured footprint.
type Representation struct {
	Layer          string          `json:"layer"`
	Format         string          `json:"format"`
	Producer       string          `json:"producer"`
	ProductEdition string          `json:"productEdition"`
	DataTime       string          `json:"dataTime"`
	RetrievedAt    time.Time       `json:"retrievedAt"`
	CRS            string          `json:"crs"`
	VerticalDatum  string          `json:"verticalDatum"`
	Resolution     string          `json:"nativeResolution"`
	Semantics      string          `json:"semantics"`
	Uncertainty    string          `json:"uncertainty"`
	Evidence       string          `json:"evidence"`
	Footprint      json.RawMessage `json:"footprint"`
}

type AssetInput struct {
	RequestKey     string         `json:"requestKey"`
	SourceID       string         `json:"sourceId"`
	AdmissionID    string         `json:"admissionId"`
	ReferenceID    string         `json:"referenceId,omitempty"`
	SceneID        string         `json:"sceneId,omitempty"`
	SHA256         string         `json:"sha256"`
	Bytes          int64          `json:"bytes"`
	RetainUntil    time.Time      `json:"retainUntil"`
	Representation Representation `json:"representation"`
}

type Asset struct {
	AssetInput
	ID                 string              `json:"id"`
	State              string              `json:"state"`
	MediaType          string              `json:"mediaType"`
	Rights             publicingest.Rights `json:"rights"`
	CurrentAdmissionID string              `json:"currentAdmissionId,omitempty"`
	ProfileRevisionID  string              `json:"profileRevisionId,omitempty"`
	objectKey          string
	owner              string
}

func media(format string) string {
	return map[string]string{"geotiff": "image/tiff", "geojson": "application/geo+json", "osm-pbf": "application/vnd.openstreetmap.data+pbf", "osm-xml": "application/xml", "geopackage": "application/geopackage+sqlite3", "shapefile-zip": "application/zip", "release-zip": "application/zip"}[format]
}

func assetSelect() string {
	return `SELECT document,object_key,owner_id,state FROM geographic_assets `
}

func scanAsset(row pgx.Row) (Asset, error) {
	var a Asset
	var raw []byte
	var state string
	err := row.Scan(&raw, &a.objectKey, &a.owner, &state)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, apperr.NotFound("geographic asset", "unavailable")
	}
	if err == nil {
		err = json.Unmarshal(raw, &a)
	}
	a.State = state
	return a, err
}

func (s Service) asset(ctx context.Context, id string) (Asset, error) {
	return scanAsset(db.Scoped(s.Pool).QueryRow(ctx, assetSelect()+`WHERE id=$1`, id))
}

func (s Service) policy(ctx context.Context, a Asset, use, raw bool) (publicingest.Admission, error) {
	return s.policyIn(ctx, nil, a, use, raw)
}

func (s Service) policyIn(ctx context.Context, tx pgx.Tx, a Asset, use, raw bool) (publicingest.Admission, error) {
	var p publicingest.Admission
	var err error
	if tx != nil {
		p, err = publicingest.LockPolicy(ctx, tx, a.SourceID, a.AdmissionID)
	} else {
		p, err = publicingest.ResolvePolicy(ctx, s.Pool, a.SourceID, a.AdmissionID)
	}
	if err != nil {
		return p, err
	}
	if !p.Rights.RetainRaw || !a.RetainUntil.After(time.Now()) {
		return p, apperr.Forbidden("source retention is unavailable")
	}
	rights := p.Rights.OutputPermissions
	if use && (!p.Rights.AllowAnalysis || rights == nil || rights.GeographicEnvironment == nil || !*rights.GeographicEnvironment) {
		return p, apperr.Forbidden("source policy lacks explicit geographic environment permission")
	}
	if raw && (rights == nil || rights.RawDistribution == nil || !*rights.RawDistribution) {
		return p, apperr.Forbidden("source policy does not allow redistribution of originals")
	}
	if err := publicingest.RequireProfileProcessing(ctx, s.Pool, p); err != nil {
		return p, err
	}
	return p, nil
}

func (s Service) reserve(ctx context.Context, in AssetInput) (Asset, error) {
	var a Asset
	if err := require(ctx, Manage, false); err != nil {
		return a, err
	}
	if err := access.Require(ctx, "sources.manage"); err != nil {
		return a, err
	}
	if s.Store == nil {
		return a, apperr.Conflict("geographic object storage unavailable")
	}
	rep := in.Representation
	if !keyPattern.MatchString(in.RequestKey) || !digestPattern.MatchString(in.SHA256) || in.Bytes < 1 || in.Bytes > artifacts.MaxGeographicBytes ||
		!in.RetainUntil.After(time.Now()) || in.RetainUntil.After(time.Now().Add(365*24*time.Hour)) || media(rep.Format) == "" || rep.Format == "release-zip" ||
		rep.Layer == "" || rep.Producer == "" || rep.ProductEdition == "" || rep.DataTime == "" || rep.RetrievedAt.IsZero() ||
		rep.CRS == "" || rep.VerticalDatum == "" || rep.Resolution == "" || rep.Semantics == "" || rep.Uncertainty == "" || rep.Evidence == "" ||
		len(rep.Footprint) == 0 || len(rep.Footprint) > 16000 || in.ReferenceID != "" && in.SceneID != "" {
		return a, apperr.Validation("asset requires exact digest, bytes, expiry and source representation metadata")
	}
	validLayer := false
	for _, p := range profiles {
		validLayer = validLayer || slices.Contains(p.Required, rep.Layer) || slices.Contains(p.Optional, rep.Layer)
	}
	if !validLayer {
		return a, apperr.Validation("unrecognized geographic layer")
	}
	p, resource, _ := access.From(ctx)
	a = Asset{AssetInput: in, ID: "geoasset_" + artifacts.Hash([]byte(resource.ID()+"\x00"+p.ID+"\x00"+in.RequestKey)), State: "PENDING", MediaType: media(rep.Format), owner: p.ID}
	a.objectKey = objectKey(resource.ID(), a)
	err := db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		ctx := db.UseTx(ctx, tx)
		var covered bool
		if err := tx.QueryRow(ctx, `SELECT CASE WHEN ST_GeometryType(g) IN ('ST_Polygon','ST_MultiPolygon') AND ST_IsValid(g) AND NOT ST_IsEmpty(g) THEN ST_CoveredBy(g,ST_MakeEnvelope(-180,-90,180,90,4326)) ELSE false END FROM (SELECT ST_SetSRID(ST_GeomFromGeoJSON($1),4326) g) x`, string(rep.Footprint)).Scan(&covered); err != nil || !covered {
			return apperr.Validation("footprint must be a valid WGS84 polygon or multipolygon")
		}
		policy, err := s.policyIn(ctx, tx, a, false, false)
		if err != nil {
			return err
		}
		if policy.Rights.ExpiresAt != nil && a.RetainUntil.After(*policy.Rights.ExpiresAt) {
			return apperr.Forbidden("reservation outlasts source rights")
		}
		bounds := policy.Policy.AOI
		if err = tx.QueryRow(ctx, `SELECT ST_CoveredBy(ST_SetSRID(ST_GeomFromGeoJSON($1),4326),ST_MakeEnvelope($2,$3,$4,$5,4326))`, string(rep.Footprint), bounds.West, bounds.South, bounds.East, bounds.North).Scan(&covered); err != nil {
			return err
		}
		if !covered {
			return apperr.Conflict("representation footprint exceeds its admitted area")
		}
		a.Rights = policy.Rights
		if in.ReferenceID != "" {
			var matches bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM reference_datasets WHERE id=$1 AND source_id=$2 AND document->>'parentArtifactSha256'=$3)`, in.ReferenceID, in.SourceID, in.SHA256).Scan(&matches); err != nil {
				return err
			}
			if !matches {
				return apperr.Conflict("retained reference does not match its claimed parent")
			}
		}
		if in.SceneID != "" {
			var matches bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM imagery_scenes WHERE id=$1 AND source_id=$2 AND admission_id=$3)`, in.SceneID, in.SourceID, in.AdmissionID).Scan(&matches); err != nil {
				return err
			}
			if !matches {
				return apperr.Conflict("retained imagery does not match its scene and source admission")
			}
		}
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, resource.ID()+":geoasset:"+p.ID); err != nil {
			return err
		}
		prior, err := s.asset(ctx, a.ID)
		if err == nil {
			old, _ := json.Marshal(prior.AssetInput)
			next, _ := json.Marshal(in)
			// jsonb may reorder footprint keys.
			if !jsonEqual(old, next) {
				return apperr.Conflict("asset request key already binds different inputs")
			}
			a = prior
			return nil
		}
		if !apperr.Is(err, apperr.CodeNotFound) {
			return err
		}
		var bytes int64
		if err = tx.QueryRow(ctx, `SELECT COALESCE(sum(size_bytes),0) FROM geographic_assets WHERE owner_id=$1 AND retain_until>clock_timestamp() AND state<>'EXPIRED'`, p.ID).Scan(&bytes); err != nil {
			return err
		}
		if bytes+in.Bytes > 16<<30 {
			return apperr.Conflict("retained geographic storage budget exceeded")
		}
		return insertAsset(ctx, tx, a, []string{a.SourceID})
	})
	return a, err
}

func objectKey(scope string, a Asset) string {
	return "emad/geographic/v1/" + artifacts.Hash([]byte(scope)) + "/" + a.ID + "/" + a.SHA256
}

func insertAsset(ctx context.Context, tx pgx.Tx, a Asset, sources []string) error {
	raw, err := json.Marshal(a)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO geographic_assets(id,source_id,admission_id,owner_id,required_sources,document,object_key,size_bytes,retain_until) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		a.ID, a.SourceID, a.AdmissionID, a.owner, sources, raw, a.objectKey, a.Bytes, a.RetainUntil)
	return err
}

func (s Service) upload(ctx context.Context, id string, body io.Reader, length int64) (Asset, error) {
	if err := require(ctx, Manage, false); err != nil {
		return Asset{}, err
	}
	if err := access.Require(ctx, "sources.manage"); err != nil {
		return Asset{}, err
	}
	a, err := s.asset(ctx, id)
	if err != nil {
		return a, err
	}
	p, _, _ := access.From(ctx)
	if a.owner != p.ID || a.Representation.Format == "release-zip" {
		return a, apperr.NotFound("geographic upload", "unavailable")
	}
	if s.Store == nil || a.State == "EXPIRED" || length != a.Bytes {
		return a, apperr.Conflict("upload unavailable or size differs from reservation")
	}
	if _, err = s.policy(ctx, a, false, false); err != nil {
		return a, err
	}
	if a.State == "READY" {
		return a, nil
	}
	if err = s.markUpload(ctx, a.ID); err != nil {
		return a, err
	}
	file, err := os.CreateTemp(s.Spool, "emad-geoasset-*")
	if err != nil {
		return a, err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	n, err := io.Copy(file, io.LimitReader(body, a.Bytes+1))
	if err != nil || n != a.Bytes {
		return a, apperr.Validation("upload size differs from reservation")
	}
	if err = s.Store.PutGeographic(ctx, a.objectKey, file, a.Bytes, a.SHA256, a.MediaType); err != nil {
		return a, apperr.Conflict("upload not verified; retry the exact reservation")
	}
	ctx, err = access.Refresh(ctx)
	if err != nil {
		return a, err
	}
	if err = require(ctx, Manage, false); err != nil {
		return a, err
	}
	if err = access.Require(ctx, "sources.manage"); err != nil {
		return a, err
	}
	err = db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		ctx := db.UseTx(ctx, tx)
		if _, err := s.policyIn(ctx, tx, a, false, false); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `UPDATE geographic_assets SET state='READY',upload_until=NULL WHERE id=$1 AND state IN ('PENDING','READY') AND retain_until>clock_timestamp()`, a.ID)
		if err == nil && tag.RowsAffected() != 1 {
			err = apperr.Conflict("upload reservation expired")
		}
		return err
	})
	if err == nil {
		a.State = "READY"
	}
	return a, err
}

func (s Service) markUpload(ctx context.Context, id string) error {
	tag, err := db.Scoped(s.Pool).Exec(ctx, `UPDATE geographic_assets SET upload_until=clock_timestamp()+interval '32 minutes' WHERE id=$1 AND state='PENDING' AND retain_until>clock_timestamp() AND (upload_until IS NULL OR upload_until<=clock_timestamp())`, id)
	if err == nil && tag.RowsAffected() != 1 {
		return apperr.Conflict("upload reservation expired or is being verified")
	}
	return err
}

func sourcesOf(area []string, assets []Asset) []string {
	x := append([]string{}, area...)
	for _, a := range assets {
		x = append(x, a.SourceID)
	}
	slices.Sort(x)
	return slices.Compact(x)
}

func jsonEqual(a, b []byte) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	p, _ := json.Marshal(x)
	q, _ := json.Marshal(y)
	return string(p) == string(q)
}

func boundedText(s string, max int) bool { return strings.TrimSpace(s) != "" && len(s) <= max }
