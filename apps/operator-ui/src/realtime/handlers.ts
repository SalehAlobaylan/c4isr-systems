import type { QueryClient } from '@tanstack/react-query'

import type { RealtimeEnvelope } from '@/lib/api'
import { queryKeys } from '@/lib/queryKeys'
import { toast } from '@/stores/toasts'

/**
 * Maps realtime envelopes onto TanStack Query cache invalidations and toasts.
 * No second state model is created: websocket events only refresh server state.
 */
export function applyRealtimeEvent(
  queryClient: QueryClient,
  envelope: RealtimeEnvelope,
  // While an operator is scrubbed back in time, the picture on screen is a
  // reconstruction of recorded evidence. Applying present-day events to it
  // would splice two moments together into a picture that never existed, so
  // the event is dropped rather than partially applied.
  suspended = false,
): void {
  if (suspended) return
  const data = envelope.data ?? {}

  const invalidate = (queryKey: readonly unknown[]) => {
    void queryClient.invalidateQueries({ queryKey })
  }
  const text = (key: string): string | undefined => {
    const value = data[key]
    return typeof value === 'string' && value.length > 0 ? value : undefined
  }

  switch (envelope.type) {
    case 'source.created':
    case 'source.updated': {
      invalidate(queryKeys.sources.all)
      break
    }

    case 'observation.received': {
      invalidate(queryKeys.observations.all)
      invalidate(queryKeys.tracks.all)
      invalidate(queryKeys.audit.all)
      break
    }

    case 'asset.created':
    case 'asset.updated':
    case 'asset.position.updated':
    case 'asset.connection.changed': {
      invalidate(queryKeys.assets.all)
      const assetId = text('assetId')
      if (assetId) invalidate(queryKeys.assets.detail(assetId))
      break
    }

    case 'telemetry.received': {
      invalidate(queryKeys.assets.all)
      const assetId = text('assetId')
      if (assetId) {
        invalidate(queryKeys.assets.detail(assetId))
        invalidate(queryKeys.assets.telemetry(assetId))
      }
      break
    }

    // Freshness is re-derived on every read, so a transition means the cached
    // copy is now describing the past. The list is invalidated rather than the
    // single entity because the map renders every marker at once, and a stale
    // marker elsewhere on the picture matters just as much.
    case 'entity.freshness.changed': {
      invalidate(queryKeys.assets.all)
      const entityId = text('entityId')
      if (entityId) invalidate(queryKeys.assets.detail(entityId))
      break
    }

    case 'track.created':
    case 'track.updated':
    case 'track.closed': {
      invalidate(queryKeys.tracks.all)
      invalidate(queryKeys.audit.all)
      const trackId = text('trackId')
      if (trackId) {
        invalidate(queryKeys.tracks.detail(trackId))
        invalidate(queryKeys.tracks.history(trackId))
      }
      break
    }

    case 'classification.created': {
      invalidate(queryKeys.tracks.all)
      const trackId = text('trackId')
      if (trackId) invalidate(queryKeys.tracks.classifications(trackId))
      break
    }

    case 'geofence.created':
    case 'geofence.breached':
    case 'geofence.exited': {
      invalidate(queryKeys.geofences.all)
      break
    }

    case 'alert.created': {
      invalidate(queryKeys.alerts.all)
      const severity = text('severity') ?? 'medium'
      // The title is now written in operator language server-side. Appending a
      // raw track id here would reintroduce the identifier this work removed:
      // the toast is the first thing an operator reads, so it has to read like
      // the alert centre does.
      toast({
        title: text('title') ?? 'New alert',
        description: [text('type'), severity].filter(Boolean).join(' · '),
        variant: severity === 'critical' || severity === 'high' ? 'error' : 'warning',
        durationMs: 10_000,
      })
      break
    }

    case 'alert.acknowledged':
    case 'alert.resolved': {
      invalidate(queryKeys.alerts.all)
      const alertId = text('alertId')
      if (alertId) invalidate(queryKeys.alerts.detail(alertId))
      break
    }

    case 'incident.created':
    case 'incident.updated': {
      invalidate(queryKeys.incidents.all)
      const incidentId = text('incidentId')
      if (incidentId) invalidate(queryKeys.incidents.detail(incidentId))
      break
    }

    case 'mission.created':
    case 'mission.updated': {
      invalidate(queryKeys.missions.all)
      const missionId = text('missionId')
      if (missionId) invalidate(queryKeys.missions.detail(missionId))
      break
    }

    case 'command.issued':
    case 'command.status.changed': {
      invalidate(queryKeys.commands.all)
      const commandId = text('commandId')
      if (commandId) invalidate(queryKeys.commands.detail(commandId))
      break
    }

    case 'assessment.created': {
      invalidate(queryKeys.assessments.all)
      break
    }

    case 'scenario.run.started':
    case 'scenario.run.updated':
    case 'scenario.run.ended': {
      invalidate(queryKeys.scenarios.all)
      break
    }

    default:
      break
  }
}
