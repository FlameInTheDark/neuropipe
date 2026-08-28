/**
 * Shared class tokens.
 * These replace the previously duplicated `inputCls` (Inspector),
 * `field` (SystemViews) and `modalInput` (WorkViews) constants.
 */

export const surface = {
  /** raised panel / card */
  panel: "rounded-xl border border-ink-700/80 bg-ink-850/40",
  /** inset well used inside panels */
  well: "rounded-lg border border-ink-700 bg-ink-950/60",
  /** floating overlay (menus, popovers, modals) */
  overlay:
    "rounded-xl border border-ink-650 bg-ink-900 shadow-[0_40px_100px_-30px_rgba(0,0,0,0.95)]",
} as const;

export const control = {
  /** standard single-line input / select height + chrome */
  input:
    "h-8 w-full rounded-md border border-ink-700 bg-ink-850 px-2.5 text-[12.5px] text-fg " +
    "placeholder:text-fg-faint transition focus:border-ink-400 focus:bg-ink-800 focus:outline-none",
  /** multi-line variant */
  textarea:
    "w-full rounded-md border border-ink-700 bg-ink-850 px-2.5 py-[6px] text-[12.5px] leading-relaxed " +
    "text-fg placeholder:text-fg-faint transition focus:border-ink-400 focus:bg-ink-800 focus:outline-none",
  /** compact inline input used in dense pin rows */
  inputSm:
    "h-7 rounded-md border border-ink-700 bg-ink-850 px-2 text-[12px] text-fg focus:border-ink-500 focus:outline-none",
  /** container for a segmented control / tab strip */
  segment: "flex items-center gap-0.5 rounded-md border border-ink-700 bg-ink-850 p-0.5",
} as const;

export const chip = {
  /** neutral metadata pill */
  muted: "rounded-md border border-ink-700 bg-ink-850 px-2 py-1 text-[10.5px] text-fg-subtle",
  /** monospace tag */
  mono: "rounded-md border border-ink-700 bg-ink-900 px-2 py-1 font-mono text-[10.5px] text-fg-subtle",
  /** keyboard hint */
  kbd: "rounded border border-ink-700 bg-ink-850 px-1 py-px font-mono text-[10px] text-fg-faint",
} as const;

export const text = {
  /** uppercase section eyebrow */
  eyebrow: "text-[10.5px] font-medium tracking-[0.09em] text-fg-subtle uppercase",
  label: "text-[11.5px] font-medium text-fg-subtle",
  hint: "text-[11px] leading-relaxed text-fg-faint",
} as const;
