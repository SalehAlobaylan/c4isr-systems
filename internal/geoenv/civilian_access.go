package geoenv

import (
	"bytes"
	"cmp"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/artifacts"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/httpx"
)

const civilianAccessSchema = "emad.civilian-road-exercise.v1"

type accessExercise struct {
	Schema                 string              `json:"schema"`
	Title                  string              `json:"title"`
	Kind                   string              `json:"kind"`
	GraphEdition           string              `json:"graphEdition"`
	RetainedManifestSHA256 string              `json:"retainedManifestSha256,omitempty"`
	ServiceClass           string              `json:"serviceClass"`
	WindowFrom             time.Time           `json:"windowFrom"`
	WindowTo               time.Time           `json:"windowTo"`
	Sources                []exerciseSource    `json:"sources"`
	Origin                 string              `json:"origin"`
	Nodes                  []string            `json:"nodes"`
	BoundaryNodes          []string            `json:"boundaryNodes"`
	Destinations           []accessDestination `json:"destinations"`
	Edges                  []accessEdge        `json:"edges"`
	TurnRestrictions       []accessTurn        `json:"turnRestrictions"`
	Reports                []accessReport      `json:"reports"`
	Associations           []reportAssociation `json:"associations"`
	Inspections            []accessInspection  `json:"inspections"`
	Permits                []accessPermit      `json:"permits"`
	Population             *exercisePopulation `json:"population,omitempty"`
	Restoration            []accessRestoration `json:"restoration,omitempty"`
	Assignments            []accessAssignment  `json:"assignments,omitempty"`
}

type exerciseSource struct {
	ID          string `json:"id"`
	Edition     string `json:"edition"`
	SHA256      string `json:"sha256"`
	Attribution string `json:"attribution"`
}

type accessDestination struct {
	ID    string `json:"id"`
	Node  string `json:"node,omitempty"`
	Label string `json:"label"`
}

type accessEdge struct {
	ID                   string  `json:"id"`
	From                 string  `json:"from"`
	To                   string  `json:"to"`
	DistanceM            float64 `json:"distanceM"`
	SourceID             string  `json:"sourceId"`
	FeatureID            string  `json:"featureId"`
	RetainedOSMFeatureID string  `json:"retainedOSMFeatureId,omitempty"`
}

type accessTurn struct {
	FromEdge string `json:"fromEdge"`
	ToEdge   string `json:"toEdge"`
}

type accessReport struct {
	ID             string     `json:"id"`
	SourceID       string     `json:"sourceId"`
	Category       string     `json:"category"`
	PlaceLabel     string     `json:"placeLabel,omitempty"`
	LocalText      string     `json:"localText,omitempty"`
	ReceiptAt      time.Time  `json:"receiptAt"`
	AvailableAt    time.Time  `json:"availableAt"`
	TicketClosedAt *time.Time `json:"ticketClosedAt,omitempty"`
}

type accessAssignment struct {
	ID             string     `json:"id"`
	SourceID       string     `json:"sourceId"`
	ReportID       string     `json:"reportId"`
	CrewLabel      string     `json:"crewLabel"`
	DispatchedAt   time.Time  `json:"dispatchedAt"`
	AvailableAt    time.Time  `json:"availableAt"`
	ArrivedAt      *time.Time `json:"arrivedAt,omitempty"`
	ArrivalKnownAt *time.Time `json:"arrivalKnownAt,omitempty"`
}

type reportAssociation struct {
	ReportID    string    `json:"reportId"`
	OtherID     string    `json:"otherId"`
	Reviewer    string    `json:"reviewer"`
	Decision    string    `json:"decision"`
	AvailableAt time.Time `json:"availableAt"`
}

type accessInspection struct {
	ID          string    `json:"id"`
	SourceID    string    `json:"sourceId"`
	ReportID    string    `json:"reportId,omitempty"`
	EdgeID      string    `json:"edgeId"`
	ObservedAt  time.Time `json:"observedAt"`
	AvailableAt time.Time `json:"availableAt"`
	Reviewer    string    `json:"reviewer"`
	Access      string    `json:"access"`
}

type accessPermit struct {
	ID          string    `json:"id"`
	SourceID    string    `json:"sourceId"`
	EdgeID      string    `json:"edgeId,omitempty"`
	RoadLabel   string    `json:"roadLabel"`
	Revision    int       `json:"revision"`
	AvailableAt time.Time `json:"availableAt"`
	From        time.Time `json:"from"`
	ToExclusive time.Time `json:"toExclusive"`
	Status      string    `json:"status"`
}

type exercisePopulation struct {
	SourceID   string         `json:"sourceId"`
	Year       int            `json:"year"`
	Allocation string         `json:"allocation,omitempty"`
	Cells      []exerciseCell `json:"cells"`
}

type exerciseCell struct {
	ID         string             `json:"id"`
	People     *float64           `json:"people"`
	ZoneShares map[string]float64 `json:"zoneShares"`
}

type accessRestoration struct {
	ID                 string     `json:"id"`
	SourceID           string     `json:"sourceId"`
	DestinationID      string     `json:"destinationId"`
	Label              string     `json:"label"`
	AvailableAt        time.Time  `json:"availableAt"`
	ExpectedCompleteAt *time.Time `json:"expectedCompleteAt,omitempty"`
	CompletedAt        *time.Time `json:"completedAt,omitempty"`
	CompletionKnownAt  *time.Time `json:"completionKnownAt,omitempty"`
	Cost               *float64   `json:"cost,omitempty"`
	Currency           string     `json:"currency,omitempty"`
	PricingYear        int        `json:"pricingYear,omitempty"`
}

type replayEdge struct {
	accessEdge
	State       string   `json:"state"`
	EvidenceIDs []string `json:"evidenceIds"`
	Planned     []string `json:"plannedPermits"`
}

type destinationPath struct {
	Outcome           string   `json:"outcome"`
	VerifiedDistanceM *float64 `json:"verifiedDistanceM"`
	EdgeIDs           []string `json:"edgeIds"`
}

type replayDestination struct {
	accessDestination
	Observed        destinationPath `json:"observed"`
	WithPlannedWork destinationPath `json:"withPlannedWork"`
	KnownPeople     *float64        `json:"knownPeople"`
	UnknownCells    int             `json:"unknownCells"`
}

type replayAssociation struct {
	ReportID string `json:"reportId"`
	OtherID  string `json:"otherId"`
	Decision string `json:"decision"`
	Reviewer string `json:"reviewer,omitempty"`
}

type associationChecks struct {
	ReviewedPairs     int `json:"reviewedPairs"`
	CorrectCandidates int `json:"correctCandidates"`
	FalseCandidates   int `json:"falseCandidates"`
	MissedLinks       int `json:"missedLinks"`
}

type civilianAccessReplay struct {
	Title                  string              `json:"title"`
	Kind                   string              `json:"kind"`
	GraphEdition           string              `json:"graphEdition"`
	RetainedManifestSHA256 string              `json:"retainedManifestSha256,omitempty"`
	ServiceClass           string              `json:"serviceClass"`
	Origin                 string              `json:"origin"`
	Cutoff                 time.Time           `json:"cutoff"`
	Sources                []exerciseSource    `json:"sources"`
	Edges                  []replayEdge        `json:"edges"`
	Destinations           []replayDestination `json:"destinations"`
	Reports                []accessReport      `json:"reports"`
	Associations           []replayAssociation `json:"associations"`
	Inspections            []accessInspection  `json:"inspections"`
	Works                  []accessPermit      `json:"works"`
	PopulationYear         *int                `json:"populationYear"`
	PopulationAllocation   string              `json:"populationAllocation,omitempty"`
	UnallocatedPeople      *float64            `json:"unallocatedPeople"`
	Restoration            []accessRestoration `json:"restoration"`
	Assignments            []accessAssignment  `json:"assignments"`
	AssociationChecks      associationChecks   `json:"associationChecks"`
}

func (s Service) reviewCivilianAccess(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	cutoff, err := time.Parse(time.RFC3339Nano, query.Get("knownAt"))
	if err != nil {
		httpx.Error(w, apperr.Validation("provide a replay cutoff with a UTC offset"))
		return
	}
	s.reviewSimulationSource(w, r, "civilian-road-exercise", "v1", query.Get("inputSha256"),
		func(raw []byte) (string, any, error) {
			report, err := replayCivilianAccess(raw, cutoff)
			if err == nil && report.RetainedManifestSHA256 != "" && report.RetainedManifestSHA256 != query.Get("manifestSha256") {
				err = apperr.Conflict("the road exercise references a different retained environment")
			}
			return "replay", report, err
		})
}

func decodeAccessExercise(raw []byte) (accessExercise, error) {
	var exercise accessExercise
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&exercise); err != nil || decoder.Decode(new(any)) != io.EOF {
		return exercise, apperr.Validation("expected one valid civilian road exercise package")
	}
	return exercise, nil
}

func replayCivilianAccess(raw []byte, cutoff time.Time) (civilianAccessReplay, error) {
	var out civilianAccessReplay
	exercise, err := decodeAccessExercise(raw)
	if err != nil {
		return out, err
	}
	index, err := exercise.validate(cutoff)
	if err != nil {
		return out, err
	}
	out = civilianAccessReplay{
		Title: exercise.Title, Kind: exercise.Kind, GraphEdition: exercise.GraphEdition,
		RetainedManifestSHA256: exercise.RetainedManifestSHA256,
		ServiceClass:           exercise.ServiceClass, Origin: exercise.Origin, Cutoff: cutoff.UTC(),
		Sources: exercise.Sources, Edges: []replayEdge{}, Destinations: []replayDestination{},
		Reports: []accessReport{}, Associations: []replayAssociation{}, Inspections: []accessInspection{}, Works: []accessPermit{},
		Restoration: []accessRestoration{},
	}
	exercise.visibleReports(cutoff, &out)
	exercise.visibleAccess(cutoff, &out)
	out.Assignments = []accessAssignment{}
	for _, assignment := range exercise.Assignments {
		if assignment.AvailableAt.After(cutoff) {
			continue
		}
		if assignment.ArrivalKnownAt != nil && assignment.ArrivalKnownAt.After(cutoff) {
			assignment.ArrivedAt, assignment.ArrivalKnownAt = nil, nil
		}
		out.Assignments = append(out.Assignments, assignment)
	}
	for _, task := range exercise.Restoration {
		if task.AvailableAt.After(cutoff) {
			continue
		}
		if task.CompletionKnownAt != nil && task.CompletionKnownAt.After(cutoff) {
			task.CompletedAt, task.CompletionKnownAt = nil, nil
		}
		out.Restoration = append(out.Restoration, task)
	}
	observed := traceDestinations(out.Edges, exercise.Origin, index.turns, false)
	planned := traceDestinations(out.Edges, exercise.Origin, index.turns, true)
	for _, destination := range exercise.Destinations {
		out.Destinations = append(out.Destinations, replayDestination{
			accessDestination: destination,
			Observed:          observed.result(destination.Node, exercise.BoundaryNodes),
			WithPlannedWork:   planned.result(destination.Node, exercise.BoundaryNodes),
		})
	}
	exercise.populationResult(&out)
	return out, nil
}

// ReadCivilianAccessReplay reproduces a reviewed result from already-issued
// local bytes. The caller supplies the input and, when used, road manifest pins.
func ReadCivilianAccessReplay(raw []byte, cutoff time.Time, inputDigest, manifestDigest string) (any, error) {
	if len(raw) == 0 || len(raw) > simulationReviewMaxBytes || !digestPattern.MatchString(inputDigest) ||
		artifacts.Hash(raw) != inputDigest {
		return nil, apperr.Validation("access package differs from its trusted digest or exceeds the input budget")
	}
	report, err := replayCivilianAccess(raw, cutoff)
	if err != nil {
		return nil, err
	}
	if report.RetainedManifestSHA256 != "" && report.RetainedManifestSHA256 != manifestDigest {
		return nil, apperr.Conflict("access package needs its pinned road manifest")
	}
	return map[string]any{"schema": "emad.civilian-access-replay.v1", "inputSha256": inputDigest, "replay": report}, nil
}

type accessIndex struct {
	nodes        map[string]bool
	sources      map[string]bool
	edges        map[string]accessEdge
	destinations map[string]bool
	reports      map[string]accessReport
	turns        map[accessTurn]bool
}

func (e accessExercise) validate(cutoff time.Time) (accessIndex, error) {
	index := accessIndex{
		nodes: map[string]bool{}, sources: map[string]bool{}, edges: map[string]accessEdge{},
		destinations: map[string]bool{}, reports: map[string]accessReport{}, turns: map[accessTurn]bool{},
	}
	if e.Schema != civilianAccessSchema || (e.Kind != "authored-exercise" && e.Kind != "reviewed-history") ||
		strings.TrimSpace(e.Title) == "" || !keyPattern.MatchString(e.GraphEdition) || strings.TrimSpace(e.ServiceClass) == "" ||
		e.WindowFrom.IsZero() || !e.WindowFrom.Before(e.WindowTo) || cutoff.Before(e.WindowFrom) || cutoff.After(e.WindowTo) {
		return index, apperr.Validation("invalid exercise edition or cutoff outside the pinned exercise window")
	}
	if e.RetainedManifestSHA256 != "" && !digestPattern.MatchString(e.RetainedManifestSHA256) {
		return index, apperr.Validation("invalid retained road manifest digest")
	}
	if len(e.Nodes) == 0 || len(e.Nodes) > 500 || len(e.Edges) == 0 || len(e.Edges) > 1500 ||
		len(e.Destinations) == 0 || len(e.Destinations) > 100 || len(e.Sources) == 0 || len(e.Sources) > 20 ||
		len(e.Reports) > 250 || len(e.Associations) > 500 || len(e.Inspections) > 3000 || len(e.Permits) > 500 ||
		len(e.TurnRestrictions) > 1000 || len(e.BoundaryNodes) > len(e.Nodes) || len(e.Restoration) > 250 ||
		len(e.Assignments) > 500 {
		return index, apperr.Validation("exercise exceeds the bounded graph or evidence budget")
	}
	for _, source := range e.Sources {
		if !keyPattern.MatchString(source.ID) || index.sources[source.ID] || strings.TrimSpace(source.Edition) == "" ||
			!digestPattern.MatchString(source.SHA256) || strings.TrimSpace(source.Attribution) == "" {
			return index, apperr.Validation("source edition, digest, attribution or identity missing or repeated")
		}
		index.sources[source.ID] = true
	}
	for _, node := range e.Nodes {
		if !keyPattern.MatchString(node) || index.nodes[node] {
			return index, apperr.Validation("invalid or repeated node")
		}
		index.nodes[node] = true
	}
	if !index.nodes[e.Origin] {
		return index, apperr.Validation("origin is outside the reviewed graph")
	}
	for _, boundary := range e.BoundaryNodes {
		if !index.nodes[boundary] {
			return index, apperr.Validation("boundary connection is outside the reviewed graph")
		}
	}
	for _, edge := range e.Edges {
		if !keyPattern.MatchString(edge.ID) || index.edges[edge.ID].ID != "" || !index.nodes[edge.From] ||
			!index.nodes[edge.To] || edge.From == edge.To || edge.DistanceM <= 0 || edge.DistanceM > 100000 ||
			!index.sources[edge.SourceID] || strings.TrimSpace(edge.FeatureID) == "" ||
			(edge.RetainedOSMFeatureID != "" && (e.RetainedManifestSHA256 == "" || !keyPattern.MatchString(edge.RetainedOSMFeatureID))) {
			return index, apperr.Validation("invalid directed edge, endpoint, distance or source reference")
		}
		index.edges[edge.ID] = edge
	}
	for _, turn := range e.TurnRestrictions {
		from, to := index.edges[turn.FromEdge], index.edges[turn.ToEdge]
		if from.ID == "" || to.ID == "" || from.To != to.From || index.turns[turn] {
			return index, apperr.Validation("invalid or repeated turn restriction")
		}
		index.turns[turn] = true
	}
	for _, destination := range e.Destinations {
		if !keyPattern.MatchString(destination.ID) || index.destinations[destination.ID] || strings.TrimSpace(destination.Label) == "" ||
			(destination.Node != "" && !index.nodes[destination.Node]) {
			return index, apperr.Validation("invalid or repeated destination")
		}
		index.destinations[destination.ID] = true
	}
	if err := e.validateEvidence(&index); err != nil {
		return index, err
	}
	return index, e.validatePopulation(index)
}

func (e accessExercise) validateEvidence(index *accessIndex) error {
	for _, report := range e.Reports {
		if !keyPattern.MatchString(report.ID) || index.reports[report.ID].ID != "" || !index.sources[report.SourceID] ||
			strings.TrimSpace(report.Category) == "" || report.ReceiptAt.IsZero() || report.AvailableAt.IsZero() ||
			report.ReceiptAt.After(report.AvailableAt) || (report.TicketClosedAt != nil && report.TicketClosedAt.Before(report.AvailableAt)) {
			return apperr.Validation("report has invalid identity, source or timeline")
		}
		index.reports[report.ID] = report
	}
	assignments := map[string]bool{}
	for _, assignment := range e.Assignments {
		report := index.reports[assignment.ReportID]
		hasArrival := assignment.ArrivalKnownAt != nil && assignment.ArrivedAt != nil
		if !keyPattern.MatchString(assignment.ID) || assignments[assignment.ID] || !index.sources[assignment.SourceID] ||
			report.ID == "" || strings.TrimSpace(assignment.CrewLabel) == "" || assignment.AvailableAt.IsZero() ||
			assignment.DispatchedAt.Before(report.AvailableAt) || assignment.AvailableAt.Before(assignment.DispatchedAt) ||
			(assignment.ArrivedAt == nil) != (assignment.ArrivalKnownAt == nil) ||
			(hasArrival && (assignment.ArrivedAt.Before(assignment.DispatchedAt) ||
				assignment.ArrivalKnownAt.Before(*assignment.ArrivedAt) || assignment.ArrivalKnownAt.Before(assignment.AvailableAt))) {
			return apperr.Validation("crew assignment has invalid source, report or timing")
		}
		assignments[assignment.ID] = true
	}
	type associationKey struct {
		pair      [2]string
		available time.Time
	}
	seenAssociations := map[associationKey]bool{}
	for _, association := range e.Associations {
		a, b := index.reports[association.ReportID], index.reports[association.OtherID]
		key := associationKey{reportPair(association.ReportID, association.OtherID), association.AvailableAt.UTC()}
		if a.ID == "" || b.ID == "" || a.ID == b.ID || strings.TrimSpace(association.Reviewer) == "" ||
			(association.Decision != "linked" && association.Decision != "separate") ||
			association.AvailableAt.Before(a.AvailableAt) || association.AvailableAt.Before(b.AvailableAt) || seenAssociations[key] {
			return apperr.Validation("report association has invalid identity, timing or adjudication")
		}
		seenAssociations[key] = true
	}
	inspectionIDs := map[string]bool{}
	for _, inspection := range e.Inspections {
		report := index.reports[inspection.ReportID]
		if !keyPattern.MatchString(inspection.ID) || inspectionIDs[inspection.ID] || !index.sources[inspection.SourceID] ||
			index.edges[inspection.EdgeID].ID == "" || strings.TrimSpace(inspection.Reviewer) == "" ||
			inspection.ObservedAt.IsZero() || inspection.AvailableAt.IsZero() || inspection.ObservedAt.After(inspection.AvailableAt) ||
			(inspection.ReportID != "" && (report.ID == "" || report.AvailableAt.After(inspection.AvailableAt))) ||
			!slices.Contains([]string{"open", "closed", "partial", "unknown"}, inspection.Access) {
			return apperr.Validation("inspection has invalid identity, scope, timing or access")
		}
		inspectionIDs[inspection.ID] = true
	}
	type permitKey struct {
		id       string
		revision int
	}
	seenPermits := map[permitKey]bool{}
	permitSources := map[string]string{}
	for _, permit := range e.Permits {
		key := permitKey{permit.ID, permit.Revision}
		if !keyPattern.MatchString(permit.ID) || !index.sources[permit.SourceID] || permit.Revision <= 0 ||
			strings.TrimSpace(permit.RoadLabel) == "" || permit.AvailableAt.IsZero() || permit.From.IsZero() ||
			!permit.From.Before(permit.ToExclusive) || (permit.Status != "scheduled" && permit.Status != "cancelled") ||
			(permit.EdgeID != "" && index.edges[permit.EdgeID].ID == "") || seenPermits[key] ||
			(permitSources[permit.ID] != "" && permitSources[permit.ID] != permit.SourceID) {
			return apperr.Validation("permit has invalid identity, revision, source or interval")
		}
		seenPermits[key] = true
		permitSources[permit.ID] = permit.SourceID
	}
	taskIDs := map[string]bool{}
	for _, task := range e.Restoration {
		hasCompletion := task.CompletedAt != nil && task.CompletionKnownAt != nil
		if !keyPattern.MatchString(task.ID) || taskIDs[task.ID] || !index.destinations[task.DestinationID] ||
			!index.sources[task.SourceID] || strings.TrimSpace(task.Label) == "" || task.AvailableAt.IsZero() ||
			(task.CompletedAt != nil) != (task.CompletionKnownAt != nil) ||
			(hasCompletion && (task.CompletedAt.After(*task.CompletionKnownAt) || task.AvailableAt.After(*task.CompletionKnownAt))) {
			return apperr.Validation("restoration task has invalid identity, destination or completion timeline")
		}
		if (task.Cost == nil && (task.Currency != "" || task.PricingYear != 0)) ||
			(task.Cost != nil && (*task.Cost < 0 || *task.Cost > 1e10 || len(task.Currency) != 3 ||
				strings.Trim(task.Currency, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") != "" || task.PricingYear < 1900 || task.PricingYear > 2100)) {
			return apperr.Validation("restoration costs require a nonnegative value, currency code and pricing year")
		}
		taskIDs[task.ID] = true
	}
	return nil
}

func reportPair(a, b string) [2]string {
	if a > b {
		return [2]string{b, a}
	}
	return [2]string{a, b}
}

func (e accessExercise) visibleReports(cutoff time.Time, out *civilianAccessReplay) {
	for _, report := range e.Reports {
		if report.AvailableAt.After(cutoff) {
			continue
		}
		if report.TicketClosedAt != nil && report.TicketClosedAt.After(cutoff) {
			report.TicketClosedAt = nil
		}
		out.Reports = append(out.Reports, report)
	}
	slices.SortFunc(out.Reports, func(a, b accessReport) int {
		if order := a.AvailableAt.Compare(b.AvailableAt); order != 0 {
			return order
		}
		return cmp.Compare(a.ID, b.ID)
	})
	visible := map[[2]string]reportAssociation{}
	for _, association := range e.Associations {
		key := reportPair(association.ReportID, association.OtherID)
		if !association.AvailableAt.After(cutoff) && association.AvailableAt.After(visible[key].AvailableAt) {
			visible[key] = association
		}
	}
	for i, a := range out.Reports {
		for _, b := range out.Reports[i+1:] {
			key := reportPair(a.ID, b.ID)
			candidate := normalizedText(a.PlaceLabel) != "" && normalizedText(a.PlaceLabel) == normalizedText(b.PlaceLabel) &&
				normalizedText(a.Category) == normalizedText(b.Category) && b.AvailableAt.Sub(a.AvailableAt) <= time.Hour
			if decision, ok := visible[key]; ok {
				out.Associations = append(out.Associations, replayAssociation{key[0], key[1], decision.Decision, decision.Reviewer})
				out.AssociationChecks.ReviewedPairs++
				switch {
				case candidate && decision.Decision == "linked":
					out.AssociationChecks.CorrectCandidates++
				case candidate && decision.Decision == "separate":
					out.AssociationChecks.FalseCandidates++
				case !candidate && decision.Decision == "linked":
					out.AssociationChecks.MissedLinks++
				}
			} else if candidate {
				out.Associations = append(out.Associations, replayAssociation{ReportID: key[0], OtherID: key[1], Decision: "suggested"})
			}
		}
	}
}

func normalizedText(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(value)), " ")
}

func (e accessExercise) visibleAccess(cutoff time.Time, out *civilianAccessReplay) {
	latest := map[string][]accessInspection{}
	for _, inspection := range e.Inspections {
		if inspection.AvailableAt.After(cutoff) {
			continue
		}
		out.Inspections = append(out.Inspections, inspection)
		prior := latest[inspection.EdgeID]
		if len(prior) == 0 || inspection.ObservedAt.After(prior[0].ObservedAt) {
			latest[inspection.EdgeID] = []accessInspection{inspection}
		} else if inspection.ObservedAt.Equal(prior[0].ObservedAt) {
			latest[inspection.EdgeID] = append(prior, inspection)
		}
	}
	slices.SortFunc(out.Inspections, func(a, b accessInspection) int {
		if order := a.AvailableAt.Compare(b.AvailableAt); order != 0 {
			return order
		}
		return cmp.Compare(a.ID, b.ID)
	})
	byPermit := map[string]accessPermit{}
	for _, permit := range e.Permits {
		if !permit.AvailableAt.After(cutoff) && permit.Revision > byPermit[permit.ID].Revision {
			byPermit[permit.ID] = permit
		}
	}
	planned := map[string][]string{}
	for _, permit := range byPermit {
		out.Works = append(out.Works, permit)
		if permit.Status == "scheduled" && permit.EdgeID != "" && !cutoff.Before(permit.From) && cutoff.Before(permit.ToExclusive) {
			planned[permit.EdgeID] = append(planned[permit.EdgeID], permit.ID)
		}
	}
	slices.SortFunc(out.Works, func(a, b accessPermit) int { return cmp.Compare(a.ID, b.ID) })
	for _, edge := range e.Edges {
		current := replayEdge{accessEdge: edge, State: "unknown", EvidenceIDs: []string{}, Planned: append([]string{}, planned[edge.ID]...)}
		if latest := latest[edge.ID]; len(latest) > 0 {
			current.State = latest[0].Access
			for _, evidence := range latest {
				current.EvidenceIDs = append(current.EvidenceIDs, evidence.ID)
				if evidence.Access != latest[0].Access {
					current.State = "disputed"
				}
			}
			slices.Sort(current.EvidenceIDs)
		}
		slices.Sort(current.Planned)
		out.Edges = append(out.Edges, current)
	}
}
