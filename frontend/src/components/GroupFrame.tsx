import { memo, useEffect, useRef, useState } from "react";
import type { NodeGroup } from "@/types";
import { GROUP_HEADER_H } from "@/features/graph/graph-ops";
import { Icon } from "./icons";
import { cn } from "../utils/cn";

/**
 * Accent tokens per group colour. Kept here so the palette lives in one
 * place. Values are theme variables (index.css); tints derive via color-mix.
 */
const ACCENTS: Record<NodeGroup["color"], { border: string; head: string; body: string; text: string; dot: string }> = {
  slate:   { border: "var(--hue-slate)", head: "color-mix(in srgb, var(--hue-slate) 20%, transparent)", body: "color-mix(in srgb, var(--hue-slate) 7%, transparent)", text: "var(--hue-slate-text)", dot: "var(--hue-slate)" },
  violet:  { border: "var(--hue-violet)", head: "color-mix(in srgb, var(--hue-violet) 18%, transparent)", body: "color-mix(in srgb, var(--hue-violet) 7%, transparent)", text: "var(--hue-violet-text)", dot: "var(--hue-violet)" },
  emerald: { border: "var(--hue-emerald)", head: "color-mix(in srgb, var(--hue-emerald) 18%, transparent)", body: "color-mix(in srgb, var(--hue-emerald) 7%, transparent)", text: "var(--hue-emerald-text)", dot: "var(--hue-emerald)" },
  amber:   { border: "var(--hue-amber)", head: "color-mix(in srgb, var(--hue-amber) 18%, transparent)", body: "color-mix(in srgb, var(--hue-amber) 7%, transparent)", text: "var(--hue-amber-text)", dot: "var(--hue-amber)" },
  sky:     { border: "var(--hue-sky)", head: "color-mix(in srgb, var(--hue-sky) 18%, transparent)", body: "color-mix(in srgb, var(--hue-sky) 7%, transparent)", text: "var(--hue-sky-text)", dot: "var(--hue-sky)" },
  rose:    { border: "var(--hue-rose)", head: "color-mix(in srgb, var(--hue-rose) 18%, transparent)", body: "color-mix(in srgb, var(--hue-rose) 7%, transparent)", text: "var(--hue-rose-text)", dot: "var(--hue-rose)" },
};

export type ResizeHandle = "nw" | "n" | "ne" | "e" | "se" | "s" | "sw" | "w";

/** Thickness of the invisible hover band along each edge (graph px). */
const EDGE = 8;

/**
 * Edge & corner hit zones. They sit just inside the frame border, are
 * invisible, and only reveal their resize cursor on hover - no persistent grips.
 */
const ZONES: { id: ResizeHandle; cls: string }[] = [
  { id: "n",  cls: "top-0 left-0 right-0 cursor-ns-resize" },
  { id: "s",  cls: "bottom-0 left-0 right-0 cursor-ns-resize" },
  { id: "w",  cls: "top-0 bottom-0 left-0 cursor-ew-resize" },
  { id: "e",  cls: "top-0 bottom-0 right-0 cursor-ew-resize" },
  { id: "nw", cls: "top-0 left-0 cursor-nwse-resize" },
  { id: "ne", cls: "top-0 right-0 cursor-nesw-resize" },
  { id: "sw", cls: "bottom-0 left-0 cursor-nesw-resize" },
  { id: "se", cls: "bottom-0 right-0 cursor-nwse-resize" },
];

export const GroupFrame = memo(function GroupFrame({
  group,
  memberCount,
  selected,
  onHeaderPointerDown,
  onResizePointerDown,
  onRename,
  onContextMenu,
  onSelect,
  autoEdit = false,
  onEditDone,
}: {
  group: NodeGroup;
  memberCount: number;
  selected: boolean;
  onHeaderPointerDown: (e: React.PointerEvent) => void;
  onResizePointerDown: (e: React.PointerEvent, handle: ResizeHandle) => void;
  onRename: (title: string) => void;
  onContextMenu: (e: React.MouseEvent) => void;
  onSelect: () => void;
  /** Driven by the context menu's "Rename group" action. */
  autoEdit?: boolean;
  onEditDone?: () => void;
}) {
  const accent = ACCENTS[group.color];
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(group.title);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => setDraft(group.title), [group.title]);
  useEffect(() => {
    if (editing) inputRef.current?.select();
  }, [editing]);

  // the context menu's rename action drops us straight into the title input
  useEffect(() => {
    if (autoEdit) setEditing(true);
  }, [autoEdit]);

  const finish = () => {
    setEditing(false);
    onEditDone?.();
  };

  const commit = () => {
    const next = draft.trim();
    if (next && next !== group.title) onRename(next);
    else setDraft(group.title);
    finish();
  };

  return (
    <div
      data-group={group.id}
      onContextMenu={onContextMenu}
      style={{
        left: group.x,
        top: group.y,
        width: group.w,
        height: group.h,
        borderColor: selected ? accent.dot : accent.border,
        background: accent.body,
      }}
      className={cn(
        "absolute rounded-xl border-2 transition-[border-color,box-shadow]",
        selected && "shadow-[0_0_0_1px_rgba(236,237,241,0.10)]",
      )}
    >
      {/* title bar - drags the frame and everything inside it */}
      <div
        onPointerDown={(e) => {
          if (editing) return;
          onSelect();
          onHeaderPointerDown(e);
        }}
        onDoubleClick={() => setEditing(true)}
        style={{ height: GROUP_HEADER_H, background: accent.head }}
        className="flex cursor-grab items-center gap-2 rounded-t-[10px] px-2.5 active:cursor-grabbing"
      >
        <span className="h-2 w-2 shrink-0 rounded-full" style={{ background: accent.dot }} />

        {editing ? (
          <input
            ref={inputRef}
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onBlur={commit}
            onPointerDown={(e) => e.stopPropagation()}
            onKeyDown={(e) => {
              e.stopPropagation();
              if (e.key === "Enter") commit();
              if (e.key === "Escape") {
                setDraft(group.title);
                finish();
              }
            }}
            className="min-w-0 flex-1 rounded bg-ink-950/60 px-1 text-[12px] font-semibold text-fg outline-none ring-1 ring-ring/60"
          />
        ) : (
          <span className="min-w-0 flex-1 truncate text-[12px] font-semibold" style={{ color: accent.text }}>
            {group.title}
          </span>
        )}

        <span className="shrink-0 rounded bg-ink-950/50 px-1.5 font-mono text-[9.5px] text-fg-subtle">
          {memberCount}
        </span>
        <Icon name="GripVertical" className="h-3.5 w-3.5 shrink-0 text-fg-faint" />
      </div>

      {/* invisible edge/corner hover bands - resize by grabbing the border */}
      {ZONES.map((z) => {
        const corner = z.id.length === 2;
        return (
          <span
            key={z.id}
            onPointerDown={(e) => {
              e.stopPropagation();
              onSelect();
              onResizePointerDown(e, z.id);
            }}
            style={{
              width: z.id === "n" || z.id === "s" ? undefined : EDGE,
              height: z.id === "e" || z.id === "w" ? undefined : EDGE,
              ...(corner ? { width: EDGE, height: EDGE } : {}),
            }}
            className={cn("absolute z-10", z.cls)}
          />
        );
      })}
    </div>
  );
});
