import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { useTranslation } from "react-i18next";
import type { LibraryCategory, LibraryItem } from "@/types";
import { Icon } from "./icons";
import { Tooltip } from "./Tooltip";
import { cn } from "../utils/cn";

const CAT_ICON: Record<string, string> = {
  AI: "Sparkles",
  Actions: "Zap",
  Archive: "Package",
  Canvas: "Frame",
  Chat: "MessagesSquare",
  Code: "Braces",
  Data: "Split",
  Database: "Database",
  Date: "Clock",
  Display: "MessageSquare",
  Files: "FileText",
};

export interface PickerAnchor {
  x: number;
  y: number;
  gx: number;
  gy: number;
}

export function NodePicker({
  at,
  library,
  snap,
  onPick,
  onClose,
  onFit,
  onToggleSnap,
  onAddComment,
}: {
  at: PickerAnchor;
  library: LibraryCategory[];
  snap: boolean;
  onPick: (item: LibraryItem, category: string) => void;
  onClose: () => void;
  onFit: () => void;
  onToggleSnap: () => void;
  onAddComment?: (at: { x: number; y: number }) => void;
}) {
  const { t } = useTranslation();
  const ref = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const [q, setQ] = useState("");
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({});
  const [hi, setHi] = useState(0);
  const [pos, setPos] = useState<{ left: number; top: number } | null>(null);

  const searching = q.trim().length > 0;

  const groups = useMemo(() => {
    const s = q.trim().toLowerCase();
    return library
      .map((c) => ({
        name: c.name,
        items: c.items.filter(
          (i) => !s || i.name.toLowerCase().includes(s) || i.desc.toLowerCase().includes(s),
        ),
      }))
      .filter((g) => g.items.length > 0);
  }, [q, library]);

  /* flat list of currently rendered items — drives keyboard nav */
  const visible = useMemo(() => {
    const out: { cat: string; item: LibraryItem }[] = [];
    groups.forEach((g) => {
      if (searching || !collapsed[g.name]) g.items.forEach((item) => out.push({ cat: g.name, item }));
    });
    return out;
  }, [groups, collapsed, searching]);

  useEffect(() => setHi(0), [q]);

  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    const r = el.getBoundingClientRect();
    let left = at.x + 2;
    let top = at.y + 2;
    if (left + r.width > window.innerWidth - 8) left = at.x - r.width - 2;
    if (top + r.height > window.innerHeight - 8) top = Math.max(8, at.y - r.height - 2);
    setPos({ left: Math.max(8, left), top: Math.max(8, top) });
  }, [at]);

  useEffect(() => {
    listRef.current?.querySelector(`[data-i="${hi}"]`)?.scrollIntoView({ block: "nearest" });
  }, [hi]);

  useEffect(() => {
    const onDown = (e: PointerEvent) => {
      if (!ref.current?.contains(e.target as Node)) onClose();
    };
    window.addEventListener("pointerdown", onDown);
    window.addEventListener("resize", onClose);
    return () => {
      window.removeEventListener("pointerdown", onDown);
      window.removeEventListener("resize", onClose);
    };
  }, [onClose]);

  const commit = (i: number) => {
    const hit = visible[i];
    if (hit) onPick(hit.item, hit.cat);
  };

  const onKey = (e: React.KeyboardEvent) => {
    if (e.key === "Escape") {
      e.preventDefault();
      onClose();
    } else if (e.key === "ArrowDown") {
      e.preventDefault();
      setHi((h) => Math.min(h + 1, visible.length - 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setHi((h) => Math.max(h - 1, 0));
    } else if (e.key === "Enter") {
      e.preventDefault();
      commit(hi);
    }
  };

  let idx = -1;

  return createPortal(
    <div
      ref={ref}
      onKeyDown={onKey}
      onContextMenu={(e) => e.preventDefault()}
      style={pos ? { left: pos.left, top: pos.top } : { left: -9999, top: -9999 }}
      className="timeline-menu fixed z-[60] flex w-[288px] flex-col overflow-hidden rounded-[10px] border border-ink-650 bg-ink-850/95 shadow-[0_22px_54px_-14px_rgba(0,0,0,0.95),0_0_0_1px_rgba(255,255,255,0.02)_inset] backdrop-blur-xl"
    >
      {/* header */}
      <div className="flex items-center gap-2 border-b border-seam px-2.5 py-2">
        <Icon name="Plus" className="h-3.5 w-3.5 text-ink-400" />
        <span className="text-[11px] font-medium tracking-[0.08em] text-ink-300 uppercase">{t("library.addNode")}</span>
        <span className="ml-auto font-mono text-[10px] text-ink-500">
          {Math.round(at.gx)}, {Math.round(at.gy)}
        </span>
      </div>

      {/* search */}
      <div className="border-b border-seam p-1.5">
        <div className="flex h-7 items-center gap-2 rounded-md border border-ink-700 bg-ink-900 px-2 focus-within:border-ink-500">
          <Icon name="Search" className="h-3.5 w-3.5 shrink-0 text-ink-500" />
          <input
            ref={inputRef}
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder={t("library.search")}
            className="min-w-0 flex-1 bg-transparent text-[12.5px] text-ink-50 placeholder:text-ink-500"
          />
          {q && (
            <button onClick={() => setQ("")} className="text-ink-500 hover:text-ink-200">
              <Icon name="X" className="h-3.5 w-3.5" />
            </button>
          )}
        </div>
      </div>

      {/* results */}
      <div ref={listRef} className="max-h-[292px] min-h-[80px] overflow-y-auto py-1">
        {groups.length === 0 && (
          <p className="px-3 py-6 text-center text-[11.5px] text-ink-500">{t("library.noMatchesFor", { q })}</p>
        )}

        {groups.map((g) => {
          const isOpen = searching || !collapsed[g.name];
          return (
            <div key={g.name}>
              <button
                onClick={() => setCollapsed((c) => ({ ...c, [g.name]: !c[g.name] }))}
                className="flex w-full items-center gap-1.5 px-2 py-1 text-left transition hover:bg-ink-800/70"
              >
                <Icon
                  name="ChevronRight"
                  className={cn("h-3 w-3 shrink-0 text-ink-600 transition-transform", isOpen && "rotate-90 text-ink-400")}
                />
                <Icon name={CAT_ICON[g.name] ?? "Boxes"} className="h-3 w-3 shrink-0 text-ink-500" />
                <span className="text-[10.5px] font-medium tracking-[0.08em] text-ink-400 uppercase">{g.name}</span>
                <span className="ml-auto font-mono text-[10px] text-ink-600">{g.items.length}</span>
              </button>

              {isOpen &&
                g.items.map((item) => {
                  idx += 1;
                  const i = idx;
                  return (
                    <button
                      key={`${g.name}-${item.name}`}
                      data-i={i}
                      onMouseEnter={() => setHi(i)}
                      onClick={() => commit(i)}
                      className={cn(
                        "flex w-full items-start gap-2.5 py-1.5 pr-2 pl-[26px] text-left transition-colors",
                        i === hi ? "bg-ink-650/70" : "hover:bg-ink-800/60",
                      )}
                    >
                      <span
                        className={cn(
                          "mt-[1px] grid h-[22px] w-[22px] shrink-0 place-items-center rounded-md border",
                          i === hi ? "border-ink-500 bg-ink-750 text-ink-50" : "border-ink-700 bg-ink-900 text-ink-300",
                        )}
                      >
                        <Icon name={item.icon} className="h-3 w-3" />
                      </span>
                      <span className="min-w-0 flex-1">
                        <span className="block truncate text-[12px] font-medium text-ink-50">{item.name}</span>
                        <span className="mt-[1px] line-clamp-1 block text-[11px] text-ink-500">{item.desc}</span>
                      </span>
                    </button>
                  );
                })}
            </div>
          );
        })}
      </div>

      {/* footer actions */}
      <div className="flex items-center gap-1 border-t border-seam px-1.5 py-1.5">
        <FootBtn icon="Maximize2" label={t("editor.fitGraph")} onClick={() => { onFit(); onClose(); }} />
        <FootBtn icon="Magnet" label={snap ? t("status.snapOn") : t("status.snapOff")} active={snap} onClick={onToggleSnap} />
        {onAddComment && (
          <FootBtn
            icon="StickyNote"
            label={t("editor.addNote")}
            onClick={() => { onAddComment({ x: at.gx, y: at.gy }); onClose(); }}
          />
        )}
        <span className="ml-auto flex items-center gap-1 pr-1 font-mono text-[10px] text-ink-600">
          <kbd className="rounded border border-ink-700 bg-ink-900 px-1">↑↓</kbd>
          <kbd className="rounded border border-ink-700 bg-ink-900 px-1">↵</kbd>
        </span>
      </div>
    </div>,
    document.body,
  );
}

function FootBtn({
  icon,
  label,
  onClick,
  active,
}: {
  icon: string;
  label: string;
  onClick: () => void;
  active?: boolean;
}) {
  return (
    <Tooltip content={label} side="top">
      <button
        onClick={onClick}
        aria-label={label}
        className={cn(
          "grid h-6 w-6 place-items-center rounded transition",
          active ? "bg-ink-700 text-ink-50" : "text-ink-400 hover:bg-ink-750 hover:text-ink-100",
        )}
      >
        <Icon name={icon} className="h-3 w-3" />
      </button>
    </Tooltip>
  );
}
