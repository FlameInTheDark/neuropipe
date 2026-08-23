import { useEffect } from "react";

export interface Hotkey {
  /** single character or key name, compared case-insensitively */
  key: string;
  /** require Cmd (mac) or Ctrl */
  mod?: boolean;
  /** skip when focus is inside an input/textarea/select */
  skipWhenTyping?: boolean;
  /** only fire when this is true */
  when?: boolean;
  run: () => void;
}

const TYPING_TAGS = ["INPUT", "TEXTAREA", "SELECT"];

/** Declarative global shortcut binding — replaces one long if/else chain. */
export function useHotkeys(bindings: Hotkey[]) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const typing =
        TYPING_TAGS.includes((e.target as HTMLElement)?.tagName) ||
        (e.target as HTMLElement)?.isContentEditable;
      const mod = e.metaKey || e.ctrlKey;

      for (const b of bindings) {
        if (b.when === false) continue;
        if (!!b.mod !== mod) continue;
        if (b.skipWhenTyping && typing) continue;
        if (e.key.toLowerCase() !== b.key.toLowerCase()) continue;
        e.preventDefault();
        b.run();
        return;
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [bindings]);
}
