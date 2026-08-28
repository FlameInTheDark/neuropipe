import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { useTranslation } from "react-i18next";
import { Icon } from "../icons";
import { cn } from "../../utils/cn";
import { useDismissKeys } from "../../hooks/useDismissKeys";

/**
 * Inclusive date(-time) range. Values are local ISO strings - either
 * "YYYY-MM-DD" or "YYYY-MM-DDTHH:mm"; "" means unbounded. When `withTime`
 * is set the popover exposes per-boundary time inputs and emitted values
 * carry the time portion too.
 */
export interface DateRange {
  from: string;
  to: string;
}

const VALUE_RE = /^(\d{4}-\d{2}-\d{2})(?:T(\d{2}:\d{2}))?$/;

const splitValue = (value: string): { date: string; time: string } => {
  const match = VALUE_RE.exec(value);
  if (!match) return { date: "", time: "" };
  return { date: match[1], time: match[2] ?? "" };
};

const isoOf = (date: Date): string => {
  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, "0");
  const d = String(date.getDate()).padStart(2, "0");
  return `${y}-${m}-${d}`;
};

const addDays = (date: Date, days: number): Date => {
  const next = new Date(date);
  next.setDate(next.getDate() + days);
  return next;
};

const todayIso = (): string => isoOf(new Date());

/** Combines a date part and a time part honouring the picker's resolution. */
const combine = (date: string, time: string, withTime: boolean): string =>
  withTime && time ? `${date}T${time}` : date;

/**
 * Clock-styled HH:MM editor: two numeric segments in a shared bordered
 * readout. Typing auto-advances from hours to minutes; ArrowUp/Down step the
 * focused segment; values are clamped and zero-padded as you type.
 */
function TimeField({
  value,
  ariaLabel,
  onChange,
}: {
  /** "HH:mm" ("" shows placeholders) */
  value: string;
  ariaLabel?: string;
  onChange: (next: string) => void;
}) {
  const hourRef = useRef<HTMLInputElement>(null);
  const minuteRef = useRef<HTMLInputElement>(null);

  /* value is exactly "HH:mm" here - parse it directly, no date part involved */
  const complete = value.includes(":");
  const hh = complete ? value.slice(0, 2) : "";
  const mm = complete ? value.slice(3, 5) : "";

  const commitSegment = (kind: "hour" | "minute", raw: string) => {
    const digits = raw.replace(/\D/g, "").slice(-2);
    if (!digits && kind === "hour") return;
    const num = Math.min(Number(digits || "0"), kind === "hour" ? 23 : 59);
    const padded = String(num).padStart(2, "0");
    onChange(`${kind === "hour" ? padded : hh || "00"}:${kind === "minute" ? padded : mm || "00"}`);
  };

  const step = (kind: "hour" | "minute", delta: number) => {
    const current = Number(kind === "hour" ? hh || "0" : mm || "0") || 0;
    const max = kind === "hour" ? 23 : 59;
    const next = (((current + delta) % (max + 1)) + max + 1) % (max + 1);
    const padded = String(next).padStart(2, "0");
    onChange(kind === "hour" ? `${padded}:${mm || "00"}` : `${hh || "00"}:${padded}`);
  };

  const segment = (kind: "hour" | "minute") => {
    const isHour = kind === "hour";
    const raw = isHour ? hh : mm;
    return (
      <input
        ref={isHour ? hourRef : minuteRef}
        value={complete || raw !== "" ? raw.padStart(2, "0") : ""}
        placeholder={isHour ? "HH" : "MM"}
        inputMode="numeric"
        maxLength={2}
        aria-label={`${ariaLabel ?? ""} ${isHour ? "hours" : "minutes"}`.trim()}
        onChange={(e) => {
          commitSegment(kind, e.target.value);
          if (kind === "hour" && e.target.value.replace(/\D/g, "").length === 2) {
            minuteRef.current?.focus();
          }
        }}
        onKeyDown={(e) => {
          e.stopPropagation();
          if (e.key === "ArrowUp") {
            e.preventDefault();
            step(kind, 1);
          } else if (e.key === "ArrowDown") {
            e.preventDefault();
            step(kind, -1);
          } else if (e.key === "Enter") {
            (kind === "hour" ? minuteRef : hourRef)?.current?.focus();
          }
        }}
        className="w-[22px] bg-transparent text-center font-mono text-[12px] tabular-nums text-fg outline-none placeholder:text-fg-faint"
      />
    );
  };

  return (
    <div
      className="flex h-7 items-center rounded-md border border-ink-700 bg-ink-850 px-1 transition focus-within:border-info/60"
      aria-label={ariaLabel}
    >
      {segment("hour")}
      <span className="px-px font-mono text-[12px] text-fg-faint">:</span>
      {segment("minute")}
    </div>
  );
}

export function DateRangePicker({
  value,
  onChange,
  withTime = false,
  className,
}: {
  value: DateRange;
  onChange: (range: DateRange) => void;
  /** Expose per-boundary time inputs next to the calendar. */
  withTime?: boolean;
  className?: string;
}) {
  const { t, i18n } = useTranslation();
  const [open, setOpen] = useState(false);
  const anchorRef = useRef<HTMLButtonElement>(null);
  const popoverRef = useRef<HTMLDivElement>(null);

  const anchorValue = splitValue(value.from).date || splitValue(value.to).date || todayIso();
  const [viewYear, setViewYear] = useState(() => Number(anchorValue.slice(0, 4)));
  const [viewMonth, setViewMonth] = useState(() => Number(anchorValue.slice(5, 7)) - 1);
  const [hoveredDate, setHoveredDate] = useState<string | null>(null);

  const fromParts = splitValue(value.from);
  const toParts = splitValue(value.to);

  useDismissKeys(() => setOpen(false));

  /* position under the trigger, clamped to the window */
  useLayoutEffect(() => {
    if (!open) return;
    const anchor = anchorRef.current;
    const popover = popoverRef.current;
    if (!anchor || !popover) return;
    const rect = anchor.getBoundingClientRect();
    const size = popover.getBoundingClientRect();
    const left = Math.min(Math.max(8, rect.left), window.innerWidth - size.width - 8);
    let top = rect.bottom + 6;
    if (top + size.height > window.innerHeight - 8) top = Math.max(8, rect.top - size.height - 6);
    popover.style.left = `${left}px`;
    popover.style.top = `${top}px`;
  }, [open, viewMonth, viewYear]);

  useEffect(() => {
    if (!open) return;
    const onDown = (e: PointerEvent) => {
      const target = e.target as Node;
      if (popoverRef.current?.contains(target) || anchorRef.current?.contains(target)) return;
      setOpen(false);
    };
    const onResize = () => setOpen(false);
    window.addEventListener("pointerdown", onDown);
    window.addEventListener("resize", onResize);
    return () => {
      window.removeEventListener("pointerdown", onDown);
      window.removeEventListener("resize", onResize);
    };
  }, [open]);

  const openAt = () => {
    const base = splitValue(value.from || value.to).date || todayIso();
    setViewYear(Number(base.slice(0, 4)));
    setViewMonth(Number(base.slice(5, 7)) - 1);
    setHoveredDate(null);
    setOpen(true);
  };


  /** Picking restarts on the first click and completes on the second; an
     earlier second click becomes the new start instead of an empty range. */
  const pick = (iso: string) => {
    if (!value.from || (value.from && value.to)) {
      onChange({ from: combine(iso, fromParts.time || (withTime ? "00:00" : ""), withTime), to: "" });
      return;
    }
    const started = value.from.slice(0, 10);
    if (iso < started) {
      onChange({ from: combine(iso, fromParts.time || (withTime ? "00:00" : ""), withTime), to: "" });
      return;
    }
    onChange({ from: value.from, to: combine(iso, toParts.time || (withTime ? "23:59" : ""), withTime) });
    setOpen(false);
  };

  const applyPreset = (daysBack: number) => {
    const end = new Date();
    const start = addDays(end, -(daysBack - 1));
    onChange({
      from: combine(isoOf(start), withTime ? "00:00" : "", withTime),
      to: combine(isoOf(end), withTime ? "23:59" : "", withTime),
    });
    setOpen(false);
  };

  /* month grid: leading blanks for the Monday-first offset */
  const days = useMemo(() => {
    const first = new Date(viewYear, viewMonth, 1);
    const offset = (first.getDay() + 6) % 7; // Monday = 0
    const count = new Date(viewYear, viewMonth + 1, 0).getDate();
    const cells: (string | null)[] = Array.from({ length: offset }, () => null);
    for (let day = 1; day <= count; day += 1) cells.push(isoOf(new Date(viewYear, viewMonth, day)));
    return cells;
  }, [viewYear, viewMonth]);

  const weekdays = useMemo(
    () =>
      Array.from({ length: 7 }, (_, index) => {
        const monday = new Date(2024, 0, 1 + index); // 2024-01-01 is a Monday
        return new Intl.DateTimeFormat(i18n.resolvedLanguage ?? "en", { weekday: "narrow" }).format(monday);
      }),
    [i18n.resolvedLanguage],
  );

  const monthTitle = new Intl.DateTimeFormat(i18n.resolvedLanguage ?? "en", {
    month: "long",
    year: "numeric",
  }).format(new Date(viewYear, viewMonth, 1));

  const formatBoundary = (value: string): string => {
    const parts = splitValue(value);
    if (!parts.date) return "";
    const date = new Date(`${parts.date}T${parts.time || "00:00"}`);
    const options: Intl.DateTimeFormatOptions = { dateStyle: "medium" };
    if (parts.time) options.timeStyle = "short";
    return new Intl.DateTimeFormat(i18n.resolvedLanguage ?? "en", options).format(date);
  };

  const inPreview = (iso: string): boolean => {
    const rawEnd = value.to || (value.from ? hoveredDate : null);
    if (!rawEnd) return false;
    const low = value.from! < rawEnd ? value.from! : rawEnd;
    const high = value.from! < rawEnd ? rawEnd : value.from!;
    const isoLow = low.slice(0, 10);
    const isoHigh = high.slice(0, 10);
    return iso >= isoLow && iso <= isoHigh;
  };

  const label =
    value.from && value.to
      ? `${formatBoundary(value.from)} – ${formatBoundary(value.to)}`
      : value.from
        ? `${formatBoundary(value.from)} – …`
        : t("reports.rangeAny");

  const shiftMonth = (delta: number) => {
    const next = new Date(viewYear, viewMonth + delta, 1);
    setViewYear(next.getFullYear());
    setViewMonth(next.getMonth());
    setHoveredDate(null);
  };

  return (
    <>
      <button
        ref={anchorRef}
        type="button"
        onClick={() => (open ? setOpen(false) : openAt())}
        aria-label={t("reports.rangeLabel")}
        className={cn(
          "flex h-8 items-center gap-2 rounded-md border px-2.5 text-[12px] transition",
          open || value.from
            ? "border-ink-500 bg-ink-800 text-fg"
            : "border-ink-700 bg-ink-850 text-fg-subtle hover:border-ink-600 hover:text-fg-muted",
          className,
        )}
      >
        <Icon name="CalendarDays" className="h-3.5 w-3.5 shrink-0 text-fg-faint" />
        <span className="max-w-[240px] truncate">{label}</span>
        <Icon name="ChevronDown" className="h-3 w-3 shrink-0 text-fg-faint" />
      </button>

      {open &&
        createPortal(
          <div
            ref={popoverRef}
            className="fixed z-[80] w-[280px] rounded-xl border border-ink-650 bg-ink-900 p-2.5 shadow-[0_24px_60px_-16px_rgba(0,0,0,0.9)]"
            style={{ left: -9999, top: -9999 }}
          >
            <div className="mb-1.5 flex items-center gap-1">
              <button
                type="button"
                aria-label={t("reports.prevMonth")}
                onClick={() => shiftMonth(-1)}
                className="grid h-6 w-6 place-items-center rounded-md text-fg-subtle transition hover:bg-ink-750 hover:text-fg"
              >
                <Icon name="ChevronLeft" className="h-3.5 w-3.5" />
              </button>
              <span className="min-w-0 flex-1 truncate text-center text-[12px] font-medium capitalize text-fg">
                {monthTitle}
              </span>
              <button
                type="button"
                aria-label={t("reports.nextMonth")}
                onClick={() => shiftMonth(1)}
                className="grid h-6 w-6 place-items-center rounded-md text-fg-subtle transition hover:bg-ink-750 hover:text-fg"
              >
                <Icon name="ChevronRight" className="h-3.5 w-3.5" />
              </button>
            </div>

            <div className="grid grid-cols-7 gap-y-0.5">
              {weekdays.map((day) => (
                <span key={day} className="grid h-6 place-items-center text-[10px] text-fg-faint">
                  {day}
                </span>
              ))}
              {days.map((iso, index) =>
                iso === null ? (
                  <span key={`blank-${index}`} />
                ) : (
                  <button
                    key={iso}
                    type="button"
                    onMouseEnter={() => setHoveredDate(iso)}
                    onMouseLeave={() => setHoveredDate((cur) => (cur === iso ? null : cur))}
                    onClick={() => pick(iso)}
                    className={cn(
                      "h-7 rounded-md text-[11.5px] tabular-nums transition",
                      iso === splitValue(value.from).date ||
                        (value.to && iso === splitValue(value.to).date)
                        ? "bg-info font-medium text-white"
                        : inPreview(iso)
                          ? "bg-info/20 text-info-fg"
                          : "text-fg-muted hover:bg-ink-750",
                      iso === todayIso() &&
                        iso !== splitValue(value.from).date &&
                        iso !== splitValue(value.to).date &&
                        "ring-1 ring-inset ring-ring/60",
                    )}
                  >
                    {Number(iso.slice(8, 10))}
                  </button>
                ),
              )}
            </div>

            {withTime && (
              <div className="mt-2 space-y-1.5 border-t border-seam pt-2">
                <div className="flex items-center gap-2">
                  <span className="w-12 shrink-0 text-[10.5px] text-fg-faint">{t("reports.from")}</span>
                  <TimeField
                    value={value.from ? splitValue(value.from).time || "00:00" : ""}
                   
                    ariaLabel={`${t("reports.from")} - HH${":"}MM`}
                    onChange={(t2) => {
                      const date = splitValue(value.from).date || todayIso();
                      onChange({
                        from: combine(date, t2, true),
                        to: value.to || combine(date, "23:59", true),
                      });
                    }}
                  />
                </div>
                <div className="flex items-center gap-2">
                  <span className="w-12 shrink-0 text-[10.5px] text-fg-faint">{t("reports.to")}</span>
                  <TimeField
                    value={value.to ? splitValue(value.to).time || "23:59" : ""}
                   
                    ariaLabel={`${t("reports.to")} - HH${":"}MM`}
                    onChange={(t2) => {
                      const date = splitValue(value.to).date || todayIso();
                      onChange({
                        from: value.from || combine(date, "00:00", true),
                        to: combine(date, t2, true),
                      });
                    }}
                  />
                </div>
              </div>
            )}

            <div className="mt-2 flex items-center gap-1 border-t border-seam pt-2">
              {[
                { label: t("reports.presetToday"), back: 1 },
                { label: t("reports.preset7"), back: 7 },
                { label: t("reports.preset30"), back: 30 },
              ].map((preset) => (
                <button
                  key={preset.back}
                  type="button"
                  onClick={() => applyPreset(preset.back)}
                  className="rounded-md px-2 py-1 text-[11px] text-fg-subtle transition hover:bg-ink-750 hover:text-fg"
                >
                  {preset.label}
                </button>
              ))}
              <button
                type="button"
                disabled={!value.from && !value.to}
                onClick={() => {
                  onChange({ from: "", to: "" });
                  setOpen(false);
                }}
                className="ml-auto rounded-md px-2 py-1 text-[11px] text-danger-fg transition hover:bg-danger/15 disabled:cursor-not-allowed disabled:opacity-40"
              >
                {t("common.clear")}
              </button>
            </div>
          </div>,
          document.body,
        )}
    </>
  );
}
