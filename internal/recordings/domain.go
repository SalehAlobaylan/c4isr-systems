// Package recordings archives bounded native output. Playback never executes a
// simulator, dispatches a command, or consults mutable COP state.
package recordings

import (
	"encoding/json"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"time"
)

func DecodeCheckpoint(raw []byte, m Manifest) (Checkpoint, error) {
	var out Checkpoint
	if json.Unmarshal(raw, &out) != nil || out.Schema != Schema || m.Schema != Schema || out.Sequence != m.Events || out.ElapsedMs != m.ElapsedMs {
		return out, apperr.Conflict("checkpoint schema or recording position incompatible")
	}
	return out, nil
}

const Schema = "emad.recording.v1"
const MetricVersion = "emad.abstract-evaluation.v2"
const MaxAttempts = 3
const ChunkBytes = 2 * 1024 * 1024
const MaxArchiveBytes = 160 * 1024 * 1024

type Event struct {
	Sequence   int             `json:"sequence"`
	ElapsedMs  int64           `json:"elapsedMs"`
	ReceivedAt time.Time       `json:"receivedAt"`
	Kind       string          `json:"kind"`
	Entity     string          `json:"entity"`
	Operation  string          `json:"operation"`
	Document   json.RawMessage `json:"document"`
}
type Blob struct {
	SHA256 string `json:"sha256"`
	Bytes  int    `json:"bytes"`
	First  int    `json:"first"`
	Last   int    `json:"last"`
	Key    string `json:"-"`
}
type Checkpoint struct {
	Schema    string                                `json:"schema"`
	Sequence  int                                   `json:"sequence"`
	ElapsedMs int64                                 `json:"elapsedMs"`
	State     map[string]map[string]json.RawMessage `json:"state"`
}
type Manifest struct {
	Schema            string    `json:"schema"`
	RunID             string    `json:"runId"`
	Partition         string    `json:"partition"`
	Epoch             time.Time `json:"epoch"`
	ElapsedMs         int64     `json:"elapsedMs"`
	Status            string    `json:"runStatus"`
	ProjectionSettled bool      `json:"projectionSettled"`
	Events            int       `json:"events"`
	Chunks            []Blob    `json:"chunks"`
	Checkpoint        Blob      `json:"checkpoint"`
	ContentSHA256     string    `json:"contentSha256"`
	CreatedAt         time.Time `json:"createdAt"`
	RetainUntil       time.Time `json:"retainUntil"`
	ReplayGuarantee   string    `json:"replayGuarantee"`
}
type Index struct {
	Status      string     `json:"status"`
	ErrorCode   string     `json:"errorCode"`
	Attempts    int        `json:"attempts"`
	MaxAttempts int        `json:"maxAttempts"`
	RetryAt     *time.Time `json:"retryAt,omitempty"`
	Manifest    *Manifest  `json:"manifest,omitempty"`
}
type Frame struct {
	Schema    string                                `json:"schema"`
	ElapsedMs int64                                 `json:"elapsedMs"`
	Sequence  int                                   `json:"sequence"`
	State     map[string]map[string]json.RawMessage `json:"state"`
	Counts    map[string]int                        `json:"counts"`
	Truncated bool                                  `json:"truncated"`
}

func apply(state map[string]map[string]json.RawMessage, e Event) {
	if state[e.Kind] == nil {
		state[e.Kind] = map[string]json.RawMessage{}
	}
	if e.Operation == "DELETE" {
		delete(state[e.Kind], e.Entity)
	} else {
		state[e.Kind][e.Entity] = e.Document
	}
}
