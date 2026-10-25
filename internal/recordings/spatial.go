package recordings

import (
	"context"
	"encoding/json"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simruntime"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simscene"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simulation"
	"github.com/jackc/pgx/v5"
	"math"
	"sort"
	"strings"
)

type SpatialContact struct {
	Contact       string             `json:"contact"`
	ActorID       string             `json:"actorId"`
	SampleMs      int64              `json:"sampleMs"`
	DeliverMs     int64              `json:"deliverMs"`
	Local         simulation.Vector  `json:"local"`
	TruthAtSample *simulation.Vector `json:"truthAtSample,omitempty"`
	ErrorM        *float64           `json:"errorM,omitempty"`
	Confidence    float64            `json:"confidence"`
}
type SpatialFrame struct {
	Schema        string               `json:"schema"`
	ElapsedMs     int64                `json:"elapsedMs"`
	TruthSnapshot *simruntime.Snapshot `json:"truthSnapshot"`
	Contacts      []SpatialContact     `json:"contacts"`
	Limitations   []string             `json:"limitations"`
}

func spatialFrame(events []Event, ms int64) (SpatialFrame, error) {
	out := SpatialFrame{Schema: "emad.spatial-replay.v1", ElapsedMs: ms, Contacts: []SpatialContact{}, Limitations: []string{"Latest retained truth snapshot and delivered readings at or before the selected time; no simulation is executed.", "Position error uses truth at the exact sample time only. Missing truth is left unresolved; contacts can be delayed or stale."}}
	truth := map[int64]map[string]simulation.Vector{}
	latest := map[string]simulation.Reading{}
	for _, event := range events {
		if event.ElapsedMs > ms {
			continue
		}
		if event.Kind == "truth" {
			var snap simruntime.Snapshot
			if err := json.Unmarshal(event.Document, &snap); err != nil {
				return out, err
			}
			positions := map[string]simulation.Vector{}
			for _, actor := range snap.Actors {
				if actor.Active {
					positions[actor.ID] = actor.Local
				}
			}
			truth[snap.ElapsedMs] = positions
			if out.TruthSnapshot == nil || snap.ElapsedMs >= out.TruthSnapshot.ElapsedMs {
				out.TruthSnapshot = &snap
			}
		}
		if event.Kind == "perception" {
			var batch simulation.PerceptionBatch
			if err := json.Unmarshal(event.Document, &batch); err != nil {
				return out, err
			}
			for _, reading := range batch.Readings {
				old, exists := latest[reading.Contact]
				if reading.DeliverMs <= ms && (!exists || reading.DeliverMs >= old.DeliverMs) {
					latest[reading.Contact] = reading
				}
			}
		}
	}
	keys := make([]string, 0, len(latest))
	for key := range latest {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) > 256 {
		keys = keys[:256]
		out.Limitations = append(out.Limitations, "Display bounded to 256 contacts.")
	}
	for _, key := range keys {
		r := latest[key]
		v := SpatialContact{Contact: r.Contact, ActorID: r.ActorID, SampleMs: r.SampleMs, DeliverMs: r.DeliverMs, Local: r.Local, Confidence: r.Confidence}
		if p, ok := truth[r.SampleMs][r.ActorID]; ok {
			v.TruthAtSample = &p
			d := math.Sqrt(math.Pow(r.Local.X-p.X, 2) + math.Pow(r.Local.Y-p.Y, 2) + math.Pow(r.Local.Z-p.Z, 2))
			v.ErrorM = &d
		}
		out.Contacts = append(out.Contacts, v)
	}
	return out, nil
}
func (s *Service) Spatial(ctx context.Context, id string, ms int64, reason string) (SpatialFrame, error) {
	c, err := s.authorize(ctx, id, "sim.evaluation.read")
	if err != nil {
		return SpatialFrame{}, err
	}
	resource, _ := access.ResourceFrom(c)
	if resource.Partition != access.ExerciseTruth {
		return SpatialFrame{}, apperr.Forbidden("privileged spatial evaluation resource required")
	}
	reason = strings.TrimSpace(reason)
	if len(reason) < 10 || len(reason) > 500 {
		return SpatialFrame{}, apperr.Validation("provide a 10–500 character spatial evaluation reason")
	}
	idx, err := s.index(c)
	if err != nil {
		return SpatialFrame{}, err
	}
	if idx.Status != "READY" || idx.Manifest == nil {
		return SpatialFrame{}, apperr.Conflict("recording is not ready")
	}
	if ms < 0 || ms > idx.Manifest.ElapsedMs {
		return SpatialFrame{}, apperr.Validation("replay position outside recording")
	}
	events, err := s.readEvents(c, *idx.Manifest)
	if err != nil {
		return SpatialFrame{}, err
	}
	out, err := spatialFrame(events, ms)
	if err != nil {
		return out, err
	}
	// Re-authorize after private object reads. The reason is retained in the audit
	// identifier, not returned to ordinary recordings or released numeric reports.
	if err = s.audit(c, "simulation.evaluation.read", id); err != nil {
		return out, err
	}
	err = db.WithTx(c, s.Pool, func(tx pgx.Tx) error {
		return simscene.Audit(c, tx, "simulation.evaluation.spatial", id+" reason: "+reason)
	})
	if err != nil {
		return out, err
	}
	return out, nil
}
