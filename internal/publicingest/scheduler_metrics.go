package publicingest

import (
	"context"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	db "github.com/SalehAlobaylan/c4isr-systems/internal/platform/db"
)

// AcquisitionMetrics is a durable, permission-filtered operational view. Queue
// age, attempts and storage reservations survive worker/API restarts.
func (s *Service) AcquisitionMetrics(ctx context.Context, source string) (map[string]any, error) {
	out, e := s.costSummary(ctx, source)
	if e != nil {
		return nil, e
	}
	var queued, failed, attempts, complete, partial int
	var age float64
	e = db.Scoped(s.repo.pool).QueryRow(ctx, `SELECT count(*)FILTER(WHERE status IN('QUEUED','WAITING','RUNNING')),count(*)FILTER(WHERE status='FAILED'),COALESCE(max(EXTRACT(EPOCH FROM clock_timestamp()-created_at))FILTER(WHERE status IN('QUEUED','WAITING','RUNNING')),0),(SELECT COALESCE(sum(attempts),0) FROM acquisition_parts WHERE source_id=$1),(SELECT count(*) FROM retrieval_receipts WHERE source_id=$1 AND (document->>'queryComplete')::boolean),(SELECT count(*) FROM retrieval_receipts WHERE source_id=$1 AND NOT (document->>'queryComplete')::boolean) FROM acquisition_requests WHERE source_id=$1`, source).Scan(&queued, &failed, &age, &attempts, &complete, &partial)
	if e != nil {
		return nil, e
	}
	out["activeRequests"] = queued
	out["failedRequests"] = failed
	out["oldestQueueAgeSeconds"] = age
	out["attempts"] = attempts
	out["completeQueryReceipts"] = complete
	out["partialQueryReceipts"] = partial
	r, _ := access.ResourceFrom(ctx)
	var reserved int64
	var active int
	e = s.repo.pool.QueryRow(ctx, `SELECT COALESCE(sum(estimated_bytes),0),count(*) FROM resource_capacity_reservations WHERE scope_id=$1 AND source_id=$2 AND settled_at IS NULL AND lease_until>clock_timestamp()`, r.ID(), source).Scan(&reserved, &active)
	if e != nil {
		return nil, e
	}
	out["reservedStorageBytes"] = reserved
	out["activeAttempts"] = active
	out["storageRecoveryOwner"] = "source administrator"
	out["quotaRecoveryOwner"] = "connector operator"
	return out, nil
}
