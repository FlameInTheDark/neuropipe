import { useEffect } from "react";

/**
 * Closes an overlay on Escape, and optionally handles Cmd/Ctrl+S as "save".
 * Previously re-implemented inside every modal.
 */
export function useDismissKeys(onClose: () => void, onSave?: () => void) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        onClose();
        return;
      }
      if (onSave && (e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "s") {
        e.preventDefault();
        onSave();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose, onSave]);
}
