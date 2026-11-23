package geoenv

import (
	"container/heap"
	"math"
	"strings"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/apperr"
)

type pathStep struct {
	node, last string
	distance   float64
	parent     *pathStep
}

type pathQueue []*pathStep

func (p pathQueue) Len() int { return len(p) }
func (p pathQueue) Less(i, j int) bool {
	if p[i].distance != p[j].distance {
		return p[i].distance < p[j].distance
	}
	return p[i].last < p[j].last
}
func (p pathQueue) Swap(i, j int)   { p[i], p[j] = p[j], p[i] }
func (p *pathQueue) Push(value any) { *p = append(*p, value.(*pathStep)) }
func (p *pathQueue) Pop() any {
	last := len(*p) - 1
	value := (*p)[last]
	(*p)[last] = nil
	*p = (*p)[:last]
	return value
}

// Incoming edge is part of the routing state. A locally shorter path into a
// junction cannot suppress a longer arrival with different legal turns.
func shortestAccess(edges map[string][]replayEdge, origin string, turns map[accessTurn]bool, verifiedOnly, usePermits bool) map[string]*pathStep {
	start := &pathStep{node: origin}
	result := map[string]*pathStep{}
	best := map[string]*pathStep{"": start}
	queue := pathQueue{start}
	for queue.Len() > 0 {
		step := heap.Pop(&queue).(*pathStep)
		if step != best[step.last] {
			continue
		}
		if _, seen := result[step.node]; !seen {
			result[step.node] = step
		}
		for _, edge := range edges[step.node] {
			if edge.State == "closed" || (verifiedOnly && edge.State != "open") ||
				(usePermits && len(edge.Planned) > 0) || turns[accessTurn{step.last, edge.ID}] {
				continue
			}
			next := step.distance + edge.DistanceM
			if prior := best[edge.ID]; prior == nil || next < prior.distance {
				advance := &pathStep{node: edge.To, last: edge.ID, distance: next, parent: step}
				best[edge.ID] = advance
				heap.Push(&queue, advance)
			}
		}
	}
	return result
}

type destinationTrace struct {
	verified map[string]*pathStep
	possible map[string]*pathStep
}

func traceDestinations(edges []replayEdge, origin string, turns map[accessTurn]bool, usePermits bool) destinationTrace {
	adjacent := map[string][]replayEdge{}
	for _, edge := range edges {
		adjacent[edge.From] = append(adjacent[edge.From], edge)
	}
	return destinationTrace{
		verified: shortestAccess(adjacent, origin, turns, true, usePermits),
		possible: shortestAccess(adjacent, origin, turns, false, usePermits),
	}
}

func (t destinationTrace) result(node string, boundary []string) destinationPath {
	out := destinationPath{Outcome: "unreachable-in-graph", EdgeIDs: []string{}}
	if node == "" {
		out.Outcome = "outside-known-network"
		return out
	}
	if current := t.verified[node]; current != nil {
		out.Outcome = "reachable"
		out.VerifiedDistanceM = &current.distance
		for cursor := current; cursor.parent != nil; cursor = cursor.parent {
			out.EdgeIDs = append(out.EdgeIDs, cursor.last)
		}
		for i, j := 0, len(out.EdgeIDs)-1; i < j; i, j = i+1, j-1 {
			out.EdgeIDs[i], out.EdgeIDs[j] = out.EdgeIDs[j], out.EdgeIDs[i]
		}
		return out
	}
	if t.possible[node] != nil {
		out.Outcome = "unknown"
		return out
	}
	for _, exit := range boundary {
		if t.possible[exit] != nil {
			out.Outcome = "outside-known-network"
			break
		}
	}
	return out
}

func (e accessExercise) validatePopulation(index accessIndex) error {
	if e.Population == nil {
		return nil
	}
	p := e.Population
	if !index.sources[p.SourceID] || p.Year < 1900 || p.Year > 2100 || len(p.Cells) == 0 || len(p.Cells) > 10000 {
		return apperr.Validation("population source, year or cells are outside the package budget")
	}
	if p.Allocation != "" && p.Allocation != "area-weighted" && p.Allocation != "cell-center" {
		return apperr.Validation("unknown native population allocation")
	}
	seen := map[string]bool{}
	for _, cell := range p.Cells {
		if strings.TrimSpace(cell.ID) == "" || seen[cell.ID] || (cell.People != nil && (*cell.People < 0 || *cell.People > 1e9)) {
			return apperr.Validation("invalid or repeated population grid cell")
		}
		seen[cell.ID] = true
		shares := 0.0
		for zone, part := range cell.ZoneShares {
			if !index.destinations[zone] || part <= 0 || part > 1 {
				return apperr.Validation("population share has an invalid zone or fraction")
			}
			shares += part
		}
		if shares > 1+1e-12 {
			return apperr.Validation("population grid cell is overallocated")
		}
	}
	return nil
}

func (e accessExercise) populationResult(out *civilianAccessReplay) {
	if e.Population == nil {
		return
	}
	out.PopulationYear = &e.Population.Year
	out.PopulationAllocation = e.Population.Allocation
	type zoneCount struct {
		known   *float64
		unknown int
	}
	byZone := map[string]*zoneCount{}
	unallocated := 0.0
	for _, cell := range e.Population.Cells {
		fractions := 0.0
		for _, destination := range e.Destinations {
			share, ok := cell.ZoneShares[destination.ID]
			if !ok {
				continue
			}
			zone := byZone[destination.ID]
			if zone == nil {
				zone = &zoneCount{}
				byZone[destination.ID] = zone
			}
			fractions += share
			if cell.People == nil {
				zone.unknown++
				continue
			}
			if zone.known == nil {
				zone.known = new(float64)
			}
			*zone.known += *cell.People * share
		}
		if cell.People != nil {
			unallocated += *cell.People * math.Max(0, 1-fractions)
		}
	}
	for i := range out.Destinations {
		if zone := byZone[out.Destinations[i].ID]; zone != nil {
			out.Destinations[i].KnownPeople = zone.known
			out.Destinations[i].UnknownCells = zone.unknown
		}
	}
	out.UnallocatedPeople = &unallocated
}
