// Package assessments models analytical conclusions drawn about operational
// subjects. Assessments carry method, confidence, and explicit evidence links,
// and never mutate operational state.
package assessments

import (
	"math"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
)

// Method identifies how a conclusion was produced.
type Method string

const (
	MethodOperator  Method = "OPERATOR"
	MethodRule      Method = "RULE"
	MethodAlgorithm Method = "ALGORITHM"
	MethodAI        Method = "AI"
)

// SubjectType identifies the kind of entity an assessment is about.
type SubjectType string

const (
	SubjectTrack       SubjectType = "track"
	SubjectAsset       SubjectType = "asset"
	SubjectIncident    SubjectType = "incident"
	SubjectObservation SubjectType = "observation"
	SubjectSource      SubjectType = "source"
)

// EvidenceType identifies the kind of record supporting an assessment.
type EvidenceType string

const (
	EvidenceObservation    EvidenceType = "observation"
	EvidenceTrack          EvidenceType = "track"
	EvidenceAsset          EvidenceType = "asset"
	EvidenceIncident       EvidenceType = "incident"
	EvidenceSource         EvidenceType = "source"
	EvidenceClassification EvidenceType = "classification"
	EvidenceAlert          EvidenceType = "alert"
)

// Evidence links an assessment to a record it was derived from.
type Evidence struct {
	Type    EvidenceType
	ID      string
	AddedAt time.Time
}

// Assessment is an analytical conclusion about a subject.
type Assessment struct {
	ID          string
	SubjectType SubjectType
	SubjectID   string
	Type        string
	Conclusion  string
	Confidence  *float64
	Method      Method
	CreatedBy   string
	CreatedAt   time.Time
	Evidence    []Evidence
}

// CreateInput is the application input for recording an assessment.
type CreateInput struct {
	SubjectType SubjectType
	SubjectID   string
	Type        string
	Conclusion  string
	Confidence  *float64
	Method      Method
	CreatedBy   string
	Evidence    []Evidence
}

// Normalize trims fields and applies defaults.
func (in *CreateInput) Normalize() {
	in.SubjectID = strings.TrimSpace(in.SubjectID)
	in.Type = strings.TrimSpace(in.Type)
	in.Conclusion = strings.TrimSpace(in.Conclusion)
	in.CreatedBy = strings.TrimSpace(in.CreatedBy)
	if in.Method == "" {
		in.Method = MethodOperator
	}
	for i := range in.Evidence {
		in.Evidence[i].ID = strings.TrimSpace(in.Evidence[i].ID)
	}
}

// Validate checks create input against domain rules.
func (in CreateInput) Validate() error {
	switch in.SubjectType {
	case SubjectTrack, SubjectAsset, SubjectIncident, SubjectObservation, SubjectSource:
	default:
		return apperr.Validation("subject type must be one of track, asset, incident, observation, source")
	}
	if in.SubjectID == "" {
		return apperr.Validation("subject id is required")
	}
	if in.Type == "" {
		return apperr.Validation("assessment type is required")
	}
	if in.Conclusion == "" {
		return apperr.Validation("conclusion is required")
	}
	switch in.Method {
	case MethodOperator, MethodRule, MethodAlgorithm, MethodAI:
	default:
		return apperr.Validation("method must be one of OPERATOR, RULE, ALGORITHM, AI")
	}
	if in.Confidence != nil {
		if math.IsNaN(*in.Confidence) || *in.Confidence < 0 || *in.Confidence > 1 {
			return apperr.Validation("confidence must be between 0 and 1")
		}
	}
	for _, e := range in.Evidence {
		if err := e.Type.Validate(); err != nil {
			return err
		}
		if e.ID == "" {
			return apperr.Validation("evidence id is required")
		}
	}
	return nil
}

// Validate checks an evidence type against domain rules.
func (t EvidenceType) Validate() error {
	switch t {
	case EvidenceObservation, EvidenceTrack, EvidenceAsset, EvidenceIncident, EvidenceSource, EvidenceClassification, EvidenceAlert:
		return nil
	default:
		return apperr.Validation("evidence type must be one of observation, track, asset, incident, source, classification, alert")
	}
}
