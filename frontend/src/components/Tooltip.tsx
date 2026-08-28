import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { cn } from "../utils/cn";

type Side = "top" | "bottom" | "left" | "right";

export function Tooltip({
  content,
  hint,
  side = "top",
  delay = 300,
  disabled,
  className,
  children,
}: {
  content: React.ReactNode;
  hint?: string;
  side?: Side;
  delay?: number;
  disabled?: boolean;
  className?: string;
  children: React.ReactNode;
}) {
  const wrapRef = useRef<HTMLSpanElement>(null);
  const timer = useRef<number | undefined>(undefined);
  const [anchor, setAnchor] = useState<DOMRect | null>(null);

  const show = () => {
    if (disabled) return;
    window.clearTimeout(timer.current);
    timer.current = window.setTimeout(() => {
      const r = wrapRef.current?.getBoundingClientRect();
      if (r && r.width > 0) setAnchor(r);
    }, delay);
  };

  const hide = () => {
    window.clearTimeout(timer.current);
    setAnchor(null);
  };

  useEffect(() => () => window.clearTimeout(timer.current), []);
  useEffect(() => {
    if (disabled) hide();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [disabled]);

  return (
    <span
      ref={wrapRef}
      className={cn("inline-flex min-w-0", className)}
      onPointerEnter={show}
      onPointerLeave={hide}
      onPointerDown={hide}
      onFocus={show}
      onBlur={hide}
    >
      {children}
      {anchor && <Bubble anchor={anchor} side={side} content={content} hint={hint} />}
    </span>
  );
}

function Bubble({
  anchor,
  side,
  content,
  hint,
}: {
  anchor: DOMRect;
  side: Side;
  content: React.ReactNode;
  hint?: string;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const [pos, setPos] = useState<{ left: number; top: number } | null>(null);

  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    const r = el.getBoundingClientRect();
    const gap = 7;
    let left = 0;
    let top = 0;
    if (side === "top") {
      left = anchor.left + anchor.width / 2 - r.width / 2;
      top = anchor.top - r.height - gap;
    } else if (side === "bottom") {
      left = anchor.left + anchor.width / 2 - r.width / 2;
      top = anchor.bottom + gap;
    } else if (side === "right") {
      left = anchor.right + gap;
      top = anchor.top + anchor.height / 2 - r.height / 2;
    } else {
      left = anchor.left - r.width - gap;
      top = anchor.top + anchor.height / 2 - r.height / 2;
    }
    left = Math.min(Math.max(6, left), window.innerWidth - r.width - 6);
    top = Math.min(Math.max(6, top), window.innerHeight - r.height - 6);
    setPos({ left, top });
  }, [anchor, side]);

  return createPortal(
    <div
      ref={ref}
      role="tooltip"
      style={pos ? { left: pos.left, top: pos.top } : { left: -9999, top: -9999 }}
      className="tip-in pointer-events-none fixed z-[80] flex items-center gap-2 rounded-md border border-ink-650 bg-ink-800/95 px-2 py-1 text-[11.5px] whitespace-nowrap text-fg shadow-[0_10px_28px_-10px_rgba(0,0,0,0.9)] backdrop-blur"
    >
      {content}
      {hint && (
        <kbd className="rounded border border-ink-600 bg-ink-850 px-1 font-mono text-[10px] text-fg-subtle">{hint}</kbd>
      )}
    </div>,
    document.body,
  );
}
