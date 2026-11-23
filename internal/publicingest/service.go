package publicingest

import (
	"context"
	"errors"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/capacity"
	"math"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/sources"
)

type SourceRegistry interface {
	Get(context.Context, string) (sources.Source, error)
}
type ConnectorIdentities interface {
	ValidateConnector(context.Context, string, access.Resource, string) error
}

type Service struct {
	retained   GeographicRetentionReader
	identities ConnectorIdentities
	repo       *Repository
	locations  Registry
	fetcher    Fetcher
	sources    SourceRegistry
}

func NewService(repo *Repository, locations Registry, fetcher Fetcher, registry SourceRegistry, identities ConnectorIdentities) *Service {
	return &Service{repo: repo, locations: locations, fetcher: fetcher, sources: registry, identities: identities}
}

func (s *Service) SetGeographicRetentionReader(reader GeographicRetentionReader) {
	s.retained = reader
}

func requirePublic(ctx context.Context, source, permission string) error {
	if err := access.Require(ctx, permission); err != nil {
		return err
	}
	r, err := access.ResourceFrom(ctx)
	if err != nil {
		return err
	}
	if r.Partition != access.PublicEvidence {
		access.Deny(ctx, permission, "public evidence resource required")
		return apperr.Forbidden("public source operation requires PUBLIC_EVIDENCE")
	}
	return access.RequireSource(ctx, source)
}

func (s *Service) location(ctx context.Context, a Admission) (Location, error) {
	r, err := access.ResourceFrom(ctx)
	if err != nil {
		return Location{}, err
	}
	location, ok := s.locations[a.ConnectorID]
	if ok && location.GateState != "" {
		return Location{}, apperr.Validation("provider access prerequisite is missing; inspect source activation")
	}
	if !ok || location.SourceID != a.SourceID || location.Resource != r || a.Policy.MaxRequestsPerDay > location.MaxRequestsPerDay {
		return Location{}, apperr.Validation("connector location/source binding or server quota is not admitted")
	}
	if location.Adapter == FIRMSAdapter && (a.Dataset != FIRMSDataset || a.ProductVersion != FIRMSProductVersion || a.RevisionPolicy != "content_hash" || a.Policy.LookbackSeconds > 10*86400 || a.Policy.MinIntervalSeconds < 60 || a.Policy.MaxRequestsPerDay > 1000) {
		return Location{}, apperr.Validation("FIRMS requires the reviewed NOAA-20 NRT product, content revisions, at most 10 days, 60-second pacing and at most 1000 requests/day")
	}
	if location.Adapter == STACAdapter && (a.Dataset != STACDataset || a.ProductVersion != "stac-1.1-l2a" || a.RevisionPolicy != "content_hash" || a.Policy.MaxRecordsPerPage > 25 || a.Policy.MinIntervalSeconds < 5) {
		return Location{}, apperr.Validation("STAC requires the reviewed Sentinel-2 L2A product, content revisions, at most 25 items and 5-second pacing")
	}
	if location.Adapter == EarthSearchAdapter && (a.Dataset != EarthSearchDataset || a.ProductVersion != "sentinel2-c1-l2a" || a.RevisionPolicy != "content_hash" || a.Policy.MaxRecordsPerPage > 25 || a.Policy.MinIntervalSeconds < 5) {
		return Location{}, apperr.Validation("Earth Search requires the reviewed Collection 1 policy")
	}
	if location.Adapter == OpenSkyAdapter || location.Adapter == AISAdapter {
		expected := OpenSkyDataset
		pace, quota := 60, 300
		if location.Adapter == AISAdapter {
			expected = AISDataset
			pace = 10
			quota = 1000
		}
		if a.Dataset != expected || a.ProductVersion != "v1" || a.RevisionPolicy != "content_hash" || !a.Rights.AllowAnalysis || a.Policy.MaxPages != 1 || a.Policy.MaxRecordsPerPage > 200 || a.Policy.MaxResponseBytes > 1048576 || a.Policy.LookbackSeconds > 300 || a.Policy.MinIntervalSeconds < pace || a.Policy.MaxRequestsPerDay > quota || (a.Policy.AOI.East-a.Policy.AOI.West)*(a.Policy.AOI.North-a.Policy.AOI.South) > 25 || (location.Adapter == AISAdapter && a.Policy.TimeoutSeconds < 6) {
			return Location{}, apperr.Validation("moving feed requires its native v1 product, reviewed analysis rights, one bounded page, at most 200 records/1 MiB/25 square degrees/5 minute lookback and provider pacing/quota")
		}
	}

	if location.Adapter == METARAdapter || isGDACS(location.Adapter) {
		dataset, lookback, records, pace := METARDataset, 6*3600, 400, 300
		if isGDACS(location.Adapter) {
			dataset, lookback, records, pace = GDACSDataset, 7*86400, 100, 900
		}
		if a.Dataset != dataset || a.ProductVersion != "v1" || a.RevisionPolicy != "content_hash" || !a.Rights.RetainRaw || !a.Rights.AllowAnalysis || a.Policy.MaxPages != 1 || a.Policy.MaxRecordsPerPage > records || a.Policy.MaxResponseBytes > 1048576 || a.Policy.LookbackSeconds > lookback || a.Policy.MinIntervalSeconds < pace || a.Policy.MaxRequestsPerDay > 96 || len(strings.TrimSpace(a.AnalystQuestion)) < 20 || len(a.AnalystQuestion) > 1000 {
			return Location{}, apperr.Validation("context pilot requires its native product, a stated analyst question, retained analysis evidence, one bounded page, 1 MiB and provider pacing/quota")
		}
	}
	if location.Adapter == EONETAdapter && (a.Dataset != EONETDataset || a.ProductVersion != "v3" || a.RevisionPolicy != "content_hash" ||
		!a.Rights.RetainRaw || !a.Rights.AllowAnalysis || a.Policy.MaxPages != 1 || a.Policy.MaxRecordsPerPage > 100 ||
		a.Policy.MaxResponseBytes > 1048576 || a.Policy.MinIntervalSeconds < 900 || a.Policy.MaxRequestsPerDay > 24 ||
		len(strings.TrimSpace(a.AnalystQuestion)) < 20) {
		return Location{}, apperr.Validation("EONET requires its bounded flood-report profile, retained evidence and a stated analyst question")
	}
	return location, nil
}

func (s *Service) Admit(ctx context.Context, source string, in AdmissionInput, expected ...string) (Admission, error) {
	if err := requirePublic(ctx, source, "sources.manage"); err != nil {
		return Admission{}, err
	}
	principal, _, _ := access.From(ctx)
	if principal.Kind != "human" {
		return Admission{}, apperr.Forbidden("source admission requires a human reviewer")
	}
	now := time.Now().UTC()
	if err := in.Validate(now); err != nil {
		return Admission{}, err
	}
	registered, err := s.sources.Get(ctx, source)
	if err != nil {
		return Admission{}, err
	}
	if registered.Type != sources.TypeExternal {
		return Admission{}, apperr.Validation("public connector admission requires an external source")
	}
	a := Admission{AdmissionInput: in, SourceID: source, ApprovedBy: principal.ID, ApprovedAt: now}
	resource, _ := access.ResourceFrom(ctx)
	if in.Enabled {
		if err = s.identities.ValidateConnector(ctx, in.ConnectorPrincipalID, resource, source); err != nil {
			return Admission{}, err
		}
	}
	if in.Enabled {
		if _, err = s.location(ctx, a); err != nil {
			return Admission{}, err
		}
	}
	return s.repo.Admit(ctx, a, expected...)
}

func (s *Service) Admission(ctx context.Context, source string) (Admission, error) {
	if err := requirePublic(ctx, source, "sources.read"); err != nil {
		return Admission{}, err
	}
	st, err := s.repo.State(ctx, source)
	return st.Admission, err
}
func (s *Service) Health(ctx context.Context, source string) (sources.IngestionSummary, error) {
	if err := requirePublic(ctx, source, "sources.read"); err != nil {
		return sources.IngestionSummary{}, err
	}
	if _, err := s.sources.Get(ctx, source); err != nil {
		return sources.IngestionSummary{}, err
	}
	all, err := s.repo.Summaries(ctx, []string{source})
	if err != nil {
		return sources.IngestionSummary{}, err
	}
	info, ok := all[source]
	if !ok {
		return sources.IngestionSummary{State: "not_admitted", Meaning: "No admitted retrieval; missing records are not evidence of absence."}, nil
	}
	return info, nil
}

func worker(ctx context.Context, source string) error {
	if err := requirePublic(ctx, source, "connectors.execute"); err != nil {
		return err
	}
	p, _, _ := access.From(ctx)
	if p.Kind != "service" {
		access.Deny(ctx, "connectors.execute", "non-human connector identity required")
		return apperr.Forbidden("connector execution requires a service identity")
	}
	return nil
}

func liveWorker(auth, work context.Context, source, principalID string) (context.Context, error) {
	fresh, err := access.RefreshWithContext(auth, work)
	if err != nil {
		return nil, err
	}
	if err = worker(fresh, source); err != nil {
		return nil, err
	}
	p, _, _ := access.From(fresh)
	if p.ID != principalID {
		return nil, apperr.Forbidden("connector principal differs from the admission")
	}
	return preserveWork(auth, access.WithRetrievalProfile(fresh, access.RetrievalProfile(auth))), nil
}

// Execute is a bounded worker invocation, not a polling daemon. Persistent
// leases, quotas, checkpoint and backoff make repeated invocations resumable.
func (s *Service) Execute(ctx context.Context, source string) (Run, error) {
	return s.ExecuteWindow(ctx, source, nil)
}

func (s *Service) ExecuteWindow(ctx context.Context, source string, window *Window) (Run, error) {
	if err := worker(ctx, source); err != nil {
		return Run{}, err
	}
	st, err := s.repo.State(ctx, source)
	if err != nil {
		return Run{}, err
	}
	p, _, _ := access.From(ctx)
	if st.Admission.ConnectorPrincipalID != p.ID {
		return Run{}, apperr.Forbidden("connector principal differs from the admission")
	}
	location, err := s.location(ctx, st.Admission)
	if err != nil {
		return Run{}, err
	}
	if window != nil && ((location.Adapter != STACAdapter && location.Adapter != EarthSearchAdapter && location.Adapter != USGSAdapter && location.Adapter != FIRMSAdapter && location.Adapter != EONETAdapter && location.Adapter != "emad-json-page-v1") || window.Validate(time.Now()) != nil || window.To.Sub(window.From) > time.Duration(st.Admission.Policy.LookbackSeconds)*time.Second) {
		return Run{}, apperr.Validation("explicit historical windows require a supported bounded query")
	}
	work, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	bound, err := liveWorker(ctx, work, source, p.ID)
	if err != nil {
		return Run{}, err
	}
	run, st, err := s.repo.Begin(bound, source, p.ID, st.Admission.ID, time.Now().UTC(), window)
	if err != nil {
		return Run{}, err
	}
	if err = capacity.Reserve(bound, s.repo.pool, run.ID, source, "connector", int64(st.Admission.Policy.MaxPages)*int64(st.Admission.Policy.MaxResponseBytes)*4, time.Now().Add(45*time.Second)); err != nil {
		run, _ = s.repo.Finish(bound, run, "PARTIAL", "storage_or_active_capacity", time.Now().Add(time.Minute), nil, time.Now().UTC())
		return run, err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = capacity.Settle(cleanup, s.repo.pool, run.ID)
	}()
	if run.QueryAOI != nil {
		st.Admission.Policy.AOI = *run.QueryAOI
	}
	status, code := "PARTIAL", "page_budget"
	var last *Page
	var retryAt time.Time
	for i := 0; i < st.Admission.Policy.MaxPages; i++ {
		if i > 0 {
			wait := time.Until(st.NextAttempt)
			deadline, _ := work.Deadline()
			if time.Now().Add(wait + time.Duration(st.Admission.Policy.TimeoutSeconds)*time.Second).After(deadline) {
				break
			}
			if wait > 0 {
				timer := time.NewTimer(wait)
				select {
				case <-timer.C:
				case <-work.Done():
					timer.Stop()
				}
				if work.Err() != nil {
					status, code = "INTERRUPTED", "cancelled"
					break
				}
			}
		}
		bound, err = liveWorker(ctx, work, source, p.ID)
		if err != nil {
			return run, err
		}

		costID := ""
		err = s.repo.ReserveRequest(bound, run, time.Now().UTC(), location.MaxRequestsPerDay, true)
		if err == nil {
			costID, err = s.reserveCost(bound, location, run)
		}
		if err == nil {
			err = s.repo.ReserveRequest(bound, run, time.Now().UTC(), location.MaxRequestsPerDay)
		}
		if err != nil {
			if costID != "" {
				_ = s.releaseUnsentCost(bound, costID)
			}
			var app *apperr.Error
			if errors.As(err, &app) && app.Code == apperr.CodeConflict {
				status, code = "PARTIAL", app.Message
			} else {
				status, code = "FAILED", "budget_commit_failed"
			}
			break
		}
		fetchCtx, fetchCancel := context.WithTimeout(work, time.Duration(st.Admission.Policy.TimeoutSeconds)*time.Second)
		page, meta, fetchErr := s.fetcher.Fetch(fetchCtx, location, FetchRequest{Admission: st.Admission, Checkpoint: st.Checkpoint, Now: time.Now().UTC(), From: *st.QueryFrom, To: *st.QueryTo})
		fetchCancel()
		if e := s.reconcileCost(context.WithoutCancel(bound), costID, meta, fetchErr); e != nil {
			status, code = "FAILED", "cost_reconciliation_failed"
			break
		}
		if location.Adapter == OpenSkyAdapter && meta.HTTPStatus != 0 {
			run.Retrieval = &meta
		}
		if fetchErr != nil {
			status, code = "FAILED", "provider_unavailable"
			var retrievalError *RetrievalError
			if errors.As(fetchErr, &retrievalError) {
				switch retrievalError.Code {
				case "provider_unavailable", "provider_rate_limit", "provider_auth", "invalid_location", "unsupported_encoding", "response_too_large", "invalid_response", "invalid_opensky_response", "invalid_ais_frame", "ais_connection_unavailable", "ais_subscription_unavailable", "ais_subscription_not_confirmed", "ais_compression_required", "ais_subscription_rejected", "ais_unconfirmed_data", "ais_credential_echo_rejected":
					code = retrievalError.Code
				}
				retryAt = retrievalError.RetryAt
			}
			if work.Err() != nil {
				status, code = "INTERRUPTED", "cancelled"
			}
			break
		}
		now := time.Now().UTC()
		if err = validatePage(page, st.Admission, st.Checkpoint, now); err != nil || page.CoverageFrom.Before(*st.QueryFrom) || (location.Adapter != AISAdapter && page.CoverageTo.After(*st.QueryTo)) {
			status, code = "FAILED", "invalid_page"
			break
		}
		evidence := make([]Evidence, 0, len(page.Records))
		for _, record := range page.Records {
			if record.Context != nil && location.Adapter != METARAdapter && location.Adapter != EONETAdapter && !isGDACS(location.Adapter) {
				err = apperr.Validation("context records require a native normalizer")
				break
			}
			if record.Moving != nil && location.Adapter != OpenSkyAdapter && location.Adapter != AISAdapter {
				err = apperr.Validation("moving observations require a native normalizer")
				break
			}
			if !permittedReference(location, record.ReferenceURL) {
				err = apperr.Validation("unadmitted provider reference")
				break
			}
			var e Evidence
			e, err = normalizeRecord(record, st.Admission, run.ID, now)
			if err != nil {
				break
			}
			e.Retrieval = meta
			e.NormalizerVersion = location.Adapter
			evidence = append(evidence, e)
		}
		if err != nil {
			status, code = "FAILED", "invalid_record"
			break
		}
		// Authenticate again after external I/O and before durable evidence or
		// publication. Revocation cannot finish a previously authorized fetch.
		bound, err = liveWorker(ctx, work, source, p.ID)
		if err != nil {
			return run, err
		}
		run.Retrieval = &meta
		run.CaptureFrom = &page.CoverageFrom
		run.CaptureTo = &page.CoverageTo
		if err = s.repo.CommitPage(bound, run, st.Checkpoint, page, evidence, now); err != nil {
			status, code = "FAILED", "evidence_commit_failed"
			if apperr.Is(err, apperr.CodeConflict) {
				var conflict *apperr.Error
				if errors.As(err, &conflict) && conflict.Message == "source_storage_budget" {
					code = "source_storage_budget"
				} else {
					code = "revision_conflict"
				}
			}
			break
		}
		last = &page
		if page.PartialReason != "" {
			status, code = "PARTIAL", page.PartialReason
			break
		}
		st.Checkpoint = page.Checkpoint
		st.NextAttempt = now.Add(time.Duration(st.Admission.Policy.MinIntervalSeconds) * time.Second)
		if !page.HasMore {
			status, code = "SUCCEEDED", ""
			break
		}
	}
	cleanup, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cleanupCancel()
	bound, err = liveWorker(ctx, cleanup, source, p.ID)
	if err != nil {
		return run, err
	}
	now := time.Now().UTC()
	next := now.Add(time.Duration(st.Admission.Policy.MinIntervalSeconds) * time.Second)
	if status == "FAILED" {
		backoff := time.Duration(math.Min(21600, math.Pow(2, float64(min(st.Failures, 12)))*float64(max(5, st.Admission.Policy.MinIntervalSeconds)))) * time.Second
		next = now.Add(backoff)
	}
	if retryAt.After(next) {
		next = retryAt
	}
	if code == "daily_quota" || code == "account_quota" || code == "account_credits_exhausted" {
		next = time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
	}
	run, err = s.repo.Finish(bound, run, status, code, next, last, now)
	if err != nil {
		return run, err
	}
	// A failed catalog projection does not roll back or refetch durable
	// evidence. The pending receipt stays visible and can be retried alone.
	projectionBudget := 100
	if location.Adapter == OpenSkyAdapter || location.Adapter == AISAdapter || location.Adapter == METARAdapter || isGDACS(location.Adapter) {
		projectionBudget = 1000
	}
	_, _ = s.Replay(bound, source, projectionBudget)
	// Projection can exhaust its own bounded cleanup budget after evidence
	// and run completion committed. The immutable receipt gets a fresh,
	// separately authenticated deadline and records any publication gap.
	receiptContext, receiptCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer receiptCancel()
	receiptBound, e := liveWorker(ctx, receiptContext, source, p.ID)
	if e != nil {
		return run, e
	}
	if e = s.repo.RecordReceipt(receiptBound, run, location.Adapter); e != nil {
		return run, e
	}
	return run, nil
}

func validatePage(p Page, a Admission, checkpoint string, now time.Time) error {
	if len(p.Records) > a.Policy.MaxRecordsPerPage || len(p.Checkpoint) > 2048 || (p.HasMore && (p.Checkpoint == "" || p.Checkpoint == checkpoint)) {
		return apperr.Validation("invalid bounded page or non-advancing checkpoint")
	}
	if p.CoverageFrom.IsZero() || p.CoverageTo.IsZero() || !p.CoverageFrom.Before(p.CoverageTo) || p.CoverageTo.After(now.Add(5*time.Minute)) || p.CoverageTo.Sub(p.CoverageFrom) > time.Duration(a.Policy.LookbackSeconds)*time.Second {
		return apperr.Validation("invalid provider coverage interval")
	}
	return nil
}

func (s *Service) Replay(ctx context.Context, source string, limit int) (int, error) {
	if err := worker(ctx, source); err != nil {
		return 0, err
	}
	st, err := s.repo.State(ctx, source)
	if err != nil {
		return 0, err
	}
	p, _, _ := access.From(ctx)
	if p.ID != st.Admission.ConnectorPrincipalID {
		return 0, apperr.Forbidden("connector principal differs from the admission")
	}
	if !st.Admission.Enabled || (st.Admission.Rights.ExpiresAt != nil && !st.Admission.Rights.ExpiresAt.After(time.Now())) {
		return 0, apperr.Conflict("source admission is disabled or expired")
	}
	count := 0
	for count < min(max(limit, 1), 1000) {
		fresh, err := liveWorker(ctx, ctx, source, p.ID)
		if err != nil {
			return count, err
		}
		did, err := s.repo.PublishOne(fresh, source, st.Admission.ID, p.ID, time.Now().UTC())
		if err != nil {
			return count, err
		}
		if !did {
			break
		}
		count++
	}
	return count, nil
}

func (s *Service) Evidence(ctx context.Context, source, id string) (Evidence, error) {
	if err := requirePublic(ctx, source, "sources.read"); err != nil {
		return Evidence{}, err
	}
	return s.repo.GetEvidence(ctx, source, id)
}
func (s *Service) EvidenceList(ctx context.Context, source string, limit, offset int) ([]Evidence, int, error) {
	if err := requirePublic(ctx, source, "sources.read"); err != nil {
		return nil, 0, err
	}
	return s.repo.ListEvidence(ctx, source, min(max(limit, 1), 100), max(offset, 0))
}
func (s *Service) Runs(ctx context.Context, source string, limit, offset int) ([]Run, int, error) {
	if err := requirePublic(ctx, source, "sources.read"); err != nil {
		return nil, 0, err
	}
	return s.repo.Runs(ctx, source, min(max(limit, 1), 100), max(offset, 0))
}

func (s *Service) AdmissionVersion(ctx context.Context, source, id string) (Admission, error) {
	if err := requirePublic(ctx, source, "sources.read"); err != nil {
		return Admission{}, err
	}
	return s.repo.AdmissionVersion(ctx, source, id)
}

// Window is an explicit frozen discovery interval; public observations keep their rolling queries.
type Window struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

func (w Window) Validate(now time.Time) error {
	if w.From.IsZero() || w.To.IsZero() || !w.To.After(w.From) || w.To.Sub(w.From) > 31*24*time.Hour || w.To.After(now) {
		return apperr.Validation("discovery interval must be ordered, historical and at most 31 days")
	}
	return nil
}
