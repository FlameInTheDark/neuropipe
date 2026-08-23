import { useCallback, useRef, useState } from "react";

export interface Toast {
  id: number;
  text: string;
  icon: string;
}

/** Transient status messages. Owns its own timer cleanup. */
export function useToast(durationMs = 2200) {
  const [toast, setToast] = useState<Toast | null>(null);
  const timer = useRef<number | undefined>(undefined);

  const notify = useCallback(
    (text: string, icon = "Check") => {
      window.clearTimeout(timer.current);
      const id = Date.now();
      setToast({ id, text, icon });
      timer.current = window.setTimeout(
        () => setToast((t) => (t?.id === id ? null : t)),
        durationMs,
      );
    },
    [durationMs],
  );

  return { toast, notify };
}
