import { create } from "zustand";
import type { FormDialogRequest, InputDialogRequest } from "@/lib/types";

/**
 * Single-channel stores for backend-initiated dialogs (Display Input Dialog
 * and Form nodes). A new request force-resolves any pending one as canceled
 * so the Go side unblocks through its Canceled pin instead of hanging.
 * Resolvers live outside the store state so responding never depends on a
 * stale closure.
 */

export interface InputDialogResponse {
  canceled: boolean;
  value: string;
}

export interface FormDialogResponse {
  canceled: boolean;
  values: Record<string, string | number>;
}

let pendingInputResolve: ((response: InputDialogResponse) => void) | undefined;
let pendingFormResolve: ((response: FormDialogResponse) => void) | undefined;

interface InputDialogState {
  request: InputDialogRequest | null;
  show: (request: InputDialogRequest) => Promise<InputDialogResponse>;
  respond: (response: InputDialogResponse) => void;
}

export const useInputDialog = create<InputDialogState>((set) => ({
  request: null,
  show: (request) =>
    new Promise<InputDialogResponse>((resolve) => {
      pendingInputResolve?.({ canceled: true, value: "" });
      pendingInputResolve = resolve;
      set({ request });
    }),
  respond: (response) => {
    pendingInputResolve?.(response);
    pendingInputResolve = undefined;
    set({ request: null });
  },
}));

interface FormDialogState {
  request: FormDialogRequest | null;
  show: (request: FormDialogRequest) => Promise<FormDialogResponse>;
  respond: (response: FormDialogResponse) => void;
}

export const useFormDialog = create<FormDialogState>((set) => ({
  request: null,
  show: (request) =>
    new Promise<FormDialogResponse>((resolve) => {
      pendingFormResolve?.({ canceled: true, values: {} });
      pendingFormResolve = resolve;
      set({ request });
    }),
  respond: (response) => {
    pendingFormResolve?.(response);
    pendingFormResolve = undefined;
    set({ request: null });
  },
}));

export function dispatchInputDialogRequest(request: InputDialogRequest) {
  if (request?.id) void useInputDialog.getState().show(request);
}

export function cancelInputDialog(id: string) {
  const { request, respond } = useInputDialog.getState();
  if (request?.id === id) respond({ canceled: true, value: "" });
}

export function dispatchFormDialogRequest(request: FormDialogRequest) {
  if (request?.id) void useFormDialog.getState().show(request);
}

export function cancelFormDialog(id: string) {
  const { request, respond } = useFormDialog.getState();
  if (request?.id === id) respond({ canceled: true, values: {} });
}
