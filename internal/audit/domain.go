// Package audit is the system's memory. Every domain event becomes an audit
// entry, and operator-driven events are attributed to the acting operator.
package audit

import "time"

// ActorType identifies who caused an audited action.
type ActorType string

const (
	ActorSystem   ActorType = "SYSTEM"
	ActorOperator ActorType = "OPERATOR"
	ActorScenario ActorType = "SCENARIO"
	ActorAI       ActorType = "AI"
)

// Entry is one immutable record of something the system observed or did.
type Entry struct {
	ID            string
	OccurredAt    time.Time
	ActorType     ActorType
	ActorID       string
	Action        string
	SubjectType   string
	SubjectID     string
	CorrelationID string
	Data          map[string]any
}

// ListFilter narrows an audit query. Zero fields are ignored.
type ListFilter struct {
	SubjectType string
	SubjectID   string
	Action      string
	Since       *time.Time
}
