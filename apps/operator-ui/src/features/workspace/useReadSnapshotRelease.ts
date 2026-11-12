import { useEffect, useRef } from 'react'
import { useQueryClient, type QueryKey } from '@tanstack/react-query'
import { useAPI } from '@/lib/api'

/**
 * A retained map read holds one of the bounded read budget slots until it
 * expires. Releasing the manifest a view has superseded — and the one it held
 * when it unmounted — returns that slot immediately instead of at expiry, so
 * ordinary navigation and repeated refreshes cannot exhaust the budget.
 *
 * The server keeps one retained read per logical view, so a refresh of the
 * same view already supersedes its predecessor. This covers the cases the
 * server cannot see: a view whose scope or filters changed, and a view the
 * operator has closed.
 */
export function useReadSnapshotRelease(readId: string | undefined, cacheKey: readonly unknown[]) {
  const api = useAPI()
  const client = useQueryClient()
  const live = useRef<string | undefined>(undefined)
  const cache = useRef<readonly unknown[]>(cacheKey)
  cache.current = cacheKey
  useEffect(() => {
    const superseded = live.current
    live.current = readId
    if (superseded && superseded !== readId) void api.releaseAnalystRead(superseded).catch(() => {})
  }, [readId])
  useEffect(() => () => {
    const held = live.current
    live.current = undefined
    if (!held) return
    void api.releaseAnalystRead(held).catch(() => {})
    // The cached manifest names a snapshot that no longer exists. Keeping it
    // would show a view whose records can no longer be paged.
    client.removeQueries({ queryKey: cache.current as QueryKey })
  }, [])
}
