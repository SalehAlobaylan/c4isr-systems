package simscene

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/jackc/pgx/v5"
)

const PresetSchema = "emad.sensor-preset.v1"

type PresetInput struct {
	Name        string  `json:"name"`
	PeriodMs    int64   `json:"periodMs"`
	DropEvery   int     `json:"dropEvery"`
	DelayMs     int64   `json:"delayMs"`
	NoiseM      float64 `json:"noiseM"`
	Confidence  float64 `json:"confidence"`
	Limitations string  `json:"limitations"`
}
type SensorPreset struct {
	ID       string `json:"id"`
	Schema   string `json:"schema"`
	Fidelity string `json:"fidelity"`
	PresetInput
}

func (p PresetInput) Sensor(id string) Sensor {
	return Sensor{ID: id, PeriodMs: p.PeriodMs, DropEvery: p.DropEvery, DelayMs: p.DelayMs, NoiseM: p.NoiseM, Confidence: p.Confidence}
}
func preset(in PresetInput) (SensorPreset, error) {
	if len(strings.TrimSpace(in.Name)) < 1 || len(in.Name) > 120 || len(strings.TrimSpace(in.Limitations)) < 1 || len(in.Limitations) > 1000 {
		return SensorPreset{}, fmt.Errorf("preset requires bounded name and limitations")
	}
	if e := (Input{Sensors: []Sensor{in.Sensor("preset")}}).ValidateSensors(); e != nil {
		return SensorPreset{}, e
	}
	raw, _ := json.Marshal(in)
	return SensorPreset{ID: "sensor_preset_" + artifacts.Hash(raw), Schema: PresetSchema, Fidelity: "abstract_behavior", PresetInput: in}, nil
}
func (s *Service) CreatePreset(ctx context.Context, in PresetInput) (SensorPreset, error) {
	if e := Require(ctx, "sim.scenarios.author", true); e != nil {
		return SensorPreset{}, e
	}
	value, e := preset(in)
	if e != nil {
		return value, apperr.Validation(e.Error())
	}
	e = db.WithTx(ctx, s.Pool, func(tx pgx.Tx) error {
		r, _ := access.ResourceFrom(ctx)
		if _, e := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, r.ID()+":sensor-presets"); e != nil {
			return e
		}
		var n int
		var exists bool
		if e := tx.QueryRow(ctx, `SELECT count(*),COALESCE(bool_or(id=$1),false) FROM simulation_sensor_presets`, value.ID).Scan(&n, &exists); e != nil {
			return e
		}
		if exists {
			return nil
		}
		if n >= 128 {
			return apperr.Conflict("sensor preset archive budget exhausted")
		}
		raw, _ := json.Marshal(value)
		if _, e := tx.Exec(ctx, `INSERT INTO simulation_sensor_presets(id,document)VALUES($1,$2)`, value.ID, raw); e != nil {
			return e
		}
		return Audit(ctx, tx, "simulation.sensor_preset.created", value.ID)
	})
	return value, e
}
func (s *Service) Preset(ctx context.Context, id string) (SensorPreset, error) {
	if e := Require(ctx, "sim.scenarios.author", false); e != nil {
		return SensorPreset{}, e
	}
	return s.loadPreset(ctx, id)
}
func (s *Service) loadPreset(ctx context.Context, id string) (SensorPreset, error) {
	var raw []byte
	var out SensorPreset
	e := db.Scoped(s.Pool).QueryRow(ctx, `SELECT document FROM simulation_sensor_presets WHERE id=$1`, id).Scan(&raw)
	if errors.Is(e, pgx.ErrNoRows) {
		return out, apperr.NotFound("sensor preset", "unavailable")
	}
	if e != nil {
		return out, e
	}
	if e = json.Unmarshal(raw, &out); e != nil {
		return out, e
	}
	verified, e := preset(out.PresetInput)
	if e != nil || verified != out {
		return out, apperr.Conflict("preset integrity failed")
	}
	return out, nil
}
func (s *Service) Presets(ctx context.Context) ([]SensorPreset, error) {
	if e := Require(ctx, "sim.scenarios.author", false); e != nil {
		return nil, e
	}
	rows, e := db.Scoped(s.Pool).Query(ctx, `SELECT document FROM simulation_sensor_presets ORDER BY created_at DESC,id LIMIT 128`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []SensorPreset{}
	for rows.Next() {
		var raw []byte
		var v SensorPreset
		if e = rows.Scan(&raw); e != nil {
			return nil, e
		}
		if e = json.Unmarshal(raw, &v); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (f Frozen) ValidatePresets() error {
	if len(f.SensorPresets) > 2 {
		return fmt.Errorf("frozen preset budget exceeded")
	}
	seen := map[string]SensorPreset{}
	for _, p := range f.SensorPresets {
		verified, e := preset(p.PresetInput)
		if e != nil || verified != p {
			return fmt.Errorf("invalid frozen sensor preset")
		}
		if _, ok := seen[p.ID]; ok {
			return fmt.Errorf("duplicate frozen preset")
		}
		seen[p.ID] = p
	}
	used := map[string]bool{}
	for _, s := range f.Input.Sensors {
		if s.PresetID == "" {
			continue
		}
		p, ok := seen[s.PresetID]
		actual := s
		actual.PresetID = ""
		if !ok || p.PresetInput.Sensor(s.ID) != actual {
			return fmt.Errorf("sensor settings differ from pinned preset")
		}
		used[p.ID] = true
	}
	if len(used) != len(seen) {
		return fmt.Errorf("unreferenced frozen sensor preset")
	}
	return nil
}
