import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { useTranslation } from "react-i18next";
import { Icon } from "./icons";
import { cn } from "../utils/cn";
import { filterDropdownOptions, type DropdownOption } from "../lib/dropdownFilter";

export type { DropdownOption };
export { filterDropdownOptions };

export function Dropdown({
  value,
  options,
  onChange,
  placeholder,
  className,
  compact,
  searchable,
  searchPlaceholder,
}: {
  value: string;
  options: DropdownOption[];
  onChange: (v: string) => void;
  placeholder?: string;
  className?: string;
  compact?: boolean;
  /* Renders a search input pinned at the top of the menu and filters the
   * options by it. Meant for long option lists (e.g. a bot's servers). */
  searchable?: boolean;
  searchPlaceholder?: string;
}) {
  const { t } = useTranslation();
  const btnRef = useRef<HTMLButtonElement>(null);
  const resolvedPlaceholder = placeholder ?? t("common.select");
  const [open, setOpen] = useState(false);
  const current = options.find((o) => o.value === value);

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
        className={cn(
          "flex items-center gap-2 rounded-md border border-ink-700 bg-ink-850 text-left transition",
          compact ? "h-[26px] px-2 text-[11px]" : "h-8 w-full px-2.5 text-[12.5px]",
          open ? "border-ink-500 bg-ink-800" : "hover:border-ink-600 hover:bg-ink-800",
          className,
        )}
      >
        {current?.icon && <Icon name={current.icon} className="h-3.5 w-3.5 shrink-0 text-fg-subtle" />}
        <span className={cn("min-w-0 flex-1 truncate", current ? "text-fg" : "text-fg-faint")}>
          {current?.label ?? resolvedPlaceholder}
        </span>
        <Icon
          name="ChevronDown"
          className={cn("h-3.5 w-3.5 shrink-0 text-fg-faint transition-transform duration-150", open && "rotate-180")}
        />
      </button>
      {open && (
        <Menu
          anchorRef={btnRef}
          options={options}
          value={value}
          searchable={searchable}
          searchPlaceholder={searchPlaceholder}
          onPick={(v) => {
            onChange(v);
            setOpen(false);
            btnRef.current?.focus();
          }}
          onClose={() => setOpen(false)}
        />
      )}
    </>
  );
}

function Menu({
  anchorRef,
  options,
  value,
  searchable,
  searchPlaceholder,
  onPick,
  onClose,
}: {
  anchorRef: React.RefObject<HTMLButtonElement | null>;
  options: DropdownOption[];
  value: string;
  searchable?: boolean;
  searchPlaceholder?: string;
  onPick: (v: string) => void;
  onClose: () => void;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const searchRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const [pos, setPos] = useState<{ left: number; top: number; width: number } | null>(null);
  const [query, setQuery] = useState("");
  const { t } = useTranslation();
  const filtered = useMemo(
    () => (searchable ? filterDropdownOptions(options, query) : options),
    [options, query, searchable],
  );
  const [hi, setHi] = useState(() => Math.max(0, options.findIndex((o) => o.value === value)));

  // A new filter invalidates the highlight: point it at the first match.
  useEffect(() => {
    setHi(0);
  }, [query]);

  // Keep the highlight inside the (possibly filtered) list.
  useEffect(() => {
    if (hi > filtered.length - 1) setHi(Math.max(0, filtered.length - 1));
  }, [filtered.length, hi]);

  // Bring the highlighted option into view: while walking with the arrow/Home/
  // End keys, and right after opening, when the highlight may start deep in a
  // long list (the currently selected model).
  useEffect(() => {
    const el = listRef.current?.querySelector<HTMLElement>(`[data-idx="${hi}"]`);
    el?.scrollIntoView({ block: "nearest" });
  }, [hi, filtered]);

  useLayoutEffect(() => {
    const anchor = anchorRef.current;
    const el = ref.current;
    if (!anchor || !el) return;
    const a = anchor.getBoundingClientRect();
    // offsetWidth/offsetHeight are the layout box: unlike
    // getBoundingClientRect they ignore the .timeline-menu entry animation's
    // scale(0.96), which under-measured the menu and let its right edge slip
    // past the viewport.
    const mw = el.offsetWidth;
    const mh = el.offsetHeight;
    const vw = document.documentElement.clientWidth;
    const vh = window.innerHeight;
    // The final menu width is floored at the anchor's width (style minWidth)
    // and hard-capped at the viewport so an unbreakable model key can never
    // push the menu off-screen either.
    const width = Math.min(Math.max(mw, a.width), vw - 12);
    let top = a.bottom + 5;
    if (top + mh > vh - 8) top = Math.max(8, a.top - mh - 5);
    const left = Math.min(Math.max(6, a.left), Math.max(6, vw - width - 6));
    setPos({ left, top, width });
    // Re-runs when the option list or a filter changes its size: the menu is
    // re-clamped against the viewport, which is what keeps long model lists
    // that arrive after opening (provider/discovery load) on-screen.
  }, [anchorRef, filtered.length, options.length]);

  useEffect(() => {
    // With a search box the input is the focus target so typing filters
    // immediately; otherwise the menu container keeps key events.
    if (searchable) searchRef.current?.focus();
    else ref.current?.focus();
    const onDown = (e: PointerEvent) => {
      const t = e.target as Node;
      if (ref.current?.contains(t) || anchorRef.current?.contains(t)) return;
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
  }, [anchorRef, onClose, searchable]);

  return createPortal(
    <div
      ref={ref}
      role="listbox"
      tabIndex={-1}
      style={
        pos
          ? { left: pos.left, top: pos.top, minWidth: pos.width, maxWidth: "calc(100vw - 12px)" }
          : { left: -9999, top: -9999 }
      }
      onKeyDown={(e) => {
        if (e.key === "Escape") {
          // Clearing the filter first matches every searchable picker the
          // app already ships; the second Escape closes the menu.
          if (searchable && query) {
            setQuery("");
            return;
          }
          onClose();
        } else if (e.key === "ArrowDown") {
          e.preventDefault();
          setHi((h) => Math.min(h + 1, filtered.length - 1));
        } else if (e.key === "ArrowUp") {
          e.preventDefault();
          setHi((h) => Math.max(h - 1, 0));
        } else if (e.key === "Home") {
          e.preventDefault();
          setHi(0);
        } else if (e.key === "End") {
          e.preventDefault();
          setHi(Math.max(0, filtered.length - 1));
        } else if (e.key === "Enter") {
          e.preventDefault();
          const o = filtered[hi];
          if (o) onPick(o.value);
        }
      }}
      className="timeline-menu fixed z-[90] rounded-[9px] border border-ink-650 bg-ink-850/95 p-1 shadow-[0_18px_44px_-12px_rgba(0,0,0,0.95),0_0_0_1px_rgba(255,255,255,0.02)_inset] outline-none backdrop-blur-xl"
    >
      {searchable ? (
        <div className="mb-1 flex h-7 items-center gap-1.5 rounded-md border border-ink-600 bg-ink-900/70 px-2">
          <Icon name="Search" className="h-3 w-3 shrink-0 text-fg-faint" />
          <input
            ref={searchRef}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={searchPlaceholder ?? t("common.search")}
            spellCheck={false}
            className="h-full min-w-0 flex-1 bg-transparent text-[12px] text-fg placeholder:text-fg-faint focus:outline-none"
          />
          {query.trim() ? (
            <span className="shrink-0 font-mono text-[10px] text-fg-faint" aria-live="polite">
              {filtered.length}/{options.length}
            </span>
          ) : null}
        </div>
      ) : null}
      <div
        ref={listRef}
        className={cn("overflow-y-auto", searchable ? "max-h-[340px]" : "max-h-[264px]")}
      >
        {filtered.length === 0 ? (
          <p className="px-2 py-2 text-center text-[11.5px] text-fg-faint">{t("common.noMatches")}</p>
        ) : (
          filtered.map((o, i) => (
            <button
              key={o.value}
              role="option"
              data-idx={i}
              aria-selected={o.value === value}
              onMouseEnter={() => setHi(i)}
              onClick={() => onPick(o.value)}
              className={cn(
                "flex h-7 w-full items-center gap-2.5 rounded-md px-2 text-left text-[12.5px] transition-colors",
                i === hi ? "bg-ink-650/80 text-fg" : "text-fg",
              )}
            >
              {o.icon ? (
                <Icon name={o.icon} className="h-[14px] w-[14px] shrink-0 text-fg-subtle" />
              ) : null}
              <span className="min-w-0 truncate">{o.label}</span>
              {o.hint && <span className="ml-auto shrink-0 font-mono text-[10px] text-fg-faint">{o.hint}</span>}
              {o.value === value && !o.hint && (
                <Icon name="Check" className="ml-auto h-3.5 w-3.5 shrink-0 text-fg-muted" />
              )}
            </button>
          ))
        )}
      </div>
    </div>,
    document.body,
  );
}


