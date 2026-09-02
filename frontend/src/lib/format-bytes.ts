/** SI byte-size rendering shared by every file/model size in the app.
 *
 *  Kept free of i18n/document imports so verification scripts can exercise
 *  the real logic in node: the locale is passed in by callers.
 *
 *  Sizes render the way download dialogs and model hubs do — decimal SI
 *  steps (1 KB = 1000 B) with one fraction digit: "890 B", "1.2 MB",
 *  "53.8 GB". Locale-aware digits keep German "53,8 GB" and Russian
 *  "53,8 GB" correct, matching the rest of the app's number formatting. */

/** SI byte units, one step per three decimal digits. */
const BYTE_UNITS = ["B", "KB", "MB", "GB", "TB", "PB"] as const;

/** Renders a byte count as a locale-aware SI size with a plain unit label.
 *  Non-finite, zero, and negative counts render as "0 B" so half-initialized
 *  rows never show "NaN" or negative nonsense. */
export function formatByteSize(bytes: number, locale: string): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return `0 ${BYTE_UNITS[0]}`;
  let tier = Math.min(Math.floor(Math.log10(bytes) / 3), BYTE_UNITS.length - 1);
  if (tier < 0) tier = 0;
  let value = bytes / 1000 ** tier;
  // A count just under a step boundary (999,999,999 B) would round to
  // "1000 MB"; promote it to the next unit so it reads "1 GB" instead.
  if (value >= 999.95 && tier < BYTE_UNITS.length - 1) {
    tier += 1;
    value = bytes / 1000 ** tier;
  }
  const number = tier === 0
    ? new Intl.NumberFormat(locale).format(bytes)
    : new Intl.NumberFormat(locale, { maximumFractionDigits: 1 }).format(value);
  return `${number} ${BYTE_UNITS[tier]}`;
}
