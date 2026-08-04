import { useCallback, useState } from "react";

function readPreference(key: string): unknown {
  try {
    const stored = window.localStorage.getItem(key);
    return stored === null ? undefined : JSON.parse(stored);
  } catch {
    return undefined;
  }
}

function writePreference(key: string, value: unknown) {
  try {
    window.localStorage.setItem(key, JSON.stringify(value));
  } catch {
    // Local preferences are optional: unavailable storage must never block the UI.
  }
}

/** Stores a versioned set of collapsed UI sections in local browser storage. */
export function usePersistedCollapsedSections(key: string) {
  const [collapsed, setCollapsed] = useState<Set<string>>(() => {
    const value = readPreference(key);
    return new Set(
      Array.isArray(value)
        ? value.filter((item): item is string => typeof item === "string")
        : [],
    );
  });

  const toggle = useCallback(
    (section: string) => {
      setCollapsed((current) => {
        const next = new Set(current);
        if (next.has(section)) next.delete(section);
        else next.add(section);
        writePreference(key, [...next].sort());
        return next;
      });
    },
    [key],
  );

  return [collapsed, toggle] as const;
}

/** Stores one value from a fixed list of UI choices in local browser storage. */
export function usePersistedChoice<T extends string>(
  key: string,
  choices: readonly T[],
  fallback: T,
) {
  const [value, setValue] = useState<T>(() => {
    const stored = readPreference(key);
    return typeof stored === "string" && choices.includes(stored as T)
      ? (stored as T)
      : fallback;
  });

  const select = useCallback(
    (next: T) => {
      setValue(next);
      writePreference(key, next);
    },
    [key],
  );

  return [value, select] as const;
}

/** Stores a small renderer-only preference. Invalid or unavailable local
 * storage falls back without affecting the desktop workspace. */
export function usePersistedValue<T>(key: string, fallback: T) {
  const [value, setValue] = useState<T>(() => {
    const stored = readPreference(key);
    return stored === undefined ? fallback : stored as T;
  });

  const update = useCallback((next: T | ((current: T) => T)) => {
    setValue((current) => {
      const resolved = typeof next === "function"
        ? (next as (current: T) => T)(current)
        : next;
      writePreference(key, resolved);
      return resolved;
    });
  }, [key]);

  return [value, update] as const;
}
