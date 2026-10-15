import { create } from 'zustand'

/**
 * Replay state for the picture scrubber.
 *
 * The scrubber is the product's distinguishing capability - the ability to ask
 * "what did this look like at the moment of the breach?" - and that only works
 * if the interface is unambiguous about which moment it is showing. Live is the
 * default and any doubt about it is worse than no replay at all, because an
 * operator acting on a stale picture believing it is current is the failure
 * mode this whole feature has to avoid.
 */
interface ScrubState {
  /** True when the picture is the present moment rather than a replay. */
  live: boolean
  /** Epoch milliseconds of the replayed instant; null when live. */
  at: number | null
  /** Bounds of the recorded evidence, from the server. */
  earliest: number | null
  latest: number | null
  /** Set when the operator has scrolled away from the live picture. */
  setAt: (at: number | null, bounds?: { earliest: number; latest: number }) => void
  setWindow: (earliest: string, latest: string) => void
  returnToLive: () => void
}

export const useScrubStore = create<ScrubState>((set) => ({
  live: true,
  at: null,
  earliest: null,
  latest: null,
  setAt: (at, bounds) =>
    set((state) => ({
      // Selecting the newest instant is still a replay until the operator
      // explicitly returns to live; silently flipping the mode would mean the
      // label lies about what the screen is showing.
      live: false,
      at,
      earliest: bounds?.earliest ?? state.earliest,
      latest: bounds?.latest ?? state.latest,
    })),
  setWindow: (earliest, latest) =>
    set({
      earliest: Date.parse(earliest) || null,
      latest: Date.parse(latest) || null,
    }),
  returnToLive: () => set({ live: true, at: null }),
}))

/** Selector: the instant to request, or undefined for the live picture. */
export function selectReplayAt(state: ScrubState): string | undefined {
  if (state.live || state.at === null) return undefined
  return new Date(state.at).toISOString()
}