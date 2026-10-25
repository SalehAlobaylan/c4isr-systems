// Package publicingest admits public sources and retains provider evidence.
// Retrieval, evidence identity, catalog publication, and later analysis are
// separate operations. The externaltracks domain owns moving hypotheses.
package publicingest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"math"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
)

type Bounds struct {
	West  float64 `json:"west"`
	South float64 `json:"south"`
	East  float64 `json:"east"`
	North float64 `json:"north"`
}

func (b Bounds) Validate() error {
	for _, n := range []float64{b.West, b.South, b.East, b.North} {
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return apperr.Validation("AOI coordinates must be finite")
		}
	}
	if b.West < -180 || b.East > 180 || b.South < -90 || b.North > 90 || b.West >= b.East || b.South >= b.North || (b.East-b.West)*(b.North-b.South) > 400 {
		return apperr.Validation("AOI must be a bounded WGS84 box of at most 400 square degrees; split dateline crossings")
	}
	return nil
}

type Rights struct {
	ID                    string     `json:"id"`
	LegalBasis            string     `json:"legalBasis"`
	TermsURL              string     `json:"termsUrl"`
	Attribution           string     `json:"attribution"`
	RetainRaw             bool       `json:"retainRaw"`
	AllowAnalysis         bool       `json:"allowAnalysis"`
	AllowSimulationExport bool       `json:"allowSimulationExport"`
	ExpiresAt             *time.Time `json:"expiresAt,omitempty"`
}

type Policy struct {
	AOI                     Bounds `json:"aoi"`
	LookbackSeconds         int    `json:"lookbackSeconds"`
	FreshForSeconds         int    `json:"freshForSeconds"`
	UnavailableAfterSeconds int    `json:"unavailableAfterSeconds"`
	MinIntervalSeconds      int    `json:"minIntervalSeconds"`
	TimeoutSeconds          int    `json:"timeoutSeconds"`
	MaxPages                int    `json:"maxPages"`
	MaxRecordsPerPage       int    `json:"maxRecordsPerPage"`
	MaxResponseBytes        int    `json:"maxResponseBytes"`
	MaxRequestsPerDay       int    `json:"maxRequestsPerDay"`
}

type AdmissionInput struct {
	AnalystQuestion      string `json:"analystQuestion,omitempty"`
	OriginalProducer     string `json:"originalProducer"`
	UpstreamProvider     string `json:"upstreamProvider"`
	ConnectorID          string `json:"connectorId"`
	ConnectorPrincipalID string `json:"connectorPrincipalId"`
	Dataset              string `json:"dataset"`
	ProductVersion       string `json:"productVersion"`
	CoverageDescription  string `json:"coverageDescription"`
	RevisionPolicy       string `json:"revisionPolicy"`
	Rights               Rights `json:"rights"`
	Policy               Policy `json:"policy"`
	Enabled              bool   `json:"enabled"`
}

type Admission struct {
	AdmissionInput
	ID         string    `json:"id"`
	SourceID   string    `json:"sourceId"`
	Version    int       `json:"version"`
	ApprovedBy string    `json:"approvedBy"`
	ApprovedAt time.Time `json:"approvedAt"`
}

func (in AdmissionInput) Validate(now time.Time) error {
	if in.AnalystQuestion != "" {
		if _, err := normalizeIdentity(in.AnalystQuestion, 1000); err != nil {
			return apperr.Validation("invalid analyst question")
		}
	}
	for _, v := range []string{in.OriginalProducer, in.UpstreamProvider, in.ConnectorID, in.ConnectorPrincipalID, in.Dataset, in.ProductVersion, in.CoverageDescription, in.Rights.ID, in.Rights.LegalBasis, in.Rights.Attribution} {
		if _, err := normalizeIdentity(v, 1000); err != nil {
			return apperr.Validation("complete producer/provider, connector, dataset, coverage and rights metadata is required")
		}
	}
	if err := publicReference(in.Rights.TermsURL); err != nil {
		return apperr.Validation("rights terms require a credential-free HTTPS reference")
	}
	if in.Enabled && in.Rights.ExpiresAt != nil && !in.Rights.ExpiresAt.After(now) {
		return apperr.Validation("source rights have expired")
	}
	if in.RevisionPolicy != "provider_version" && in.RevisionPolicy != "content_hash" {
		return apperr.Validation("revisionPolicy must be provider_version or content_hash")
	}
	p := in.Policy
	if err := p.AOI.Validate(); err != nil {
		return err
	}
	if p.LookbackSeconds < 1 || p.LookbackSeconds > 31*86400 || p.FreshForSeconds < 1 || p.UnavailableAfterSeconds <= p.FreshForSeconds || p.UnavailableAfterSeconds > 90*86400 || p.MinIntervalSeconds < 1 || p.MinIntervalSeconds > 86400 || p.TimeoutSeconds < 1 || p.TimeoutSeconds > 10 || p.MaxPages < 1 || p.MaxPages > 10 || p.MaxRecordsPerPage < 1 || p.MaxRecordsPerPage > 1000 || p.MaxResponseBytes < 128 || p.MaxResponseBytes > 4*1024*1024 || p.MaxRequestsPerDay < 1 || p.MaxRequestsPerDay > 10000 {
		return apperr.Validation("retrieval/freshness policy exceeds bounded execution limits")
	}
	return nil
}

// Record is an adapter result, never a claim that a provider's interpretation
// is authoritative. OriginalRecord is retained only when reviewed rights allow.
type Record struct {
	Context           *ContextRecord       `json:"context,omitempty"`
	Moving            *MovingObservation   `json:"moving,omitempty"`
	Scene             json.RawMessage      `json:"scene,omitempty"`
	OriginalBytes     []byte               `json:"originalBytes,omitempty"`
	OriginalMediaType string               `json:"originalMediaType,omitempty"`
	Observation       *ReportedObservation `json:"observation,omitempty"`
	ProviderRecordID  string               `json:"providerRecordId"`
	ProviderRevision  string               `json:"providerRevision"`
	ObservedAt        *time.Time           `json:"observedAt,omitempty"`
	ProviderUpdatedAt *time.Time           `json:"providerUpdatedAt,omitempty"`
	ReferenceURL      string               `json:"referenceUrl"`
	OriginalRecord    json.RawMessage      `json:"originalRecord"`
}

type Page struct {
	PartialReason   string    `json:"partialReason,omitempty"`
	ExcludedRecords int       `json:"excludedRecords,omitempty"`
	Records         []Record  `json:"records"`
	Checkpoint      string    `json:"checkpoint"`
	HasMore         bool      `json:"hasMore"`
	CoverageFrom    time.Time `json:"coverageFrom"`
	CoverageTo      time.Time `json:"coverageTo"`
}

type Evidence struct {
	Context           *ContextRecord       `json:"context,omitempty"`
	Moving            *MovingObservation   `json:"moving,omitempty"`
	Scene             json.RawMessage      `json:"scene,omitempty"`
	OriginalBytes     []byte               `json:"-"`
	OriginalMediaType string               `json:"originalMediaType,omitempty"`
	NormalizerVersion string               `json:"normalizerVersion,omitempty"`
	Observation       *ReportedObservation `json:"observation,omitempty"`
	OriginRecordKey   string               `json:"originRecordKey"`
	Retrieval         Retrieval            `json:"retrieval"`
	ID                string               `json:"id"`
	SourceID          string               `json:"sourceId"`
	AdmissionID       string               `json:"admissionId"`
	ConnectorRunID    string               `json:"connectorRunId"`
	ProviderRecordID  string               `json:"providerRecordId"`
	ProviderRevision  string               `json:"providerRevision"`
	ContentHash       string               `json:"contentHash"`
	RawHash           string               `json:"rawHash"`
	HashVersion       string               `json:"hashVersion"`
	OriginalProducer  string               `json:"originalProducer"`
	UpstreamProvider  string               `json:"upstreamProvider"`
	Dataset           string               `json:"dataset"`
	ProductVersion    string               `json:"productVersion"`
	RightsID          string               `json:"rightsId"`
	ReferenceURL      string               `json:"referenceUrl"`
	ObservedAt        *time.Time           `json:"observedAt,omitempty"`
	ProviderUpdatedAt *time.Time           `json:"providerUpdatedAt,omitempty"`
	RetrievedAt       time.Time            `json:"retrievedAt"`
	OriginalRecord    json.RawMessage      `json:"originalRecord,omitempty"`
}

type Run struct {
	Retrieval   *Retrieval `json:"retrieval,omitempty"`
	CaptureFrom *time.Time `json:"captureFrom,omitempty"`
	CaptureTo   *time.Time `json:"captureTo,omitempty"`
	ID          string     `json:"id"`
	SourceID    string     `json:"sourceId"`
	AdmissionID string     `json:"admissionId"`
	ConnectorID string     `json:"connectorId"`
	PrincipalID string     `json:"principalId"`
	StartedAt   time.Time  `json:"startedAt"`
	FinishedAt  *time.Time `json:"finishedAt,omitempty"`
	Status      string     `json:"status"`
	FailureCode string     `json:"failureCode,omitempty"`
	Requests    int        `json:"requests"`
	NewEvidence int        `json:"newEvidence"`
	Duplicates  int        `json:"duplicates"`
}

func normalizeIdentity(v string, max int) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" || len(v) > max || !utf8.ValidString(v) {
		return "", apperr.Validation("invalid provider identity")
	}
	for _, r := range v {
		if unicode.IsControl(r) {
			return "", apperr.Validation("provider identity contains control characters")
		}
	}
	return v, nil // Provider identifiers are case-sensitive; do not lowercase.
}

func publicReference(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return apperr.Validation("invalid public reference")
	}
	return nil
}

func normalizeRecord(r Record, a Admission, runID string, now time.Time) (Evidence, error) {
	if r.Context != nil {
		if r.Moving != nil || r.Observation != nil || len(r.Scene) > 0 || r.Context.Validate(now) != nil || !within(a.Policy.AOI, r.Context.Position) {
			return Evidence{}, apperr.Validation("invalid contextual record or admitted AOI")
		}
	}
	if r.Moving != nil {
		if err := r.Moving.Validate(now); err != nil {
			return Evidence{}, err
		}
		if r.Observation != nil || len(r.Scene) > 0 || !within(a.Policy.AOI, r.Moving.Position) {
			return Evidence{}, apperr.Validation("moving record semantics or admitted AOI mismatch")
		}
		at := r.Moving.ObservedAt
		r.ObservedAt = &at
	}
	if r.Observation != nil {
		if err := r.Observation.Validate(now); err != nil {
			return Evidence{}, err
		}
		if !r.Observation.Withdrawn {
			if r.ObservedAt == nil {
				at := r.Observation.ObservedAt
				r.ObservedAt = &at
			} else if !r.ObservedAt.Equal(r.Observation.ObservedAt) {
				return Evidence{}, apperr.Validation("normalized origin time differs from provider observation time")
			}
		}
	}
	id, err := normalizeIdentity(r.ProviderRecordID, 512)
	if err != nil {
		return Evidence{}, err
	}
	if len(r.OriginalRecord) == 0 || len(r.OriginalRecord) > 256*1024 || !utf8.Valid(r.OriginalRecord) {
		return Evidence{}, apperr.Validation("provider record exceeds retention limits")
	}
	decoder := json.NewDecoder(bytes.NewReader(r.OriginalRecord))
	decoder.UseNumber()
	var value any
	if err = decoder.Decode(&value); err != nil {
		return Evidence{}, apperr.Validation("provider record must be valid JSON")
	}
	if err = decoder.Decode(new(any)); err != io.EOF {
		return Evidence{}, apperr.Validation("provider record contains trailing JSON")
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return Evidence{}, apperr.Validation("provider record cannot be normalized")
	}
	h := sha256.Sum256(canonical)
	content := hex.EncodeToString(h[:])
	retained := []byte(r.OriginalRecord)
	if r.OriginalBytes != nil {
		if (r.OriginalMediaType != "text/csv" && r.OriginalMediaType != "application/xml") || len(r.OriginalBytes) < 1 || len(r.OriginalBytes) > 256*1024 || !utf8.Valid(r.OriginalBytes) {
			return Evidence{}, apperr.Validation("invalid original structured record bytes")
		}
		retained = r.OriginalBytes
	} else if r.OriginalMediaType != "" && r.OriginalMediaType != "application/json" {
		return Evidence{}, apperr.Validation("unsupported original media type")
	}
	rawHash := sha256.Sum256(retained)
	revision := "sha256:" + content
	if a.RevisionPolicy == "provider_version" {
		revision, err = normalizeIdentity(r.ProviderRevision, 512)
		if err != nil {
			return Evidence{}, err
		}
	}
	if r.ReferenceURL != "" {
		if err = publicReference(r.ReferenceURL); err != nil {
			return Evidence{}, err
		}
	}
	if !a.Rights.RetainRaw && r.ReferenceURL == "" {
		return Evidence{}, apperr.Validation("reference-only rights require a retained provider reference")
	}
	for _, at := range []*time.Time{r.ObservedAt, r.ProviderUpdatedAt} {
		if at != nil && (at.IsZero() || at.After(now.Add(5*time.Minute))) {
			return Evidence{}, apperr.Validation("invalid provider timestamp")
		}
	}
	origin, _ := json.Marshal([]string{a.OriginalProducer, a.Dataset, a.ProductVersion, id, revision})
	originHash := sha256.Sum256(origin)
	identity, _ := json.Marshal([]string{a.SourceID, id, revision})
	key := sha256.Sum256(identity)
	e := Evidence{Context: r.Context, Moving: r.Moving, Scene: r.Scene, OriginalMediaType: r.OriginalMediaType, Observation: r.Observation, OriginRecordKey: "origin_" + hex.EncodeToString(originHash[:]), ID: "evi_" + hex.EncodeToString(key[:]), SourceID: a.SourceID, AdmissionID: a.ID, ConnectorRunID: runID,
		ProviderRecordID: id, ProviderRevision: revision, ContentHash: content, RawHash: hex.EncodeToString(rawHash[:]), HashVersion: "emad-json-v1",
		OriginalProducer: a.OriginalProducer, UpstreamProvider: a.UpstreamProvider, Dataset: a.Dataset, ProductVersion: a.ProductVersion,
		RightsID: a.Rights.ID, ReferenceURL: r.ReferenceURL, ObservedAt: r.ObservedAt, ProviderUpdatedAt: r.ProviderUpdatedAt, RetrievedAt: now}
	if a.Rights.RetainRaw {
		if r.OriginalMediaType == "text/csv" || r.OriginalMediaType == "application/xml" {
			e.OriginalBytes = append([]byte(nil), retained...)
		} else {
			e.OriginalRecord = append(json.RawMessage(nil), r.OriginalRecord...)
		}
	}
	return e, nil
}
