package geoenv

import (
	"cmp"
	"encoding/json"
	"slices"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
)

const regionalMaxSlots = 128

type regionalService struct {
	Region      string             `json:"region"`
	From        time.Time          `json:"from"`
	ReferenceAt *time.Time         `json:"referenceAt"`
	Access      json.RawMessage    `json:"access,omitempty"`
	Capacity    []regionalCapacity `json:"capacity"`
	Workloads   []regionalWorkload `json:"workloads"`
}

type regionalCapacity struct {
	ID          string    `json:"id"`
	Label       string    `json:"label"`
	SourceID    string    `json:"sourceId"`
	Kind        string    `json:"kind"`
	AvailableAt time.Time `json:"availableAt"`
	Slots       *int      `json:"staffedSlots"`
}

type regionalWorkload struct {
	ID       string            `json:"id"`
	Label    string            `json:"label"`
	Kind     string            `json:"kind"`
	SourceID string            `json:"sourceId"`
	Arrivals []regionalArrival `json:"arrivals"`
}

type regionalArrival struct {
	ID      string    `json:"id"`
	Zone    string    `json:"zone"`
	At      time.Time `json:"arrivedAt"`
	KnownAt time.Time `json:"availableAt"`
	Seconds int       `json:"serviceSeconds"`
}

type regionalServiceReview struct {
	Region       string                   `json:"region"`
	ReferenceAt  *time.Time               `json:"referenceAt"`
	GraphEdition string                   `json:"graphEdition,omitempty"`
	Zones        []regionalZone           `json:"zones"`
	Capacity     []regionalCapacity       `json:"capacity"`
	Workloads    []regionalWorkloadReview `json:"workloads"`
}

type regionalZone struct {
	ID        string          `json:"id"`
	Label     string          `json:"label"`
	Reference destinationPath `json:"reference"`
	Current   destinationPath `json:"current"`
}

type regionalWorkloadReview struct {
	ID        string                 `json:"id"`
	Label     string                 `json:"label"`
	Kind      string                 `json:"kind"`
	SourceID  string                 `json:"sourceId"`
	Planned   *int                   `json:"plannedArrivals"`
	Arrivals  int                    `json:"arrivals"`
	ByZone    map[string]int         `json:"byZone"`
	Reference *regionalAccessOutcome `json:"reference,omitempty"`
	regionalAccessOutcome
}

type regionalAccessOutcome struct {
	Reachable     int                       `json:"reachable"`
	Disconnected  int                       `json:"disconnected"`
	AccessUnknown int                       `json:"accessUnknown"`
	Queues        map[string]*regionalQueue `json:"queues"`
}

type regionalAccessStates struct {
	reference map[string]string
	current   map[string]string
}

type regionalQueue struct {
	Started    int      `json:"started"`
	Completed  int      `json:"completed"`
	Waiting    int      `json:"awaitingService"`
	InService  int      `json:"inServiceAtCutoff"`
	MedianWait *float64 `json:"medianStartedWaitSeconds"`
	P90Wait    *float64 `json:"p90StartedWaitSeconds"`
}

func (service regionalService) review(cutoff time.Time, manifest string, slots *int, sources map[string]bool) (*regionalServiceReview, error) {
	if !regionalLabel(service.Region) || service.From.IsZero() || !cutoff.After(service.From) || cutoff.Sub(service.From) > 31*24*time.Hour ||
		len(service.Capacity) == 0 || len(service.Capacity) > 8 || len(service.Workloads) == 0 || len(service.Workloads) > 8 {
		return nil, apperr.Validation("invalid service exercise time, region or comparison budget")
	}
	out := &regionalServiceReview{Region: service.Region, ReferenceAt: service.ReferenceAt, Zones: []regionalZone{},
		Capacity: []regionalCapacity{}, Workloads: []regionalWorkloadReview{}}
	seen := map[string]bool{"analyst-override": true}
	for _, capacity := range service.Capacity {
		if !keyPattern.MatchString(capacity.ID) || seen[capacity.ID] || !regionalLabel(capacity.Label) || !sources[capacity.SourceID] ||
			capacity.AvailableAt.IsZero() || !slices.Contains([]string{"authored", "observed", "unavailable", "unknown"}, capacity.Kind) ||
			(capacity.Kind == "unknown") != (capacity.Slots == nil) ||
			(capacity.Slots != nil && (*capacity.Slots < 0 || *capacity.Slots > regionalMaxSlots || (capacity.Kind == "unavailable" && *capacity.Slots != 0))) {
			return nil, apperr.Validation("invalid regional capacity evidence or staffing assumption")
		}
		seen[capacity.ID] = true
		if !capacity.AvailableAt.After(cutoff) {
			out.Capacity = append(out.Capacity, capacity)
		}
	}
	if slots != nil {
		out.Capacity = append(out.Capacity, regionalCapacity{ID: "analyst-override", Label: "Edited staffing assumption",
			Kind: "authored", Slots: slots, AvailableAt: cutoff.UTC()})
	}
	zones, err := service.reviewZones(cutoff, manifest, out)
	if err != nil {
		return nil, err
	}
	seen = map[string]bool{}
	planSize := -1
	for _, workload := range service.Workloads {
		if !keyPattern.MatchString(workload.ID) || seen[workload.ID] || !regionalLabel(workload.Label) || !sources[workload.SourceID] ||
			(workload.Kind != "authored" && workload.Kind != "reviewed-history") || len(workload.Arrivals) == 0 || len(workload.Arrivals) > 2048 {
			return nil, apperr.Validation("invalid or duplicate regional workload")
		}
		seen[workload.ID] = true
		if workload.Kind == "authored" {
			if planSize != -1 && planSize != len(workload.Arrivals) {
				return nil, apperr.Validation("authored profiles must compare equal total demand")
			}
			planSize = len(workload.Arrivals)
		}
		profile, err := workload.review(service.From, cutoff, out.Capacity, zones)
		if err != nil {
			return nil, err
		}
		out.Workloads = append(out.Workloads, profile)
	}
	return out, nil
}

func (service regionalService) reviewZones(cutoff time.Time, manifest string, out *regionalServiceReview) (regionalAccessStates, error) {
	var states regionalAccessStates
	if len(service.Access) == 0 {
		if service.ReferenceAt != nil {
			return states, apperr.Validation("access reference needs a reviewed graph")
		}
		return states, nil
	}
	if service.ReferenceAt == nil || service.ReferenceAt.IsZero() || service.ReferenceAt.After(cutoff) {
		return states, apperr.Validation("select an access reference no later than the review cutoff")
	}
	exercise, err := decodeAccessExercise(service.Access)
	if err != nil {
		return states, err
	}
	index, err := exercise.validate(cutoff)
	if err != nil {
		return states, err
	}
	if service.ReferenceAt.Before(exercise.WindowFrom) {
		return states, apperr.Validation("reference cutoff precedes the reviewed road window")
	}
	if exercise.RetainedManifestSHA256 != "" && exercise.RetainedManifestSHA256 != manifest {
		return states, apperr.Conflict("the regional replay references a different road environment")
	}
	if exercise.Kind == "reviewed-history" && exercise.RetainedManifestSHA256 == "" {
		return states, apperr.Validation("historical regional access needs its retained road manifest")
	}
	var prior, current civilianAccessReplay
	exercise.visibleAccess(*service.ReferenceAt, &prior)
	exercise.visibleAccess(cutoff, &current)
	out.GraphEdition = exercise.GraphEdition
	states.reference = make(map[string]string, len(exercise.Destinations))
	states.current = make(map[string]string, len(exercise.Destinations))
	for _, zone := range exercise.Destinations {
		path := func(edges []replayEdge) destinationPath {
			if zone.Node == "" {
				return destinationPath{Outcome: "outside-known-network", EdgeIDs: []string{}}
			}
			fromZone := traceDestinations(edges, zone.Node, index.turns, false)
			return fromZone.result(exercise.Origin, exercise.BoundaryNodes)
		}
		before, now := path(prior.Edges), path(current.Edges)
		states.reference[zone.ID], states.current[zone.ID] = before.Outcome, now.Outcome
		out.Zones = append(out.Zones, regionalZone{zone.ID, zone.Label, before, now})
	}
	return states, nil
}

func (workload regionalWorkload) review(from, cutoff time.Time, capacity []regionalCapacity, zones regionalAccessStates) (regionalWorkloadReview, error) {
	out := regionalWorkloadReview{ID: workload.ID, Label: workload.Label, SourceID: workload.SourceID, Kind: workload.Kind,
		ByZone: map[string]int{}}
	if workload.Kind == "authored" {
		total := len(workload.Arrivals)
		out.Planned = &total
	}
	seen := map[string]bool{}
	var known []regionalArrival
	for _, arrival := range workload.Arrivals {
		_, hasZone := zones.current[arrival.Zone]
		if !keyPattern.MatchString(arrival.ID) || seen[arrival.ID] || arrival.At.Before(from) ||
			arrival.KnownAt.Before(arrival.At) || arrival.At.Sub(from) > 31*24*time.Hour || arrival.Seconds < 1 || arrival.Seconds > 24*3600 ||
			(zones.current != nil && !hasZone) || !keyPattern.MatchString(arrival.Zone) {
			return out, apperr.Validation("invalid arrival ID, time, service duration or zone")
		}
		seen[arrival.ID] = true
		if !arrival.At.Before(cutoff) || arrival.KnownAt.After(cutoff) {
			continue
		}
		out.Arrivals++
		out.ByZone[arrival.Zone]++
		known = append(known, arrival)
	}
	slices.SortFunc(known, func(a, b regionalArrival) int {
		if order := a.At.Compare(b.At); order != 0 {
			return order
		}
		return cmp.Compare(a.ID, b.ID)
	})
	out.regionalAccessOutcome = regionalQueues(known, capacity, zones.current, from, cutoff)
	if zones.reference != nil {
		base := regionalQueues(known, capacity, zones.reference, from, cutoff)
		out.Reference = &base
	}
	return out, nil
}

func regionalQueues(known []regionalArrival, capacity []regionalCapacity, zones map[string]string, from, cutoff time.Time) regionalAccessOutcome {
	out := regionalAccessOutcome{Queues: map[string]*regionalQueue{}}
	var eligible []regionalArrival
	for _, arrival := range known {
		state := zones[arrival.Zone]
		switch {
		case zones == nil || state == "reachable":
			out.Reachable++
			eligible = append(eligible, arrival)
		case state == "unreachable-in-graph":
			out.Disconnected++
		default:
			out.AccessUnknown++
		}
	}
	for _, profile := range capacity {
		out.Queues[profile.ID] = nil
		if profile.Slots != nil {
			out.Queues[profile.ID] = replayRegionalQueue(eligible, from, cutoff, *profile.Slots)
		}
	}
	return out
}

func replayRegionalQueue(arrivals []regionalArrival, from, cutoff time.Time, count int) *regionalQueue {
	slots := make([]time.Time, count)
	for i := range slots {
		slots[i] = from
	}
	var waits []float64
	out := &regionalQueue{}
	for _, arrival := range arrivals {
		if len(slots) == 0 {
			break
		}
		best := 0
		for i := 1; i < len(slots); i++ {
			if slots[i].Before(slots[best]) {
				best = i
			}
		}
		start := arrival.At
		if slots[best].After(start) {
			start = slots[best]
		}
		if !start.Before(cutoff) {
			break
		}
		slots[best] = start.Add(time.Duration(arrival.Seconds) * time.Second)
		if !slots[best].After(cutoff) {
			out.Completed++
		}
		waits = append(waits, start.Sub(arrival.At).Seconds())
	}
	out.Started, out.Waiting, out.InService = len(waits), len(arrivals)-len(waits), len(waits)-out.Completed
	if len(waits) > 0 {
		slices.Sort(waits)
		median := (waits[(len(waits)-1)/2] + waits[len(waits)/2]) / 2
		p90 := waits[(9*len(waits)+9)/10-1]
		out.MedianWait, out.P90Wait = &median, &p90
	}
	return out
}
