import { useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { useTranslation } from "react-i18next";
import { Icon } from "./icons";
import { Tooltip } from "./Tooltip";
import { AddButton, EditorLabel, EmptyHint } from "./KVFieldEditors";
import { dataPinColor } from "../lib/node-pins";
import { cn } from "../utils/cn";
import {
  MAX_BUILD_ROWS,
  buildItemsPayload,
  buildMapEntriesPayload,
  literalIssue,
  nextBuildRowID,
  parseBuildItems,
  parseMapEntries,
  previewValue,
  type BuildItemRow,
  type BuildMapEntryRow,
  type LiteralIssue,
} from "../lib/build-nodes";
import type { DataType } from "../lib/types";

/**
 * Inspector editor for Build Array's items and Build Map's entries — the two
 * row-configured data builders. The node-level collection type (element or
 * value type) is configured with the select field above this editor; every
 * row's pin and constant is validated against it, like a []T or
 * map[string]V. The inspector shows a compact summary (count plus pin names
 * or keys); the "Open editor" button opens a spacious modal with column
 * headers, reordering, duplication, live per-row validation, and an output
 * preview. The modal edits a local draft and commits only on Apply, so
 * Cancel and Escape never write half-finished rows.
 *
 * Row identity mirrors the Go contract: ids are minted once and never
 * re-derived from content, so canvas wires stay attached through renames and
 * reordering.
 */

type RowKind = "array" | "map";

type DraftRow = { id: string; label: string; value: string; key: string };

const SUMMARY_CHIPS = 8;

export function BuildRowsEditor({
  kind,
  label,
  nodeTitle,
  dataType,
  value,
  onChange,
}: {
  kind: RowKind;
  label: string;
  nodeTitle?: string;
  dataType: string;
  value: unknown;
  onChange: (next: unknown) => void;
}) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const openButtonRef = useRef<HTMLButtonElement>(null);
  const rows = useMemo<ReadonlyArray<BuildItemRow | BuildMapEntryRow>>(
    () => (kind === "array" ? parseBuildItems(value) : parseMapEntries(value)),
    [kind, value],
  );
  const names: string[] = rows.map((row) => ("key" in row ? row.key : row.label || row.id));

  return (
    <div className="space-y-2">
      <EditorLabel count={rows.length}>{label}</EditorLabel>
      {rows.length === 0 ? (
        <EmptyHint text={kind === "array" ? t("editor.buildEmptyItems") : t("editor.buildEmptyEntries")} />
      ) : (
        <div className="flex flex-wrap gap-1">
          {names.slice(0, SUMMARY_CHIPS).map((name, index) => (
            <span
              key={`${rows[index].id}-${index}`}
              className="max-w-[120px] truncate rounded border border-ink-700 bg-ink-850 px-1.5 py-px font-mono text-[10.5px] text-fg-subtle"
              title={name}
            >
              {name}
            </span>
          ))}
          {names.length > SUMMARY_CHIPS && (
            <span className="rounded px-1 py-px font-mono text-[10.5px] text-fg-faint">
              +{names.length - SUMMARY_CHIPS}
            </span>
          )}
        </div>
      )}
      <button
        ref={openButtonRef}
        type="button"
        onClick={() => setOpen(true)}
        className="flex h-7 w-full items-center justify-center gap-1.5 rounded-md border border-ink-700 bg-ink-850 px-2 text-[11.5px] text-fg-muted transition hover:border-ink-500 hover:bg-ink-750"
      >
        <Icon name={kind === "array" ? "List" : "Braces"} className="h-3.5 w-3.5" />
        {t("editor.buildOpenEditor")}
      </button>
      {open &&
        createPortal(
          <BuildRowsModal
            kind={kind}
            label={label}
            nodeTitle={nodeTitle}
            dataType={dataType}
            value={value}
            onApply={onChange}
            onClose={() => {
              setOpen(false);
              openButtonRef.current?.focus();
            }}
          />,
          document.body,
        )}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* Modal editor                                                        */
/* ------------------------------------------------------------------ */

type RowIssue = { key: boolean; duplicate: boolean; literal: LiteralIssue | null };

function BuildRowsModal({
  kind,
  label,
  nodeTitle,
  dataType,
  value,
  onApply,
  onClose,
}: {
  kind: RowKind;
  label: string;
  nodeTitle?: string;
  dataType: string;
  value: unknown;
  onApply: (next: unknown) => void;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const containerRef = useRef<HTMLDivElement>(null);
  const [draft, setDraft] = useState<DraftRow[]>(() => parseDraft(kind, value));
  const originalPayload = useMemo(() => payloadOf(kind, parseDraft(kind, value)), [kind, value]);
  const dirty = useMemo(
    () => JSON.stringify(payloadOf(kind, draft)) !== JSON.stringify(originalPayload),
    [kind, draft, originalPayload],
  );

  /* Per-row validation: required and duplicate keys, constant coercion. */
  const issues = useMemo(() => {
    const result: RowIssue[] = draft.map((row) => ({
      key: kind === "map" && row.key.trim() === "",
      duplicate: false,
      literal: literalIssue(dataType, row.value),
    }));
    if (kind === "map") {
      const seen = new Map<string, number>();
      draft.forEach((row, index) => {
        const key = row.key.trim();
        if (key === "") return;
        if (seen.has(key)) {
          result[seen.get(key)!].duplicate = true;
          result[index].duplicate = true;
        } else {
          seen.set(key, index);
        }
      });
    }
    return result;
  }, [draft, kind, dataType]);
  const effectiveRows = kind === "map" ? draft.filter((row) => row.key.trim() !== "") : draft;
  const emptyError = effectiveRows.length === 0;
  const errorCount = issues.filter((issue) => issue.key || issue.duplicate || issue.literal).length + (emptyError ? 1 : 0);
  const atLimit = draft.length >= MAX_BUILD_ROWS;

  const apply = () => {
    if (errorCount > 0) return;
    onApply(payloadOf(kind, draft));
    onClose();
  };

  /* Escape closes (unless a dropdown menu holds focus), Cmd/Ctrl+S applies. */
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape" && !(document.activeElement as HTMLElement | null)?.closest?.('[role="listbox"]')) {
        onClose();
      }
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "s") {
        event.preventDefault();
        apply();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [draft, errorCount, onClose]);

  /* Initial focus lands on the first editable control. */
  useEffect(() => {
    containerRef.current?.querySelector<HTMLElement>("input, button")?.focus();
  }, []);

  const patch = (index: number, next: Partial<DraftRow>) =>
    setDraft((rows) => rows.map((row, i) => (i === index ? { ...row, ...next } : row)));

  const addRow = () => {
    if (atLimit) return;
    setDraft((rows) => [
      ...rows,
      {
        id: nextBuildRowID(rows),
        label: kind === "array" ? `Item ${rows.length + 1}` : "",
        value: "",
        key: "",
      },
    ]);
    requestAnimationFrame(() => {
      const rows = containerRef.current?.querySelectorAll<HTMLElement>("[data-build-row]");
      const fresh = rows?.[rows.length - 1];
      (fresh?.querySelector<HTMLElement>("input, button") ?? fresh)?.focus();
    });
  };

  const duplicateRow = (index: number) => {
    if (atLimit) return;
    setDraft((rows) => {
      const next = [...rows];
      next.splice(index + 1, 0, { ...rows[index], id: nextBuildRowID(rows) });
      return next;
    });
  };

  const moveRow = (index: number, delta: number) => {
    const target = index + delta;
    if (target < 0 || target >= draft.length) return;
    setDraft((rows) => {
      const next = [...rows];
      [next[index], next[target]] = [next[target], next[index]];
      return next;
    });
  };

  /* Enter in the last row appends another row, spreadsheet-style. */
  const onRowKeyDown = (event: React.KeyboardEvent, index: number) => {
    if (event.key === "Enter" && index === draft.length - 1 && !atLimit) {
      event.preventDefault();
      addRow();
    }
  };

  /* Keyboard trap keeps Tab inside the dialog. */
  const onKeyDown = (event: React.KeyboardEvent) => {
    if (event.key !== "Tab") return;
    const focusables = containerRef.current?.querySelectorAll<HTMLElement>(
      'button:not(:disabled), input, [tabindex]:not([tabindex="-1"])',
    );
    if (!focusables || focusables.length === 0) return;
    const first = focusables[0];
    const last = focusables[focusables.length - 1];
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  };

  const title = nodeTitle ? `${nodeTitle} — ${label}` : label;
  const gridCls =
    kind === "array"
      ? "grid grid-cols-[20px_minmax(0,1.1fr)_minmax(0,1fr)_102px] gap-1.5"
      : "grid grid-cols-[20px_minmax(0,1fr)_minmax(0,1fr)_minmax(0,1fr)_102px] gap-1.5";

  return createPortal(
    <div
      className="fixed inset-0 z-[70] flex items-center justify-center bg-black/70 p-4 backdrop-blur-[3px] sm:p-6"
      onClick={() => {
        if (!dirty) onClose();
      }}
    >
      <div
        ref={containerRef}
        role="dialog"
        aria-modal="true"
        aria-label={title}
        onKeyDown={onKeyDown}
        className={cn(
          "pop-in flex w-full flex-col overflow-hidden rounded-xl border border-ink-650 bg-ink-900 shadow-[0_40px_100px_-30px_rgba(0,0,0,0.95)]",
          "h-[min(92vh,760px)] max-w-[min(96vw,880px)]",
        )}
        onClick={(event) => event.stopPropagation()}
      >
        {/* ── header ── */}
        <div className="flex h-11 shrink-0 items-center gap-2.5 border-b border-seam px-4">
          <Icon name={kind === "array" ? "List" : "Braces"} className="h-4 w-4 text-fg-subtle" />
          <h2 className="truncate text-[13px] font-semibold text-fg">{title}</h2>
          <Tooltip content={t("editor.buildTypeChipTitle")} side="bottom">
            <span className="flex shrink-0 items-center gap-1.5 rounded border border-ink-700 bg-ink-850 px-1.5 py-px font-mono text-[10.5px] text-fg-subtle">
              <span className="h-1.5 w-1.5 rounded-full" style={{ backgroundColor: dataPinColor(dataType as DataType) }} />
              {dataType}
            </span>
          </Tooltip>
          {dirty && (
            <span className="flex items-center gap-1.5 text-[11px] text-fg-subtle">
              <span className="h-1.5 w-1.5 rounded-full bg-warning/80" />
              {t("editor.buildModified")}
            </span>
          )}
          <div className="ml-auto flex items-center gap-1">
            <Tooltip content={t("common.close")} hint="Esc" side="bottom">
              <button
                type="button"
                onClick={onClose}
                className="grid h-7 w-7 place-items-center rounded-md text-fg-subtle transition hover:bg-ink-800 hover:text-fg"
              >
                <Icon name="X" className="h-4 w-4" />
              </button>
            </Tooltip>
          </div>
        </div>

        {/* ── body: rows + preview ── */}
        <div className="flex min-h-0 flex-1">
          <div className="flex min-w-0 flex-1 flex-col">
            <div className="min-h-0 flex-1 overflow-y-auto p-4">
              {draft.length > 0 && (
                <div className={cn(gridCls, "mb-2 px-0.5 text-[10px] uppercase tracking-wide text-fg-faint")}>
                  <span>#</span>
                  {kind === "map" && <span>{t("editor.buildColKey")}</span>}
                  <span>{t("editor.buildColPinName")}</span>
                  <span>{t("editor.buildColValue")}</span>
                </div>
              )}
              <div className="space-y-1.5">
                {draft.map((row, index) => {
                  const issue = issues[index];
                  const literalError = issue.literal;
                  return (
                    <div
                      key={index}
                      data-build-row={index}
                      className="space-y-1 rounded-md border border-ink-700/70 bg-ink-850/50 p-2"
                    >
                      <div className={gridCls}>
                        <span className="pt-1.5 text-right font-mono text-[11px] text-fg-faint">{index + 1}</span>
                        {kind === "map" && (
                          <input
                            value={row.key}
                            onChange={(e) => patch(index, { key: e.target.value })}
                            onKeyDown={(e) => onRowKeyDown(e, index)}
                            aria-label={t("editor.buildColKey")}
                            placeholder="id"
                            className={cn(
                              "h-7 min-w-0 rounded-md border bg-ink-900 px-2 font-mono text-[11.5px] text-fg transition focus:outline-none",
                              issue.key || issue.duplicate
                                ? "border-danger/70 focus:border-danger"
                                : "border-ink-700 focus:border-ink-500",
                            )}
                          />
                        )}
                        <input
                          value={row.label}
                          onChange={(e) => patch(index, { label: e.target.value })}
                          onKeyDown={(e) => onRowKeyDown(e, index)}
                          aria-label={t("editor.buildColPinName")}
                          placeholder={kind === "array" ? `Item ${index + 1}` : row.key || "id"}
                          className="h-7 min-w-0 rounded-md border border-ink-700 bg-ink-900 px-2 text-[11.5px] text-fg transition focus:border-ink-500 focus:outline-none"
                        />
                        <input
                          value={row.value}
                          onChange={(e) => patch(index, { value: e.target.value })}
                          onKeyDown={(e) => onRowKeyDown(e, index)}
                          aria-label={t("editor.buildColValue")}
                          placeholder={t("editor.pinValuePlaceholder")}
                          className={cn(
                            "h-7 min-w-0 rounded-md border bg-ink-900 px-2 font-mono text-[11.5px] text-fg transition focus:outline-none",
                            literalError
                              ? "border-danger/70 focus:border-danger"
                              : "border-ink-700 focus:border-ink-500",
                          )}
                        />
                        <div className="flex items-center justify-end gap-0.5">
                          <button
                            type="button"
                            onClick={() => moveRow(index, -1)}
                            disabled={index === 0}
                            aria-label={t("editor.buildMoveUp")}
                            className="grid h-6 w-6 place-items-center rounded-md text-fg-faint transition hover:bg-ink-750 hover:text-fg disabled:pointer-events-none disabled:opacity-30"
                          >
                            <Icon name="ChevronUp" className="h-3.5 w-3.5" />
                          </button>
                          <button
                            type="button"
                            onClick={() => moveRow(index, 1)}
                            disabled={index === draft.length - 1}
                            aria-label={t("editor.buildMoveDown")}
                            className="grid h-6 w-6 place-items-center rounded-md text-fg-faint transition hover:bg-ink-750 hover:text-fg disabled:pointer-events-none disabled:opacity-30"
                          >
                            <Icon name="ChevronDown" className="h-3.5 w-3.5" />
                          </button>
                          <button
                            type="button"
                            onClick={() => duplicateRow(index)}
                            disabled={atLimit}
                            aria-label={t("editor.buildDuplicateRow")}
                            className="grid h-6 w-6 place-items-center rounded-md text-fg-faint transition hover:bg-ink-750 hover:text-fg disabled:pointer-events-none disabled:opacity-30"
                          >
                            <Icon name="Copy" className="h-3.5 w-3.5" />
                          </button>
                          <button
                            type="button"
                            onClick={() => setDraft((rows) => rows.filter((_, i) => i !== index))}
                            aria-label={t("common.delete")}
                            className="grid h-6 w-6 place-items-center rounded-md text-fg-faint transition hover:bg-danger/15 hover:text-danger-fg"
                          >
                            <Icon name="Trash2" className="h-3.5 w-3.5" />
                          </button>
                        </div>
                      </div>
                      {(kind === "map" && issue.key) || issue.duplicate || literalError ? (
                        <p className="flex items-center gap-1.5 pl-7 text-[10.5px] text-danger-fg">
                          <Icon name="AlertTriangle" className="h-3 w-3 shrink-0" />
                          {issue.key
                            ? t("editor.buildKeyRequired")
                            : issue.duplicate
                              ? t("editor.buildDuplicateKey")
                              : literalError === "number"
                                ? t("editor.buildLiteralNumber")
                                : literalError === "boolean"
                                  ? t("editor.buildLiteralBoolean")
                                  : literalError === "json"
                                    ? t("editor.buildLiteralJson")
                                    : t("editor.buildLiteralUnsupported")}
                        </p>
                      ) : null}
                    </div>
                  );
                })}
              </div>
              {emptyError && draft.length > 0 && (
                <p className="mt-2 flex items-center gap-1.5 text-[10.5px] text-danger-fg">
                  <Icon name="AlertTriangle" className="h-3 w-3 shrink-0" />
                  {t("editor.buildRowRequired")}
                </p>
              )}
              {draft.length === 0 && (
                <EmptyHint text={kind === "array" ? t("editor.buildEmptyItems") : t("editor.buildEmptyEntries")} />
              )}
              <div className="mt-2">
                {atLimit ? (
                  <p className="rounded-md border border-dashed border-ink-700 px-2.5 py-2 text-[11px] text-fg-faint">
                    {t("editor.buildRowLimit", { count: MAX_BUILD_ROWS })}
                  </p>
                ) : (
                  <AddButton
                    label={kind === "array" ? t("editor.buildAddItem") : t("editor.buildAddEntry")}
                    onClick={addRow}
                  />
                )}
              </div>
            </div>
          </div>

          {/* ── output preview ── */}
          <aside className="hidden w-[264px] shrink-0 flex-col border-l border-seam lg:flex">
            <div className="flex h-9 shrink-0 items-center gap-1.5 border-b border-seam px-3 text-[11px] font-medium text-fg-subtle">
              <Icon name="Eye" className="h-3.5 w-3.5" />
              {t("editor.buildPreviewTitle")}
            </div>
            <div className="min-h-0 flex-1 overflow-y-auto p-3">
              <pre className="whitespace-pre-wrap font-mono text-[11px] leading-relaxed text-fg">
                {kind === "array" ? renderArrayPreview(draft, dataType) : renderMapPreview(draft, dataType)}
              </pre>
            </div>
            <p className="border-t border-seam px-3 py-2.5 text-[10.5px] leading-relaxed text-fg-faint">
              {t("editor.buildPreviewLegend")}
            </p>
          </aside>
        </div>

        {/* ── footer ── */}
        <div className="flex h-10 shrink-0 items-center gap-3 border-t border-seam px-4 text-[11px] text-fg-faint">
          <span>
            {kind === "array"
              ? t("editor.buildItemCount", { count: draft.length })
              : t("editor.buildEntryCount", { count: draft.length })}
          </span>
          <span className="h-3 w-px bg-ink-700" />
          {errorCount > 0 ? (
            <span className="truncate text-danger-fg">
              {t("editor.buildFixRows", { count: errorCount })}
            </span>
          ) : (
            <span className="truncate">{t("editor.buildPinHint")}</span>
          )}
          <div className="ml-auto flex shrink-0 items-center gap-2">
            <kbd className="rounded border border-ink-700 bg-ink-850 px-1 py-px font-mono text-[10px] text-fg-faint">
              ⌘S
            </kbd>
            <button
              type="button"
              onClick={onClose}
              className="h-7 rounded-md border border-ink-700 bg-ink-850 px-3 text-[12px] text-fg-muted transition hover:bg-ink-750"
            >
              {t("common.cancel")}
            </button>
            <button
              type="button"
              onClick={apply}
              disabled={errorCount > 0}
              className={cn(
                "h-7 rounded-md px-3 text-[12px] font-medium transition",
                errorCount > 0
                  ? "cursor-not-allowed bg-ink-800 text-fg-faint"
                  : "bg-ink-50 text-fg-onEmphasis hover:bg-ink-25",
              )}
            >
              {t("editor.buildApply")}
            </button>
          </div>
        </div>
      </div>
    </div>,
    document.body,
  );
}

/* ------------------------------------------------------------------ */
/* Preview rendering                                                   */
/* ------------------------------------------------------------------ */

function parseDraft(kind: RowKind, value: unknown): DraftRow[] {
  if (kind === "array") {
    return parseBuildItems(value).map((row: BuildItemRow) => ({ ...row, key: "" }));
  }
  return parseMapEntries(value).map((row: BuildMapEntryRow) => ({ ...row }));
}

function payloadOf(kind: RowKind, rows: DraftRow[]): unknown[] {
  if (kind === "array") {
    return buildItemsPayload(rows.map(({ key: _key, ...row }) => row));
  }
  return buildMapEntriesPayload(rows.map((row) => ({ ...row })));
}

function renderArrayPreview(rows: DraftRow[], dataType: string): React.ReactNode {
  if (rows.length === 0) return "[]";
  return (
    <>
      {"[\n"}
      {rows.map((row, index) => (
        <span key={index}>
          {"  "}
          {renderValue(dataType, row.value, row.label || row.id)}
          {index < rows.length - 1 ? "," : ""}
          {"\n"}
        </span>
      ))}
      {"]"}
    </>
  );
}

function renderMapPreview(rows: DraftRow[], dataType: string): React.ReactNode {
  const keyed = rows.filter((row) => row.key.trim() !== "");
  if (keyed.length === 0) return "{}";
  return (
    <>
      {"{\n"}
      {keyed.map((row, index) => (
        <span key={index}>
          {"  "}
          {`"${row.key.trim()}": `}
          {renderValue(dataType, row.value, row.label || row.key)}
          {index < keyed.length - 1 ? "," : ""}
          {"\n"}
        </span>
      ))}
      {"}"}
    </>
  );
}

function renderValue(dataType: string, value: string, fallbackName: string): React.ReactNode {
  const constant = previewValue(dataType, value);
  if (constant === undefined) {
    return <span className="italic text-fg-faint">{fallbackName}</span>;
  }
  return JSON.stringify(constant);
}
