// Package eventrules evaluates reported event geometry with PostGIS. It never
// correlates events into tracks or interprets a point as a hazard perimeter.
package eventrules

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/events"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	platformdb "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Evaluate runs inside the caller's evidence/publication transaction. The
// geofence's stored geometry and updated time identify the rule snapshot.
func Evaluate(ctx context.Context, tx pgx.Tx, observationID, evidenceID string) error {
	rows, err := tx.Query(ctx, `SELECT g.id,g.name,g.severity,g.updated_at,ST_AsGeoJSON(g.geometry),o.source_id,o.observation_type,o.semantic_kind,o.geometry_role
 FROM observations o JOIN geofences g ON g.active AND CASE WHEN o.area IS NOT NULL THEN ST_Intersects(g.geometry,o.area::geometry) ELSE ST_Covers(g.geometry,o.position::geometry) END
 WHERE o.id=$1 AND o.semantic_kind IN ('point_event','area') ORDER BY g.id LIMIT 101`, observationID)
	if err != nil {
		return err
	}
	type match struct {
		id, name, severity, geometry, source, typ, semantic, role string
		version                                                   time.Time
	}
	matches := []match{}
	for rows.Next() {
		var m match
		if err = rows.Scan(&m.id, &m.name, &m.severity, &m.version, &m.geometry, &m.source, &m.typ, &m.semantic, &m.role); err != nil {
			rows.Close()
			return err
		}
		matches = append(matches, m)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(matches) > 100 {
		return fmt.Errorf("event rule match budget exceeded")
	}
	for _, m := range matches {
		reference, _ := json.Marshal(map[string]any{"rule": "public.event.spatial.v1", "ruleVersion": m.version.UTC().Format(time.RFC3339Nano), "ruleGeometry": json.RawMessage(m.geometry), "observationId": observationID, "evidenceId": evidenceID, "sourceId": m.source, "semanticKind": m.semantic, "geometryRole": m.role, "geofenceId": m.id, "reportedType": m.typ})
		_, err = tx.Exec(ctx, `INSERT INTO alerts(id,type,severity,title,message,geofence_id,source_reference) SELECT $1,'public.event.spatial',$2,$3,$4,$5,$6 WHERE NOT EXISTS(SELECT 1 FROM alerts WHERE type='public.event.spatial' AND geofence_id=$5 AND source_reference->>'observationId'=$7)`, ids.New("alr"), m.severity, "Reported "+m.typ+" intersects "+m.name, "Provider-reported geometry matched a stored zone; this does not establish an impact area.", m.id, reference, observationID)
		if err != nil {
			return err
		}
	}
	return nil
}

// New subscribes only to direct point-event inputs. Existing area rules retain
// their established behavior; normalized revision publication calls Evaluate
// atomically with its own evidence/current-state transaction.
func New(pool *pgxpool.Pool, bus *events.Dispatcher) {
	bus.Subscribe(events.TopicObservationReceived, func(ctx context.Context, ev events.Event) {
		o, ok := ev.(events.ObservationReceived)
		if !ok || o.SemanticKind != "point_event" {
			return
		}
		if access.Require(ctx, "geofences.evaluate") != nil {
			return
		}
		err := platformdb.WithTx(ctx, pool, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `SELECT id FROM observations WHERE id=$1 FOR UPDATE`, o.ObservationID); err != nil {
				return err
			}
			if err := Evaluate(ctx, tx, o.ObservationID, ""); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `UPDATE observations SET processed_at=now() WHERE id=$1`, o.ObservationID)
			return err
		})
		if err != nil {
			slog.Default().Error("point-event spatial projection failed", "observation_id", o.ObservationID)
		}
	})
}
