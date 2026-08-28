import { memo, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import type { EditorComment } from "@/types";
import { Icon } from "./icons";
import { cn } from "../utils/cn";

/**
 * Soft acrylic colors for sticky notes. Values are theme variables
 * (index.css) so cards stay legible on light and dark canvases; tints are
 * derived from the hue vars via color-mix.
 */
const ACCENTS: Record<EditorComment["color"], { border: string; bg: string; text: string; dot: string; head: string }> = {
  slate:   { border: "var(--hue-slate-deep)", bg: "var(--hue-slate-card)", text: "var(--hue-slate-text)", dot: "var(--hue-slate)", head: "color-mix(in srgb, var(--hue-slate) 8%, transparent)" },
  violet:  { border: "var(--hue-violet-deep)", bg: "color-mix(in srgb, var(--hue-violet-deep) 6%, transparent)", text: "var(--hue-violet-text)", dot: "var(--hue-violet)", head: "color-mix(in srgb, var(--hue-violet) 12%, transparent)" },
  emerald: { border: "var(--hue-emerald-deep)", bg: "color-mix(in srgb, var(--hue-emerald-deep) 6%, transparent)", text: "var(--hue-emerald-text)", dot: "var(--hue-emerald)", head: "color-mix(in srgb, var(--hue-emerald) 12%, transparent)" },
  amber:   { border: "var(--hue-amber-deep)", bg: "color-mix(in srgb, var(--hue-amber-deep) 6%, transparent)", text: "var(--hue-amber-text)", dot: "var(--hue-amber)", head: "color-mix(in srgb, var(--hue-amber) 12%, transparent)" },
  sky:     { border: "var(--hue-sky-deep)", bg: "color-mix(in srgb, var(--hue-sky-deep) 6%, transparent)", text: "var(--hue-sky-text)", dot: "var(--hue-sky)", head: "color-mix(in srgb, var(--hue-sky) 12%, transparent)" },
  rose:    { border: "var(--hue-rose-deep)", bg: "color-mix(in srgb, var(--hue-rose-deep) 6%, transparent)", text: "var(--hue-rose-text)", dot: "var(--hue-rose)", head: "color-mix(in srgb, var(--hue-rose) 12%, transparent)" },
};

const RESIZE_SIZE = 12;

export const CommentCard = memo(function CommentCard({
  comment,
  selected,
  onHeaderPointerDown,
  onResizePointerDown,
  onRename,
  onContextMenu,
  onSelect,
  autoEdit = false,
  onEditDone,
}: {
  comment: EditorComment;
  selected: boolean;
  onHeaderPointerDown: (e: React.PointerEvent) => void;
  onResizePointerDown: (e: React.PointerEvent) => void;
  onRename: (text: string) => void;
  onContextMenu: (e: React.MouseEvent) => void;
  onSelect: () => void;
  autoEdit?: boolean;
  onEditDone?: () => void;
}) {
  const { t } = useTranslation();
  const accent = ACCENTS[comment.color ?? "amber"];
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(comment.text);
  const textRef = useRef<HTMLTextAreaElement>(null);

  useEffect(() => setDraft(comment.text), [comment.text]);
  useEffect(() => {
    if (editing) textRef.current?.focus();
  }, [editing]);
  useEffect(() => {
    if (autoEdit) setEditing(true);
  }, [autoEdit]);

  const finish = () => { setEditing(false); onEditDone?.(); };

  const commit = () => {
    const next = draft.trim();
    if (next !== comment.text) onRename(next);
    else setDraft(comment.text);
    finish();
  };

  return (
    <div
      data-comment={comment.id}
      onPointerDown={(e) => { e.stopPropagation(); if (!editing) onSelect(); }}
      onContextMenu={onContextMenu}
      style={{
        left: comment.x, top: comment.y,
        width: comment.w, height: comment.h,
        borderColor: selected ? accent.dot : accent.border,
        background: accent.bg,
      }}
      className={cn(
        "absolute overflow-hidden rounded-xl border backdrop-blur-[2px] transition-[border-color,box-shadow]",
        selected
          ? "shadow-[0_16px_32px_-12px_rgba(0,0,0,0.9),0_0_0_1px_rgba(236,237,241,0.18)]"
          : "shadow-[0_6px_16px_-10px_rgba(0,0,0,0.7)] hover:shadow-[0_10px_22px_-12px_rgba(0,0,0,0.85)]",
      )}
    >
      {/* drag header */}
      <div
        onPointerDown={(e) => { if (editing) return; onSelect(); onHeaderPointerDown(e); }}
        onDoubleClick={() => setEditing(true)}
        style={{ background: accent.head }}
        className="flex h-7 shrink-0 cursor-grab items-center gap-1.5 px-2.5 active:cursor-grabbing"
      >
        <span style={{ color: accent.dot }}><Icon name="StickyNote" className="h-3.5 w-3.5" /></span>
        <span className="text-[10px] font-semibold uppercase tracking-[0.12em] text-fg-subtle">{t("editor.note")}</span>
        <Icon name="GripVertical" className="ml-auto h-3.5 w-3.5 text-fg-faint opacity-60" />
      </div>

      {/* body */}
      <div className="h-[calc(100%-28px)] w-full p-3">
        {editing ? (
          <textarea
            ref={textRef}
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onBlur={commit}
            onPointerDown={(e) => e.stopPropagation()}
            onKeyDown={(e) => {
              e.stopPropagation();
              if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); commit(); }
              if (e.key === "Escape") { setDraft(comment.text); finish(); }
            }}
            className="h-full w-full resize-none rounded-md bg-ink-950/70 p-2 text-[12px] leading-relaxed text-fg outline-none ring-1 ring-ring/60"
          />
        ) : (
          <p
            onDoubleClick={() => setEditing(true)}
            style={{ color: accent.text }}
            className="h-full w-full select-text overflow-y-auto whitespace-pre-wrap pr-1 text-[12.5px] leading-[1.55]"
          >
            {comment.text}
          </p>
        )}
      </div>

      {/* resize corner */}
      <span
        onPointerDown={(e) => { e.stopPropagation(); onSelect(); onResizePointerDown(e); }}
        style={{ width: RESIZE_SIZE, height: RESIZE_SIZE }}
        className="absolute bottom-1 right-1 cursor-se-resize opacity-60 hover:opacity-100"
      >
        <svg viewBox="0 0 10 10" className="h-full w-full text-fg-faint">
          <path d="M10,0 L0,10 M10,4 L4,10 M10,8 L8,10" stroke="currentColor" strokeWidth="1" strokeLinecap="round" />
        </svg>
      </span>
    </div>
  );
});
