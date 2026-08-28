import { useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { useTranslation } from "react-i18next";
import JsonView from "@uiw/react-json-view";
import type { LogEntry } from "@/types";
import { Icon } from "./icons";
import { Tooltip } from "./Tooltip";
import { Dot } from "./ui";
import { formatBytes, formatDateTime, formatDuration } from "@/lib/format";
import { jsonPathToString } from "@/lib/jsonPath";
import { cn } from "../utils/cn";

/**
 * Full-screen inspector for a single execution-log entry: the input packet on
 * the left, the output packet on the right, each rendered as a collapsible
 * JSON tree (or raw pretty-printed text) so large payloads stay navigable.
 */
export function JsonViewerModal({ entry, onClose }: { entry: LogEntry; onClose: () => void }) {
  const { t } = useTranslation();

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  const tone =
    entry.status === "completed" ? "done"
    : entry.status === "running" ? "running"
    : entry.status === "failed" ? "error"
    : "idle";

  return createPortal(
    <div
      className="fixed inset-0 z-[70] flex items-center justify-center bg-black/70 p-4 backdrop-blur-[3px] sm:p-6"
      onClick={onClose}
    >
      <div
        className={cn(
          "pop-in flex w-full flex-col overflow-hidden rounded-xl border border-ink-650 bg-ink-900",
          "shadow-[0_40px_100px_-30px_rgba(0,0,0,0.95)]",
          // wide two-pane layout: grow with the viewport, capped at 1500×960
          "h-[min(94vh,960px)] max-w-[min(96vw,1500px)]",
        )}
        onClick={(e) => e.stopPropagation()}
      >
        {/* ── header: node identity + run meta ── */}
        <div className="flex h-11 shrink-0 items-center gap-2.5 border-b border-seam px-4">
          <Icon name="Braces" className="h-4 w-4 shrink-0 text-info-fg" />
          <h2 className="min-w-0 truncate text-[13px] font-semibold text-fg" title={entry.node}>
            {entry.node}
          </h2>
          <span className="flex shrink-0 items-center gap-1.5 rounded-md border border-ink-700 bg-ink-850 px-2 py-0.5 text-[10.5px] capitalize text-fg-subtle">
            <Dot tone={tone} />
            {t(`runStatus.${entry.status}`)}
          </span>
          <span className="shrink-0 font-mono text-[10.5px] text-fg-faint">{formatDuration(entry.ms)}</span>
          <span className="shrink-0 text-[10.5px] text-fg-faint">·</span>
          <span className="shrink-0 font-mono text-[10.5px] text-fg-faint">{formatDateTime(entry.time)}</span>

          <div className="ml-auto flex items-center gap-1">
            <Tooltip content={t("common.close")} hint="Esc" side="bottom">
              <button
                onClick={onClose}
                className="grid h-7 w-7 place-items-center rounded-md text-fg-subtle transition hover:bg-ink-800 hover:text-fg"
              >
                <Icon name="X" className="h-4 w-4" />
              </button>
            </Tooltip>
          </div>
        </div>

        {/* ── error strip (failed nodes) ── */}
        {entry.error && (
          <pre className="max-h-[88px] shrink-0 overflow-auto border-b border-seam bg-danger/5 px-4 py-2 font-mono text-[10.5px] leading-relaxed whitespace-pre-wrap text-danger-fg">
            {entry.error}
          </pre>
        )}

        {/* ── body: input | output ── */}
        {/* grid-rows-1 pins the single row to the body's height (repeat(1, minmax(0,1fr)))
            — without it the implicit auto row grows to content height, so a tall JSON
            tree blew past the panes' overflow-auto and painted over the footer */}
        <div className="grid min-h-0 flex-1 grid-cols-2 grid-rows-1 divide-x divide-seam">
          <DataPane side="input" value={entry.input} />
          <DataPane side="output" value={entry.output} />
        </div>

        {/* ── footer ── */}
        <div className="flex h-9 shrink-0 items-center gap-3 border-t border-seam px-4 text-[10.5px] text-fg-faint">
          <span className="truncate font-mono" title={entry.type}>
            {entry.type}
          </span>
          <span className="h-3 w-px shrink-0 bg-ink-700" />
          <span className="shrink-0">{t("jsonViewer.treeHint")}</span>

          <div className="ml-auto flex shrink-0 items-center gap-2">
            <kbd className="rounded border border-ink-700 bg-ink-850 px-1 py-px font-mono text-[10px] text-fg-faint">Esc</kbd>
            <span>{t("common.close")}</span>
          </div>
        </div>
      </div>
    </div>,
    document.body,
  );
}

/* ---------- one side of the modal ---------- */

function DataPane({ side, value }: { side: "input" | "output"; value: unknown }) {
  const { t } = useTranslation();
  /** tree = collapsible JSON tree, raw = pretty-printed text */
  const [mode, setMode] = useState<"tree" | "raw">("tree");
  /** tri-state fold control: null = default depth, true = all expanded, false = all collapsed */
  const [expandAll, setExpandAll] = useState<boolean | null>(null);
  /** bumped on fold toggles — JsonView seeds its expand store on mount, so a
      full remount is the reliable way to apply a new global fold level */
  const [resetKey, setResetKey] = useState(0);
  const [copied, setCopied] = useState(false);
  const copyTimer = useRef<number | undefined>(undefined);

  useEffect(() => () => window.clearTimeout(copyTimer.current), []);

  const has = value !== undefined && value !== null;
  const stats = useMemo(() => (has ? dataStats(value) : null), [has, value]);
  const isInput = side === "input";

  const copy = async () => {
    try {
      await navigator.clipboard?.writeText(formatJson(value));
    } catch {
      /* clipboard unavailable — still flash the check so the click isn't a dead end */
    }
    setCopied(true);
    window.clearTimeout(copyTimer.current);
    copyTimer.current = window.setTimeout(() => setCopied(false), 1400);
  };

  /** Hover actions rendered next to every JSON node: copy the element's value
      (the library's built-in handler, surfaced through the Copied section's
      render override) and copy the dotted path to the node. `keys` is the full
      key chain from the tree root including the node's own key. */
  const renderNodeTools = (
    props: React.SVGProps<SVGSVGElement>,
    result: { keys?: ReadonlyArray<string | number> },
  ) => {
    const elementCopied = Boolean((props as { "data-copied"?: unknown })["data-copied"]);
    const path = result.keys?.length ? jsonPathToString(result.keys) : "";
    return (
      <span className="ml-1.5 inline-flex translate-y-[1px] items-center gap-1 align-middle">
        <NodeToolButton
          testId="copy-element"
          label={t("jsonViewer.copyElement")}
          onClick={props.onClick as unknown as React.MouseEventHandler<HTMLButtonElement>}
          icon={elementCopied ? "Check" : "Copy"}
          iconClass={elementCopied ? "text-success-fg" : undefined}
        />
        {path && <CopyPathButton path={path} label={t("jsonViewer.copyPath")} />}
      </span>
    );
  };

  return (
    <section className="flex min-h-0 min-w-0 flex-col">
      {/* pane header */}
      <div className="flex h-9 shrink-0 items-center gap-2 border-b border-seam px-3">
        <Icon
          name={isInput ? "FileInput" : "FileOutput"}
          className={cn("h-3.5 w-3.5 shrink-0", isInput ? "text-info-fg/80" : "text-success-fg/80")}
        />
        <span className="shrink-0 text-[10.5px] font-medium tracking-[0.09em] text-fg-subtle uppercase">
          {t(isInput ? "editor.entryInput" : "editor.entryOutput")}
        </span>
        {stats && (
          <span
            className="min-w-0 truncate font-mono text-[10px] text-fg-subtle"
            title={`${stats.kind} · ${stats.bytes.toLocaleString()} chars`}
          >
            {formatBytes(stats.bytes)} · {stats.kind}
          </span>
        )}

        {has && (
          <div className="ml-auto flex shrink-0 items-center gap-1">
            {/* tree / raw toggle */}
            <div className="flex items-center gap-0.5 rounded-md border border-ink-700 bg-ink-850 p-0.5">
              {(["tree", "raw"] as const).map((m) => (
                <Tooltip key={m} content={t(`jsonViewer.${m}`)} side="bottom" delay={200}>
                  <button
                    onClick={() => setMode(m)}
                    className={cn(
                      "grid h-[20px] w-[24px] place-items-center rounded transition",
                      mode === m ? "bg-ink-700 text-fg" : "text-fg-subtle hover:text-fg",
                    )}
                  >
                    <Icon name={m === "tree" ? "ListTree" : "Text"} className="h-3 w-3" />
                  </button>
                </Tooltip>
              ))}
            </div>

            {/* collapse / expand everything */}
            <PaneBtn
              icon={expandAll === false ? "ChevronsUpDown" : "ChevronsDownUp"}
              label={expandAll === false ? t("jsonViewer.expandAll") : t("jsonViewer.collapseAll")}
              onClick={() => {
                setExpandAll((prev) => (prev === null ? false : !prev));
                setResetKey((k) => k + 1);
              }}
            />

            {/* copy whole payload */}
            <PaneBtn
              icon={copied ? "Check" : "Copy"}
              label={copied ? t("jsonViewer.copied") : t("common.copy")}
              onClick={copy}
              iconClass={copied ? "text-success-fg" : undefined}
            />
          </div>
        )}
      </div>

      {/* pane body */}
      {!has ? (
        <div className="flex min-h-0 flex-1 flex-col items-center justify-center gap-2.5 px-6 py-10 text-center">
          <Icon name={isInput ? "FileInput" : "FileOutput"} className="h-7 w-7 text-fg-faint" />
          <p className="max-w-[240px] text-[12px] leading-relaxed text-fg-faint">
            {t(isInput ? "jsonViewer.noInput" : "jsonViewer.noOutput")}
          </p>
        </div>
      ) : mode === "raw" ? (
        <pre className="min-h-0 flex-1 overflow-auto px-3.5 py-3 font-mono text-[11px] leading-[1.7] whitespace-pre text-fg-muted select-text">
          {formatJson(value)}
        </pre>
      ) : (
        <div className="min-h-0 flex-1 overflow-auto px-3.5 py-3">
          <JsonView
            key={resetKey}
            value={(isStructured(value) ? value : { value }) as object}
            collapsed={expandAll === null ? 2 : !expandAll}
            displayDataTypes={false}
            highlightUpdates={false}
            shortenTextAfterLength={100}
            indentWidth={14}
            style={{ ...jsonTheme, fontSize: 12, lineHeight: 1.75 }}
          >
            {/* replaces the built-in hover copy icon with two app-styled actions:
                copy element (reuses the library's own click handler + feedback
                state, forwarded through render props) and copy JSON path */}
            <JsonView.Copied render={renderNodeTools} />
          </JsonView>
        </div>
      )}
    </section>
  );
}

/* ---------- pane toolbar button ---------- */

function PaneBtn({
  icon,
  label,
  onClick,
  iconClass,
}: {
  icon: string;
  label: string;
  onClick: () => void;
  iconClass?: string;
}) {
  return (
    <Tooltip content={label} side="bottom" delay={200}>
      <button
        onClick={onClick}
        className="grid h-6 w-6 place-items-center rounded-md text-fg-subtle transition hover:bg-ink-750 hover:text-fg active:scale-95"
      >
        <Icon name={icon} className={cn("h-3.5 w-3.5", iconClass)} />
      </button>
    </Tooltip>
  );
}

/* ---------- per-node hover actions (inside the JSON tree) ---------- */

/** Tiny icon button sized to live inline inside a tree row (12px icons on a
    16px hit target) without inflating the 21px line height. Tooltips use the
    app's portal-based component — native titles are OS-styled and would sit
    behind/clipped by the pane's overflow-auto, and every other button in this
    modal already uses the app Tooltip. */
function NodeToolButton({
  testId,
  label,
  tooltip,
  onClick,
  icon,
  iconClass,
}: {
  testId: string;
  /** accessible name — doubles as the tooltip text when no richer content is passed */
  label: string;
  /** tooltip content overriding the plain label (e.g. with a path preview) */
  tooltip?: React.ReactNode;
  onClick?: React.MouseEventHandler<HTMLButtonElement>;
  icon: string;
  iconClass?: string;
}) {
  return (
    <Tooltip content={tooltip ?? label} side="bottom" delay={200}>
      <button
        type="button"
        data-testid={testId}
        aria-label={label}
        onClick={onClick}
        className={cn(
          "grid h-4 w-4 place-items-center rounded text-fg-faint transition-colors",
          "hover:text-info-fg focus-visible:outline focus-visible:outline-1 focus-visible:outline-info/60",
          iconClass,
        )}
      >
        <Icon name={icon} className="h-3 w-3" />
      </button>
    </Tooltip>
  );
}

/** Copies the dotted JSON path of one node. Self-contained so clicking it
    never re-renders the (potentially huge) tree: feedback lives in local state.
    stopPropagation is mandatory — object/array rows sit inside the expand
    toggle, and without it a copy would also fold the node. */
function CopyPathButton({ path, label }: { path: string; label: string }) {
  const [copied, setCopied] = useState(false);
  const timer = useRef<number | undefined>(undefined);

  useEffect(() => () => window.clearTimeout(timer.current), []);

  const onClick = async (e: React.MouseEvent) => {
    e.stopPropagation();
    try {
      await navigator.clipboard?.writeText(path);
    } catch {
      /* clipboard unavailable — still flash the check so the click isn't a dead end */
    }
    setCopied(true);
    window.clearTimeout(timer.current);
    timer.current = window.setTimeout(() => setCopied(false), 1400);
  };

  return (
    <NodeToolButton
      testId="copy-path"
      label={label}
      tooltip={
        <>
          <span>{label}</span>
          {/* mono + key color to mirror the tree; capped so a deep path can never
              push the nowrap bubble past the viewport (position clamps only) */}
          <span className="max-w-[420px] truncate font-mono text-info-fg/90">{path}</span>
        </>
      }
      onClick={onClick}
      icon={copied ? "Check" : "Route"}
      iconClass={copied ? "text-success-fg" : undefined}
    />
  );
}

/* ---------- helpers ---------- */

/** JSON tree colors resolved from the theme variables (index.css) — flips with [data-theme]. */
const jsonTheme = {
  "--w-rjv-font-family": "var(--font-mono)",
  "--w-rjv-background-color": "transparent",
  "--w-rjv-color": "var(--fg-subtle)", // braces & brackets
  "--w-rjv-line-color": "var(--seam)", // seam indent guides
  "--w-rjv-arrow-color": "var(--fg-faint)", // fold arrows
  "--w-rjv-info-color": "var(--fg-faint)", // "N items" counts
  "--w-rjv-ellipsis-color": "var(--fg-subtle)",
  "--w-rjv-key-string": "var(--code-number)", // object keys
  "--w-rjv-key-number": "var(--code-number)", // array indices
  "--w-rjv-curlybraces-color": "var(--fg-subtle)",
  "--w-rjv-brackets-color": "var(--fg-subtle)",
  "--w-rjv-colon-color": "var(--fg-faint)",
  "--w-rjv-quotes-color": "var(--code-number)",
  "--w-rjv-quotes-string-color": "var(--code-fg)",
  "--w-rjv-type-string-color": "var(--code-fg)", // string values
  "--w-rjv-type-int-color": "var(--code-function)", // numbers
  "--w-rjv-type-float-color": "var(--code-function)",
  "--w-rjv-type-bigint-color": "var(--code-function)",
  "--w-rjv-type-boolean-color": "var(--code-keyword)", // booleans
  "--w-rjv-type-null-color": "var(--fg-faint)",
  "--w-rjv-type-undefined-color": "var(--fg-faint)",
  "--w-rjv-type-date-color": "var(--fg-subtle)",
  "--w-rjv-type-url-color": "var(--code-number)",
  "--w-rjv-copied-color": "var(--code-number)",
  "--w-rjv-copied-success-color": "var(--status-success)",
} as React.CSSProperties;

/** Objects/arrays render natively in the tree; primitives get wrapped because
    the viewer misrenders bare roots (strings split into characters, null throws). */
function isStructured(value: unknown): value is object {
  return typeof value === "object" && value !== null;
}

/** Size + shape summary for a pane header, e.g. "2.4 kB · object{12}". */
function dataStats(value: unknown): { bytes: number; kind: string } {
  let text = "";
  try {
    text = JSON.stringify(value) ?? String(value);
  } catch {
    text = String(value);
  }
  const bytes = text.length;
  if (Array.isArray(value)) return { bytes, kind: `array[${value.length}]` };
  if (value && typeof value === "object") return { bytes, kind: `object{${Object.keys(value as object).length}}` };
  if (typeof value === "string") return { bytes, kind: `string[${value.length}]` };
  return { bytes, kind: typeof value };
}

/** Pretty-prints a value as JSON; returns the string as-is on failure. */
function formatJson(value: unknown): string {
  try {
    return JSON.stringify(value, null, 2) ?? String(value);
  } catch {
    return String(value);
  }
}
