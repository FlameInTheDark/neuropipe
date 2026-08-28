import { create } from "zustand";
import { readStoredValue, writeStoredValue } from "../lib/prefs";

/**
 * Dual-theme state. Dark is the default and matches the app's history;
 * the persisted choice uses the shared prefs storage format (JSON string).
 */

export type Theme = "dark" | "light";

const STORAGE_KEY = "neuropipe.theme.v1";

function isTheme(v: unknown): v is Theme {
  return v === "dark" || v === "light";
}

function applyThemeToDocument(theme: Theme): void {
  const root = document.documentElement;
  root.dataset.theme = theme;
  root.style.colorScheme = theme;
}

interface ThemeState {
  theme: Theme;
  setTheme: (theme: Theme) => void;
}

export const useThemeStore = create<ThemeState>((set) => ({
  theme: readStoredValue<Theme>(STORAGE_KEY, "dark") ?? "dark",
  setTheme: (theme) => {
    if (!isTheme(theme)) return;
    writeStoredValue(STORAGE_KEY, theme);
    applyThemeToDocument(theme);
    set({ theme });
  },
}));

/** Apply the persisted theme to <html> — called once before first render. */
export function initTheme(): void {
  let theme = readStoredValue<Theme>(STORAGE_KEY, "dark");
  if (!isTheme(theme)) theme = "dark";
  applyThemeToDocument(theme);
  if (useThemeStore.getState().theme !== theme) {
    useThemeStore.setState({ theme });
  }
}

/** Reactive hook for components that must re-render on theme flips. */
export function useTheme(): Theme {
  return useThemeStore((s) => s.theme);
}
