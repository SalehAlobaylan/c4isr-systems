package realtime

import (
	"context"
	"sort"
	"strings"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/access"
)

type eventPolicy struct {
	permissions []string
	source      string
}

// New domain topics are private until explicitly added to this catalogue and
// the typed mapper. Input/truth topics never belong to the COP catalogue.
var topicPermissions = map[string][]string{
	"source.created": {"sources.read"}, "source.updated": {"sources.read"},
	"observation.received": {"observations.read"}, "observation.rejected": {"observations.read"},
	"asset.created": {"assets.read"}, "asset.updated": {"assets.read"}, "asset.position.updated": {"assets.read"}, "asset.connection.changed": {"assets.read"},
	"telemetry.received": {"telemetry.read"}, "track.created": {"tracks.read"}, "track.updated": {"tracks.read"}, "track.closed": {"tracks.read"},
	"classification.created": {"classifications.read"}, "geofence.created": {"geofences.read"}, "geofence.breached": {"geofences.read", "tracks.read"}, "geofence.exited": {"geofences.read", "tracks.read"},
	"alert.created": {"alerts.read"}, "alert.acknowledged": {"alerts.read"}, "alert.resolved": {"alerts.read"}, "incident.created": {"incidents.read"}, "incident.updated": {"incidents.read"},
	"mission.created": {"missions.read"}, "mission.updated": {"missions.read"}, "command.issued": {"commands.read"}, "command.status.changed": {"commands.read"}, "assessment.created": {"assessments.read"},
	"scenario.run.started": {"scenarios.read"}, "scenario.run.updated": {"scenarios.read"}, "scenario.run.ended": {"scenarios.read"}, "entity.freshness.changed": {},
}

func policy(env Envelope) (eventPolicy, bool) {
	permissions, ok := topicPermissions[env.Type]
	if !ok || len(env.Data) == 0 {
		return eventPolicy{}, false
	}
	if env.Type == "entity.freshness.changed" {
		switch env.Data["entityType"] {
		case "asset":
			permissions = []string{"assets.read"}
		case "track":
			permissions = []string{"tracks.read"}
		default:
			return eventPolicy{}, false
		}
	}
	source, _ := env.Data["sourceId"].(string)
	return eventPolicy{permissions, source}, true
}
func (p eventPolicy) allowed(ctx context.Context) bool {
	for _, permission := range p.permissions {
		if !access.Allows(ctx, permission) {
			return false
		}
	}
	if ids := access.SourceIDs(ctx); len(ids) > 0 {
		for _, id := range ids {
			if id == p.source {
				return true
			}
		}
		return false
	}
	return true
}
func topicAllowed(ctx context.Context, topic string) bool {
	permissions, ok := topicPermissions[topic]
	if !ok {
		return false
	}
	if topic == "entity.freshness.changed" {
		return len(access.SourceIDs(ctx)) == 0 && (access.Allows(ctx, "assets.read") || access.Allows(ctx, "tracks.read"))
	}
	if len(access.SourceIDs(ctx)) > 0 && !strings.HasPrefix(topic, "source.") && !strings.HasPrefix(topic, "observation.") && topic != "telemetry.received" {
		return false
	}
	for _, p := range permissions {
		if !access.Allows(ctx, p) {
			return false
		}
	}
	return true
}
func selectTopics(ctx context.Context, selection string) (map[string]bool, bool) {
	chosen := map[string]bool{}
	if selection == "" {
		for topic := range topicPermissions {
			if topicAllowed(ctx, topic) {
				chosen[topic] = true
			}
		}
		return chosen, len(chosen) > 0
	}
	if len(selection) > 2048 {
		return nil, false
	}
	for _, topic := range strings.Split(selection, ",") {
		if !topicAllowed(ctx, topic) {
			return nil, false
		}
		chosen[topic] = true
	}
	return chosen, len(chosen) > 0
}
func sortedTopics(chosen map[string]bool) []string {
	result := []string{}
	for topic := range chosen {
		result = append(result, topic)
	}
	sort.Strings(result)
	return result
}
