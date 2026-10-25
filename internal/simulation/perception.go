package simulation

// Reading crosses only the authenticated runtime transport. ActorID is private
// and is removed before projection into ordinary exercise evidence.
type Reading struct {
	ID         string  `json:"id"`
	ActorID    string  `json:"actorId"`
	Contact    string  `json:"contact"`
	SensorID   string  `json:"sensorId"`
	AssetID    string  `json:"assetId,omitempty"`
	Kind       string  `json:"kind"`
	SampleMs   int64   `json:"sampleMs"`
	DeliverMs  int64   `json:"deliverMs"`
	Local      Vector  `json:"local"`
	Heading    float64 `json:"heading"`
	Confidence float64 `json:"confidence"`
	AccuracyM  float64 `json:"accuracyM"`
}
type ControlledBinding struct {
	ActorID string `json:"actorId"`
	AssetID string `json:"assetId"`
}
type RuntimeTask struct {
	CommandID  string `json:"commandId"`
	AssetID    string `json:"assetId"`
	Type       string `json:"type"`
	IssuedMs   int64  `json:"issuedMs"`
	DeadlineMs int64  `json:"deadlineMs"`
}
type TaskOutcome struct {
	CommandID string `json:"commandId"`
	State     string `json:"state"`
	ElapsedMs int64  `json:"elapsedMs"`
	Code      string `json:"code"`
}
type PerceptionBatch struct {
	Readings []Reading     `json:"readings,omitempty"`
	Outcomes []TaskOutcome `json:"outcomes,omitempty"`
}
