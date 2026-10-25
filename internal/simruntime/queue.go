package simruntime

import (
	"context"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
	"github.com/SalehAlobaylan/c4isr-systems/internal/simscene"
)

type Capability struct {
	RuntimeBuild   string `json:"runtimeBuild"`
	RuntimeVersion string `json:"runtimeVersion"`
	State          string `json:"state"`
}

func (b *Broker) Capabilities(ctx context.Context) ([]Capability, error) {
	if e := access.Require(ctx, "sim.runtime.control"); e != nil {
		return nil, e
	}
	out := []Capability{}
	rows, e := db.Scoped(b.Pool).Query(ctx, `SELECT DISTINCT runtime_build FROM platform_worker_status WHERE feature='runtime' AND last_seen_at>now()-interval '15 seconds' ORDER BY runtime_build`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	for rows.Next() {
		var build string
		if e = rows.Scan(&build); e != nil {
			return nil, e
		}
		for _, admitted := range b.AdmittedBuilds {
			if admitted == build {
				out = append(out, Capability{build, simscene.PerceptionRuntime, "READY"})
				out = append(out, Capability{build, simscene.RuntimeVersion, "READY"})
			}
		}
	}
	return out, rows.Err()
}
func (b *Broker) Queue(ctx context.Context) ([]Run, error) {
	if e := worker(ctx); e != nil {
		return nil, e
	}
	if e := b.expire(ctx); e != nil {
		return nil, e
	}
	rows, e := db.Scoped(b.Pool).Query(ctx, runSelect+`WHERE status='QUEUED' ORDER BY created_at,id LIMIT 4`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Run{}
	for rows.Next() {
		v, e := scanRun(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
