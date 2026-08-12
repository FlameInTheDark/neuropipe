import { useEffect, useId, useRef, type ReactNode } from "react";
import { createPortal } from "react-dom";

import { cn } from "@/lib/utils";

interface DialogProps {
  open: boolean;
  title: string;
  description?: string;
  children: ReactNode;
  className?: string;
  onOpenChange: (open: boolean) => void;
}

/** Shared application dialog with keyboard dismissal and focus restoration. */
export function Dialog({
  open,
  title,
  description,
  children,
  className,
  onOpenChange,
}: DialogProps) {
  const titleID = useId();
  const descriptionID = useId();
  const previousFocus = useRef<HTMLElement | undefined>(undefined);
  // The effect below restores focus when the dialog closes; a ref keeps the
  // identity of the callback out of the dependency list so inline closures do
  // not re-fire the effect and steal focus from form fields on each render.
  const onOpenChangeRef = useRef(onOpenChange);
  useEffect(() => {
    onOpenChangeRef.current = onOpenChange;
  }, [onOpenChange]);

  useEffect(() => {
    if (!open) return;
    previousFocus.current = document.activeElement instanceof HTMLElement
      ? document.activeElement
      : undefined;
    const dismiss = (event: KeyboardEvent) => {
      if (event.key === "Escape") onOpenChangeRef.current(false);
    };
    window.addEventListener("keydown", dismiss);
    return () => {
      window.removeEventListener("keydown", dismiss);
      previousFocus.current?.focus();
    };
  }, [open]);

  if (!open) return null;

  return createPortal(
    <div
      className="fixed inset-0 z-[120] flex items-center justify-center bg-black/70 p-5 backdrop-blur-[2px]"
      onPointerDown={(event) => {
        if (event.target === event.currentTarget) onOpenChange(false);
      }}
    >
      <section
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleID}
        aria-describedby={description ? descriptionID : undefined}
        className={cn(
          "flex max-h-[calc(100vh-40px)] w-full flex-col overflow-hidden rounded-xl border border-zinc-700 bg-zinc-950 shadow-2xl shadow-black/70",
          className,
        )}
      >
        <div className="border-b border-zinc-800 px-5 py-4">
          <h2 id={titleID} className="text-base font-semibold text-zinc-100">{title}</h2>
          {description ? <p id={descriptionID} className="mt-1 text-xs leading-5 text-zinc-400">{description}</p> : null}
        </div>
        {children}
      </section>
    </div>,
    document.body,
  );
}
