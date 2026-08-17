import { create } from 'zustand'
import type { InputDialogRequest, InputDialogResponse } from '@/lib/types'

interface InputDialogState {
  request?: InputDialogRequest
  resolve?: (response: InputDialogResponse) => void
  /** queue a request from a Wails event; only one dialog is visible at a time. */
  show: (request: InputDialogRequest) => void
  /** respond to the active dialog; sends the value to the Go side through the bridge. */
  respond: (response: InputDialogResponse) => void
}

/**
 * A single app-owned input dialog channel, shared by every pipeline that
 * raises a Display Input Dialog node. Only one dialog is visible at a time;
 * a new request cancels any pending request as cancelled so the Go side can
 * unblock and route from the Canceled exec pin.
 */
export const useInputDialogStore = create<InputDialogState>((set, get) => ({
  show: (request) => {
    // A new request must not leave an earlier caller waiting indefinitely.
    get().resolve?.({ canceled: true, value: '' })
    set({ request, resolve: undefined })
  },
  respond: (response) => {
    const resolve = get().resolve
    set({ request: undefined, resolve: undefined })
    resolve?.(response)
  },
}))

/**
 * Bridge a Wails event into the input dialog store. The Go side emits
 * `dialog.input.request` when a Display Input Dialog node fires; the host
 * component (App.tsx) wires the event to this dispatcher.
 */
export function dispatchInputDialogRequest(request: InputDialogRequest) {
  useInputDialogStore.getState().show(request)
}

/**
 * Bridge a Wails cancel event into the input dialog store. The Go side emits
 * `dialog.input.cancel` when the execution context is cancelled before the
 * user responds.
 */
export function cancelInputDialog(requestID: string) {
  const state = useInputDialogStore.getState()
  if (state.request?.id === requestID) {
    state.respond({ canceled: true, value: '' })
  }
}
