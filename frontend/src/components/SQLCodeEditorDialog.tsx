import { useEffect, useRef, useState } from "react";
import { ChevronRight, CircleHelp, Columns3, Database as DatabaseIcon, Loader2, Play, Plus, RefreshCw, Sparkles, Table2, Trash2 } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Tooltip } from "@/components/ui/tooltip";
import { LLMCodeAssistantDialog } from "@/components/LLMCodeAssistantDialog";
import { desktop } from "@/lib/bridge";
import { loadMonaco } from "@/lib/monaco";
import type { Database, DatabaseSchema, SQLParameter, SQLResult, TypeKind } from "@/lib/types";

const typeKinds: TypeKind[] = ["any", "string", "int", "float", "bool", "bytes"];
const placeholderPattern = /(?:^|[^:\w])[:@$]([A-Za-z_][A-Za-z0-9_]*)/g;
const identifierPattern = /^[A-Za-z_][A-Za-z0-9_]*$/;

function parametersFromConfig(value: unknown): SQLParameter[] {
  if (!Array.isArray(value)) return [];
  return value.flatMap((entry) => {
    if (!entry || typeof entry !== "object" || Array.isArray(entry)) return [];
    const item = entry as Record<string, unknown>;
    if (typeof item.id !== "string" || typeof item.name !== "string") return [];
    const type = item.type && typeof item.type === "object" && !Array.isArray(item.type) ? item.type as SQLParameter["type"] : { kind: "any" as const };
    return [{ id: item.id, name: item.name, label: typeof item.label === "string" ? item.label : item.name, type, required: Boolean(item.required) }];
  });
}

function nextParameter(parameters: SQLParameter[]): SQLParameter {
  const ids = new Set(parameters.map((item) => item.id));
  let index = parameters.length + 1;
  while (ids.has(`parameter${index}`)) index += 1;
  return { id: `parameter${index}`, name: `parameter${index}`, label: `parameter${index}`, type: { kind: "any" }, required: false };
}

function debugValue(raw: string, kind: TypeKind): unknown {
  if (kind === "int" || kind === "float") return raw === "" ? null : Number(raw);
  if (kind === "bool") return raw === "true";
  if (kind === "any") { try { return JSON.parse(raw); } catch { return raw; } }
  return raw;
}

function errorMessage(reason: unknown, fallback: string): string {
  if (reason instanceof Error && reason.message) return reason.message;
  if (typeof reason === "string" && reason) return reason;
  if (reason && typeof reason === "object" && "message" in reason && typeof reason.message === "string" && reason.message) return reason.message;
  return fallback;
}

function FieldLabel({ label, description }: { label: string; description: string }) {
  return <span className="mb-1 flex items-center gap-1 text-[10px] font-medium uppercase tracking-wide text-zinc-600"><span>{label}</span><Tooltip content={description} side="top" align="start" size="body" className="w-56 px-2.5 py-2 text-zinc-300"><button type="button" className="flex size-4 items-center justify-center rounded-full text-zinc-600 hover:bg-zinc-800 hover:text-zinc-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-zinc-500" aria-label={description}><CircleHelp className="size-3" /></button></Tooltip></span>;
}

export function SQLCodeEditorDialog({ open, config, onClose, onSave }: { open: boolean; config: Record<string, unknown>; onClose: () => void; onSave: (config: Record<string, unknown>) => void }) {
  const { t } = useTranslation();
  const [databases, setDatabases] = useState<Database[]>([]);
  const [databaseId, setDatabaseId] = useState(typeof config.databaseId === "string" ? config.databaseId : "");
  const [sql, setSQL] = useState(typeof config.sql === "string" ? config.sql : "");
  const [parameters, setParameters] = useState(() => parametersFromConfig(config.parameters));
  const [debugValues, setDebugValues] = useState<Record<string, string>>({});
  const [schema, setSchema] = useState<DatabaseSchema>();
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const [result, setResult] = useState<SQLResult>();
  const [error, setError] = useState("");
  const [running, setRunning] = useState(false);
  const [refreshingSchema, setRefreshingSchema] = useState(false);
  const [aiOpen, setAIOpen] = useState(false);
  const editorElement = useRef<HTMLDivElement>(null);
  const editorRef = useRef<import("monaco-editor").editor.IStandaloneCodeEditor>();
  const sqlRef = useRef(sql);
  const translateRef = useRef(t);
  useEffect(() => { sqlRef.current = sql; }, [sql]);
  useEffect(() => { translateRef.current = t; }, [t]);
  useEffect(() => { void desktop.listDatabases().then(setDatabases).catch((reason) => setError(reason instanceof Error ? reason.message : t("sql.loadFailed"))); }, [t]);
  const refreshSchema = async () => {
    if (!databaseId) {
      setSchema(undefined);
      return;
    }
    setRefreshingSchema(true);
    try {
      setSchema(await desktop.inspectDatabase(databaseId));
    } catch (reason) {
      setError(errorMessage(reason, t("sql.schemaFailed")));
    } finally {
      setRefreshingSchema(false);
    }
  };
  useEffect(() => { void refreshSchema(); }, [databaseId]);
  useEffect(() => {
    if (!open || !editorElement.current) return;
    let cancelled = false;
    let editor: import("monaco-editor").editor.IStandaloneCodeEditor | undefined;
    void loadMonaco().then((monaco) => {
      if (cancelled || !editorElement.current) return;
      monaco.editor.defineTheme("neuropipe-sql", { base: "vs-dark", inherit: true, rules: [], colors: { "editor.background": "#09090b", "editorGutter.background": "#09090b" } });
      editor = monaco.editor.create(editorElement.current, { value: sqlRef.current, language: "sql", theme: "neuropipe-sql", automaticLayout: true, minimap: { enabled: false }, fontSize: 13, lineHeight: 21, padding: { top: 12, bottom: 12 }, scrollBeyondLastLine: false, wordWrap: "on", editContext: false });
      editor.onDidChangeModelContent(() => setSQL(editor?.getValue() ?? ""));
      editorRef.current = editor;
      editor.focus();
    }).catch(() => setError(translateRef.current("sql.editorUnavailable")));
    return () => { cancelled = true; editor?.dispose(); };
  }, [open]);
  const updateParameter = (index: number, change: Partial<SQLParameter>) => setParameters((current) => current.map((item, itemIndex) => itemIndex === index ? { ...item, ...change } : item));
  const run = async () => {
    setRunning(true); setError(""); setResult(undefined);
    try {
      setResult(await desktop.debugDatabase({ databaseId, sql, parameters: parameters.map((item) => ({ name: item.name, value: debugValue(debugValues[item.id] ?? "", item.type.kind) })), maxRows: 200 }));
      await refreshSchema();
    }
    catch (reason) { setError(errorMessage(reason, t("sql.runFailed"))); }
    finally { setRunning(false); }
  };
  const placeholderNames = [...sql.matchAll(placeholderPattern)].map((match) => match[1]);
  const missingPlaceholders = [...new Set(placeholderNames.filter((name) => !parameters.some((item) => item.name === name)))];
  const parameterError = (() => {
    const ids = new Set<string>();
    const names = new Set<string>();
    for (const parameter of parameters) {
      if (!identifierPattern.test(parameter.id) || !identifierPattern.test(parameter.name)) return t("sql.invalidParameter");
      if (ids.has(parameter.id) || names.has(parameter.name)) return t("sql.duplicateParameter");
      ids.add(parameter.id);
      names.add(parameter.name);
    }
    return missingPlaceholders.length > 0 ? t("sql.missingParameters", { names: missingPlaceholders.join(", ") }) : "";
  })();
  return <Dialog open={open} title={t("sql.title")} description={t("sql.description")} onOpenChange={(next) => { if (!next && !running) onClose(); }} className="h-[min(920px,calc(100vh-40px))] max-w-[min(1440px,calc(100vw-40px))]">
    <div className="flex items-center gap-3 border-b border-zinc-800 px-4 py-2.5"><span className="text-xs text-zinc-500">{t("sql.database")}</span><Select value={databaseId} onValueChange={setDatabaseId} options={databases.map((item) => ({ value: item.id, label: item.name }))} placeholder={t("sql.selectDatabase")} ariaLabel={t("sql.database")} /><Button size="sm" variant="ghost" className="ml-auto h-7 px-2 text-xs text-violet-300 hover:text-violet-200" onClick={() => setAIOpen(true)}><Sparkles className="size-3.5" />{t("codeAssistant.title", "AI Code Assistant")}</Button></div>
    <div className="grid min-h-0 flex-1 grid-cols-[220px_minmax(0,1fr)_340px]">
      <aside className="muted-scroll min-h-0 overflow-y-auto border-r border-zinc-800 p-3"><div className="mb-3 flex items-center justify-between gap-2"><h3 className="flex items-center gap-2 text-xs font-medium text-zinc-300"><DatabaseIcon className="size-3.5" />{t("sql.schema")}</h3><Button size="sm" variant="ghost" className="size-7 p-0" onClick={() => void refreshSchema()} disabled={!databaseId || refreshingSchema} aria-label={t("sql.refreshSchema")}><RefreshCw className={`size-3.5 ${refreshingSchema ? "animate-spin" : ""}`} /></Button></div>{schema?.tables.length ? <div className="space-y-1">{schema.tables.map((table) => <div key={table.name}><button type="button" className="flex w-full items-center gap-1.5 rounded px-1.5 py-1 text-left text-xs text-zinc-300 hover:bg-zinc-900" onClick={() => setExpanded((current) => { const next = new Set(current); next.has(table.name) ? next.delete(table.name) : next.add(table.name); return next; })}><ChevronRight className={`size-3 transition-transform ${expanded.has(table.name) ? "rotate-90" : ""}`} /><Table2 className="size-3.5 text-emerald-300" /><span className="truncate">{table.name}</span></button>{expanded.has(table.name) ? <div className="ml-5 space-y-0.5 py-1">{table.columns.map((column) => <div key={column.name} className="flex items-center gap-1.5 px-1 py-0.5 text-[11px] text-zinc-500"><Columns3 className="size-3" /><span className="min-w-0 flex-1 truncate">{column.name}</span><span className="truncate text-zinc-700">{column.dataType}</span></div>)}</div> : null}</div>)}</div> : <p className="text-xs leading-5 text-zinc-600">{databaseId ? t("sql.noTables") : t("sql.selectDatabaseHint")}</p>}</aside>
      <div className="flex min-h-0 flex-col"><div ref={editorElement} className="min-h-48 flex-1" /><div className="max-h-64 min-h-24 overflow-auto border-t border-zinc-800">{error ? <p className="p-3 font-mono text-xs text-red-300">{error}</p> : result ? <ResultTable result={result} /> : <p className="flex h-full min-h-24 items-center justify-center text-xs text-zinc-600">{t("sql.noResult")}</p>}</div></div>
       <aside className="muted-scroll min-h-0 overflow-y-auto border-l border-zinc-800 bg-zinc-950/40 p-3"><div className="mb-3 flex items-center justify-between gap-3"><div><h3 className="text-xs font-semibold text-zinc-200">{t("sql.parameters")}</h3><p className="mt-0.5 text-[10px] text-zinc-600">{t("sqlHelp.parameters")}</p></div><Button size="sm" variant="outline" className="h-7 shrink-0 px-2 text-[11px]" onClick={() => setParameters((current) => [...current, nextParameter(current)])}><Plus className="size-3.5" />{t("sql.addParameter")}</Button></div>{missingPlaceholders.length ? <p className="mb-3 rounded-md border border-amber-500/20 bg-amber-500/5 p-2 text-[11px] leading-4 text-amber-200">{t("sql.missingParameters", { names: missingPlaceholders.join(", ") })}</p> : null}<div className="space-y-2.5">{parameters.map((item, index) => <div key={item.id} className="rounded-lg border border-zinc-800 bg-zinc-900/45 p-2.5 shadow-inner shadow-black/10"><div className="mb-2 flex items-center justify-between gap-2"><span className="flex min-w-0 items-center gap-1.5 font-mono text-[11px] text-emerald-300"><span className="text-zinc-600">$</span><span className="truncate">{item.name || t("sql.parameterName")}</span></span><Button size="sm" variant="ghost" className="size-6 shrink-0 p-0 text-zinc-500 hover:text-red-300" onClick={() => setParameters((current) => current.filter((_, itemIndex) => itemIndex !== index))} aria-label={t("sql.removeParameter", { name: item.name })}><Trash2 className="size-3.5" /></Button></div><div className="space-y-1.5"><label className="block"><FieldLabel label={t("sql.parameterName")} description={t("sqlHelp.parameterName")} /><Input className="h-8 font-mono text-xs" value={item.name} onChange={(event) => updateParameter(index, { name: event.target.value })} placeholder={t("sql.parameterName")} aria-label={t("sql.parameterName")} /></label><label className="block"><FieldLabel label={t("sql.pinId")} description={t("sqlHelp.pinId")} /><Input className="h-8 font-mono text-xs" value={item.id} onChange={(event) => updateParameter(index, { id: event.target.value })} placeholder={t("sql.pinId")} aria-label={t("sql.pinId")} /></label><label className="block"><FieldLabel label={t("sql.label")} description={t("sqlHelp.label")} /><Input className="h-8 text-xs" value={item.label} onChange={(event) => updateParameter(index, { label: event.target.value })} placeholder={t("sql.label")} aria-label={t("sql.label")} /></label><div className="grid grid-cols-[minmax(0,1fr)_auto] items-end gap-2 pt-1"><label className="min-w-0"><FieldLabel label={t("sql.type")} description={t("sqlHelp.type")} /><Select className="w-full" value={item.type.kind} onValueChange={(kind) => updateParameter(index, { type: { kind: kind as TypeKind } })} options={typeKinds.map((kind) => ({ value: kind, label: t(`sql.types.${kind}`) }))} ariaLabel={t("sql.type")} /></label><div><FieldLabel label={t("sql.required")} description={t("sqlHelp.required")} /><div className="flex h-8 items-center justify-center rounded-md border border-zinc-800 bg-zinc-950/50 px-2"><Switch checked={Boolean(item.required)} onCheckedChange={(required) => updateParameter(index, { required })} label={t("sql.required")} /></div></div></div><label className="block border-t border-zinc-800/80 pt-2"><FieldLabel label={t("sql.debugValue")} description={t("sqlHelp.debugValue")} /><Input className="h-8 font-mono text-xs" value={debugValues[item.id] ?? ""} onChange={(event) => setDebugValues((current) => ({ ...current, [item.id]: event.target.value }))} placeholder={t("sql.debugValue")} aria-label={t("sql.debugValueFor", { name: item.name })} /></label></div></div>)}</div>{parameters.length === 0 ? <p className="rounded-md border border-dashed border-zinc-800 px-3 py-4 text-center text-[11px] leading-4 text-zinc-600">{t("sqlHelp.noParameters")}</p> : null}</aside>
    </div>
    <div className="flex items-center justify-between border-t border-zinc-800 px-5 py-3"><Button variant="outline" onClick={() => void run()} disabled={running || !databaseId || !sql.trim() || Boolean(parameterError)}>{running ? <Loader2 className="size-3.5 animate-spin" /> : <Play className="size-3.5" />}{t("sql.run")}</Button><div className="flex min-w-0 items-center gap-2">{parameterError ? <span className="max-w-sm truncate text-xs text-amber-300">{parameterError}</span> : null}<Button variant="ghost" onClick={onClose} disabled={running}>{t("common.cancel")}</Button><Button onClick={() => { onSave({ ...config, databaseId, sql, parameters }); onClose(); }} disabled={!databaseId || !sql.trim() || Boolean(parameterError)}>{t("sql.save")}</Button></div></div>
    <LLMCodeAssistantDialog
      open={aiOpen}
      request={{
        editorType: "sql",
        currentCode: sql,
        sqlContext: {
          databaseName: databases.find((db) => db.id === databaseId)?.name ?? "",
          schema,
          parameters: parameters.map((p) => ({ name: p.name, type: p.type.kind })),
        },
      }}
      onApply={(generatedSQL) => {
        setSQL(generatedSQL);
        sqlRef.current = generatedSQL;
        if (editorRef.current) { editorRef.current.setValue(generatedSQL); }
      }}
      onClose={() => setAIOpen(false)}
    />
  </Dialog>;
}

function ResultTable({ result }: { result: SQLResult }) {
  const { t } = useTranslation();
  if (!result.columns.length) return <p className="p-3 text-xs text-zinc-400">{t("sql.rowsAffected", { count: result.rowsAffected })}{result.lastInsertId !== undefined ? ` · ${t("sql.lastInsertId", { id: result.lastInsertId })}` : ""}</p>;
  return <table className="w-full border-collapse text-left font-mono text-xs"><thead className="sticky top-0 bg-zinc-900"><tr>{result.columns.map((column) => <th key={column} className="border-b border-r border-zinc-800 px-2 py-1.5 font-medium text-zinc-300">{column}</th>)}</tr></thead><tbody>{result.rows.map((row, index) => <tr key={index}>{result.columns.map((column) => <td key={column} className="max-w-80 truncate border-b border-r border-zinc-900 px-2 py-1.5 text-zinc-500">{row[column] === null ? "NULL" : typeof row[column] === "object" ? JSON.stringify(row[column]) : String(row[column] ?? "")}</td>)}</tr>)}</tbody></table>;
}
