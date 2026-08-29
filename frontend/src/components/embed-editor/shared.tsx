import { useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { desktop } from "@/lib/bridge";

/** Label with a live length/max counter; the counter turns red over limit. */
export function FieldLabel({
  text,
  value,
  max,
  htmlFor,
}: {
  text: string;
  value: string;
  max: number;
  htmlFor?: string;
}) {
  const over = value.length > max;
  return (
    <span className="mb-1 flex items-baseline justify-between gap-2">
      <label htmlFor={htmlFor} className="text-[10px] font-semibold uppercase tracking-[0.1em] text-fg-faint">
        {text}
      </label>
      <span className={`text-[10px] tabular-nums ${over ? "font-semibold text-danger" : "text-fg-faint/70"}`}>
        {value.length} / {max}
      </span>
    </span>
  );
}

const inputClasses =
  "w-full rounded border border-ink-700 bg-ink-950 px-2 py-1.5 text-[12px] text-fg outline-none transition focus:border-ink-500 placeholder:text-fg-faint/60";

export function TextField({
  label,
  value,
  max,
  placeholder,
  mono,
  onChange,
}: {
  label: string;
  value: string;
  max: number;
  placeholder?: string;
  mono?: boolean;
  onChange: (value: string) => void;
}) {
  const over = value.length > max;
  return (
    <label className="block min-w-0">
      <FieldLabel text={label} value={value} max={max} />
      <input
        value={value}
        placeholder={placeholder}
        spellCheck={false}
        onChange={(event) => onChange(event.target.value)}
        className={`${inputClasses} ${mono ? "font-mono text-[11.5px]" : ""} ${over ? "border-danger/70" : ""}`}
      />
    </label>
  );
}

export function TextAreaField({
  label,
  value,
  max,
  placeholder,
  rows = 3,
  onChange,
}: {
  label: string;
  value: string;
  max: number;
  placeholder?: string;
  rows?: number;
  onChange: (value: string) => void;
}) {
  const { t } = useTranslation();
  const over = value.length > max;
  return (
    <label className="block min-w-0">
      <FieldLabel text={label} value={value} max={max} />
      <textarea
        value={value}
        placeholder={placeholder}
        rows={rows}
        spellCheck={false}
        onChange={(event) => onChange(event.target.value)}
        className={`muted-scroll w-full resize-y rounded border border-ink-700 bg-ink-950 px-2 py-1.5 text-[12px] leading-[1.5] text-fg outline-none transition focus:border-ink-500 placeholder:text-fg-faint/60 ${
          over ? "border-danger/70" : ""
        }`}
      />
      {over ? <span className="mt-0.5 block text-[10px] text-danger">{t("embedEditor.overLimit")}</span> : null}
    </label>
  );
}

/** Hex color input paired with a native color picker swatch. */
export function ColorField({
  label,
  value,
  onChange,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
}) {
  const pickerValue = /^#[0-9a-fA-F]{6}$/.test(value) ? value : "#5865F2";
  return (
    <label className="block min-w-0">
      <FieldLabel text={label} value={value} max={7} />
      <span className="flex items-center gap-1.5">
        <input
          value={value}
          placeholder="#5865F2"
          spellCheck={false}
          onChange={(event) => onChange(event.target.value)}
          className={`${inputClasses} font-mono text-[11.5px]`}
        />
        <span className="relative h-[30px] w-[34px] shrink-0 overflow-hidden rounded border border-ink-700 bg-ink-950">
          <input
            type="color"
            value={pickerValue}
            onChange={(event) => onChange(event.target.value.toUpperCase())}
            className="absolute -inset-2 h-[46px] w-[50px] cursor-pointer border-0 bg-transparent p-0"
          />
        </span>
      </span>
    </label>
  );
}

/** Compact datetime-local input storing ISO strings, with a clear button. */
export function TimestampField({
  label,
  value,
  onChange,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
}) {
  const { t } = useTranslation();
  const localValue = useMemo(() => isoToLocal(value), [value]);
  return (
    <label className="block min-w-0">
      <span className="mb-1 block text-[10px] font-semibold uppercase tracking-[0.1em] text-fg-faint">{label}</span>
      <span className="flex items-center gap-1">
        <input
          type="datetime-local"
          value={localValue}
          onChange={(event) => onChange(localToISO(event.target.value))}
          className={`${inputClasses} tabular-nums`}
        />
        {value ? (
          <button
            type="button"
            title={t("embedEditor.clearTimestamp")}
            onClick={() => onChange("")}
            className="grid h-[26px] w-[26px] shrink-0 place-items-center rounded border border-ink-700 bg-ink-850 text-fg-faint transition hover:text-danger"
          >
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" className="h-3.5 w-3.5">
              <path d="M3 6h18M8 6V4h8v2M6 6l1 14h10l1-14" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
          </button>
        ) : null}
      </span>
    </label>
  );
}

function isoToLocal(iso: string): string {
  if (!iso) return "";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "";
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

function localToISO(local: string): string {
  if (!local) return "";
  const date = new Date(local);
  if (Number.isNaN(date.getTime())) return "";
  return date.toISOString();
}

/** Collapsible section with a chevron header. */
export function Section({
  title,
  meta,
  defaultOpen = true,
  children,
}: {
  title: string;
  meta?: string;
  defaultOpen?: boolean;
  children: React.ReactNode;
}) {
  const [open, setOpen] = useState(defaultOpen);
  return (
    <div className="rounded-md border border-ink-700/70 bg-ink-900/40">
      <button
        type="button"
        onClick={() => setOpen(!open)}
        className="flex w-full items-center justify-between gap-2 px-2.5 py-2 text-left"
      >
        <span className="flex items-center gap-1.5 text-[11px] font-semibold uppercase tracking-[0.1em] text-fg-subtle">
          <svg
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            className={`h-3 w-3 transition-transform ${open ? "rotate-90" : ""}`}
          >
            <path d="M9 18l6-6-6-6" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
          {title}
        </span>
        {meta ? <span className="text-[10px] tabular-nums text-fg-faint">{meta}</span> : null}
      </button>
      {open ? <div className="space-y-2.5 border-t border-ink-700/50 p-2.5">{children}</div> : null}
    </div>
  );
}

/** Small icon-only action button used on embed and field cards. */
export function CardIconButton({
  title,
  danger,
  disabled,
  onClick,
  path,
}: {
  title: string;
  danger?: boolean;
  disabled?: boolean;
  onClick: () => void;
  path: string;
}) {
  return (
    <button
      type="button"
      title={title}
      disabled={disabled}
      onClick={onClick}
      className={`grid h-6 w-6 place-items-center rounded transition ${
        disabled
          ? "cursor-not-allowed text-fg-faint/30"
          : danger
            ? "text-fg-faint hover:bg-ink-750 hover:text-danger-fg"
            : "text-fg-faint hover:bg-ink-750 hover:text-fg"
      }`}
    >
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" className="h-3.5 w-3.5">
        <path d={path} strokeLinecap="round" strokeLinejoin="round" />
      </svg>
    </button>
  );
}

export const ICON_PATHS = {
  up: "M12 19V5M5 12l7-7 7 7",
  down: "M12 5v14M19 12l-7 7-7-7",
  copy: "M9 9h10v10H9zM5 15V5h10",
  trash: "M3 6h18M8 6V4h8v2M6 6l1 14h10l1-14",
  check: "M20 6L9 17l-5-5",
} as const;

/* ------------------------------------------------------------------ */
/* preview image proxy                                                */
/* ------------------------------------------------------------------ */

const dataURLCache = new Map<string, string>();

/**
 * Resolves an image URL or local path into a data URL through the backend
 * bridge so the Discord preview shows real images without CORS limits.
 * Falls back to the raw value (works for direct URLs in a browser).
 */
export function useProxiedImage(value: string): string {
  const [resolved, setResolved] = useState(() => dataURLCache.get(value) ?? value);
  const currentRef = useRef(value);
  currentRef.current = value;

  useEffect(() => {
    let cancelled = false;
    if (!value || dataURLCache.has(value)) {
      setResolved(dataURLCache.get(value) ?? value);
      return () => {
        cancelled = true;
      };
    }
    desktop
      .drawImageLoadImageSource(value.startsWith("http://") || value.startsWith("https://") ? "url" : "path", value)
      .then((dataURL) => {
        if (cancelled || currentRef.current !== value || !dataURL) return;
        dataURLCache.set(value, dataURL);
        setResolved(dataURL);
      })
      .catch(() => {
        // keep the raw URL; the <img> either renders or hides itself
      });
    return () => {
      cancelled = true;
    };
  }, [value]);

  return resolved;
}

/** An <img> that vanishes when the source cannot be loaded. */
export function PreviewImage({ src, className, alt }: { src: string; className?: string; alt?: string }) {
  const [failed, setFailed] = useState(false);
  const proxied = useProxiedImage(src);
  useEffect(() => setFailed(false), [proxied]);
  if (!proxied || failed) return null;
  return <img src={proxied} alt={alt ?? ""} className={className} onError={() => setFailed(true)} />;
}

/** Formats an ISO timestamp the way Discord's footer renders it. */
export function formatEmbedTimestamp(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${pad(date.getDate())}/${pad(date.getMonth() + 1)}/${date.getFullYear()}`;
}
