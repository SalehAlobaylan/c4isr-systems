import { create } from 'zustand'
import { persist } from 'zustand/middleware'

export type LayoutDensity = 'comfortable' | 'compact'
export type LayoutToggleKey =
  | 'chipsOpen'
  | 'runOpen'
  | 'overlayOpen'
  | 'railOpen'
  | 'layersOpen'
  | 'trayOpen'

export interface LayoutState {
  density: LayoutDensity
  chipsOpen: boolean
  runOpen: boolean
  overlayOpen: boolean
  railOpen: boolean
  railWidth: number
  layersOpen: boolean
  trayOpen: boolean
  setDensity: (density: LayoutDensity) => void
  toggle: (key: LayoutToggleKey) => void
  setRailWidth: (width: number) => void
  reset: () => void
}

const defaults = {
  density: 'comfortable' as LayoutDensity,
  chipsOpen: true,
  runOpen: false,
  // The board overlay hangs over the top of the plot. The picture is the
  // workspace, so it starts collapsed and the operator opens it deliberately —
  // the same reasoning as the telemetry dock. It is one keystroke away.
  overlayOpen: false,
  railOpen: true,
  railWidth: 380,
  layersOpen: false,
  trayOpen: false,
}

export const useLayoutStore = create<LayoutState>()(
  persist(
    (set) => ({
      ...defaults,
      setDensity: (density) => set({ density }),
      toggle: (key) => set((state) => ({ [key]: !state[key] })),
      setRailWidth: (width) => {
        if (Number.isFinite(width)) set({ railWidth: Math.max(300, Math.min(600, Math.round(width))) })
      },
      reset: () => set(defaults),
    }),
    {
      name: 'aegis.c2.layout.v1',
      partialize: ({ density, chipsOpen, runOpen, overlayOpen, railOpen, railWidth, layersOpen, trayOpen }) => ({
        density, chipsOpen, runOpen, overlayOpen, railOpen, railWidth, layersOpen, trayOpen,
      }),
    },
  ),
)
