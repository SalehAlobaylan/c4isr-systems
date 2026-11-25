package main

import (
	"context"
	"encoding/json"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
	"github.com/SalehAlobaylan/c4isr-systems/internal/publicingest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"time"
)

// One-time correction of this installer's original GDACS RSS request bug:
// Accept: application/json produced HTTP 406 and a local exponential backoff.
// Preserve immutable attempts, request quota and the minimum request interval.
// This is trusted local installation maintenance, never a public retry API.
func reconcileGDACS(ctx context.Context, pool *pgxpool.Pool, resource access.Resource, source string) error {
	registry, err := publicingest.LoadRegistry(os.Getenv("C4ISR_PUBLIC_CONNECTORS_FILE"))
	if err != nil {
		return err
	}
	binding, ok := registry["local-gdacs"]
	if !ok || binding.Adapter != publicingest.GDACSRSSAdapter || binding.SourceID != source {
		return nil
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	const receipt = "aud_local_gdacs_xml_accept_v1"
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM audit_events WHERE id=$1)`, receipt).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	var previous time.Time
	var attempted *time.Time
	var failure string
	var interval, quota int
	err = tx.QueryRow(ctx, `SELECT s.next_attempt_at,s.last_attempt_at,s.last_failure_code,(a.document->'policy'->>'minIntervalSeconds')::integer,s.requests_today FROM source_connector_state s JOIN source_admissions a ON a.id=s.admission_id AND a.scope_id=s.scope_id WHERE s.scope_id=$1 AND s.source_id=$2 AND a.document->>'connectorId'='local-gdacs' AND a.document->>'connectorPrincipalId'='worker-gdacs' FOR UPDATE OF s`, resource.ID(), source).Scan(&previous, &attempted, &failure, &interval, &quota)
	if err == pgx.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	next := previous
	now := time.Now().UTC()
	if failure == "provider_unavailable" && attempted != nil {
		floor := attempted.Add(time.Duration(interval) * time.Second)
		if floor.Before(now) {
			floor = now
		}
		if next.After(floor) {
			next = floor
			if _, err = tx.Exec(ctx, `UPDATE source_connector_state SET next_attempt_at=$3 WHERE scope_id=$1 AND source_id=$2`, resource.ID(), source, next); err != nil {
				return err
			}
		}
	}
	document, _ := json.Marshal(map[string]any{"correction": "GDACS RSS now requests XML; JSON content negotiation was rejected with HTTP 406", "previousNextAttemptAt": previous, "nextAttemptAt": next, "lastAttemptAt": attempted, "retainedFailureCode": failure, "requestsTodayUnchanged": quota, "minimumIntervalSeconds": interval, "immutableAttemptsPreserved": true})
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events(id,scope_id,occurred_at,actor_type,actor_id,action,subject_type,subject_id,correlation_id,data)VALUES($1,$2,now(),'SYSTEM','internal:local-installer','local.connector.request-corrected','source',$3,'',$4)`, receipt, resource.ID(), source, document); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Reconciles the local OpenSky account budget hash when trusted local connector
// configuration updates daily credits (for example to support regional Saudi
// polling). Preserves daily spent counters, reset timestamps, and reported
// provider remaining credits.
func reconcileOpenSkyBudget(ctx context.Context, pool *pgxpool.Pool, resource access.Resource, source string) error {
	registry, err := publicingest.LoadRegistry(os.Getenv("C4ISR_PUBLIC_CONNECTORS_FILE"))
	if err != nil {
		return err
	}
	binding, ok := registry["local-opensky"]
	if !ok || binding.Adapter != publicingest.OpenSkyAdapter || binding.SourceID != source || binding.AccountBudget == nil {
		return nil
	}
	wantHash := binding.AccountBudget.PolicyHash()
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var priorHash string
	var spent int64
	err = tx.QueryRow(ctx, `SELECT policy_hash,spent FROM provider_budget_state WHERE group_key=$1 FOR UPDATE`, binding.AccountBudget.Group).Scan(&priorHash, &spent)
	if err == pgx.ErrNoRows || (err == nil && priorHash == wantHash) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE provider_budget_state SET policy_hash=$2 WHERE group_key=$1`, binding.AccountBudget.Group, wantHash); err != nil {
		return err
	}
	document, _ := json.Marshal(map[string]any{"correction": "Trusted local OpenSky account budget reconciled for regional Saudi coverage", "previousPolicyHash": priorHash, "policyHash": wantHash, "dailyCost": binding.AccountBudget.DailyCost, "spentPreserved": spent})
	if _, err = tx.Exec(ctx, `INSERT INTO audit_events(id,scope_id,occurred_at,actor_type,actor_id,action,subject_type,subject_id,correlation_id,data)VALUES($1,$2,now(),'SYSTEM','internal:local-installer','local.connector.budget-reconciled','source',$3,'',$4)`, ids.New("aud"), resource.ID(), source, document); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
