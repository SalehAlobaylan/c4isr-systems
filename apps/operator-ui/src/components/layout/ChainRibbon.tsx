import { useWorkspace } from '@/lib/workspaceContext'
import { useQuery } from '@/lib/scopedQueries'

import { Link } from '@tanstack/react-router'

import { useAPI } from '@/lib/api'
import { useQueryKeys } from '@/lib/queryKeys'

/**
 * The chain of custody, read live.
 *
 * The footer used to print the pipeline as a string — "sources → observations
 * → tracks → picture → decision → command" — with no numbers, which is a
 * claim about how the system is built rather than a reading of what it
 * holds. It also repeated the brand and the operator identity, both of which
 * the header already carries.
 *
 * This is the same chain with what is actually at each stage, and each stage
 * links to the view that shows it. A number that moves is the only kind of
 * footer an operator reads.
 */
const STAGES = [
  { key: 'sources', one: 'source', many: 'sources', to: null },
  { key: 'observations', one: 'observation', many: 'observations', to: null },
  { key: 'tracks', one: 'track', many: 'tracks', to: '/tracks' },
  { key: 'picture', one: 'entity', many: 'entities', to: '/map' },
  { key: 'decisions', one: 'decision', many: 'decisions', to: '/incidents' },
  { key: 'commands', one: 'command', many: 'commands', to: '/incidents' },
] as const

/**
 * One small request per stage, asking only for the first row so the server
 * answers with a total rather than a page of data. The counts are context,
 * not decisions, so they are allowed to be a minute old — but they are never
 * older than the page they are on.
 */
const LIST_LIMIT = 1

export function ChainRibbon() {
  const publicContext = useWorkspace().session.resource.partition === 'PUBLIC_EVIDENCE'
  const api = useAPI()
  const queryKeys = useQueryKeys()
  const sources = useQuery({
    queryKey: queryKeys.sources.list({ limit: LIST_LIMIT }),
    queryFn: () => api.listSources({ limit: LIST_LIMIT }),
    staleTime: 60_000,
    retry: false,
  })
  const observations = useQuery({
    queryKey: queryKeys.observations.list({ limit: LIST_LIMIT }),
    queryFn: () => api.listObservations({ limit: LIST_LIMIT }),
    staleTime: 60_000,
    retry: false,
  })
  const tracks = useQuery({
    queryKey: queryKeys.tracks.list({ limit: LIST_LIMIT }),
    queryFn: () => api.listTracks({ limit: LIST_LIMIT }),
    staleTime: 60_000,
    retry: false,
  })
  const assets = useQuery({
    queryKey: queryKeys.assets.list({ limit: LIST_LIMIT }),
    queryFn: () => api.listAssets({ limit: LIST_LIMIT }),
    staleTime: 60_000,
    retry: false,
  })
  const incidents = useQuery({
    queryKey: queryKeys.incidents.list({ limit: LIST_LIMIT }),
    queryFn: () => api.listIncidents({ limit: LIST_LIMIT }),
    staleTime: 60_000,
    retry: false,
  })
  const commands = useQuery({
    queryKey: queryKeys.commands.list({ limit: LIST_LIMIT }),
    queryFn: () => api.listCommands({ limit: LIST_LIMIT }),
    staleTime: 60_000,
    retry: false,
  })


  const counts: Record<(typeof STAGES)[number]['key'], number | null> = {
    sources: sources.data?.total ?? null,
    observations: observations.data?.total ?? null,
    tracks: tracks.data?.total ?? null,
    picture: tracks.data ? tracks.data.total + (assets.data?.total ?? 0) : null,
    decisions: incidents.data?.total ?? null,
    commands: commands.data?.total ?? null,
  }

  return (
    <nav className="aegis-chain-ribbon" aria-label="Chain of custody">
      {STAGES.filter((stage) => !publicContext || stage.key !== 'commands').map((stage, index) => {
        const count = counts[stage.key]
        // A count of one reads as one. "1 observations" is a defect in the
        // number, not in the wording.
        const label = count === 1 ? stage.one : stage.many
        return (
          <span key={stage.key} className="aegis-chain-stage">
            {index > 0 ? (
              <span className="aegis-chain-arrow" aria-hidden="true">
                →
              </span>
            ) : null}
            {stage.to ? (
              <Link to={stage.to} className="aegis-chain-link" title={`${label} — open`}>
                <span className="aegis-chain-count">{count ?? '—'}</span>{' '}
                <span className="aegis-chain-label">{label}</span>
              </Link>
            ) : (
              <span className="aegis-chain-link" aria-label={`${label}: ${count ?? 'unknown'}`}>
                <span className="aegis-chain-count">{count ?? '—'}</span>{' '}
                <span className="aegis-chain-label">{label}</span>
              </span>
            )}
          </span>
        )
      })}
    </nav>
  )
}
