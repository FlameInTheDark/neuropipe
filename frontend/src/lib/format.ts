import i18n from "@/i18n";

/** Locale-aware date formatting for list rows and headers. */
export function formatDate(value?: string): string {
  if (!value) return i18n.t("common.never");
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return value;
  return new Intl.DateTimeFormat(i18n.resolvedLanguage ?? "en", {
    dateStyle: "medium",
  }).format(d);
}

/** Locale-aware short date+time for run rows and report metadata. */
export function formatDateTime(value?: string): string {
  if (!value) return i18n.t("common.never");
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return value;
  return new Intl.DateTimeFormat(i18n.resolvedLanguage ?? "en", {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(d);
}

/** Compact relative grouping label for chat conversation lists. */
export function conversationGroup(updatedAt: string): "today" | "yesterday" | "week" | "older" {
  const d = new Date(updatedAt);
  const now = new Date();
  const startOfToday = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();
  const t = d.getTime();
  if (t >= startOfToday) return "today";
  if (t >= startOfToday - 86_400_000) return "yesterday";
  if (t >= startOfToday - 7 * 86_400_000) return "week";
  return "older";
}

/** Locale-aware compact number formatting (12.4k, 1.2M). */
export function formatCompact(value: number): string {
  return new Intl.NumberFormat(i18n.resolvedLanguage ?? "en", {
    notation: "compact",
    maximumFractionDigits: 1,
  }).format(value);
}

/** Locale-aware number formatting for KPI cards. */
export function formatNumber(value: number): string {
  return new Intl.NumberFormat(i18n.resolvedLanguage ?? "en").format(value);
}

/** Locale-aware percent formatting. */
export function formatPercent(value: number): string {
  return new Intl.NumberFormat(i18n.resolvedLanguage ?? "en", {
    style: "percent",
    maximumFractionDigits: 1,
  }).format(value);
}

/** Locale-aware duration formatting (ms under a second, otherwise s). */
export function formatDuration(ms: number): string {
  if (!Number.isFinite(ms)) return "–";
  if (ms < 1000) return i18n.t("time.ms", { count: Math.round(ms) });
  const seconds = ms / 1000;
  if (seconds < 60) return i18n.t("time.s", { count: Math.round(seconds * 10) / 10 });
  const minutes = Math.floor(seconds / 60);
  const rest = Math.round(seconds % 60);
  return i18n.t("time.min", { count: minutes, rest });
}

/** Locale-aware currency formatting for estimated cost. */
export function formatUsd(value: number): string {
  return new Intl.NumberFormat(i18n.resolvedLanguage ?? "en", {
    style: "currency",
    currency: "USD",
    maximumFractionDigits: value < 1 ? 4 : 2,
  }).format(value);
}

/** Locale-aware byte size formatting for install progress. */
export function formatBytes(bytes: number): string {
  return new Intl.NumberFormat(i18n.resolvedLanguage ?? "en", {
    style: "unit",
    unit: "byte",
    notation: "compact",
    maximumFractionDigits: 1,
  }).format(bytes);
}

/** JSON preview with a hard length clamp for list rows. */
export function valuePreview(value: unknown, max = 120): string {
  let text: string;
  try {
    text = JSON.stringify(value) ?? "";
  } catch {
    text = String(value);
  }
  return text.length > max ? `${text.slice(0, max)}…` : text;
}
