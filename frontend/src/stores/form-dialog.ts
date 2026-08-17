import { create } from 'zustand'
import type { FormDialogRequest, FormDialogResponse } from '@/lib/types'

interface FormDialogState {
  request?: FormDialogRequest
  resolve?: (response: FormDialogResponse) => void
  /** queue a request from a Wails event; only one dialog is visible at a time. */
  show: (request: FormDialogRequest) => void
  /** respond to the active dialog; sends the values to the Go side through the bridge. */
  respond: (response: FormDialogResponse) => void
}

/**
 * A single app-owned form dialog channel, shared by every pipeline that
 * raises a Form node. Only one dialog is visible at a time; a new request
 * cancels any pending request as cancelled so the Go side can unblock and
 * route from the Canceled exec pin.
 */
export const useFormDialogStore = create<FormDialogState>((set, get) => ({
  show: (request) => {
    // A new request must not leave an earlier caller waiting indefinitely.
    get().resolve?.({ canceled: true, values: {} })
    set({ request, resolve: undefined })
  },
  respond: (response) => {
    const resolve = get().resolve
    set({ request: undefined, resolve: undefined })
    resolve?.(response)
  },
}))

/**
 * Bridge a Wails event into the form dialog store. The Go side emits
 * `dialog.form.request` when a Form node fires; the host component (App.tsx)
 * wires the event to this dispatcher.
 */
export function dispatchFormDialogRequest(request: FormDialogRequest) {
  useFormDialogStore.getState().show(request)
}

/**
 * Bridge a Wails cancel event into the form dialog store. The Go side emits
 * `dialog.form.cancel` when the execution context is cancelled before the
 * user responds.
 */
export function cancelFormDialog(requestID: string) {
  const state = useFormDialogStore.getState()
  if (state.request?.id === requestID) {
    state.respond({ canceled: true, values: {} })
  }
}
