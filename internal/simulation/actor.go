package simulation

// SimulationActor is a runtime instance reference, distinct from a visual model,
// reusable ActorTemplate, public ExternalTrack or exercise-controlled Asset.
// Actor IDs belong to protected runtime/evaluator contracts, never ordinary COP
// payloads. S1.1 defines the reference; the S2 runtime will own instantiation.
type SimulationActor struct {
	ID             string `json:"id"`
	RunID          string `json:"runId"`
	TemplateID     string `json:"templateId"`
	ModelVersionID string `json:"modelVersionId"`
	RuntimeBuild   string `json:"runtimeBuild"`
}
