import { Icon } from "../icons";
import type { Toast } from "../../hooks/useToast";

/** Transient confirmation message pinned above the status bar. */
export function Toaster({ toast }: { toast: Toast | null }) {
  if (!toast) return null;
  return (
    <div className="pop-in pointer-events-none fixed bottom-9 left-1/2 z-50 flex -translate-x-1/2 items-center gap-2 rounded-lg border border-ink-650 bg-ink-800/95 px-3 py-2 text-[12.5px] text-ink-50 shadow-[0_16px_40px_-16px_rgba(0,0,0,0.9)] backdrop-blur">
      <Icon name={toast.icon} className="h-3.5 w-3.5 text-ink-300" />
      {toast.text}
    </div>
  );
}
