package geoenv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/areas"
	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/jsoncanon"
	"github.com/jackc/pgx/v5"
)

type DefinitionInput struct {
	Name             string   `json:"name"`
	AreaRevisionID   string   `json:"areaRevisionId"`
	ProfileID        string   `json:"profileId"`
	AssetIDs         []string `json:"assetIds"`
	ExpectedRevision int      `json:"expectedRevision"`
}
type Definition struct {
	DefinitionInput
	ID       string `json:"id"`
	Revision int    `json:"revision"`
}
type Recipe struct {
	Schema     string         `json:"schema"`
	Toolchain  string         `json:"toolchainSha256"`
	Definition Definition     `json:"definition"`
	Area       areas.Revision `json:"area"`
	Profile    Profile        `json:"profile"`
	Inputs     []Asset        `json:"inputs"`
}
type Preflight struct {
	Eligible       bool     `json:"eligible"`
	Diagnostics    []string `json:"diagnostics"`
	Inputs         []Asset  `json:"inputs"`
	InputBytes     int64    `json:"inputBytes"`
	RebuildAllowed bool     `json:"rebuildAllowed"`
}

func (s Service) definition(ctx context.Context, id string, revision int) (Definition, error) {
	var d Definition
	var raw []byte
	err := db.Scoped(s.Pool).QueryRow(ctx, `SELECT document FROM geographic_environment_definitions WHERE id=$1 AND ($2=0 OR revision=$2) ORDER BY revision DESC LIMIT 1`, id, revision).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return d, apperr.NotFound("environment definition", "unavailable")
	}
	if err == nil {
		err = json.Unmarshal(raw, &d)
	}
	return d, err
}

func (s Service) saveDefinition(ctx context.Context, id string, in DefinitionInput) (Definition, error) {
	var d Definition
	if err := require(ctx, Manage, false); err != nil {
		return d, err
	}
	if _, ok := profile(in.ProfileID); !ok || !boundedText(in.Name, 160) || len(in.AssetIDs) > 64 || in.ExpectedRevision < 0 {
		return d, apperr.Validation("valid name, profile, revision and bounded layer bindings required")
	}
	if id == "" && in.ExpectedRevision != 0 || id != "" && in.ExpectedRevision < 1 {
		return d, apperr.Validation("expected revision is required for an edit")
	}
	area, err := (areas.Service{Pool: s.Pool}).Get(ctx, in.AreaRevisionID)
	if err != nil {
		return d, err
	}
	if len(area.Geometry) == 0 || len(area.Components) < 1 || len(area.Components) > 16 || area.AreaM2 > 2.5e9 || area.AreaM2 <= 0 {
		return d, apperr.Validation("choose a saved area of at most 2,500 km² and 16 components")
	}
	var boundsOK bool
	err = db.Scoped(s.Pool).QueryRow(ctx, `SELECT ST_XMax(geometry)-ST_XMin(geometry)<180 AND ST_YMin(geometry)>=-85 AND ST_YMax(geometry)<=85 FROM area_revisions WHERE id=$1`, in.AreaRevisionID).Scan(&boundsOK)
	if err != nil {
		return d, err
	}
	if !boundsOK {
		return d, apperr.Validation("date-line and polar preparation are unsupported")
	}
	in.Name = strings.TrimSpace(in.Name)
	in.AssetIDs = append([]string{}, in.AssetIDs...)
	slices.Sort(in.AssetIDs)
	if len(slices.Compact(slices.Clone(in.AssetIDs))) != len(in.AssetIDs) {
		return d, apperr.Validation("duplicate geographic asset binding")
	}
	assets := make([]Asset, 0, len(in.AssetIDs))
	for _, aID := range in.AssetIDs {
		a, err := s.asset(ctx, aID)
		if err != nil {
			return d, err
		}
		if a.Representation.Format == "release-zip" {
			return d, apperr.Validation("release bundles are not source inputs")
		}
		if _, err = s.policy(ctx, a, false, false); err != nil {
			return d, err
		}
		assets = append(assets, a)
	}
	if id == "" {
		id = ids.New("geoenv")
	}
	d = Definition{DefinitionInput: in, ID: id, Revision: in.ExpectedRevision + 1}
	src := sourcesOf(area.RequiredSources, assets)
	_, resource, _ := access.From(ctx)
	err = db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		if _, e := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, resource.ID()+":geoenv:"+id); e != nil {
			return e
		}
		var head int
		if e := tx.QueryRow(ctx, `SELECT COALESCE(max(revision),0) FROM geographic_environment_definitions WHERE id=$1`, id).Scan(&head); e != nil {
			return e
		}
		if head != in.ExpectedRevision {
			return apperr.Conflict("environment revision changed or is unavailable")
		}
		raw, _ := json.Marshal(d)
		tag, e := tx.Exec(ctx, `INSERT INTO geographic_environment_definitions(id,revision,area_revision_id,profile_id,required_sources,document) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`, d.ID, d.Revision, d.AreaRevisionID, d.ProfileID, src, raw)
		if e == nil && tag.RowsAffected() != 1 {
			return apperr.Conflict("environment revision changed or is unavailable")
		}
		return e
	})
	return d, err
}

func (s Service) preflight(ctx context.Context, d Definition) (Recipe, Preflight, error) {
	r := Recipe{Schema: "emad.environment-recipe.v1", Toolchain: s.Toolchain, Definition: d, Inputs: []Asset{}}
	pf := Preflight{Diagnostics: []string{}, Inputs: []Asset{}, RebuildAllowed: true}
	var err error
	r.Area, err = (areas.Service{Pool: s.Pool}).Get(ctx, d.AreaRevisionID)
	if err != nil {
		return r, pf, err
	}
	var known bool
	r.Profile, known = profile(d.ProfileID)
	if !known {
		return r, pf, apperr.Conflict("profile version unavailable")
	}
	if !digestPattern.MatchString(s.Toolchain) || s.Store == nil {
		pf.Diagnostics = append(pf.Diagnostics, "Preparation storage or toolchain has not been configured.")
	}
	layers := map[string][]string{}
	needsGeoid := false
	for _, id := range d.AssetIDs {
		a, e := s.asset(ctx, id)
		if e != nil {
			return r, pf, e
		}
		p, e := s.policy(ctx, a, true, false)
		if e != nil || a.State != "READY" {
			pf.Diagnostics = append(pf.Diagnostics, a.Representation.Layer+": retained bytes or source permission unavailable.")
			continue
		}
		a.Rights = p.Rights
		a.CurrentAdmissionID = p.CurrentAdmissionID
		a.ProfileRevisionID = p.ProfileRevisionID
		rights := p.Rights.OutputPermissions
		if rights.RawDistribution == nil || !*rights.RawDistribution {
			pf.RebuildAllowed = false
		}
		kind := a.Representation.Layer
		if !slices.Contains(r.Profile.Required, kind) && !slices.Contains(r.Profile.Optional, kind) {
			pf.Diagnostics = append(pf.Diagnostics, kind+": unsupported layer for this profile.")
		}
		if kind == "surface" {
			if a.Representation.Format != "geotiff" || strings.EqualFold(a.Representation.Uncertainty, "unknown") || a.Representation.VerticalDatum != "ellipsoid" && a.Representation.VerticalDatum != "EGM2008" {
				pf.Diagnostics = append(pf.Diagnostics, "Surface requires stated source uncertainty, GeoTIFF and a supported vertical datum.")
			}
			needsGeoid = needsGeoid || a.Representation.VerticalDatum == "EGM2008"
		}
		if kind == "imagery" && (a.Representation.Format != "geotiff" || strings.EqualFold(a.Representation.DataTime, "unknown")) {
			pf.Diagnostics = append(pf.Diagnostics, "Imagery requires dated retained GeoTIFF pixels.")
		}
		layers[kind] = append(layers[kind], string(a.Representation.Footprint))
		r.Inputs = append(r.Inputs, a)
		pf.Inputs = append(pf.Inputs, a)
		pf.InputBytes += a.Bytes
	}
	if pf.InputBytes > 4<<30 {
		pf.Diagnostics = append(pf.Diagnostics, "Input bytes exceed the preparation scratch budget.")
	}
	required := slices.Clone(r.Profile.Required)
	if needsGeoid {
		required = append(required, "geoid")
	}
	for _, kind := range required {
		if len(layers[kind]) == 0 {
			pf.Diagnostics = append(pf.Diagnostics, kind+": no eligible retained input.")
			continue
		}
		var covered bool
		err = db.Scoped(s.Pool).QueryRow(ctx, `SELECT ST_CoveredBy(geometry,(SELECT ST_Union(ST_SetSRID(ST_GeomFromGeoJSON(f),4326)) FROM unnest($2::text[]) f)) FROM area_revisions WHERE id=$1`, d.AreaRevisionID, layers[kind]).Scan(&covered)
		if err != nil {
			return r, pf, err
		}
		if !covered {
			pf.Diagnostics = append(pf.Diagnostics, kind+": declared source footprints do not cover the selected area.")
		}
	}
	pf.Eligible = len(pf.Diagnostics) == 0
	return r, pf, nil
}

type BuildInput struct {
	RequestKey string `json:"requestKey"`
	Revision   int    `json:"revision"`
}
type Job struct {
	ID                               string    `json:"id"`
	DefinitionID                     string    `json:"definitionId"`
	DefinitionRevision               int       `json:"definitionRevision"`
	State                            string    `json:"state"`
	Attempt                          int       `json:"attempt"`
	FailureCode                      string    `json:"failureCode,omitempty"`
	CreatedAt                        time.Time `json:"createdAt"`
	InputSHA256                      string    `json:"inputSha256"`
	owner, credential, worker, lease string
	sources                          []string
	recipe                           Recipe
}

const jobSelect = `SELECT id,definition_id,definition_revision,state,attempt,failure_code,created_at,input_hash,owner_id,credential_id,worker_id,lease_id,required_sources,recipe FROM geographic_environment_builds `

func scanJob(row pgx.Row) (Job, error) {
	var j Job
	var raw []byte
	err := row.Scan(&j.ID, &j.DefinitionID, &j.DefinitionRevision, &j.State, &j.Attempt, &j.FailureCode, &j.CreatedAt, &j.InputSHA256, &j.owner, &j.credential, &j.worker, &j.lease, &j.sources, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return j, apperr.NotFound("environment build", "unavailable")
	}
	if err == nil {
		err = json.Unmarshal(raw, &j.recipe)
	}
	return j, err
}
func (s Service) job(ctx context.Context, id string) (Job, error) {
	return scanJob(db.Scoped(s.Pool).QueryRow(ctx, jobSelect+`WHERE id=$1`, id))
}

func (s Service) submit(ctx context.Context, id string, in BuildInput) (Job, error) {
	var j Job
	if err := require(ctx, Build, false); err != nil {
		return j, err
	}
	p, r, _ := access.From(ctx)
	if p.CredentialID == "" || !keyPattern.MatchString(in.RequestKey) || in.Revision < 1 {
		return j, apperr.Validation("explicit credential, request key and definition revision required")
	}
	err := db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		ctx := db.UseTx(ctx, tx)
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, r.ID()+":geobuild:"+p.ID); err != nil {
			return err
		}
		var err error
		j, err = scanJob(tx.QueryRow(ctx, jobSelect+`WHERE owner_id=$1 AND request_key=$2`, p.ID, in.RequestKey))
		if err == nil {
			if j.DefinitionID != id || j.DefinitionRevision != in.Revision {
				return apperr.Conflict("request key already binds different inputs")
			}
			return s.checkInputs(ctx, j.recipe, false)
		}
		if !apperr.Is(err, apperr.CodeNotFound) {
			return err
		}
		d, err := s.definition(ctx, id, in.Revision)
		if err != nil {
			return err
		}
		recipe, pf, err := s.preflight(ctx, d)
		if err != nil {
			return err
		}
		if !pf.Eligible {
			return apperr.Conflict(strings.Join(pf.Diagnostics, " "))
		}
		raw, err := jsoncanon.Marshal(recipe)
		if err != nil {
			return err
		}
		if len(raw) > 7<<20 {
			return apperr.Validation("frozen recipe exceeds metadata budget")
		}
		j = Job{ID: ids.New("geobuild"), DefinitionID: id, DefinitionRevision: in.Revision, State: "QUEUED", CreatedAt: time.Now().UTC(), InputSHA256: artifacts.Hash(raw), owner: p.ID, credential: p.CredentialID, sources: sourcesOf(recipe.Area.RequiredSources, recipe.Inputs), recipe: recipe}
		_, err = tx.Exec(ctx, `INSERT INTO geographic_environment_builds(id,definition_id,definition_revision,owner_id,credential_id,request_key,input_hash,recipe,required_sources,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, j.ID, id, in.Revision, p.ID, p.CredentialID, in.RequestKey, j.InputSHA256, raw, j.sources, j.CreatedAt)
		if err != nil {
			return err
		}
		return buildEvent(ctx, tx, j, "QUEUED", "")
	})
	return j, err
}

func buildEvent(ctx context.Context, tx pgx.Tx, j Job, state, code string) error {
	_, err := tx.Exec(ctx, `INSERT INTO geographic_environment_build_events(id,build_id,required_sources,attempt,state,code) VALUES($1,$2,$3,$4,$5,$6)`, ids.New("geoevent"), j.ID, j.sources, j.Attempt, state, code)
	return err
}

func (s Service) checkInputs(ctx context.Context, r Recipe, raw bool) error {
	return s.checkInputsIn(ctx, nil, r, raw)
}

func (s Service) checkInputsIn(ctx context.Context, tx pgx.Tx, r Recipe, raw bool) error {
	for _, source := range r.Area.RequiredSources {
		if err := access.RequireSource(ctx, source); err != nil {
			return err
		}
	}
	for _, frozen := range r.Inputs {
		a, err := s.asset(ctx, frozen.ID)
		if err != nil {
			return err
		}
		if a.State != "READY" || a.SHA256 != frozen.SHA256 || a.AdmissionID != frozen.AdmissionID {
			return apperr.Conflict("frozen input bytes are unavailable")
		}
		if _, err = s.policyIn(ctx, tx, a, true, raw); err != nil {
			return err
		}
	}
	return nil
}

func (s Service) actor(ctx context.Context, j Job) (context.Context, error) {
	store := access.NewStore(s.Pool)
	session, err := store.AuthenticateExplicitCredential(ctx, j.credential)
	if err != nil {
		return ctx, err
	}
	resource, err := access.ResourceFrom(ctx)
	if err != nil {
		return ctx, err
	}
	if session.Principal.ID != j.owner {
		return ctx, apperr.Forbidden("original builder is unavailable")
	}
	// Never reuse a worker's scoped SQL transaction for the submitting human.
	human, err := store.BindSession(ctx, session, resource)
	if err != nil {
		return ctx, err
	}
	if err = require(human, Build, false); err != nil {
		return ctx, err
	}
	return human, nil
}

func (s Service) changeJob(ctx context.Context, id, action string) (Job, error) {
	var j Job
	if err := require(ctx, Build, false); err != nil {
		return j, err
	}
	p, _, _ := access.From(ctx)
	err := db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		var err error
		j, err = scanJob(tx.QueryRow(ctx, jobSelect+`WHERE id=$1 AND owner_id=$2 FOR UPDATE`, id, p.ID))
		if err != nil {
			return err
		}
		switch action {
		case "cancel":
			if j.State == "CANCELED" {
				return nil
			}
			if j.State != "QUEUED" && j.State != "RUNNING" {
				return apperr.Conflict("build is already terminal")
			}
			j.State = "CANCELED"
		case "retry":
			if j.State != "FAILED" || j.Attempt >= 3 || j.FailureCode != "transient_io" {
				return apperr.Conflict("build cannot be retried; correct its inputs and submit a new request")
			}
			if err = s.checkInputs(db.UseTx(ctx, tx), j.recipe, false); err != nil {
				return err
			}
			j.State = "QUEUED"
		default:
			return fmt.Errorf("invalid build transition")
		}
		j.FailureCode = ""
		_, err = tx.Exec(ctx, `UPDATE geographic_environment_builds SET state=$2,lease_until=NULL,lease_id='',worker_id='',failure_code='',next_attempt_at=clock_timestamp() WHERE id=$1`, id, j.State)
		if err != nil {
			return err
		}
		return buildEvent(ctx, tx, j, j.State, action)
	})
	return j, err
}
