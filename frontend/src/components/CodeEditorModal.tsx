import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { useTranslation } from "react-i18next";
import Editor, { loader } from "@monaco-editor/react";
import type { editor } from "monaco-editor";
import { Icon } from "./icons";
import { Tooltip } from "./Tooltip";
import { Dropdown } from "./Dropdown";
import { cn } from "../utils/cn";
import type { PinDataType, Port } from "@/types";
import type { CodeGenerationRequest, DatabaseTable, SQLResult } from "@/lib/types";
import type { EditorApi } from "@/features/graph/PipelineEditor";
import { ASSIGNABLE_PIN_TYPES, pinColor } from "../lib/pins";
import { Toggle } from "./ui";
import { control } from "./primitives/styles";
import { SQL_PIN_ID, buildSqlPayload, parseSqlParams, type SqlParamRow } from "@/lib/sql-params";
import { TypeSpecField, specTopToken, tokenToPinDataType } from "./TypeSpecField";

/* ---- monaco dark theme ---- */
loader.init().then((monaco) => {
  monaco.editor.defineTheme("neuropipe", {
    base: "vs-dark",
    inherit: true,
    rules: [
      { token: "comment", foreground: "55555f", fontStyle: "italic" },
      { token: "keyword", foreground: "c4b5fd" },
      { token: "string", foreground: "f9a8d4" },
      { token: "number", foreground: "7dd3fc" },
      { token: "type", foreground: "6ee7b7" },
      { token: "identifier", foreground: "c9c9d2" },
      { token: "delimiter", foreground: "7c7c88" },
      { token: "operator", foreground: "94a3b8" },
    ],
    colors: {
      "editor.background": "#0c0c0e",
      "editor.foreground": "#c9c9d2",
      "editor.lineHighlightBackground": "#ffffff06",
      "editor.selectionBackground": "#ffffff18",
      "editorLineNumber.foreground": "#3a3a43",
      "editorLineNumber.activeForeground": "#7c7c88",
      "editorCursor.foreground": "#ecedf1",
      "editorGutter.background": "#0c0c0e",
      "editorWidget.background": "#141417",
      "editorWidget.border": "#1c1c21",
      "input.background": "#101012",
      "input.border": "#1c1c21",
      focusBorder: "#3a3a43",
      "list.activeSelectionBackground": "#1c1c21",
      "list.hoverBackground": "#17171b",
    },
  });
});

const PIN_TYPES = ASSIGNABLE_PIN_TYPES;

export function CodeEditorModal({
  title,
  kind,
  code,
  database,
  databases,
  inputs,
  outputs,
  api,
  onSave,
  onClose,
  onPortsChange,
  onChangeDatabase,
  sqlParameters,
  onChangeSqlParameters,
}: {
  title: string;
  kind: "js" | "sql";
  code: string;
  database?: string;
  databases: { value: string; label: string; icon: string }[];
  inputs: Port[];
  outputs: Port[];
  api: Pick<EditorApi, "validateJavaScript" | "debugDatabase" | "inspectDatabase" | "generateCode">;
  onSave: (code: string, database?: string) => void;
  onClose: () => void;
  onPortsChange?: (inputs: Port[], outputs: Port[]) => void;
  onChangeDatabase?: (db: string) => void;
  /** SQL only: raw config.parameters contract + writer */
  sqlParameters?: unknown;
  onChangeSqlParameters?: (next: unknown) => void;
}) {
  const { t } = useTranslation();
  const [draft, setDraft] = useState(code);
  const [selDB, setSelDB] = useState(database || databases[0]?.value || "");
  /** display name for the selected database id (ids are opaque uuids) */
  const dbLabel = databases.find((d) => d.value === selDB)?.label ?? selDB;
  const [sideTab, setSideTab] = useState<"pins" | "schema">(kind === "sql" ? "schema" : "pins");
  const [running, setRunning] = useState(false);
  const [result, setResult] = useState<SQLResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [resultsOpen, setResultsOpen] = useState(true);
  const [assistantOpen, setAssistantOpen] = useState(false);
  const [validating, setValidating] = useState(false);
  const editorRef = useRef<editor.IStandaloneCodeEditor | null>(null);
  const dirty = draft !== code || (kind === "sql" && selDB !== database);

  /* keyboard shortcuts */
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "s") {
        e.preventDefault();
        void commitSave();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [draft, selDB]);

  /** Applies the draft; JavaScript is validated by the backend before saving. */
  const commitSave = async () => {
    if (!dirty) return;
    if (kind === "js") {
      setValidating(true);
      setError(null);
      try {
        await api.validateJavaScript(draft);
      } catch (err) {
        setError(err instanceof Error ? err.message : String(err));
        setValidating(false);
        return;
      }
      setValidating(false);
    }
    onSave(draft, kind === "sql" ? selDB : undefined);
  };

  /** Executes the statement against the selected database (debug run). */
  const runDebug = useCallback(async () => {
    if (kind !== "sql") return;
    setRunning(true);
    setResultsOpen(true);
    setError(null);
    try {
      const parameters = inputs
        .filter((p) => p.kind === "data" && p.id !== SQL_PIN_ID)
        .map((p) => ({ name: p.id, value: "" }));
      const res = await api.debugDatabase({
        databaseId: selDB,
        sql: draft,
        parameters,
        maxRows: 200,
      });
      setResult(res);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
      setResult(null);
    } finally {
      setRunning(false);
    }
  }, [api, draft, inputs, kind, selDB]);

  const stats = useMemo(() => {
    const lines = draft.split("\n").length;
    const chars = draft.length;
    return { lines, chars };
  }, [draft]);

  const onEditorMount = useCallback(
    (ed: editor.IStandaloneCodeEditor) => {
      editorRef.current = ed;
      ed.focus();
      ed.addAction({
        id: "run-code",
        label: "Run",
        keybindings: [2048 /* CtrlCmd */ | 3 /* Enter */],
        run: () => void runDebug(),
      });
    },
    [runDebug],
  );

  /* insert a :placeholder for a pin into the SQL editor */
  const insertPlaceholder = useCallback((label: string) => {
    const ed = editorRef.current;
    if (!ed) return;
    const pos = ed.getPosition();
    if (!pos) return;
    ed.executeEdits("pin-insert", [
      {
        range: {
          startLineNumber: pos.lineNumber,
          startColumn: pos.column,
          endLineNumber: pos.lineNumber,
          endColumn: pos.column,
        },
        text: `:${label}`,
      },
    ]);
    ed.focus();
  }, []);

  return createPortal(
    <div className="fixed inset-0 z-[70] flex items-center justify-center bg-black/70 p-3 backdrop-blur-[3px]" onClick={onClose}>
      <div
        className="pop-in flex w-full flex-col overflow-hidden rounded-xl border border-ink-650 bg-ink-900 shadow-[0_40px_100px_-30px_rgba(0,0,0,0.95)]"
        style={{ maxWidth: "min(98vw, 1500px)", height: "min(96vh, 960px)" }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* header */}
        <div className="flex h-11 shrink-0 items-center gap-2.5 border-b border-seam px-4">
          <Icon name={kind === "js" ? "Braces" : "Database"} className="h-4 w-4 text-ink-400" />
          <h2 className="truncate text-[13px] font-semibold text-ink-50">{title}</h2>
          {dirty && (
            <span className="flex items-center gap-1.5 text-[11px] text-amber-400/90">
              <span className="h-1.5 w-1.5 rounded-full bg-amber-400" />
              {t("common.unsaved")}
            </span>
          )}
          <span className="ml-1 rounded bg-ink-800 px-1.5 py-px text-[10px] font-medium text-ink-400 uppercase">
            {kind === "js" ? "JavaScript" : "SQL"}
          </span>

          <div className="ml-auto flex items-center gap-1">
            <Tooltip content={t("codeAssistant.title")} side="bottom">
              <button
                onClick={() => setAssistantOpen(true)}
                className="grid h-7 w-7 place-items-center rounded-md text-ink-400 transition hover:bg-ink-800 hover:text-ink-50"
              >
                <Icon name="Sparkles" className="h-3.5 w-3.5" />
              </button>
            </Tooltip>

            <div className="flex items-center gap-0.5 rounded-md border border-ink-700 bg-ink-850 p-0.5">
              <TabBtn id="pins" icon="Cable" label={t("javascript.inputs")} active={sideTab === "pins"} onClick={() => setSideTab("pins")} />
              {kind === "sql" && (
                <TabBtn id="schema" icon="Table2" label={t("sql.schema")} active={sideTab === "schema"} onClick={() => setSideTab("schema")} />
              )}
            </div>

            {kind === "sql" && (
              <Dropdown
                value={selDB}
                onChange={(v) => {
                  setSelDB(v);
                  onChangeDatabase?.(v);
                }}
                compact
                className="ml-1 w-[130px]"
                options={databases}
              />
            )}

            {kind === "sql" && (
              <Tooltip content={t("sql.run")} hint="⌘↵" side="bottom">
                <button
                  onClick={() => void runDebug()}
                  className="ml-1 grid h-7 w-7 place-items-center rounded-md text-ink-400 transition hover:bg-ink-800 hover:text-ink-50"
                >
                  <Icon name={running ? "Loader2" : "Play"} className={cn("h-3.5 w-3.5", running && "animate-spin")} />
                </button>
              </Tooltip>
            )}

            <span className="mx-1 h-4 w-px bg-ink-700" />

            <Tooltip content={t("common.close")} hint="Esc" side="bottom">
              <button onClick={onClose} className="grid h-7 w-7 place-items-center rounded-md text-ink-400 transition hover:bg-ink-800 hover:text-ink-50">
                <Icon name="X" className="h-4 w-4" />
              </button>
            </Tooltip>
          </div>
        </div>

        {/* error bar */}
        {error && (
          <div className="flex h-8 shrink-0 items-center gap-2 border-b border-seam bg-rose-500/10 px-4 text-[11.5px] text-rose-200">
            <Icon name="AlertTriangle" className="h-3.5 w-3.5 shrink-0" />
            <span className="min-w-0 flex-1 truncate">{error}</span>
            <button onClick={() => setError(null)} className="text-rose-300 hover:text-white">
              <Icon name="X" className="h-3 w-3" />
            </button>
          </div>
        )}

        {/* hint bar */}
        <div className="flex h-8 shrink-0 items-center gap-2 border-b border-seam px-4 text-[11px] text-ink-500">
          <Icon name="Info" className="h-3.5 w-3.5 shrink-0 text-ink-600" />
          {kind === "js"
            ? t("javascript.editorHint")
            : t("sql.pinOverrideHint")}
        </div>

        {/* body */}
        <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
          <div className="flex min-h-0 flex-1 overflow-hidden">
            {/* monaco editor */}
            <div className="min-w-0 flex-1">
              <Editor
                theme="neuropipe"
                language={kind === "js" ? "javascript" : "sql"}
                value={draft}
                onChange={(v) => setDraft(v ?? "")}
                onMount={onEditorMount}
                options={{
                  fontSize: 13,
                  fontFamily: "var(--font-mono), 'JetBrains Mono', monospace",
                  fontLigatures: true,
                  lineHeight: 20,
                  minimap: { enabled: true, scale: 2, showSlider: "mouseover" },
                  scrollBeyondLastLine: false,
                  smoothScrolling: true,
                  cursorSmoothCaretAnimation: "on",
                  cursorBlinking: "smooth",
                  bracketPairColorization: { enabled: true },
                  autoClosingBrackets: "always",
                  autoClosingQuotes: "always",
                  formatOnPaste: true,
                  wordWrap: "on",
                  tabSize: 2,
                  padding: { top: 12, bottom: 12 },
                  renderLineHighlight: "all",
                  renderWhitespace: "boundary",
                  guides: { indentation: true, bracketPairs: true },
                  suggest: { showKeywords: true, showSnippets: true },
                  quickSuggestions: { other: true, strings: true, comments: false },
                }}
              />
            </div>

            {/* side panel */}
            <div className="w-[240px] shrink-0 overflow-y-auto border-l border-seam bg-ink-900/80">
              {sideTab === "pins" ? (
                <PinPanel
                  kind={kind}
                  inputs={inputs}
                  outputs={outputs}
                  paramsRaw={sqlParameters}
                  onParamsChange={onChangeSqlParameters}
                  onPortsChange={onPortsChange}
                  onInsertPlaceholder={kind === "sql" ? insertPlaceholder : undefined}
                />
              ) : (
                <SchemaPanel db={selDB} name={dbLabel} api={api} />
              )}
            </div>
          </div>

          {/* SQL debug results panel */}
          {kind === "sql" && (result || running || error) && (
            <ResultsPanel
              result={result}
              running={running}
              open={resultsOpen}
              onToggle={() => setResultsOpen((v) => !v)}
              onClose={() => setResult(null)}
            />
          )}
        </div>

        {/* footer */}
        <div className="flex h-9 shrink-0 items-center gap-3 border-t border-seam px-4 text-[10.5px] text-ink-500">
          <span className="font-mono">{stats.lines} lines</span>
          <span className="h-3 w-px bg-ink-700" />
          <span className="font-mono">{stats.chars.toLocaleString()} chars</span>
          <span className="h-3 w-px bg-ink-700" />
          <span className="font-mono uppercase">{kind === "js" ? "JavaScript" : `SQL · ${dbLabel}`}</span>

          <div className="ml-auto flex items-center gap-2">
            <kbd className="rounded border border-ink-700 bg-ink-850 px-1 py-px font-mono text-[10px] text-ink-500">⌘S</kbd>
            <button onClick={onClose} className="h-7 rounded-md border border-ink-700 bg-ink-850 px-3 text-[11.5px] text-ink-200 transition hover:bg-ink-750">
              {t("common.cancel")}
            </button>
            <button
              onClick={() => void commitSave()}
              disabled={!dirty || validating}
              className={cn(
                "h-7 rounded-md px-3 text-[11.5px] font-medium transition",
                dirty && !validating
                  ? "bg-ink-50 text-ink-950 hover:bg-white"
                  : "cursor-not-allowed bg-ink-800 text-ink-500",
              )}
            >
              {validating ? t("common.saving") : t("javascript.save")}
            </button>
          </div>
        </div>
      </div>

      {assistantOpen && (
        <AssistantModal
          kind={kind}
          code={draft}
          selDB={selDB}
          api={api}
          onClose={() => setAssistantOpen(false)}
          onApply={(next) => {
            setDraft(next);
            setAssistantOpen(false);
          }}
        />
      )}
    </div>,
    document.body,
  );
}

/* ================================================================== */

function TabBtn({ id, icon, label, active, onClick }: { id: string; icon: string; label: string; active: boolean; onClick: () => void }) {
  void id;
  return (
    <button
      onClick={onClick}
      className={cn(
        "flex h-[22px] items-center gap-1.5 rounded px-2 text-[11px] transition",
        active ? "bg-ink-700 text-ink-50" : "text-ink-400 hover:text-ink-100",
      )}
    >
      <Icon name={icon} className="h-3 w-3" />
      {label}
    </button>
  );
}

/* ================================================================== */
/*  Pin panel                                                          */
/* ================================================================== */

function PinPanel({
  kind,
  inputs,
  outputs,
  onPortsChange,
  onInsertPlaceholder,
  paramsRaw,
  onParamsChange,
}: {
  kind: "js" | "sql";
  inputs: Port[];
  outputs: Port[];
  onPortsChange?: (inputs: Port[], outputs: Port[]) => void;
  onInsertPlaceholder?: (label: string) => void;
  paramsRaw?: unknown;
  onParamsChange?: (next: unknown) => void;
}) {
  const { t } = useTranslation();
  const dataIns = inputs.filter((p) => p.kind !== "exec");
  const dataOuts = outputs.filter((p) => p.kind !== "exec");
  const execIn = inputs.find((p) => p.kind === "exec");
  const execOut = outputs.find((p) => p.kind === "exec");

  /* SQL parameters are edited right here (as in the prototype); the pins
     themselves arrive resolved from the backend contract. */
  const sqlParamRows = kind === "sql" ? parseSqlParams(paramsRaw) : [];
  const commitParams = (next: SqlParamRow[]) => onParamsChange?.(buildSqlPayload(next));
  const patchParam = (index: number, patch: Partial<SqlParamRow>) =>
    commitParams(sqlParamRows.map((r, i) => (i === index ? { ...r, ...patch } : r)));

  const rebuildPorts = (nextIns: Port[], nextOuts: Port[]) => {
    if (!onPortsChange) return;
    const ei = execIn ? [execIn] : [];
    const eo = execOut ? [execOut] : [];
    onPortsChange([...ei, ...nextIns], [...eo, ...nextOuts]);
  };

  const addPin = (side: "in" | "out") => {
    const list = side === "in" ? dataIns : dataOuts;
    const next: Port = {
      id: `pin_${Math.random().toString(36).slice(2, 7)}`,
      label: `pin_${list.length + 1}`,
      kind: "data",
      dataType: "any",
    };
    if (side === "in") rebuildPorts([...dataIns, next], dataOuts);
    else rebuildPorts(dataIns, [...dataOuts, next]);
  };

  const removePin = (side: "in" | "out", id: string) => {
    if (side === "in") rebuildPorts(dataIns.filter((p) => p.id !== id), dataOuts);
    else rebuildPorts(dataIns, dataOuts.filter((p) => p.id !== id));
  };

  const renamePin = (side: "in" | "out", id: string, label: string) => {
    const update = (list: Port[]) => list.map((p) => (p.id === id ? { ...p, label } : p));
    if (side === "in") rebuildPorts(update(dataIns), dataOuts);
    else rebuildPorts(dataIns, update(dataOuts));
  };

  const changeType = (side: "in" | "out", id: string, dataType: PinDataType) => {
    const update = (list: Port[]) => list.map((p) => (p.id === id ? { ...p, dataType } : p));
    if (side === "in") rebuildPorts(update(dataIns), dataOuts);
    else rebuildPorts(dataIns, update(dataOuts));
  };

  /* SQL splits its inputs: the static "sql" pin carries a replacement
     statement over a wire (never editable, never a placeholder), while the
     remaining data pins are the configured query parameters. */
  const sqlStatementPin = kind === "sql" ? dataIns.find((p) => p.id === SQL_PIN_ID) : undefined;

  return (
    <div className="space-y-4 p-3">
      {/* inputs */}
      <div>
        <div className="mb-1.5 flex items-center gap-1.5">
          <Icon name="LogIn" className="h-3.5 w-3.5 text-ink-500" />
          <span className="text-[10.5px] font-medium tracking-[0.08em] text-ink-400 uppercase">{t("javascript.inputs")}</span>
          <span className="ml-auto font-mono text-[10px] text-ink-600">{dataIns.length}</span>
          {onPortsChange && kind === "js" && (
            <Tooltip content={t("javascript.add")} side="top">
              <button onClick={() => addPin("in")} aria-label={t("javascript.add")} className="grid h-5 w-5 place-items-center rounded text-ink-500 hover:bg-ink-750 hover:text-ink-100">
                <Icon name="Plus" className="h-3 w-3" />
              </button>
            </Tooltip>
          )}
        </div>

        {execIn && <LockedPin port={execIn} />}

        {sqlStatementPin && <LockedPin port={sqlStatementPin} hint={t("sql.sqlPinHint")} />}

        {kind === "sql" ? (
          <>
            {sqlParamRows.length === 0 && (
              <p className="px-1 text-[11px] text-ink-600">{t("sql.noParameters")}</p>
            )}
            {sqlParamRows.map((row, i) => (
              <SqlParamPinRow
                key={i}
                row={row}
                onInsert={onInsertPlaceholder ? () => onInsertPlaceholder(row.name) : undefined}
                onPatch={(p) => patchParam(i, p)}
                onRemove={() => commitParams(sqlParamRows.filter((_, j) => j !== i))}
              />
            ))}
            <button
              onClick={() => commitParams([...sqlParamRows, { name: `param_${sqlParamRows.length + 1}`, label: "", kind: "text", required: false }])}
              className="mt-2 flex h-6 w-full items-center justify-center gap-1.5 rounded-md border border-ink-700 bg-ink-850 px-2 text-[11px] text-ink-200 hover:bg-ink-750"
            >
              <Icon name="Plus" className="h-3 w-3" />
              {t("sql.addParameter")}
            </button>
          </>
        ) : (
          <>
            {dataIns.length === 0 && <p className="px-1 text-[11px] text-ink-600">{t("editor.noDataInputs")}</p>}
            {dataIns.map((p) => (
              <EditablePin
                key={p.id}
                port={p}
                onRename={(l) => renamePin("in", p.id, l)}
                onChangeType={(tp) => changeType("in", p.id, tp)}
                onRemove={() => removePin("in", p.id)}
                onInsert={onInsertPlaceholder ? () => onInsertPlaceholder(p.id) : undefined}
              />
            ))}
          </>
        )}
      </div>

      {/* outputs */}
      <div>
        <div className="mb-1.5 flex items-center gap-1.5">
          <Icon name="LogOut" className="h-3.5 w-3.5 text-ink-500" />
          <span className="text-[10.5px] font-medium tracking-[0.08em] text-ink-400 uppercase">{t("javascript.outputs")}</span>
          <span className="ml-auto font-mono text-[10px] text-ink-600">{dataOuts.length}</span>
          {kind === "js" && onPortsChange && (
            <Tooltip content={t("javascript.add")} side="top">
              <button onClick={() => addPin("out")} aria-label={t("javascript.add")} className="grid h-5 w-5 place-items-center rounded text-ink-500 hover:bg-ink-750 hover:text-ink-100">
                <Icon name="Plus" className="h-3 w-3" />
              </button>
            </Tooltip>
          )}
        </div>

        {execOut && <LockedPin port={execOut} />}

        {dataOuts.length === 0 && <p className="px-1 text-[11px] text-ink-600">{t("editor.noDataOutputs")}</p>}
        {dataOuts.map((p) =>
          kind === "js" ? (
            <EditablePin
              key={p.id}
              port={p}
              onRename={(l) => renamePin("out", p.id, l)}
              onChangeType={(tp) => changeType("out", p.id, tp)}
              onRemove={() => removePin("out", p.id)}
            />
          ) : (
            <LockedPin key={p.id} port={p} />
          ),
        )}
      </div>
    </div>
  );
}

/** A SQL parameter row styled like a regular input pin. Clicking the name
 *  inserts its :id placeholder; the pencil expands label/required editing.
 *  Rows are keyed by position (not name) so typing never steals focus. */
function SqlParamPinRow({
  row,
  onInsert,
  onPatch,
  onRemove,
}: {
  row: SqlParamRow;
  onInsert?: () => void;
  onPatch: (patch: Partial<SqlParamRow>) => void;
  onRemove: () => void;
}) {
  const { t } = useTranslation();
  const [editing, setEditing] = useState(false);
  const dt = tokenToPinDataType(specTopToken(row.spec));

  return (
    <div className="mb-1">
      <div className="group flex items-center gap-1 rounded px-1 py-[2px] hover:bg-ink-850">
        <span className="h-2 w-2 shrink-0 rounded-full" style={{ background: pinColor(dt) }} />

        <Tooltip content={t("sql.insertPlaceholderHint")} side="top">
          <button
            onClick={() => onInsert?.()}
            aria-label={t("sql.insertPlaceholderHint")}
            className="min-w-0 flex-1 truncate text-left font-mono text-[11px] text-ink-100 hover:text-ink-50"
          >
            {row.name}
          </button>
        </Tooltip>

        <Tooltip content={t("common.edit")} side="top">
          <button
            onClick={() => setEditing((v) => !v)}
            aria-label={t("common.edit")}
            aria-expanded={editing}
            className={cn(
              "grid h-4 w-4 shrink-0 place-items-center rounded text-ink-700 transition hover:text-ink-100",
              editing && "bg-ink-750 text-ink-100",
            )}
          >
            <Icon name="Pencil" className="h-3 w-3" />
          </button>
        </Tooltip>

        <TypeSpecField
          compact
          value={row.spec}
          onChange={(spec) => onPatch({ spec })}
        />

        <Tooltip content={t("common.delete")} side="top">
          <button
            onClick={onRemove}
            aria-label={t("common.delete")}
            className="grid h-4 w-4 shrink-0 place-items-center rounded text-ink-700 opacity-0 transition group-hover:opacity-100 hover:text-rose-300"
          >
            <Icon name="X" className="h-3 w-3" />
          </button>
        </Tooltip>
      </div>

      {editing && (
        <div className="mb-1 ml-[14px] space-y-1 rounded border border-ink-700/70 bg-ink-850/50 px-1.5 py-1">
          <input
            value={row.name}
            placeholder={t("sql.parameterName")}
            onChange={(e) => onPatch({ name: e.target.value })}
            className="h-5 w-full rounded border border-ink-700 bg-ink-900 px-1.5 font-mono text-[11px] text-ink-100"
          />
          <input
            value={row.label}
            placeholder={t("sql.parameterLabel")}
            onChange={(e) => onPatch({ label: e.target.value })}
            className="h-5 w-full rounded border border-ink-700 bg-ink-900 px-1.5 text-[11px] text-ink-200"
          />
          <span className="flex items-center gap-1.5 text-[10px] text-ink-400">
            {t("sql.required")}
            <Toggle on={row.required} onChange={(v) => onPatch({ required: v })} />
          </span>
        </div>
      )}
    </div>
  );
}

function LockedPin({ port, children, hint }: { port: Port; children?: React.ReactNode; hint?: string }) {
  return (
    <div className="mb-1 flex flex-wrap items-center gap-1.5 rounded px-1.5 py-[3px]">
      <span className="h-2 w-2 shrink-0 rounded-full" style={{ background: pinColor(port.dataType) }} />
      <span className="min-w-0 flex-1 truncate font-mono text-[11px] text-ink-200">{port.label || port.id}</span>
      {hint ? (
        <Tooltip content={hint} side="top">
          <Icon name="Lock" className="h-3 w-3 shrink-0 cursor-help text-ink-500" />
        </Tooltip>
      ) : (
        <Icon name="Lock" className="h-3 w-3 shrink-0 text-ink-700" />
      )}
      <span className="rounded bg-ink-800 px-1 font-mono text-[9px] text-ink-500">{port.dataType ?? "any"}</span>
      {children && (
        <span className="w-full shrink-0 basis-full pl-4 text-[10px] leading-snug text-ink-600">{children}</span>
      )}
    </div>
  );
}

function EditablePin({
  port,
  onRename,
  onChangeType,
  onRemove,
  onInsert,
}: {
  port: Port;
  onRename: (label: string) => void;
  onChangeType: (dataType: PinDataType) => void;
  onRemove: () => void;
  onInsert?: () => void;
}) {
  const { t } = useTranslation();
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(port.label);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (editing) inputRef.current?.select();
  }, [editing]);

  const commit = () => {
    setEditing(false);
    const next = draft.trim();
    if (next && next !== port.label) onRename(next);
    else setDraft(port.label);
  };

  return (
    <div className="group mb-1 flex items-center gap-1 rounded px-1 py-[2px] hover:bg-ink-850">
      <span className="h-2 w-2 shrink-0 rounded-full" style={{ background: pinColor(port.dataType) }} />

      {editing ? (
        <input
          ref={inputRef}
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onBlur={commit}
          onKeyDown={(e) => {
            if (e.key === "Enter") commit();
            if (e.key === "Escape") {
              setDraft(port.label);
              setEditing(false);
            }
          }}
          className="min-w-0 flex-1 rounded bg-ink-850 px-1 py-[1px] font-mono text-[11px] text-ink-50 outline-none ring-1 ring-ink-500"
        />
      ) : (
        <button
          onClick={() => (onInsert ? onInsert() : setEditing(true))}
          onDoubleClick={() => setEditing(true)}
          aria-label={onInsert ? `:${port.label}` : t("library.renameHint")}
          className="min-w-0 flex-1 truncate text-left font-mono text-[11px] text-ink-100 hover:text-ink-50"
        >
          {port.label}
        </button>
      )}

      <Dropdown
        compact
        value={port.dataType ?? "any"}
        onChange={(v) => onChangeType(v as PinDataType)}
        className="h-4 shrink-0 px-1 font-mono text-[9px] [&>svg]:h-2 [&>svg]:w-2"
        options={PIN_TYPES.map((tp) => ({ value: tp, label: tp }))}
      />

      <Tooltip content={t("common.delete")} side="top">
        <button
          onClick={onRemove}
          aria-label={t("common.delete")}
          className="grid h-4 w-4 shrink-0 place-items-center rounded text-ink-700 opacity-0 transition group-hover:opacity-100 hover:text-rose-300"
        >
          <Icon name="X" className="h-3 w-3" />
        </button>
      </Tooltip>
    </div>
  );
}

/* ================================================================== */
/*  Schema panel — live introspection via InspectDatabase               */
/* ================================================================== */

function SchemaPanel({ db, name, api }: { db: string; name: string; api: Pick<EditorApi, "inspectDatabase"> }) {
  const { t } = useTranslation();
  const [tables, setTables] = useState<DatabaseTable[]>([]);
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    let cancelled = false;
    setTables([]);
    setFailed(false);
    if (!db) return;
    api
      .inspectDatabase(db)
      .then((schema) => {
        if (cancelled) return;
        setTables(schema.tables ?? []);
        const init: Record<string, boolean> = {};
        (schema.tables ?? []).slice(0, 3).forEach((tb) => (init[tb.name] = true));
        setExpanded(init);
      })
      .catch(() => !cancelled && setFailed(true));
    return () => {
      cancelled = true;
    };
  }, [api, db]);

  return (
    <div className="p-3">
      <div className="mb-2 flex items-center gap-1.5">
        <Icon name="Database" className="h-3.5 w-3.5 text-ink-500" />
        <span className="text-[10.5px] font-medium tracking-[0.08em] text-ink-400 uppercase">{name}</span>
        <span className="ml-auto font-mono text-[10px] text-ink-600">{tables.length}</span>
        <Tooltip content={t("sql.refreshSchema")} side="top">
          <button
            onClick={() => {
              void api.inspectDatabase(db).then((s) => setTables(s.tables ?? [])).catch(() => setFailed(true));
            }}
            aria-label={t("sql.refreshSchema")}
            className="grid h-5 w-5 place-items-center rounded text-ink-500 hover:bg-ink-750 hover:text-ink-100"
          >
            <Icon name="RefreshCw" className="h-3 w-3" />
          </button>
        </Tooltip>
      </div>

      {failed && <p className="px-1 text-[10.5px] text-rose-300">{t("sql.schemaFailed")}</p>}

      {tables.map((tbl) => {
        const open = expanded[tbl.name] ?? false;
        return (
          <div key={tbl.name} className="mb-1">
            <button
              onClick={() => setExpanded((e) => ({ ...e, [tbl.name]: !e[tbl.name] }))}
              className="flex w-full items-center gap-1.5 rounded px-1 py-[3px] text-left transition hover:bg-ink-850"
            >
              <Icon name="ChevronRight" className={cn("h-3 w-3 text-ink-600 transition-transform", open && "rotate-90 text-ink-400")} />
              <Icon name="Table2" className="h-3 w-3 text-ink-500" />
              <span className="font-mono text-[11px] text-ink-100">{tbl.name}</span>
              <span className="ml-auto font-mono text-[9px] text-ink-600">{tbl.columns.length}</span>
            </button>

            {open && (
              <div className="ml-[18px] border-l border-ink-800 pl-2">
                {tbl.columns.map((col) => (
                  <div key={col.name} className="flex w-full items-center gap-1.5 rounded px-1 py-[2px]">
                    {col.primaryKey && <Icon name="KeyRound" className="h-2.5 w-2.5 shrink-0 text-amber-400/70" />}
                    {!col.primaryKey && <span className="h-2.5 w-2.5 shrink-0" />}
                    <span className="min-w-0 flex-1 truncate font-mono text-[10.5px] text-ink-300">{col.name}</span>
                    <span className="shrink-0 font-mono text-[9px] text-ink-600">{col.dataType}</span>
                  </div>
                ))}
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}

/* ================================================================== */
/*  SQL debug results panel                                            */
/* ================================================================== */

function ResultsPanel({
  result,
  running,
  open,
  onToggle,
  onClose,
}: {
  result: SQLResult | null;
  running: boolean;
  open: boolean;
  onToggle: () => void;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const hasError = false; // backend errors surface through the error bar

  return (
    <div className="shrink-0 border-t border-seam bg-ink-900">
      <button onClick={onToggle} className="flex h-8 w-full items-center gap-2 px-4 text-left transition hover:bg-ink-850/60">
        <Icon name="ChevronRight" className={cn("h-3 w-3 text-ink-500 transition-transform", open && "rotate-90")} />
        <Icon
          name={running ? "Loader2" : hasError ? "AlertTriangle" : "Table2"}
          className={cn("h-3.5 w-3.5", running && "animate-spin text-ink-400", hasError ? "text-rose-400" : "text-ink-500")}
        />
        <span className="text-[11px] font-medium text-ink-200">
          {running ? t("codeAssistant.thinking") : t("sql.run")}
        </span>
        {!running && result && !hasError && (
          <span className="text-[10.5px] text-ink-500">
            {result.rowsAffected > 0
              ? t("sql.rowsAffected", { count: result.rowsAffected })
              : `${result.rows.length}${result.truncated ? "+" : ""}`}
          </span>
        )}
        {!running && result && (
          <button
            onClick={(e) => {
              e.stopPropagation();
              onClose();
            }}
            className="ml-auto grid h-5 w-5 place-items-center rounded text-ink-500 hover:bg-ink-750 hover:text-ink-100"
          >
            <Icon name="X" className="h-3 w-3" />
          </button>
        )}
      </button>

      {open && result && !hasError && (
        <div className="max-h-[220px] overflow-auto border-t border-seam">
          {result.rowsAffected > 0 && result.columns.length === 0 ? (
            <p className="px-4 py-2 text-[11.5px] text-ink-400">{t("sql.rowsAffected", { count: result.rowsAffected })}</p>
          ) : (
            <table className="w-full border-collapse text-left">
              <thead>
                <tr className="border-b border-seam">
                  {result.columns.map((c) => (
                    <th key={c} className="whitespace-nowrap px-3 py-1.5 font-mono text-[10.5px] font-medium text-ink-300">
                      {c}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {result.rows.map((row, i) => (
                  <tr key={i} className="border-b border-seam/60 last:border-b-0 hover:bg-ink-850/60">
                    {result.columns.map((c) => (
                      <td key={c} className="max-w-[280px] truncate whitespace-nowrap px-3 py-1 font-mono text-[10.5px] text-ink-200">
                        {format_cell(row[c])}
                      </td>
                    ))}
                  </tr>
                ))}
                {result.rows.length === 0 && (
                  <tr>
                    <td colSpan={Math.max(result.columns.length, 1)} className="px-3 py-2 text-[11.5px] text-ink-500">
                      {t("sql.noResult")}
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          )}
        </div>
      )}
    </div>
  );
}

function format_cell(value: unknown): string {
  if (value === null || value === undefined) return "NULL";
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
}

/* ================================================================== */
/*  LLM code assistant                                                 */
/* ================================================================== */

function AssistantModal({
  kind,
  code,
  selDB,
  api,
  onClose,
  onApply,
}: {
  kind: "js" | "sql";
  code: string;
  selDB: string;
  api: Pick<EditorApi, "generateCode">;
  onClose: () => void;
  onApply: (code: string) => void;
}) {
  const { t } = useTranslation();
  const [prompt, setPrompt] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const generate = async () => {
    if (!prompt.trim() || busy) return;
    setBusy(true);
    setError(null);
    try {
      const request: CodeGenerationRequest = {
        editorType: kind === "js" ? "javascript" : "sql",
        prompt,
        currentCode: code,
        sqlContext: kind === "sql" ? { databaseName: selDB, parameters: [] } : undefined,
      };
      const response = await api.generateCode(request);
      if (!response.code) {
        setError(t("codeAssistant.empty"));
        setBusy(false);
        return;
      }
      onApply(response.code);
    } catch (err) {
      setError(err instanceof Error ? err.message : t("codeAssistant.failed"));
      setBusy(false);
    }
  };

  return createPortal(
    <div className="fixed inset-0 z-[80] flex items-center justify-center bg-black/70 p-4 backdrop-blur-[3px]" onClick={onClose}>
      <div
        className="pop-in w-[520px] max-w-[94vw] space-y-3 rounded-xl border border-ink-650 bg-ink-900 p-4 shadow-[0_40px_100px_-30px_rgba(0,0,0,0.95)]"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center gap-2">
          <Icon name="Sparkles" className="h-4 w-4 text-violet-300" />
          <h3 className="text-[13px] font-semibold text-ink-50">{t("codeAssistant.title")}</h3>
        </div>
        <p className="text-[12px] leading-relaxed text-ink-400">{t("codeAssistant.description")}</p>
        <textarea
          rows={4}
          autoFocus
          value={prompt}
          onChange={(e) => setPrompt(e.target.value)}
          onKeyDown={(e) => {
            if ((e.metaKey || e.ctrlKey) && e.key === "Enter") void generate();
          }}
          placeholder={t("codeAssistant.placeholder")}
          className={control.textarea}
        />
        {error && <p className="text-[11.5px] text-rose-300">{error}</p>}
        <div className="flex justify-end gap-2">
          <button onClick={onClose} className="h-7 rounded-md border border-ink-700 bg-ink-850 px-3 text-[11.5px] text-ink-200 transition hover:bg-ink-750">
            {t("common.cancel")}
          </button>
          <button
            onClick={() => void generate()}
            disabled={busy || !prompt.trim()}
            className={cn(
              "h-7 rounded-md px-3 text-[11.5px] font-medium transition",
              busy || !prompt.trim()
                ? "cursor-not-allowed bg-ink-800 text-ink-500"
                : "bg-ink-50 text-ink-950 hover:bg-white",
            )}
          >
            {busy ? t("codeAssistant.thinking") : t("codeAssistant.generate")}
          </button>
        </div>
      </div>
    </div>,
    document.body,
  );
}














