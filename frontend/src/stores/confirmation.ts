import { create } from 'zustand'

export interface ConfirmationRequest {
  title: string
  description: string
  confirmLabel: string
}

interface ConfirmationState {
  request?: ConfirmationRequest
  resolve?: (confirmed: boolean) => void
  ask: (request: ConfirmationRequest) => Promise<boolean>
  respond: (confirmed: boolean) => void
}

/** A single app-owned confirmation channel, shared by every destructive action. */
export const useConfirmationStore = create<ConfirmationState>((set, get) => ({
  ask: (request) => new Promise<boolean>((resolve) => {
    // A new request must not leave an earlier caller waiting indefinitely.
    get().resolve?.(false)
    set({ request, resolve })
  }),
  respond: (confirmed) => {
    const resolve = get().resolve
    set({ request: undefined, resolve: undefined })
    resolve?.(confirmed)
  },
}))
