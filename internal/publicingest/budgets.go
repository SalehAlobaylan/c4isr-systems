package publicingest

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SalehAlobaylan/c4isr-systems/internal/areas"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/ids"
	"time"
)

type AccountBudget struct {
	Group              string `json:"group"`
	DailyCost          int64  `json:"dailyCost"`
	RequestCost        int64  `json:"requestCost"`
	Concurrency        int    `json:"concurrency"`
	MinIntervalSeconds int    `json:"minIntervalSeconds"`
}

func (l Location) budget() AccountBudget {
	if l.AccountBudget != nil {
		return *l.AccountBudget
	}
	// Legacy configuration has an isolated conservative request allowance.
	// Operators sharing an upstream account must bind the same explicit group.
	return AccountBudget{Group: "location:" + l.ID, DailyCost: int64(l.MaxRequestsPerDay), RequestCost: 1, Concurrency: 1, MinIntervalSeconds: 1}
}
func (b AccountBudget) validate() error {
	if _, e := normalizeIdentity(b.Group, 160); e != nil || b.DailyCost < 1 || b.DailyCost > 100000000 || b.RequestCost < 1 || b.RequestCost > b.DailyCost || b.Concurrency < 1 || b.Concurrency > 16 || b.MinIntervalSeconds < 1 || b.MinIntervalSeconds > 86400 {
		return errors.New("invalid trusted provider-account budget")
	}
	return nil
}

// reserveCost commits an uncertain charge BEFORE I/O. A lost process does not
// refund the charge. Only the concurrency lease expires; the day's cost stays
// spent until the configured UTC reset. No account identifiers reach the API.
func (s *Service) reserveCost(ctx context.Context, l Location, run Run) (string, error) {
	if e := worker(ctx, run.SourceID); e != nil {
		return "", e
	}
	b := l.budget()
	if e := b.validate(); e != nil {
		return "", e
	}
	now := time.Now().UTC()
	groupPolicy := b
	groupPolicy.RequestCost = 0
	raw, _ := json.Marshal(groupPolicy)
	hash := areas.Hash(raw)
	tx, e := s.repo.pool.Begin(ctx)
	if e != nil {
		return "", e
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	_, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "provider-budget:"+b.Group)
	if e != nil {
		return "", e
	}
	_, e = tx.Exec(ctx, `INSERT INTO provider_budget_state(group_key,day,policy_hash)VALUES($1,$2,$3)ON CONFLICT DO NOTHING`, b.Group, now.Format("2006-01-02"), hash)
	if e != nil {
		return "", e
	}
	var spent int64
	var day time.Time
	var next time.Time
	var priorHash string
	var remaining *int64
	e = tx.QueryRow(ctx, `SELECT day,spent,next_at,policy_hash,reported_remaining FROM provider_budget_state WHERE group_key=$1 FOR UPDATE`, b.Group).Scan(&day, &spent, &next, &priorHash, &remaining)
	if e != nil {
		return "", e
	}
	if priorHash != hash {
		return "", apperr.Conflict("provider_budget_configuration_changed: reconcile the trusted account policy before acquisition")
	}
	if day.Format("2006-01-02") != now.Format("2006-01-02") {
		spent = 0
		remaining = nil
	}
	if remaining != nil && *remaining < b.RequestCost {
		return "", apperr.Conflict("account_credits_exhausted")
	}
	if spent+b.RequestCost > b.DailyCost {
		return "", apperr.Conflict("account_quota")
	}
	if next.After(now) {
		return "", apperr.Conflict("account_pacing")
	}
	var active int
	e = tx.QueryRow(ctx, `SELECT count(*) FROM provider_cost_reservations WHERE group_key=$1 AND state='UNCERTAIN' AND lease_until>$2`, b.Group, now).Scan(&active)
	if e != nil {
		return "", e
	}
	if active >= b.Concurrency {
		return "", apperr.Conflict("account_concurrency")
	}
	id := ids.New("cost")
	r, _ := access.ResourceFrom(ctx)
	_, e = tx.Exec(ctx, `INSERT INTO provider_cost_reservations(id,group_key,run_id,source_id,scope_id,cost,created_at,lease_until,state)VALUES($1,$2,$3,$4,$5,$6,$7,$8,'UNCERTAIN')`, id, b.Group, run.ID, run.SourceID, r.ID(), b.RequestCost, now, now.Add(time.Duration(l.budgetTimeout())*time.Second))
	if e != nil {
		return "", e
	}
	if remaining != nil {
		*remaining -= b.RequestCost
	}
	_, e = tx.Exec(ctx, `UPDATE provider_budget_state SET day=$2,spent=$3,next_at=$4,reported_remaining=$5 WHERE group_key=$1`, b.Group, now.Format("2006-01-02"), spent+b.RequestCost, now.Add(time.Duration(b.MinIntervalSeconds)*time.Second), remaining)
	if e != nil {
		return "", e
	}
	return id, tx.Commit(ctx)
}
func (l Location) budgetTimeout() int { return 45 }
func (s *Service) reconcileCost(ctx context.Context, id string, meta Retrieval, fetchErr error) error {
	if id == "" {
		return nil
	}
	r, _ := access.ResourceFrom(ctx)
	// HTTP response observed is a known send. Neither this nor unknown sends
	// refund estimated credits. Provider remaining credits are advisory metadata.
	state := "UNCERTAIN"
	if meta.HTTPStatus != 0 {
		state = "RECONCILED"
	}
	tx, e := s.repo.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	var group string
	e = tx.QueryRow(ctx, `UPDATE provider_cost_reservations SET state=$3,credits_remaining=$4,lease_until=CASE WHEN $3='RECONCILED' THEN clock_timestamp() ELSE lease_until END WHERE id=$1 AND scope_id=$2 RETURNING group_key`, id, r.ID(), state, meta.CreditsRemaining).Scan(&group)
	if e != nil {
		return e
	}
	if meta.CreditsRemaining != nil {
		if _, e = tx.Exec(ctx, `UPDATE provider_budget_state SET reported_remaining=CASE WHEN reported_remaining IS NULL THEN $2 ELSE LEAST(reported_remaining,$2) END WHERE group_key=$1`, group, meta.CreditsRemaining); e != nil {
			return e
		}
	}
	var re *RetrievalError
	if errors.As(fetchErr, &re) {
		next := time.Now().UTC().Add(30 * time.Second)
		if re.RetryAt.After(next) {
			next = re.RetryAt
		}
		if _, e = tx.Exec(ctx, `UPDATE provider_budget_state SET next_at=GREATEST(next_at,$2) WHERE group_key=$1`, group, next); e != nil {
			return e
		}
	}
	return tx.Commit(ctx)
}

// costSummary discloses this source's attempts only, never an account's other
// sources, credentials, budget-group name or cross-resource totals.
func (s *Service) costSummary(ctx context.Context, source string) (map[string]any, error) {
	if e := requirePublic(ctx, source, "sources.read"); e != nil {
		return nil, e
	}
	r, _ := access.ResourceFrom(ctx)
	var uncertain, attempts int
	var spent int64
	e := s.repo.pool.QueryRow(ctx, `SELECT count(*)FILTER(WHERE state<>'UNSENT'),count(*)FILTER(WHERE state='UNCERTAIN'),COALESCE(sum(cost)FILTER(WHERE state<>'UNSENT'),0) FROM provider_cost_reservations WHERE scope_id=$1 AND source_id=$2 AND created_at>=(date_trunc('day',clock_timestamp() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC')`, r.ID(), source).Scan(&attempts, &uncertain, &spent)
	return map[string]any{"attemptsToday": attempts, "uncertainCharges": uncertain, "estimatedCostToday": spent, "remainingCredits": "unknown", "reconciliation": "conservative UTC daily reset; uncertain sends remain charged"}, e
}
