package scenarios

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/contextpackage"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	platformdb "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/reference"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simulation"
)

const RuntimeVersion = "emad-yaml-runner-v3"

type ClockConfig struct {
	Epoch             time.Time `json:"epoch" yaml:"epoch"`
	ElapsedUnit       string    `json:"elapsedUnit" yaml:"elapsed_unit"`
	Order             string    `json:"order" yaml:"order"`
	SourceTimeMapping string    `json:"sourceTimeMapping" yaml:"source_time_mapping"`
}

func (c ClockConfig) Validate() error {
	if c.Epoch.IsZero() || c.Epoch.Year() < 1970 || c.Epoch.Year() > 2200 || c.ElapsedUnit != "milliseconds" || c.Order != "virtual_time_then_sequence" || c.SourceTimeMapping != "preserve_source_time" {
		return apperr.Validation("invalid simulation clock contract")
	}
	return nil
}
func defaultClock() ClockConfig {
	return ClockConfig{time.Now().UTC().Truncate(time.Millisecond), "milliseconds", "virtual_time_then_sequence", "preserve_source_time"}
}

// Version is protected authoring input. Ordinary run responses contain only its ID.
type Version struct {
	RuntimeBuild string                  `json:"runtimeBuild"`
	Context      []contextpackage.Bundle `json:"context"`
	Schema       string                  `json:"schema"`
	Definition   *Scenario               `json:"definition"`
	Runtime      string                  `json:"runtime"`
}
type FrozenRun struct {
	RuntimeBuild      string      `json:"runtimeBuild"`
	ContextPackageIDs []string    `json:"contextPackageIds"`
	Schema            string      `json:"schema"`
	VersionID         string      `json:"versionId"`
	Runtime           string      `json:"runtime"`
	Seed              int64       `json:"seed"`
	InitialScale      float64     `json:"initialScale"`
	Clock             ClockConfig `json:"clock"`
	RestartOf         string      `json:"restartOf,omitempty"`
}
type versionRepository interface {
	SaveVersion(context.Context, Version) (string, error)
	LoadVersion(context.Context, string) (Version, error)
}

func inputContext(ctx context.Context) context.Context {
	r, _ := access.ResourceFrom(ctx)
	r.Partition = access.ExerciseInput
	r.RunID = ""
	return access.Internal(ctx, r, "frozen-scenario-input")
}
func (r *PostgresRepository) SaveVersion(ctx context.Context, v Version) (string, error) {
	if v.RuntimeBuild == "" {
		return "", apperr.Internal(fmt.Errorf("runtime build identity unavailable"))
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	if len(raw) > 4*1024*1024 {
		return "", apperr.Validation("frozen scenario exceeds 4 MiB")
	}
	id := "sv_" + reference.Hash(raw)
	_, err = platformdb.Scoped(r.pool).Exec(inputContext(ctx), `INSERT INTO simulation_versions(id,document) VALUES($1,$2) ON CONFLICT(scope_id,id) DO NOTHING`, id, raw)
	return id, err
}
func (r *PostgresRepository) LoadVersion(ctx context.Context, id string) (Version, error) {
	var raw []byte
	var v Version
	if err := platformdb.Scoped(r.pool).QueryRow(inputContext(ctx), `SELECT document FROM simulation_versions WHERE id=$1`, id).Scan(&raw); err != nil {
		return v, apperr.Conflict("frozen scenario version unavailable")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&v); err != nil {
		return v, err
	}
	if "sv_"+reference.Hash(raw) != id || v.Schema != "emad.scenario.v1" || v.Definition == nil {
		return v, apperr.Conflict("frozen scenario integrity/runtime mismatch")
	}
	if err := v.Definition.Validate(); err != nil {
		return v, err
	}
	if len(v.Context) != len(v.Definition.ContextPackages) {
		return v, apperr.Conflict("frozen context dependencies incomplete")
	}
	for i, b := range v.Context {
		if b.ID != v.Definition.ContextPackages[i] {
			return v, apperr.Conflict("frozen context dependency identity mismatch")
		}
		if err := b.Validate(time.Now(), true); err != nil {
			return v, apperr.Conflict(err.Error())
		}
	}
	return v, nil
}

// Manifest returns immutable run configuration without exposing authored actors.
func (s *Service) Manifest(ctx context.Context, id string) (FrozenRun, error) {
	if err := access.Require(ctx, "scenarios.read"); err != nil {
		return FrozenRun{}, err
	}
	r, err := s.repo.Get(ctx, id)
	return r.Frozen, err
}
func (s *Service) Version(ctx context.Context, id string) (Version, error) {
	if err := access.Require(ctx, "sim.scenarios.author"); err != nil {
		return Version{}, err
	}
	r, err := s.repo.Get(ctx, id)
	if err != nil {
		return Version{}, err
	}
	repo, ok := s.repo.(versionRepository)
	if !ok {
		return Version{}, fmt.Errorf("version repository unavailable")
	}
	return repo.LoadVersion(ctx, r.Frozen.VersionID)
}

// ClockNow resolves only the selected run. Public and legacy data use wall time.
// The resolver is carried by the request context, never process-global state.
func (s *Service) ClockNow(ctx context.Context, wall time.Time) time.Time {
	resource, err := access.ResourceFrom(ctx)
	if err != nil || resource.Partition != access.ExerciseObservation || resource.RunID == "legacy" {
		return wall
	}
	r, err := s.repo.Get(ctx, resource.RunID)
	if err != nil || r.Frozen.Clock.Epoch.IsZero() {
		return wall
	}
	s.mu.Lock()
	eng := s.engines[r.ID]
	s.mu.Unlock()
	elapsed := r.VirtualTimeMs
	if eng != nil && !terminalStatus(r.Status) {
		elapsed = eng.VirtualTimeMs()
	}
	return r.Frozen.Clock.Epoch.Add(time.Duration(elapsed) * time.Millisecond)
}
func legacyFidelity(profile string) simulation.Fidelity {
	return simulation.Fidelity{Profile: profile, ModelVersion: RuntimeVersion, Simplifications: []string{"Scripted two-dimensional paths; no physical dynamics or validated sensor model"}}
}
