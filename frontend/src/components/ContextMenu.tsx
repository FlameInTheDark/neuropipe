import { type ReactNode, useEffect, useRef } from "react";

import { cn } from "@/lib/utils";

export interface ContextMenuPoint {
  clientX: number;
  clientY: number;
}

export interface ContextMenuPosition {
  x: number;
  y: number;
}

export interface ContextMenuSize {
  width: number;
  height: number;
}

/** Clamps a cursor-triggered menu so every action remains in the viewport. */
export function contextMenuPosition(
  point: ContextMenuPoint,
  size: ContextMenuSize,
  containerBounds?: Pick<DOMRect, "left" | "top" | "width" | "height">,
): ContextMenuPosition {
  const gutter = 8;
  if (containerBounds) {
    const relativeX = point.clientX - containerBounds.left;
    const relativeY = point.clientY - containerBounds.top;
    return {
      x: Math.max(gutter, Math.min(relativeX, Math.max(gutter, containerBounds.width - size.width - gutter))),
      y: Math.max(gutter, Math.min(relativeY, Math.max(gutter, containerBounds.height - size.height - gutter))),
    };
  }
  const viewportWidth = typeof window === "undefined" ? point.clientX + size.width + gutter : window.innerWidth;
  const viewportHeight = typeof window === "undefined" ? point.clientY + size.height + gutter : window.innerHeight;
  return {
    x: Math.max(gutter, Math.min(point.clientX, viewportWidth - size.width - gutter)),
    y: Math.max(gutter, Math.min(point.clientY, viewportHeight - size.height - gutter)),
  };
}

/** Uses a predictable in-row location for keyboard context-menu shortcuts. */
export function contextMenuPointFromElement(element: HTMLElement): ContextMenuPoint {
  const bounds = element.getBoundingClientRect();
  return { clientX: bounds.left + 24, clientY: bounds.top + 24 };
}

/**
 * Shared accessible menu surface for list and canvas context actions.
 * It owns focus, outside-click dismissal, Escape, and viewport-safe placement.
 */
export function ContextMenu({
  position,
  ariaLabel,
  onClose,
  children,
  className,
  positionMode = "fixed",
}: {
  position: ContextMenuPosition;
  ariaLabel: string;
  onClose: () => void;
  children: ReactNode;
  className?: string;
  positionMode?: "fixed" | "absolute";
}) {
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : undefined;
    const focusInitialItem = () => {
      const menu = menuRef.current;
      const target = menu?.querySelector<HTMLElement>("[data-context-menu-initial-focus], [role='menuitem']:not([disabled])");
      target?.focus();
    };
    const animationFrame = requestAnimationFrame(focusInitialItem);
    const dismiss = (event: PointerEvent) => {
      if (!menuRef.current?.contains(event.target as Node)) onClose();
    };
    const escape = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    window.addEventListener("pointerdown", dismiss);
    window.addEventListener("keydown", escape);
    return () => {
      cancelAnimationFrame(animationFrame);
      window.removeEventListener("pointerdown", dismiss);
      window.removeEventListener("keydown", escape);
      previousFocus?.focus();
    };
  }, [onClose]);

  return (
    <div
      ref={menuRef}
      role="menu"
      aria-label={ariaLabel}
      className={cn(
        positionMode === "fixed" ? "fixed" : "absolute",
        "z-50 overflow-hidden rounded-lg border border-zinc-700 bg-zinc-950 p-1 shadow-2xl shadow-black/60",
        className,
      )}
      style={{ left: position.x, top: position.y }}
      onContextMenu={(event) => event.preventDefault()}
    >
      {children}
    </div>
  );
}
