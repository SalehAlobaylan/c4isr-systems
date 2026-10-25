package simscene

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/contextpackage"
	"github.com/SalehAlobaylan/c4isr-systems/internal/modelassets"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	platformdb "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	Pool     *pgxpool.Pool
	Assets   *modelassets.Service
	Packages *contextpackage.Service
}

func Require(ctx context.Context, permission string, human bool) error {
	if e := access.Require(ctx, permission); e != nil {
		return e
	}
	p, r, _ := access.From(ctx)
	if r.Partition != access.ExerciseInput || r.RunID != "" {
		return apperr.Forbidden("explicit EXERCISE_INPUT environment required")
	}
	if human && p.Kind != "human" {
		return apperr.Forbidden("human simulation author required")
	}
	return nil
}
func (s *Service) Context(ctx context.Context) (context.Context, error) {
	if e := Require(ctx, "sim.scenarios.author", false); e != nil {
		return nil, e
	}
	r, _ := access.ResourceFrom(ctx)
	r.Partition = access.PublicEvidence
	c, e := access.Rebind(ctx, r)
	if e != nil {
		return nil, e
	}
	return c, access.Require(c, "context.export")
}
func Audit(ctx context.Context, tx pgx.Tx, action, id string) error {
	p, _, _ := access.From(ctx)
	kind := "SYSTEM"
	if p.Kind == "human" {
		kind = "OPERATOR"
	}
	_, e := tx.Exec(ctx, `INSERT INTO audit_events(id,occurred_at,actor_type,actor_id,action,subject_type,subject_id,correlation_id,data)VALUES($1,now(),$2,$3,$4,'simulation_studio',$5,'','{}')`, ids.New("aud"), kind, p.ID, action, id)
	return e
}
func (s *Service) Create(ctx context.Context, in Input) (Scene, error) {
	if e := Require(ctx, "sim.scenarios.author", true); e != nil {
		return Scene{}, e
	}
	if len(in.Actors) < 1 || len(in.Actors) > 16 {
		return Scene{}, apperr.Validation("scene requires 1 to 16 abstract actors")
	}
	c, e := s.Context(ctx)
	if e != nil {
		return Scene{}, e
	}
	bundle, e := s.Packages.Load(c, in.ContextPackageID)
	if e != nil {
		return Scene{}, e
	}
	env, e := s.Assets.Definition(ctx, "environments", in.EnvironmentID)
	if e != nil {
		return Scene{}, e
	}
	f := Frozen{Schema: Schema, Runtime: in.Runtime(), Input: in, Context: bundle, Templates: []modelassets.ActorTemplate{}, Models: []modelassets.ModelAsset{}}
	if e = json.Unmarshal(env, &f.Environment); e != nil {
		return Scene{}, e
	}
	templates := map[string]bool{}
	models := map[string]bool{}
	addModel := func(id string) error {
		if models[id] {
			return nil
		}
		v, e := s.Assets.Model(ctx, id)
		if e != nil {
			return e
		}
		f.Models = append(f.Models, v)
		models[id] = true
		return nil
	}
	if e = addModel(f.Environment.ModelVersionID); e != nil {
		return Scene{}, e
	}
	for _, a := range in.Actors {
		if templates[a.TemplateID] {
			continue
		}
		raw, e := s.Assets.Definition(ctx, "templates", a.TemplateID)
		if e != nil {
			return Scene{}, e
		}
		var t modelassets.ActorTemplate
		if e = json.Unmarshal(raw, &t); e != nil {
			return Scene{}, e
		}
		templates[t.ID] = true
		f.Templates = append(f.Templates, t)
		if e = addModel(t.ModelVersionID); e != nil {
			return Scene{}, e
		}
	}
	presets := map[string]bool{}
	for _, sensor := range in.Sensors {
		if sensor.PresetID == "" || presets[sensor.PresetID] {
			continue
		}
		p, e := s.loadPreset(ctx, sensor.PresetID)
		if e != nil {
			return Scene{}, e
		}
		f.SensorPresets = append(f.SensorPresets, p)
		presets[p.ID] = true
	}
	scene, raw, e := Freeze(f)
	if e != nil {
		return Scene{}, apperr.Validation(e.Error())
	}
	fresh, e := access.Refresh(ctx)
	if e != nil {
		return Scene{}, e
	}
	if e = Require(fresh, "sim.scenarios.author", true); e != nil {
		return Scene{}, e
	}
	e = platformdb.WithTx(fresh, s.Pool, func(tx pgx.Tx) error {
		r, _ := access.ResourceFrom(fresh)
		if _, e := tx.Exec(fresh, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, r.ID()+":studio-quota"); e != nil {
			return e
		}
		var n int
		if e := tx.QueryRow(fresh, `SELECT count(*) FROM simulation_scenes`).Scan(&n); e != nil {
			return e
		}
		var exists bool
		if e := tx.QueryRow(fresh, `SELECT EXISTS(SELECT 1 FROM simulation_scenes WHERE id=$1)`, scene.ID).Scan(&exists); e != nil {
			return e
		}
		if exists {
			return nil
		}
		if n >= 128 {
			return apperr.Conflict("scene archive budget exhausted")
		}
		if _, e := tx.Exec(fresh, `INSERT INTO simulation_scenes(id,name,document)VALUES($1,$2,$3)`, scene.ID, in.Name, raw); e != nil {
			return e
		}
		return Audit(fresh, tx, "simulation.scene.frozen", scene.ID)
	})
	return scene, e
}

// Load is also used by an authenticated worker after the runtime lease was
// checked. It never changes resource or grants access to a different environment.
func (s *Service) Load(ctx context.Context, id string) (Scene, error) {
	var f Frozen
	var raw []byte
	e := platformdb.Scoped(s.Pool).QueryRow(ctx, `SELECT document FROM simulation_scenes WHERE id=$1`, id).Scan(&raw)
	if errors.Is(e, pgx.ErrNoRows) {
		return Scene{}, apperr.NotFound("scene", "unavailable")
	}
	if e != nil {
		return Scene{}, e
	}
	if "scene_"+artifacts.Hash(raw) != id || json.Unmarshal(raw, &f) != nil {
		return Scene{}, apperr.Conflict("scene integrity failure")
	}
	if e = f.Validate(time.Now()); e != nil {
		return Scene{}, apperr.Conflict(e.Error())
	}
	return Scene{id, f}, nil
}

type Summary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (s *Service) List(ctx context.Context) ([]Summary, error) {
	if e := Require(ctx, "sim.scenarios.author", false); e != nil {
		return nil, e
	}
	rows, e := platformdb.Scoped(s.Pool).Query(ctx, `SELECT id,name FROM simulation_scenes ORDER BY created_at DESC,id LIMIT 128`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Summary{}
	for rows.Next() {
		var v Summary
		if e = rows.Scan(&v.ID, &v.Name); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
