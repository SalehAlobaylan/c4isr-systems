import { create } from 'zustand'
import { persist } from 'zustand/middleware'

export type LayoutDensity = 'comfortable' | 'compact'
export type LayoutToggleKey =
  | 'chipsOpen'
  | 'runOpen'
  | 'overlayOpen'
  | 'talliesOpen'
  | 'railOpen'
  | 'layersOpen'

export interface LayoutState {
  density: LayoutDensity
  chipsOpen: boolean
  runOpen: boolean
  overlayOpen: boolean
  talliesOpen: boolean
  railOpen: boolean
  railWidth: number
  layersOpen: boolean
  setDensity: (density: LayoutDensity) => void
  toggle: (key: LayoutToggleKey) => void
  setRailWidth: (width: number) => void
  reset: () => void
}

const defaults = {
  density: 'comfortable' as LayoutDensity,
  chipsOpen: true,
  runOpen: false,
  overlayOpen: true,
  talliesOpen: true,
  railOpen: true,
  railWidth: 380,
  layersOpen: false,
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
      partialize: ({ density, chipsOpen, runOpen, overlayOpen, talliesOpen, railOpen, railWidth, layersOpen }) => ({
        density, chipsOpen, runOpen, overlayOpen, talliesOpen, railOpen, railWidth, layersOpen,
      }),
    },
  ),
)
