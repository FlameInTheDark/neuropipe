import { useEffect, useState, type Dispatch, type SetStateAction } from "react";

/**
 * Renderer-local UI preferences. Only cosmetic state is persisted here —
 * every piece of user data goes through the Desktop bridge.
 * Storage must never block rendering, so every access is silently guarded.
 */

function read<T>(key: string, fallback: T): T {
  try {
    const raw = window.localStorage.getItem(key);
    if (raw === null) return fallback;
    return JSON.parse(raw) as T;
  } catch {
    return fallback;
  }
}

function write(key: string, value: unknown) {
  try {
    window.localStorage.setItem(key, JSON.stringify(value));
  } catch {
    /* ignore quota/security errors */
  }
}

function useStored<T>(key: string, fallback: T): [T, Dispatch<SetStateAction<T>>] {
  const [value, setValue] = useState<T>(() => read(key, fallback));
  useEffect(() => {
    write(key, value);
  }, [key, value]);
  return [value, setValue];
}

/** Persisted string choice restricted to a fixed set of options. */
export function usePersistedChoice<T extends string>(
  key: string,
  choices: readonly T[],
  fallback: T,
): [T, (v: T) => void] {
  const [value, setValue] = useStored<T>(key, fallback);
  const set = (v: T | ((prev: T) => T)) => {
    const next = typeof v === "function" ? (v as (prev: T) => T)(value) : v;
    if (choices.includes(next)) setValue(next);
  };
  return [value, set as (v: T) => void];
}

/** Persisted arbitrary JSON value. */
export function usePersistedValue<T>(key: string, fallback: T): [T, Dispatch<SetStateAction<T>>] {
  return useStored<T>(key, fallback);
}
