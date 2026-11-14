package geoenv

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/jsoncanon"
	"github.com/jackc/pgx/v5"
)

type DisplayBuildRecipe struct {
	ReleaseID      string `json:"releaseId"`
	ManifestSHA256 string `json:"manifestSha256"`
	Recipe         string `json:"recipe"`
	SceneMetadata  string `json:"sceneMetadata,omitempty"`
}

type DisplayBuildInput struct {
	RequestKey     string `json:"requestKey"`
	ManifestSHA256 string `json:"manifestSha256"`
	Recipe         string `json:"recipe"`
	SceneMetadata  string `json:"sceneMetadata,omitempty"`
}

func (s Service) displayBuildToolchain(recipe string) string {
	switch recipe {
	case "terrain-glb-v1":
		return s.DisplayToolchain
	case "terrain-glb-v2", "imagery-display-v1", "roads-display-v1", "buildings-display-v1", "water-display-v1":
		return s.LayerToolchain
	default:
		return ""
	}
}

func (s Service) submitDisplayBuild(ctx context.Context, releaseID string, in DisplayBuildInput) (Job, error) {
	var j Job
	if err := require(ctx, Qualify, false); err != nil {
		return j, err
	}
	p, scope, _ := access.From(ctx)
	if p.CredentialID == "" || !keyPattern.MatchString(in.RequestKey) || !digestPattern.MatchString(in.ManifestSHA256) {
		return j, apperr.Validation("exact manifest digest, display recipe and request key required")
	}
	if in.Recipe == "imagery-display-v1" {
		if len(in.SceneMetadata) > 48<<10 || !json.Valid([]byte(in.SceneMetadata)) {
			return j, apperr.Validation("imagery requires the retained Sentinel-2 scene JSON within 48 KiB")
		}
	} else if in.SceneMetadata != "" {
		return j, apperr.Validation("scene metadata applies to imagery builds")
	}
	display := DisplayBuildRecipe{ReleaseID: releaseID, ManifestSHA256: in.ManifestSHA256, Recipe: in.Recipe, SceneMetadata: in.SceneMetadata}
	err := db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		ctx := db.UseTx(ctx, tx)
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, scope.ID()+":geobuild:"+p.ID); err != nil {
			return err
		}
		rel, err := s.displayParent(ctx, tx, releaseID)
		if err != nil {
			return err
		}
		if rel.ManifestSHA256 != in.ManifestSHA256 {
			return apperr.Conflict("display build manifest digest changed")
		}
		j, err = scanJob(tx.QueryRow(ctx, jobSelect+`WHERE owner_id=$1 AND request_key=$2`, p.ID, in.RequestKey))
		if err == nil {
			if j.recipe.Display == nil || *j.recipe.Display != display {
				return apperr.Conflict("request key already binds different inputs")
			}
			j.setDisplayID(scope.ID())
			return nil
		}
		if !apperr.Is(err, apperr.CodeNotFound) {
			return err
		}
		toolchain := s.displayBuildToolchain(in.Recipe)
		if !digestPattern.MatchString(toolchain) || toolchain == s.Toolchain || s.Store == nil {
			return apperr.Conflict("display build processor is unavailable for this recipe")
		}
		var m Manifest
		if err := json.Unmarshal(rel.Manifest, &m); err != nil {
			return err
		}
		if m.ProfileID != "reference-surface-v1" && m.ProfileID != "reference-context-v1" {
			return apperr.Conflict("this profile uses its own scoped display publication workflow")
		}
		dependencies, available := displayDependencies(m, in.Recipe)
		if !available || len(dependencies) > 128 {
			return apperr.Conflict("release does not have the exact native members required by this display recipe")
		}
		if in.Recipe == "imagery-display-v1" && !sceneMatchesDisplay(in.SceneMetadata, m) {
			return apperr.Validation("scene metadata must match the admitted Sentinel-2 bands and acquisition")
		}
		var pending int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM geographic_environment_builds
			WHERE owner_id=$1 AND recipe ? 'display' AND state IN ('QUEUED','RUNNING')`, p.ID).Scan(&pending); err != nil {
			return err
		}
		if pending >= 8 {
			return apperr.Conflict("display build queue is full")
		}
		parent, err := s.job(ctx, rel.BuildID)
		if err != nil {
			return err
		}
		recipe := Recipe{Schema: "emad.environment-display-build.v1", Toolchain: toolchain, Display: &display}
		raw, err := jsoncanon.Marshal(recipe)
		if err != nil {
			return err
		}
		j = Job{ID: ids.New("geobuild"), DefinitionID: parent.DefinitionID, DefinitionRevision: parent.DefinitionRevision,
			State: "QUEUED", CreatedAt: time.Now().UTC(), InputSHA256: artifacts.Hash(raw),
			owner: p.ID, credential: p.CredentialID, sources: rel.sources, recipe: recipe}
		return insertBuild(ctx, tx, j, in.RequestKey, raw)
	})
	return j, err
}

func sceneMatchesDisplay(raw string, m Manifest) bool {
	var scene struct {
		ID, Collection string
		Properties     struct{ Datetime string }
	}
	if json.Unmarshal([]byte(raw), &scene) != nil || scene.Collection != "sentinel-2-c1-l2a" || scene.ID == "" || scene.Properties.Datetime == "" {
		return false
	}
	for _, band := range opticalBands {
		found := false
		for _, asset := range m.Inputs {
			rep := asset.Representation
			if rep.Layer == "imagery" && rep.ProductEdition == scene.ID+" "+band && rep.DataTime == scene.Properties.Datetime {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (j *Job) setDisplayID(scope string) {
	if j.recipe.Display != nil && j.State == "SUCCEEDED" {
		j.DisplayID = displayIdentity(scope, j.recipe.Display.ReleaseID, j.owner, "display-build:"+j.ID)
	}
}

func (s Service) visibleJob(ctx context.Context, id string) (Job, error) {
	j, err := s.job(ctx, id)
	if err != nil || j.recipe.Display == nil {
		return j, err
	}
	if _, err := s.displayParent(ctx, nil, j.recipe.Display.ReleaseID); err != nil {
		return j, err
	}
	scope, _ := access.ResourceFrom(ctx)
	j.setDisplayID(scope.ID())
	return j, nil
}

func (s Service) prepareDisplay(work, auth context.Context, j Job) (string, error) {
	pinned := j.recipe.Display
	human, err := s.actor(work, j)
	if err != nil {
		return "builder_unavailable", err
	}
	rel, err := s.displayParent(human, nil, pinned.ReleaseID)
	if err != nil {
		return "input_policy_unavailable", err
	}
	if rel.ManifestSHA256 != pinned.ManifestSHA256 {
		return "invalid_input", ErrInvalidInputs
	}
	if err := s.eligibleRelease(work, rel); err != nil {
		return "worker_policy_unavailable", err
	}
	bundle, err := s.asset(work, rel.BundleID)
	if err != nil {
		return "input_unavailable", err
	}
	dir, err := os.MkdirTemp(s.Spool, attemptDirectoryPrefix)
	if err != nil {
		return "transient_io", err
	}
	defer os.RemoveAll(dir)
	if err := s.spoolInput(work, bundle, filepath.Join(dir, "prepared.zip")); err != nil {
		return "transient_io", err
	}
	processor, output := s.DisplayProcessor, "terrain.glb"
	if pinned.Recipe != "terrain-glb-v1" {
		processor = func(ctx context.Context, dir, _ string) error { return s.LayerProcessor(ctx, dir, *pinned) }
		output = "display.bin"
	}
	if err := processor(work, dir, pinned.ManifestSHA256); err != nil {
		if errors.Is(err, ErrScratchBudget) {
			return "scratch_budget_exceeded", err
		}
		if errors.Is(err, ErrInvalidInputs) {
			return "invalid_input", err
		}
		return "transient_io", err
	}
	intent, err := readSmall(filepath.Join(dir, "display-request.json"), 32<<10)
	if err != nil {
		return "invalid_output", err
	}
	raw, err := readSmall(filepath.Join(dir, output), maxDisplayBytes)
	if err != nil {
		return "invalid_output", err
	}
	var in DisplayInput
	var manifest Manifest
	if json.Unmarshal(intent, &in) != nil || json.Unmarshal(rel.Manifest, &manifest) != nil || in.ManifestSHA256 != pinned.ManifestSHA256 ||
		in.ToolchainSHA256 != j.recipe.Toolchain || in.Recipe != pinned.Recipe || int64(len(raw)) != in.Bytes ||
		artifacts.Hash(raw) != in.SHA256 {
		return "invalid_output", ErrInvalidInputs
	}
	bounds, err := s.areaBounds(work, manifest.Area)
	if err != nil {
		return "transient_io", err
	}
	if !validDisplayBytes(raw, Display{DisplayInput: in, Bounds: bounds}, manifest) {
		return "invalid_output", ErrInvalidInputs
	}
	if pinned.SceneMetadata != "" {
		var optical struct {
			Data struct{ SceneMetadataSHA256 string }
		}
		if json.Unmarshal(raw, &optical) != nil || optical.Data.SceneMetadataSHA256 != artifacts.Hash([]byte(pinned.SceneMetadata)) {
			return "invalid_output", ErrInvalidInputs
		}
	}
	in.RequestKey = "display-build:" + j.ID
	human, err = s.actor(work, j)
	if err != nil {
		return "builder_unavailable", err
	}
	display, err := s.reserveDisplayOutput(human, pinned.ReleaseID, in)
	if err != nil {
		if apperr.Is(err, apperr.CodeConflict) || apperr.Is(err, apperr.CodeForbidden) ||
			apperr.Is(err, apperr.CodeNotFound) || apperr.Is(err, apperr.CodeValidation) {
			return "publication_unavailable", err
		}
		return "transient_io", err
	}
	if err := s.publishBuiltDisplay(work, auth, j, display, raw); err != nil {
		if errors.Is(err, errPublicationPolicy) || apperr.Is(err, apperr.CodeForbidden) {
			return "publication_policy_unavailable", err
		}
		return "transient_io", err
	}
	return "", nil
}

func (s Service) publishBuiltDisplay(work, auth context.Context, j Job, d Display, raw []byte) error {
	ctx, err := access.RefreshWithContext(auth, work)
	if err != nil {
		return err
	}
	if err := require(ctx, Execute, true); err != nil {
		return err
	}
	a, err := s.asset(ctx, d.assetID)
	if err != nil {
		return err
	}
	if err := db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		if err := lockLease(ctx, tx, j); err != nil {
			return err
		}
		// Only the worker can upload this reserved key. An expired attempt can
		// finish writing the same immutable bytes but cannot publish them.
		tag, err := tx.Exec(ctx, `UPDATE geographic_assets SET upload_until=clock_timestamp()+interval '32 minutes'
			WHERE id=$1 AND state='PENDING' AND retain_until>clock_timestamp()`, a.ID)
		if err == nil && tag.RowsAffected() != 1 {
			return apperr.Conflict("display reservation is unavailable")
		}
		return err
	}); err != nil {
		return err
	}
	if err := s.Store.PutGeographic(ctx, a.objectKey, bytes.NewReader(raw), a.Bytes, a.SHA256, a.MediaType); err != nil {
		return err
	}
	ctx, err = access.RefreshWithContext(auth, work)
	if err != nil {
		return err
	}
	if err := require(ctx, Execute, true); err != nil {
		return err
	}
	return db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		if err := lockLease(ctx, tx, j); err != nil {
			return err
		}
		human, err := s.actor(ctx, j)
		if err != nil {
			return errPublicationPolicy
		}
		if _, err := s.displayParent(human, nil, d.ReleaseID); err != nil {
			return errPublicationPolicy
		}
		ctx := db.UseTx(ctx, tx)
		if _, err := s.displayParentFor(ctx, tx, d.ReleaseID, true); err != nil {
			return errPublicationPolicy
		}
		tag, err := tx.Exec(ctx, `UPDATE geographic_assets SET state='READY',upload_until=NULL
			WHERE id=$1 AND state='PENDING' AND retain_until>clock_timestamp()`, a.ID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return apperr.Conflict("display reservation expired")
		}
		tag, err = tx.Exec(ctx, `UPDATE geographic_environment_builds SET state='SUCCEEDED',lease_id='',worker_id='',lease_until=NULL
			WHERE id=$1 AND lease_until>clock_timestamp()`, j.ID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return apperr.Conflict("display lease expired during publication")
		}
		return buildEvent(ctx, tx, j, "SUCCEEDED", "")
	})
}
