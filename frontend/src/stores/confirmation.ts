import { create } from "zustand";

export interface ConfirmationRequest {
  title: string;
  description: string;
  confirmLabel: string;
  danger?: boolean;
}

let pendingResolve: ((confirmed: boolean) => void) | undefined;

interface ConfirmationState {
  request: ConfirmationRequest | null;
  ask: (request: ConfirmationRequest) => Promise<boolean>;
  respond: (confirmed: boolean) => void;
}

/**
 * Promise-based confirmation flow shared app-wide.
 * A new request force-resolves any pending one as declined so callers
 * never hang when dialogs are replaced faster than they are answered.
 */
export const useConfirmation = create<ConfirmationState>((set) => ({
  request: null,
  ask: (request) =>
    new Promise<boolean>((resolve) => {
      pendingResolve?.(false);
      pendingResolve = resolve;
      set({ request });
    }),
  respond: (confirmed) => {
    pendingResolve?.(confirmed);
    pendingResolve = undefined;
    set({ request: null });
  },
}));

export function ask(request: ConfirmationRequest): Promise<boolean> {
  return useConfirmation.getState().ask(request);
}
