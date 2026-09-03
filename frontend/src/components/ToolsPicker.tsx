import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { useTranslation } from "react-i18next";
import { Icon } from "./icons";
import { Toggle } from "./ui";
import { cn } from "../utils/cn";

/** One published LLM tool function offered to the assistant. */
export interface ToolsPickerTool {
  id: string;
  name: string;
  description?: string;
}

/* Multi-select picker for conversation LLM tools: a compact trigger button
 * (matching the model/effort dropdowns in the chat composer) opening a
 * searchable portal menu whose rows carry toggle switches. Positioning,
 * outside-click, wheel, resize, Escape, and keyboard navigation follow the
 * shared Dropdown menu so both pickers behave identically. */
export function ToolsPicker({
  tools,
  enabled,
  onChange,
  className,
}: {
  tools: ToolsPickerTool[];
  /** IDs of the tool functions currently offered to the model. */
  enabled: string[];
  onChange: (ids: string[]) => void;
  className?: string;
}) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const btnRef = useRef<HTMLButtonElement>(null);
  const enabledSet = useMemo(() => new Set(enabled), [enabled]);
  const missing = useMemo(
    () => enabled.filter((id) => !tools.some((tool) => tool.id === id)),
    [enabled, tools],
  );

  return (
    <>
      <button
        ref={btnRef}
        type="button"
        aria-haspopup="listbox"
        aria-expanded={open}
        onClick={() => setOpen((o) => !o)}
        onKeyDown={(e) => {
          if (e.key === "ArrowDown" || e.key === "ArrowUp") {
            e.preventDefault();
            setOpen(true);
          }
        }}
        title={t("chat.tools")}
        className={cn(
          "flex h-[26px] shrink-0 items-center gap-1.5 rounded-md border border-ink-700 bg-ink-850 px-2 text-[11px] text-left transition",
          open ? "border-ink-500 bg-ink-800" : "hover:border-ink-600 hover:bg-ink-800",
          className,
        )}
      >
        <Icon name="Bot" className={cn("h-3.5 w-3.5 shrink-0", enabled.length > 0 ? "text-success-fg" : "text-fg-subtle")} />
        <span className={cn("truncate", enabled.length > 0 ? "text-fg" : "text-fg-subtle")}>{t("chat.tools")}</span>
        {enabled.length > 0 && (
          <span className="grid h-[15px] min-w-[15px] shrink-0 place-items-center rounded-full bg-success/15 px-1 font-mono text-[9.5px] leading-none text-success-fg">
            {enabled.length}
          </span>
        )}
        <Icon
          name="ChevronDown"
          className={cn("h-3.5 w-3.5 shrink-0 text-fg-faint transition-transform duration-150", open && "rotate-180")}
        />
      </button>
      {open && (
        <ToolsMenu
          anchorRef={btnRef}
          tools={tools}
          enabledSet={enabledSet}
          missing={missing}
          onToggle={(id) => {
            const next = enabledSet.has(id)
              ? enabled.filter((existing) => existing !== id)
              : [...enabled, id];
            onChange(next);
          }}
          onClose={() => {
            setOpen(false);
            btnRef.current?.focus();
          }}
        />
      )}
    </>
  );
}

function ToolsMenu({
  anchorRef,
  tools,
  enabledSet,
  missing,
  onToggle,
  onClose,
}: {
  anchorRef: React.RefObject<HTMLButtonElement | null>;
  tools: ToolsPickerTool[];
  enabledSet: Set<string>;
  missing: string[];
  onToggle: (id: string) => void;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const ref = useRef<HTMLDivElement>(null);
  const searchRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const [pos, setPos] = useState<{ left: number; top: number; width: number } | null>(null);
  const [query, setQuery] = useState("");
  const [hi, setHi] = useState(0);

  const needle = query.trim().toLowerCase();
  const filtered = useMemo(
    () =>
      tools.filter(
        (tool) =>
          !needle ||
          tool.name.toLowerCase().includes(needle) ||
          (tool.description ?? "").toLowerCase().includes(needle) ||
          tool.id.toLowerCase().includes(needle),
      ),
    [tools, needle],
  );
  const filteredMissing = useMemo(
    () => missing.filter((id) => !needle || id.toLowerCase().includes(needle)),
    [missing, needle],
  );
  const rowCount = filtered.length + filteredMissing.length;

  // A new filter invalidates the highlight: point it at the first row.
  useEffect(() => {
    setHi(0);
  }, [query]);

  useEffect(() => {
    if (hi > rowCount - 1) setHi(Math.max(0, rowCount - 1));
  }, [rowCount, hi]);

  // Bring the highlighted row into view, also right after opening.
  useEffect(() => {
    const el = listRef.current?.querySelector<HTMLElement>(`[data-idx="${hi}"]`);
    el?.scrollIntoView({ block: "nearest" });
  }, [hi, filtered.length, filteredMissing.length]);

  // Anchor-clamped portal positioning, matching the Dropdown menu: open
  // upward when the viewport bottom is too close, never off-screen sideways.
  useLayoutEffect(() => {
    const anchor = anchorRef.current;
    const el = ref.current;
    if (!anchor || !el) return;
    const a = anchor.getBoundingClientRect();
    const mw = el.offsetWidth;
    const mh = el.offsetHeight;
    const vw = document.documentElement.clientWidth;
    const vh = window.innerHeight;
    const width = Math.min(Math.max(mw, a.width), vw - 12);
    let top = a.bottom + 5;
    if (top + mh > vh - 8) top = Math.max(8, a.top - mh - 5);
    const left = Math.min(Math.max(6, a.left), Math.max(6, vw - width - 6));
    setPos({ left, top, width });
  }, [anchorRef, filtered.length, filteredMissing.length]);

  useEffect(() => {
    searchRef.current?.focus();
    const onDown = (e: PointerEvent) => {
      const target = e.target as Node;
      if (ref.current?.contains(target) || anchorRef.current?.contains(target)) return;
      onClose();
    };
    const onWheel = (e: WheelEvent) => {
      if (!ref.current?.contains(e.target as Node)) onClose();
    };
    window.addEventListener("pointerdown", onDown);
    window.addEventListener("wheel", onWheel, { passive: true });
    window.addEventListener("resize", onClose);
    return () => {
      window.removeEventListener("pointerdown", onDown);
      window.removeEventListener("wheel", onWheel);
      window.removeEventListener("resize", onClose);
    };
  }, [anchorRef, onClose]);

  const toggleAt = (index: number) => {
    if (index < filtered.length) {
      onToggle(filtered[index].id);
      return;
    }
    // unavailable rows cannot be toggled; they clear when the saved selection updates
  };

  return createPortal(
    <div
      ref={ref}
      role="listbox"
      aria-multiselectable
      tabIndex={-1}
      style={
        pos
          ? { left: pos.left, top: pos.top, minWidth: pos.width, maxWidth: "calc(100vw - 12px)" }
          : { left: -9999, top: -9999 }
      }
      onKeyDown={(e) => {
        if (e.key === "Escape") {
          // Clearing the filter first matches every searchable picker in
          // the app; the second Escape closes the menu.
          if (query) {
            setQuery("");
            return;
          }
          onClose();
        } else if (e.key === "ArrowDown") {
          e.preventDefault();
          setHi((h) => Math.min(h + 1, rowCount - 1));
        } else if (e.key === "ArrowUp") {
          e.preventDefault();
          setHi((h) => Math.max(h - 1, 0));
        } else if (e.key === "Home") {
          e.preventDefault();
          setHi(0);
        } else if (e.key === "End") {
          e.preventDefault();
          setHi(Math.max(0, rowCount - 1));
        } else if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          toggleAt(hi);
        }
      }}
      className="timeline-menu fixed z-[90] w-[300px] rounded-[9px] border border-ink-650 bg-ink-850/95 p-1 shadow-[0_18px_44px_-12px_rgba(0,0,0,0.95),0_0_0_1px_rgba(255,255,255,0.02)_inset] outline-none backdrop-blur-xl"
    >
      <div className="mb-1 flex h-7 items-center gap-1.5 rounded-md border border-ink-600 bg-ink-900/70 px-2">
        <Icon name="Search" className="h-3 w-3 shrink-0 text-fg-faint" />
        <input
          ref={searchRef}
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={t("chat.toolsSearch")}
          spellCheck={false}
          className="h-full min-w-0 flex-1 bg-transparent text-[12px] text-fg placeholder:text-fg-faint focus:outline-none"
        />
        {query.trim() ? (
          <span className="shrink-0 font-mono text-[10px] text-fg-faint" aria-live="polite">
            {filtered.length}/{tools.length}
          </span>
        ) : null}
      </div>
      <div ref={listRef} className="max-h-[340px] overflow-y-auto">
        {rowCount === 0 ? (
          <p className="px-2 py-2 text-center text-[11.5px] text-fg-faint">{t("chat.toolsEmpty")}</p>
        ) : (
          <>
            {filtered.map((tool, i) => (
              <button
                key={tool.id}
                role="option"
                data-idx={i}
                aria-selected={enabledSet.has(tool.id)}
                onMouseEnter={() => setHi(i)}
                onClick={() => toggleAt(i)}
                className={cn(
                  "flex w-full items-center gap-2.5 rounded-md px-2 py-1.5 text-left transition-colors",
                  i === hi ? "bg-ink-650/80" : "hover:bg-ink-650/40",
                )}
              >
                <Toggle on={enabledSet.has(tool.id)} onChange={() => onToggle(tool.id)} />
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-[12.5px] leading-tight text-fg">{tool.name}</span>
                  {tool.description ? (
                    <span className="mt-0.5 block truncate text-[10.5px] leading-tight text-fg-faint">
                      {tool.description}
                    </span>
                  ) : null}
                </span>
              </button>
            ))}
            {filteredMissing.map((id, i) => {
              const index = filtered.length + i;
              return (
                <div
                  key={`missing-${id}`}
                  data-idx={index}
                  onMouseEnter={() => setHi(index)}
                  className="flex w-full items-center gap-2.5 rounded-md px-2 py-1.5 text-left opacity-60"
                >
                  <Toggle on disabled onChange={() => onToggle(id)} />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-[12.5px] leading-tight text-fg-subtle">
                      {t("chat.toolsMissing")}
                    </span>
                    <span className="mt-0.5 block truncate font-mono text-[10px] leading-tight text-fg-faint">
                      {id}
                    </span>
                  </span>
                </div>
              );
            })}
          </>
        )}
      </div>
      <p className="mt-1 border-t border-ink-700/70 px-2 pt-1.5 pb-0.5 text-[10.5px] leading-snug text-fg-faint">
        {t("chat.toolsHint")}
      </p>
    </div>,
    document.body,
  );
}
