/// <reference types="vite/client" />

interface Window {
  go?: { app?: { Desktop?: Record<string, (...args: unknown[]) => Promise<unknown>> } }
}
