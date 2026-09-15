import { create } from 'zustand'

export type EntityKind = 'track' | 'asset'

export interface Selection {
  kind: EntityKind
  id: string
}

export type MapMode = 'inspect' | 'measure'

export interface GeoPointTuple {
  lng: number
  lat: number
}

interface UIState {
  /** Current map interaction mode. */
  mapMode: MapMode
  setMapMode: (mode: MapMode) => void

  /** Measure/drawing state (WGS84 [lng, lat] pairs). */
  drawPoints: GeoPointTuple[]
  addDrawPoint: (point: GeoPointTuple) => void
  clearDrawing: () => void

  /** Map selection mirror of the `/map?selected=` search param. */
  selected: Selection | null
  setSelected: (selection: Selection | null) => void

  /** Detail panel visibility. */
  panelOpen: boolean
  setPanelOpen: (open: boolean) => void

  /** Selected track history polyline visibility. */
  showTrackHistory: boolean
  toggleTrackHistory: () => void
}

export const useUiStore = create<UIState>((set) => ({
  mapMode: 'inspect',
  setMapMode: (mode) => set({ mapMode: mode }),

  drawPoints: [],
  addDrawPoint: (point) =>
    set((state) => ({ drawPoints: [...state.drawPoints, point].slice(-2) })),
  clearDrawing: () => set({ drawPoints: [] }),

  selected: null,
  setSelected: (selection) => set({ selected: selection }),

  panelOpen: true,
  setPanelOpen: (open) => set({ panelOpen: open }),

  showTrackHistory: true,
  toggleTrackHistory: () => set((state) => ({ showTrackHistory: !state.showTrackHistory })),
}))

export function parseSelection(raw?: string | null): Selection | null {
  if (!raw) return null
  const separator = raw.indexOf(':')
  if (separator <= 0) return null
  const kind = raw.slice(0, separator)
  const id = raw.slice(separator + 1)
  if ((kind === 'track' || kind === 'asset') && id) return { kind, id }
  return null
}

export function formatSelection(selection: Selection): string {
  return `${selection.kind}:${selection.id}`
}
