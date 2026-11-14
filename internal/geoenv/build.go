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
	Schema                string              `json:"schema"`
	Toolchain             string              `json:"toolchainSha256"`
	Definition            Definition          `json:"definition"`
	Area                  areas.Revision      `json:"area"`
	Profile               Profile             `json:"profile"`
	Inputs                []Asset             `json:"inputs"`
	Display               *DisplayBuildRecipe `json:"display,omitempty"`
	ProfileContract       *profileContract    `json:"profileContract,omitempty"`
	ProfileContractSHA256 string              `json:"profileContractSha256,omitempty"`
}
type Preflight struct {
	Eligible              bool            `json:"eligible"`
	Diagnostics           []string        `json:"diagnostics"`
	Inputs                []Asset         `json:"inputs"`
	InputBytes            int64           `json:"inputBytes"`
	RebuildAllowed        bool            `json:"rebuildAllowed"`
	Coverage              []LayerCoverage `json:"coverage"`
	ProfileContractSHA256 string          `json:"profileContractSha256"`
}

// LayerCoverage describes catalog footprints before the worker reads samples.
type LayerCoverage struct {
	Layer    string `json:"layer"`
	Required bool   `json:"required"`
	Status   string `json:"status"`
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
	p, ok := profile(in.ProfileID)
	if !ok || !boundedText(in.Name, 160) || len(in.AssetIDs) > 64 || in.ExpectedRevision < 0 {
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
	if diagnostic := areaDiagnostic(area, capabilities(p)); diagnostic != "" {
		return d, apperr.Validation(diagnostic)
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
		if !originalFormat(a.Representation.Format) {
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
	pf := Preflight{Diagnostics: []string{}, Inputs: []Asset{}, RebuildAllowed: true, Coverage: []LayerCoverage{}}
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
	contract := capabilities(r.Profile)
	r.ProfileContract = &contract
	r.ProfileContractSHA256 = profileDigest(contract)
	pf.ProfileContractSHA256 = r.ProfileContractSHA256
	if diagnostic := areaDiagnostic(r.Area, contract); diagnostic != "" {
		pf.Diagnostics = append(pf.Diagnostics, diagnostic)
	}
	if r.Profile.ID == enrichmentProfile {
		r.Toolchain = s.EnrichmentToolchain
	}
	if r.Profile.ID == observationProfile {
		r.Toolchain = s.ObservationToolchain
	}
	if r.Profile.ID == coastalProfile {
		r.Toolchain = s.CoastalToolchain
	}
	if !digestPattern.MatchString(r.Toolchain) || s.Store == nil {
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
			pf.RebuildAllowed = false
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
		for _, supported := range contract.Layers {
			if supported.Layer == kind && !slices.Contains(supported.Formats, a.Representation.Format) {
				pf.Diagnostics = append(pf.Diagnostics, kind+": retained format is unsupported by the selected profile.")
				break
			}
		}
		if a.Representation.Format == "geotiff" && !horizontalEPSG.MatchString(a.Representation.CRS) {
			pf.Diagnostics = append(pf.Diagnostics, kind+": declare a supported horizontal EPSG reference for the raster.")
		}
		if kind == "geoid" && (a.SHA256 != egm2008Grid || a.Representation.VerticalDatum != "EGM2008") {
			pf.Diagnostics = append(pf.Diagnostics, "Geoid requires the pinned NGA EGM2008 grid and the EGM2008 datum.")
		}
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
		rep := a.Representation
		if kind == "bathymetry" && (rep.Format != "gebco-pair-zip" || rep.ProductEdition != "GEBCO_2026" || rep.CRS != "EPSG:4326" ||
			rep.VerticalDatum != "GEBCO mixed sources; nominal mean sea level" || rep.Semantics != "signed-elevation-and-source-type" || a.Bytes > 8<<20) {
			pf.Diagnostics = append(pf.Diagnostics, "Bathymetry requires a retained GEBCO 2026 elevation/TID pair with separate coastline.")
		}
		if kind == "radar" && (rep.Format != "radar-capture-zip" || !radarProduct.MatchString(rep.ProductEdition) ||
			!radarCRS.MatchString(rep.CRS) || rep.VerticalDatum != "not-applicable" || rep.Semantics != "gamma0-linear-power" || a.Bytes > 16<<20) {
			pf.Diagnostics = append(pf.Diagnostics, "Radar requires a pinned Sentinel-1 IW VV RTC capture with native linear-power samples.")
		}
		if kind == "weather-field" && (rep.Format != "era5-pressure-json" || rep.ProductEdition != weatherEdition || rep.CRS != "EPSG:4326" ||
			rep.VerticalDatum != "pressure-hPa" || rep.Semantics != "reanalysis-pressure-field" || a.Bytes > 128<<10) {
			pf.Diagnostics = append(pf.Diagnostics, "Weather fields require a bounded native ERA5 pressure subset.")
		}
		if kind == "land-cover" && (rep.Format != "geotiff" || rep.CRS != "EPSG:4326" || rep.VerticalDatum != "not-applicable" ||
			rep.ProductEdition != "ESA WorldCover 2021 v200" || rep.DataTime != "2021" || rep.Semantics != "categorical-land-cover") {
			pf.Diagnostics = append(pf.Diagnostics, "Land cover requires the supported WorldCover categorical representation.")
		}
		if kind == "ground" && (rep.Format != "geotiff" || rep.CRS != "EPSG:4326" || rep.VerticalDatum != "EGM2008" ||
			rep.ProductEdition != "GEDTM30 filtered v20250619" || rep.DataTime != "2006/2015" || rep.Semantics != "modeled-bare-earth") {
			pf.Diagnostics = append(pf.Diagnostics, "Ground requires the supported modeled GEDTM30 EGM2008 representation.")
		}
		if kind == "modeled-building-context" && (rep.Format != "geojson" || rep.CRS != "EPSG:4326" || rep.VerticalDatum != "above-ground" ||
			rep.ProductEdition != "GHS-OBAT R2024A V1.0" || rep.DataTime != "2020" || rep.Semantics != "modeled-building-height-context") {
			pf.Diagnostics = append(pf.Diagnostics, "Building context requires the supported modeled GHS-OBAT point representation.")
		}
		layers[kind] = append(layers[kind], string(a.Representation.Footprint))
		r.Inputs = append(r.Inputs, a)
		pf.Inputs = append(pf.Inputs, a)
		pf.InputBytes += a.Bytes
	}
	if pf.InputBytes > contract.MaxInputBytes {
		pf.Diagnostics = append(pf.Diagnostics, "Input bytes exceed the preparation scratch budget.")
	}
	if len(layers["geoid"]) > 1 {
		pf.Diagnostics = append(pf.Diagnostics, "Select exactly one retained EGM2008 grid for the preparation.")
	}
	required := slices.Clone(r.Profile.Required)
	if r.Profile.ID == coastalProfile && len(layers["bathymetry"]) != 1 {
		pf.Diagnostics = append(pf.Diagnostics, "Select one exact paired bathymetry capture.")
	}
	if r.Profile.ID == observationProfile {
		if len(layers["radar"])+len(layers["weather-field"]) == 0 {
			pf.Diagnostics = append(pf.Diagnostics, "Select at least one observation layer.")
		}
		for _, kind := range []string{"radar", "weather-field"} {
			if len(layers[kind]) > 1 {
				pf.Diagnostics = append(pf.Diagnostics, kind+": select only one exact input.")
			}
			if len(layers[kind]) > 0 {
				required = append(required, kind)
			}
		}
	}
	if needsGeoid && !slices.Contains(required, "geoid") {
		required = append(required, "geoid")
	}
	if r.Profile.ID == enrichmentProfile && len(layers["ground"]) > 0 {
		required = append(required, "ground")
		if slices.ContainsFunc(r.Inputs, func(a Asset) bool {
			return a.Representation.Layer == "surface" && a.Representation.VerticalDatum != "EGM2008"
		}) {
			pf.Diagnostics = append(pf.Diagnostics, "The ground comparison requires surface and ground elevations in EGM2008.")
		}
	}
	profileLayers := slices.Clone(required)
	for _, kind := range r.Profile.Optional {
		if !slices.Contains(required, kind) {
			profileLayers = append(profileLayers, kind)
		}
	}
	for _, kind := range profileLayers {
		isRequired := slices.Contains(required, kind)
		if len(layers[kind]) == 0 {
			pf.Coverage = append(pf.Coverage, LayerCoverage{kind, isRequired, "NO_ELIGIBLE_INPUT"})
			if isRequired {
				pf.Diagnostics = append(pf.Diagnostics, kind+": no eligible retained input.")
			}
			continue
		}
		var covered bool
		err = db.Scoped(s.Pool).QueryRow(ctx, `SELECT ST_CoveredBy(geometry,(SELECT ST_Union(ST_SetSRID(ST_GeomFromGeoJSON(f),4326)) FROM unnest($2::text[]) f)) FROM area_revisions WHERE id=$1`, d.AreaRevisionID, layers[kind]).Scan(&covered)
		if err != nil {
			return r, pf, err
		}
		status := "DECLARED_COVERED"
		if !covered {
			status = "DECLARED_INCOMPLETE"
			if isRequired {
				pf.Diagnostics = append(pf.Diagnostics, kind+": declared source footprints do not cover the selected area.")
			}
		}
		pf.Coverage = append(pf.Coverage, LayerCoverage{kind, isRequired, status})
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
	DisplayID                        string    `json:"displayId,omitempty"`
	owner, credential, worker, lease string
	leaseUntil                       time.Time
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
			if j.recipe.Display != nil || j.DefinitionID != id || j.DefinitionRevision != in.Revision {
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
		return insertBuild(ctx, tx, j, in.RequestKey, raw)
	})
	return j, err
}

func insertBuild(ctx context.Context, tx pgx.Tx, j Job, key string, raw []byte) error {
	_, err := tx.Exec(ctx, `INSERT INTO geographic_environment_builds(id,definition_id,definition_revision,owner_id,credential_id,request_key,input_hash,recipe,required_sources,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, j.ID, j.DefinitionID, j.DefinitionRevision, j.owner, j.credential, key, j.InputSHA256, raw, j.sources, j.CreatedAt)
	if err != nil {
		return err
	}
	return buildEvent(ctx, tx, j, "QUEUED", "")
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
	permission := Build
	if j.recipe.Display != nil {
		permission = Qualify
	}
	if err = require(human, permission, false); err != nil {
		return ctx, err
	}
	return human, nil
}

func (s Service) changeJob(ctx context.Context, id, action string) (Job, error) {
	return s.changeBuild(ctx, id, action, false)
}

func (s Service) changeBuild(ctx context.Context, id, action string, display bool) (Job, error) {
	var j Job
	permission := Build
	if display {
		permission = Qualify
	}
	if err := require(ctx, permission, false); err != nil {
		return j, err
	}
	p, _, _ := access.From(ctx)
	err := db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		var err error
		j, err = scanJob(tx.QueryRow(ctx, jobSelect+`WHERE id=$1 AND owner_id=$2 FOR UPDATE`, id, p.ID))
		if err != nil {
			return err
		}
		if (j.recipe.Display != nil) != display {
			return apperr.NotFound("environment build", "unavailable")
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
