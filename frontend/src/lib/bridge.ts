// Bridge between the React renderer and the Wails v3 backend. The Go side
// registers the Desktop service via application.NewService(desktop) in
// main.go; Wails v3 exposes every exported method through the named-call API
// provided by @wailsio/runtime. This module wraps that surface so the rest
// of the renderer never imports the runtime package directly.
import i18n from "@/i18n";
import { desktop as binding } from "../../bindings/neuropipe/desktop.js";

// Re-export the binding object so call sites can import { desktop } from '@/lib/bridge'.
export const desktop = binding;

// Mirrors the i18n key used when the runtime binding is not present (for
// example, when running the renderer outside Wails in a plain browser).
export function wailsUnavailable(): Error {
  return new Error(i18n.t("app.unavailable"));
}
