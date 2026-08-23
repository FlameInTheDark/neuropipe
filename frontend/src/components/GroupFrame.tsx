import { memo, useEffect, useRef, useState } from "react";
import type { NodeGroup } from "@/types";
import { GROUP_HEADER_H } from "@/features/graph/graph-ops";
import { Icon } from "./icons";
import { cn } from "../utils/cn";

/** Accent tokens per group colour. Kept here so the palette lives in one place. */
const ACCENTS: Record<NodeGroup["color"], { border: string; head: string; body: string; text: string; dot: string }> = {
  slate:   { border: "#55555f", head: "rgba(85,85,95,0.20)",    body: "rgba(85,85,95,0.07)",    text: "#cbd5e1", dot: "#94a3b8" },
  violet:  { border: "#7c3aed", head: "rgba(167,139,250,0.20)", body: "rgba(167,139,250,0.07)",  text: "#c4b5fd", dot: "#a78bfa" },
  emerald: { border: "#059669", head: "rgba(52,211,153,0.18)",  body: "rgba(52,211,153,0.06)",   text: "#6ee7b7", dot: "#34d399" },
  amber:   { border: "#ea580c", head: "rgba(251,146,60,0.18)",  body: "rgba(251,146,60,0.06)",   text: "#fdba74", dot: "#fb923c" },
  sky:     { border: "#0284c7", head: "rgba(56,189,248,0.18)",  body: "rgba(56,189,248,0.06)",   text: "#7dd3fc", dot: "#38bdf8" },
  rose:    { border: "#e11d48", head: "rgba(251,113,133,0.18)", body: "rgba(251,113,133,0.06)",  text: "#fda4af", dot: "#fb7185" },
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
            className="min-w-0 flex-1 rounded bg-ink-950/60 px-1 text-[12px] font-semibold text-ink-50 outline-none ring-1 ring-ink-500"
          />
        ) : (
          <span className="min-w-0 flex-1 truncate text-[12px] font-semibold" style={{ color: accent.text }}>
            {group.title}
          </span>
        )}

        <span className="shrink-0 rounded bg-ink-950/50 px-1.5 font-mono text-[9.5px] text-ink-300">
          {memberCount}
        </span>
        <Icon name="GripVertical" className="h-3.5 w-3.5 shrink-0 text-ink-500" />
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
