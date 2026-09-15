import { create } from 'zustand'

export type ToastVariant = 'info' | 'success' | 'warning' | 'error'

export interface ToastItem {
  id: string
  title: string
  description?: string
  variant: ToastVariant
  createdAt: number
  durationMs: number
}

export interface ToastInput {
  title: string
  description?: string
  variant?: ToastVariant
  durationMs?: number
}

interface ToastState {
  toasts: ToastItem[]
  push: (input: ToastInput) => string
  dismiss: (id: string) => void
}

let counter = 0

export const useToastStore = create<ToastState>((set, get) => ({
  toasts: [],
  push: (input) => {
    const id = `toast-${++counter}`
    const item: ToastItem = {
      id,
      title: input.title,
      description: input.description,
      variant: input.variant ?? 'info',
      durationMs: input.durationMs ?? 6000,
      createdAt: Date.now(),
    }
    set((state) => ({ toasts: [...state.toasts, item].slice(-5) }))
    if (item.durationMs > 0) {
      window.setTimeout(() => get().dismiss(id), item.durationMs)
    }
    return id
  },
  dismiss: (id) => set((state) => ({ toasts: state.toasts.filter((toast) => toast.id !== id) })),
}))

export function toast(input: ToastInput): string {
  return useToastStore.getState().push(input)
}
