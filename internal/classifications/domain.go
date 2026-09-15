// Package classifications records hypotheses about what a track is. A
// classification is never absolute truth: labels may compete, carry a
// confidence, and name the method that produced them.
package classifications

import (
	"math"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
)

// Method names the origin of a classification hypothesis.
type Method string

const (
	MethodScenario  Method = "SCENARIO"
	MethodOperator  Method = "OPERATOR"
	MethodRule      Method = "RULE"
	MethodAlgorithm Method = "ALGORITHM"
	MethodAI        Method = "AI"
)

// Classification is one hypothesis about a track.
type Classification struct {
	ID              string
	TrackID         string
	Label           string
	Confidence      *float64
	Method          Method
	SourceReference string
	CreatedBy       string
	CreatedAt       time.Time
}

// CreateInput is the application input for recording a classification.
type CreateInput struct {
	TrackID         string
	Label           string
	Confidence      *float64
	Method          Method
	SourceReference string
	CreatedBy       string
}

// Normalize trims text fields and applies defaults.
func (in *CreateInput) Normalize() {
	in.TrackID = strings.TrimSpace(in.TrackID)
	in.Label = strings.TrimSpace(in.Label)
	in.SourceReference = strings.TrimSpace(in.SourceReference)
	in.CreatedBy = strings.TrimSpace(in.CreatedBy)
	if in.Method == "" {
		in.Method = MethodOperator
	}
}

// Validate checks the classification against domain rules.
func (in CreateInput) Validate() error {
	if in.TrackID == "" {
		return apperr.Validation("trackId is required")
	}
	if in.Label == "" {
		return apperr.Validation("label is required")
	}
	switch in.Method {
	case MethodScenario, MethodOperator, MethodRule, MethodAlgorithm, MethodAI:
	default:
		return apperr.Validation("method must be one of SCENARIO, OPERATOR, RULE, ALGORITHM, AI")
	}
	if in.Confidence != nil {
		confidence := *in.Confidence
		if math.IsNaN(confidence) || confidence < 0 || confidence > 1 {
			return apperr.Validation("confidence must be between 0 and 1")
		}
	}
	return nil
}
