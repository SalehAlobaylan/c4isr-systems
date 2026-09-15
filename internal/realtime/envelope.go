// Package realtime delivers domain events to operator interfaces over
// WebSocket. Transport payloads are explicit maps so that internal event
// structs never become the permanent browser contract.
package realtime

import (
	"time"
)

// EnvelopeVersion is the schema version carried by every realtime message.
const EnvelopeVersion = 1

// Envelope is the versioned realtime message contract.
type Envelope struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Version    int            `json:"version"`
	OccurredAt time.Time      `json:"occurredAt"`
	Data       map[string]any `json:"data"`
}
