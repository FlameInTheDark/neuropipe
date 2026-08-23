import { useState, type ReactNode } from "react";
import { cn } from "../../utils/cn";

/** Draggable edge used to resize a floating panel. */
export function Resizer({
  onDrag,
  side,
}: {
  onDrag: (dx: number) => void;
  side: "left" | "right";
}) {
  const [active, setActive] = useState(false);

  const start = (e: React.PointerEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setActive(true);
    let last = e.clientX;
    const move = (ev: PointerEvent) => {
      onDrag(ev.clientX - last);
      last = ev.clientX;
    };
    const up = () => {
      setActive(false);
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", up);
    };
    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", up);
  };

  return (
    <div
      onPointerDown={start}
      className={cn(
        "group absolute top-0 z-20 flex h-full w-[9px] cursor-col-resize items-center justify-center",
        side === "left" ? "-left-[4px]" : "-right-[4px]",
      )}
    >
      <span
        className={cn(
          "h-10 w-[3px] rounded-full bg-ink-700 transition-colors group-hover:bg-ink-500",
          active && "bg-ink-400",
        )}
      />
    </div>
  );
}

/** Glass panel that floats over the canvas (node library / inspector). */
export function FloatPanel({
  open,
  side,
  width,
  offset = 12,
  onResize,
  children,
}: {
  open: boolean;
  side: "left" | "right";
  width: number;
  offset?: number;
  /** omit to make the panel non-resizable */
  onResize?: (dx: number) => void;
  children: ReactNode;
}) {
  if (!open) return null;
  return (
    <div
      style={{ width, [side]: offset }}
      className="pop-in absolute top-3 bottom-3 z-30 flex flex-col rounded-xl border border-ink-700 bg-ink-900/92 shadow-[0_24px_60px_-20px_rgba(0,0,0,0.95)] backdrop-blur-xl transition-[left,right] duration-[220ms] ease-[cubic-bezier(0.4,0,0.2,1)]"
    >
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-xl">{children}</div>
      {onResize && <Resizer side={side === "left" ? "right" : "left"} onDrag={onResize} />}
    </div>
  );
}
