import { useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import type { GraphNode, LogEntry, Port } from "@/types";
import type { Execution } from "@/lib/types";
import type { EditorApi } from "@/features/graph/PipelineEditor";
import { Icon } from "./icons";
import { Badge, Dot, Empty, Toggle } from "./ui";
import { Dropdown, type DropdownOption } from "./Dropdown";
import { Tooltip } from "./Tooltip";
import { TextEditorModal } from "./TextEditorModal";
import { CodeEditorModal } from "./CodeEditorModal";
import { JsonViewerModal } from "./JsonViewerModal";
import { FormBuilderEditor } from "./FormBuilderEditor";
import { DrawImageEditor } from "./DrawImageEditor";
import { EmbedEditor } from "./EmbedEditor";
import { RouteOptionsEditor, SchemaEditor, SwitchCasesEditor } from "./StructuredFieldEditors";
import { HtmlExtractionsEditor } from "./HtmlExtractionsEditor";
import { HeadersEditor } from "./HeadersEditor";
import { KVArgumentsEditor, KVHashFieldsEditor, KVScoredEntriesEditor, KVStringListEditor } from "./KVFieldEditors";
import { TypeSpecField, specTopToken, tokenToPinDataType } from "./TypeSpecField";
import { typeSpecFromDataType } from "../lib/type-spec";
import { unmapDataType } from "../lib/adapters";
import { formatDuration } from "@/lib/format";
import type { TypeSpec } from "../lib/types";
import { control } from "./primitives/styles";
import { cn } from "../utils/cn";

/* ---------- small building blocks ---------- */

function Label({ text, required }: { text: string; required?: boolean }) {
  return (
    <span className="flex items-center gap-1 text-[11.5px] font-medium text-fg-subtle">
      {text}
      {required && <span className="text-fg-faint">*</span>}
    </span>
  );
}

const inputCls = control.textarea;

function Section({
  title,
  children,
  right,
  defaultOpen = true,
}: {
  title: string;
  children: React.ReactNode;
  right?: React.ReactNode;
  defaultOpen?: boolean;
}) {
  const [open, setOpen] = useState(defaultOpen);
  return (
    <div className="border-b border-seam">
      <button
        onClick={() => setOpen(!open)}
        className="flex w-full items-center gap-1.5 px-3 py-2 text-left transition hover:bg-ink-850"
      >
        <Icon
          name="ChevronRight"
          className={cn("h-3 w-3 text-fg-faint transition-transform", open && "rotate-90 text-fg-subtle")}
        />
        <span className="text-[10.5px] font-medium tracking-[0.09em] text-fg-subtle uppercase">{title}</span>
        <span className="ml-auto flex items-center gap-1">{right}</span>
      </button>
      {open && <div className="px-3 pt-0.5 pb-3">{children}</div>}
    </div>
  );
}

type FunctionKind = "pure" | "impure" | "tool";

/* ---------- inspector ---------- */

export function Inspector({
  node,
  log,
  api,
  onChange,
  onPortsChange,
  onFunctionKindChange,
}: {
  node: GraphNode | null;
  log: LogEntry[];
  api: EditorApi;
  onChange: (key: string, value: unknown) => void;
  onPortsChange?: (nodeId: string, inputs: Port[], outputs: Port[]) => void;
  onFunctionKindChange?: (kind: FunctionKind) => void;
}) {
  const { t } = useTranslation();
  const [tab, setTab] = useState<"inspect" | "log">("inspect");

  useEffect(() => setTab("inspect"), [node?.id]);

  return (
    <div className="flex min-h-0 flex-1 flex-col bg-ink-900">
      {/* tabs */}
      <div className="flex h-9 shrink-0 items-center gap-0.5 border-b border-seam px-1.5">
        {(
          [
            ["inspect", t("editorActions.inspector"), "Settings2"],
            ["log", t("editorActions.executionLog"), "History"],
          ] as const
        ).map(([id, label, icon]) => (
          <button
            key={id}
            onClick={() => setTab(id)}
            className={cn(
              "flex h-7 items-center gap-1.5 rounded-md px-2.5 text-[12px] font-medium transition",
              tab === id ? "bg-ink-750 text-fg" : "text-fg-subtle hover:bg-ink-850 hover:text-fg-muted",
            )}
          >
            <Icon name={icon} className="h-3.5 w-3.5" />
            {label}
          </button>
        ))}
        <Tooltip content={t("docs.open")} side="bottom">
          <button
            className="ml-auto grid h-7 w-7 place-items-center rounded-md text-fg-subtle transition hover:bg-ink-850 hover:text-fg"
            aria-label={t("docs.open")}
            onClick={() => api.openDocs(node?.type)}
          >
            <Icon name="BookOpen" className="h-[15px] w-[15px]" />
          </button>
        </Tooltip>
      </div>

      {tab === "inspect" ? (
        node ? (
          <InspectBody
            node={node}
            functionKind={(node.values.functionKind as FunctionKind) ?? "impure"}
            api={api}
            onChange={onChange}
            onPortsChange={onPortsChange}
            onFunctionKindChange={(k) => {
              onChange("functionKind", k);
              onFunctionKindChange?.(k);
            }}
            />
        ) : (
          <div className="flex flex-1 items-center justify-center">
            <Empty icon="MousePointer2" text={t("editor.selectNode")} />
          </div>
        )
      ) : (
        <LogBody log={log} executions={api.executions} onSelect={api.onLoadExecution} />
      )}
    </div>
  );
}

const KIND_META: Record<FunctionKind, { labelKey: string; descKey: string; icon: string }> = {
  pure: { labelKey: "functions.types.pure.title", descKey: "editor.kindPure", icon: "Sparkles" },
  impure: { labelKey: "functions.types.workflow.title", descKey: "editor.kindImpure", icon: "Zap" },
  tool: { labelKey: "functions.types.tool.title", descKey: "editor.kindTool", icon: "Bot" },
};

/* ---------- function boundary editor ---------- */

function FunctionPortsEditor({
  node,
  kind,
  onPortsChange,
  onKindChange,
}: {
  node: GraphNode;
  kind: FunctionKind;
  onPortsChange?: (nodeId: string, inputs: Port[], outputs: Port[]) => void;
  onKindChange?: (kind: FunctionKind) => void;
}) {
  const { t } = useTranslation();
  const isEntry = node.type === "function:entry";
  const execPins = (isEntry ? node.outputs : node.inputs).filter((p) => p.kind === "exec");
  const editable = (isEntry ? node.outputs : node.inputs).filter((p) => p.kind !== "exec");

  const commit = (ports: Port[]) => {
    if (!onPortsChange) return;
    if (isEntry) onPortsChange(node.id, node.inputs, [...execPins, ...ports]);
    else onPortsChange(node.id, [...execPins, ...ports], node.outputs);
  };

  const rename = (id: string, label: string) =>
    commit(editable.map((p) => (p.id === id ? { ...p, label } : p)));
  const changeType = (id: string, spec: TypeSpec) =>
    commit(
      editable.map((p) =>
        p.id === id
          ? { ...p, spec, dataType: tokenToPinDataType(specTopToken(spec)) }
          : p,
      ),
    );
  const remove = (id: string) => commit(editable.filter((p) => p.id !== id));
  const setDescription = (id: string, description: string) =>
    commit(editable.map((p) => (p.id === id ? { ...p, description } : p)));
  const setRequired = (id: string, required: boolean) =>
    commit(editable.map((p) => (p.id === id ? { ...p, required } : p)));
  const add = () => {
    const next = `pin_${Math.random().toString(36).slice(2, 7)}`;
    commit([
      ...editable,
      {
        id: next,
        label: next,
        kind: "data",
        dataType: "text",
        spec: { kind: "string" },
        required: true,
        description: "",
      },
    ]);
  };

  return (
    <>
      <Section title={t("editor.functionType")}>
        <div className="grid grid-cols-3 gap-1.5">
          {(Object.keys(KIND_META) as FunctionKind[]).map((k) => {
            const meta = KIND_META[k];
            const active = k === kind;
            return (
              <button
                key={k}
                onClick={() => onKindChange?.(k)}
                className={cn(
                  "flex flex-col items-start gap-1 rounded-md border p-2 text-left transition",
                  active
                    ? "border-ink-400 bg-ink-750/80 text-fg"
                    : "border-ink-700 bg-ink-850 text-fg-subtle hover:border-ink-500 hover:bg-ink-750 hover:text-fg",
                )}
              >
                <span className="flex items-center gap-1.5">
                  <Icon name={meta.icon} className="h-3 w-3" />
                  <span className="text-[12px] font-medium">{t(meta.labelKey)}</span>
                </span>
                <span className="line-clamp-2 text-[10.5px] leading-tight text-fg-faint">{t(meta.descKey)}</span>
              </button>
            );
          })}
        </div>
      </Section>

      {kind === "tool" && (
        <Section
          title={t(isEntry ? "editor.toolInputsTitle" : "editor.toolOutputsTitle")}
          right={<span className="font-mono text-[10px] text-fg-faint">{editable.length}</span>}
        >
          <p className="mb-2 text-[11.5px] leading-relaxed text-fg-faint">
            {t(isEntry ? "editor.toolInputsHint" : "editor.toolOutputsHint")}
          </p>
          <div className="space-y-2">
            {editable.map((p) => (
              <div key={p.id} className="space-y-1.5 rounded-md border border-ink-700/70 bg-ink-850/50 p-2">
                <div className="flex items-center gap-1.5">
                  <input
                    value={p.label}
                    onChange={(e) => rename(p.id, e.target.value)}
                    placeholder={t("sql.parameterName")}
                    className="h-7 min-w-0 flex-1 rounded-md border border-ink-700 bg-ink-900 px-2 font-mono text-[12px] text-fg"
                  />
                  {isEntry && (
                    <span className="flex shrink-0 items-center gap-1.5 text-[10.5px] text-fg-subtle">
                      {t("sql.required")}
                      <Toggle on={p.required ?? false} onChange={(v) => setRequired(p.id, v)} />
                    </span>
                  )}
                  <button
                    onClick={() => remove(p.id)}
                    aria-label={t("common.delete")}
                    className="grid h-7 w-7 shrink-0 place-items-center rounded-md text-fg-faint hover:bg-danger/15 hover:text-danger-fg"
                  >
                    <Icon name="Trash2" className="h-3.5 w-3.5" />
                  </button>
                </div>
                <TypeSpecField
                  value={p.spec ?? typeSpecFromDataType(unmapDataType(p.dataType ?? "any"))}
                  onChange={(spec) => changeType(p.id, spec)}
                  allowAny={false}
                />
                <textarea
                  rows={2}
                  value={p.description ?? ""}
                  onChange={(e) => setDescription(p.id, e.target.value)}
                  placeholder={t("functionEditor.pinGuidancePlaceholder")}
                  className={cn(control.textarea, "resize-y text-[11.5px] leading-relaxed")}
                />
              </div>
            ))}
            {editable.length === 0 && (
              <p className="rounded-md border border-dashed border-ink-700 px-2.5 py-2 text-[11px] text-fg-faint">
                {t(isEntry ? "editor.noInputPins" : "editor.noOutputPins")}
              </p>
            )}
          </div>
          <button
            onClick={add}
            className="mt-2 flex h-7 items-center gap-1.5 rounded-md border border-ink-700 bg-ink-850 px-2 text-[11.5px] text-fg-muted hover:bg-ink-750"
          >
            <Icon name="Plus" className="h-3.5 w-3.5" />
            {t(isEntry ? "editor.addInputPin" : "editor.addOutputPin")}
          </button>
        </Section>
      )}

      {kind !== "tool" && (
        <Section
          title={isEntry ? t("editor.inputPins") : t("editor.outputPins")}
          right={<span className="font-mono text-[10px] text-fg-faint">{editable.length}</span>}
        >
          <div className="space-y-2">
            {editable.map((p) => (
              <div key={p.id} className="space-y-1 rounded-md border border-ink-700/70 bg-ink-850/40 p-1.5">
                <div className="flex items-center gap-1.5">
                  <input
                    value={p.label}
                    onChange={(e) => rename(p.id, e.target.value)}
                    className="h-7 min-w-0 flex-1 rounded-md border border-ink-700 bg-ink-850 px-2 text-[12px] text-fg"
                  />
                  <button
                    onClick={() => remove(p.id)}
                    aria-label={t("common.delete")}
                    className="grid h-7 w-7 shrink-0 place-items-center rounded-md text-fg-faint hover:bg-danger/15 hover:text-danger-fg"
                  >
                    <Icon name="Trash2" className="h-3.5 w-3.5" />
                  </button>
                </div>
                <TypeSpecField
                  value={p.spec ?? typeSpecFromDataType(unmapDataType(p.dataType ?? "any"))}
                  onChange={(spec) => changeType(p.id, spec)}
                />
              </div>
            ))}
            {editable.length === 0 && (
              <p className="rounded-md border border-dashed border-ink-700 px-2.5 py-2 text-[11px] text-fg-faint">
                {t(isEntry ? "editor.noInputPins" : "editor.noOutputPins")}
              </p>
            )}
          </div>
          <button
            onClick={add}
            className="mt-2 flex h-7 items-center gap-1.5 rounded-md border border-ink-700 bg-ink-850 px-2 text-[11.5px] text-fg-muted hover:bg-ink-750"
          >
            <Icon name="Plus" className="h-3.5 w-3.5" />
            {t(isEntry ? "editor.addInputPin" : "editor.addOutputPin")}
          </button>
        </Section>
      )}
    </>
  );
}

/* ---------- main body ---------- */

function InspectBody({
  node,
  functionKind,
  api,
  onChange,
  onPortsChange,
  onFunctionKindChange,
}: {
  node: GraphNode;
  functionKind: FunctionKind;
  api: EditorApi;
  onChange: (key: string, value: unknown) => void;
  onPortsChange?: (nodeId: string, inputs: Port[], outputs: Port[]) => void;
  onFunctionKindChange?: (kind: FunctionKind) => void;
}) {
  const { t } = useTranslation();
  const [expandedField, setExpandedField] = useState<{ key: string; label: string } | null>(null);
  const [codeOpen, setCodeOpen] = useState(false);
  /* The code statement lives under different config keys per node family
     (JavaScript → "code", SQL → "sql"); the database select is likewise
     "databaseId" on SQL nodes. Derive everything from field kinds. */
  const codeField = node.fields.find((f) => f.kind === "javascript-editor" || f.kind === "sql-editor");
  const dbField = node.fields.find((f) => f.kind === "database-select");
  const codeKey = codeField?.key ?? "code";
  const dbKey = dbField?.key ?? "databaseId";
  const codeKind: "js" | "sql" = codeField?.kind === "sql-editor" ? "sql" : "js";

  const statusTone: "ok" | "run" | "muted" =
    node.status === "done" ? "ok" : node.status === "running" ? "run" : "muted";

  const dbOptions = useMemo(
    () => api.databases.filter((d) => d.driver !== "redis" && d.driver !== "sugardb").map((d) => ({ value: d.id, label: d.name, icon: "Database" })),
    [api.databases],
  );
  const kvDbOptions = useMemo(
    () => api.databases.filter((d) => d.driver === "redis" || d.driver === "sugardb").map((d) => ({ value: d.id, label: d.name, icon: "Database" })),
    [api.databases],
  );
  const storageOptions = useMemo(
    () => api.storages.map((s) => ({ value: s.id, label: s.name, icon: s.driver === "s3" ? "Cloud" : "Globe" })),
    [api.storages],
  );
  const secretOptions = useMemo<{ value: string; label: string; icon?: string }[]>(
    () => [
      { value: "", label: t("editor.secretPlaceholder") },
      ...api.secrets.map((name) => ({ value: name, label: name, icon: "KeyRound" })),
    ],
    [api.secrets, t],
  );
  const identityOptions = useMemo<{ value: string; label: string; icon?: string }[]>(
    () => [
      { value: "", label: t("twitch.identityPlaceholder") },
      ...api.identities.map((i) => ({
        value: i.id,
        label: i.status === "connected" ? i.label : `${i.label} · ${t("twitch.reconnectRequired")}`,
      })),
    ],
    [api.identities, t],
  );
  const discordIdentityOptions = useMemo<{ value: string; label: string; icon?: string }[]>(
    () => [
      { value: "", label: t("discord.identityPlaceholder") },
      ...api.discordIdentities.map((i) => ({
        value: i.id,
        label: i.status === "connected" ? i.label : `${i.label} · ${t("discord.invalidIdentity")}`,
      })),
    ],
    [api.discordIdentities, t],
  );
  const telegramIdentityOptions = useMemo<{ value: string; label: string; icon?: string }[]>(
    () => [
      { value: "", label: t("telegram.identityPlaceholder") },
      ...api.telegramIdentities.map((i) => ({
        value: i.id,
        label: i.status === "connected" ? i.label : `${i.label} · ${t("telegram.invalidIdentity")}`,
      })),
    ],
    [api.telegramIdentities, t],
  );
  const pipelineOptions = useMemo(
    () =>
      api.pipelines.map((p) => ({
        value: p.id,
        label: p.status === "published" ? p.name : `${p.name} · ${t("editor.draft")}`,
        icon: "Workflow",
      })),
    [api.pipelines, t],
  );

  return (
    <div key={node.id} className="fade-in min-h-0 flex-1 overflow-y-auto overscroll-contain">
      {/* full-screen code editor for code nodes */}
      {codeOpen && (
        <CodeEditorModal
          title={node.title}
          kind={codeKind}
          code={String(node.values[codeKey] ?? "")}
          database={String(node.values[dbKey] ?? "") || api.databases[0]?.id || ""}
          databases={dbOptions}
          inputs={
            codeKind === "js"
              ? // The reserved "Code" input pin replaces this editor's code at
                // runtime when wired — it must not be editable from within.
                node.inputs.filter((p) => p.id !== "code")
              : node.inputs
          }
          outputs={node.outputs}
          api={api}
          onSave={(v, db) => {
            onChange(codeKey, v);
            if (db !== undefined) onChange(dbKey, db);
            setCodeOpen(false);
          }}
          onClose={() => setCodeOpen(false)}
          onPortsChange={(inputs, outputs) => onPortsChange?.(node.id, inputs, outputs)}
          onChangeDatabase={(db) => onChange(dbKey, db)}
          sqlParameters={node.values[parametersKeyOf(node)]}
          onChangeSqlParameters={(next) => onChange(parametersKeyOf(node), next)}
        />
      )}

      {/* full-screen text editor */}
      {expandedField && (
        <TextEditorModal
          title={`${node.title} — ${expandedField.label}`}
          value={stringifyValue(node.values[expandedField.key])}
          onSave={(v) => {
            let parsed: unknown = v;
            if (expandedField.key !== "code") {
              try {
                parsed = JSON.parse(v);
              } catch {
                parsed = v;
              }
            }
            onChange(expandedField.key, parsed as string);
            setExpandedField(null);
          }}
          onClose={() => setExpandedField(null)}
        />
      )}

      {/* identity */}
      <div className="flex items-start gap-2.5 border-b border-seam px-3 py-3">
        <span className="grid h-8 w-8 shrink-0 place-items-center rounded-lg border border-ink-700 bg-ink-850 text-fg">
          <Icon name={node.icon} className="h-4 w-4" />
        </span>
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <h2 className="truncate text-[14px] font-semibold text-fg">{node.title}</h2>
            {node.group && <Badge tone="muted">{node.group}</Badge>}
          </div>
          <p className="mt-0.5 font-mono text-[10.5px] text-fg-faint">{node.type}</p>
        </div>
      </div>

      {node.summary && (
        <p className="border-b border-seam px-3 py-2.5 text-[12px] leading-relaxed text-fg-subtle">{node.summary}</p>
      )}

      {node.lastRun && (
        <div className="border-b border-seam px-3 py-2.5">
          <div className="flex items-center gap-2">
            <span className="text-[10.5px] font-medium tracking-[0.09em] text-fg-subtle uppercase">
              {t("editorActions.latestResult")}
            </span>
            <Badge tone={statusTone} className="ml-auto">
              {node.lastRun.status}
            </Badge>
          </div>
          {node.lastRun.error && (
            <pre className="mt-1.5 max-h-[120px] overflow-auto whitespace-pre-wrap rounded-md border border-danger/20 bg-danger/5 px-2.5 py-2 font-mono text-[10.5px] leading-[1.6] text-danger-fg">
              {node.lastRun.error}
            </pre>
          )}
        </div>
      )}

      {(node.type === "function:entry" || node.type === "function:return") && (
        <FunctionPortsEditor
          node={node}
          kind={functionKind}
          onPortsChange={onPortsChange}
          onKindChange={onFunctionKindChange}
        />
      )}

      <Section title={t("editor.parameters")} right={<span className="font-mono text-[10px] text-fg-faint">{visibleCount(node)}</span>}>
          <div className="space-y-2.5">
            {visibleFieldsOf(node).map((f) => (
              <InspectorField
                key={f.key}
                fieldKey={f.key}
                field={f}
                node={node}
                dbOptions={dbOptions}
                kvDbOptions={kvDbOptions}
                storageOptions={storageOptions}
                secretOptions={secretOptions}
                identityOptions={identityOptions}
                discordIdentityOptions={discordIdentityOptions}
                telegramIdentityOptions={telegramIdentityOptions}
                pipelineOptions={pipelineOptions}
                onChange={onChange}
                onExpand={(key, label) =>
                  f.type === "code-js" || f.type === "code-sql"
                    ? setCodeOpen(true)
                    : setExpandedField({ key, label })
                }
                onCode={() => setCodeOpen(true)}
              />
            ))}
          </div>
        </Section>

      {node.outputSchema && node.outputSchema.length > 0 && (
        <Section title={t("editor.knownOutputFields")}>
          <div className="overflow-hidden rounded-md border border-ink-700/70">
            {node.outputSchema.map((o, i) => (
              <div
                key={o.key}
                className={cn(
                  "flex items-center justify-between bg-ink-850 px-2.5 py-[6px]",
                  i > 0 && "border-t border-seam",
                )}
              >
                <span className="font-mono text-[11px] text-fg">{o.key}</span>
                <span className="text-[10.5px] text-fg-faint">{o.type}</span>
              </div>
            ))}
          </div>
        </Section>
      )}

      <Section title={t("editor.connections")} defaultOpen={false}>
        <div className="grid grid-cols-2 gap-2">
          {[
            [t("editor.input"), node.inputs],
            [t("editor.output"), node.outputs],
          ].map(([label, ports]) => (
            <div key={label as string}>
              <p className="mb-1 text-[10.5px] tracking-wide text-fg-faint uppercase">{label as string}</p>
              <ul className="space-y-1">
                {(ports as GraphNode["inputs"]).map((p) => (
                  <li key={p.id} className="flex items-center gap-1.5 text-[11.5px] text-fg-subtle">
                    <span
                      className={cn(
                        "h-[6px] w-[6px] shrink-0 border border-ink-500",
                        p.kind === "exec" ? "rounded-[1px] bg-ink-300" : "rounded-full",
                      )}
                    />
                    <span className="truncate">{p.label}</span>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>
      </Section>
    </div>
  );
}

/* ---------- single field renderer ---------- */

/** Config key holding the SQL parameter contract for a node. */
function parametersKeyOf(_node: GraphNode): string {
  return "parameters";
}

/** Fields shown in the generic section; SQL parameters get their own editor. */
function visibleFieldsOf(node: GraphNode): import("@/types").FieldDef[] {
  return node.fields.filter((f) => !(f.kind === "sql-parameters" || (f.key === "parameters" && f.kind === undefined && f.type === "json")));
}

/** Count for the section badge, mirroring visibleFieldsOf. */
function visibleCount(node: GraphNode): number {
  return visibleFieldsOf(node).length;
}

function stringifyValue(value: unknown): string {
  if (value === undefined || value === null) return "";
  if (typeof value === "string") return value;
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

/** Attachment names for the embed editor's Discord preview: URL entries,
 * local paths, and the named data file, in the order Discord would show them. */
function embedPreviewAttachments(node: GraphNode): Array<{ name: string }> {
  const names: string[] = [];
  const pushLines = (value: unknown) => {
    if (typeof value !== "string") return;
    for (const line of value.split("\n")) {
      const trimmed = line.trim();
      if (!trimmed) continue;
      const withoutQuery = trimmed.split("?")[0].split("#")[0];
      const base = withoutQuery.split("/").filter(Boolean).pop() ?? trimmed;
      names.push(base || trimmed);
    }
  };
  pushLines(node.values.fileUrl);
  pushLines(node.values.filePath);
  const hasInlinePayload =
    (typeof node.values.fileBase64 === "string" && node.values.fileBase64.trim() !== "") ||
    node.values.fileData !== undefined && node.values.fileData !== null;
  if (typeof node.values.fileName === "string" && node.values.fileName.trim() !== "") {
    names.push(node.values.fileName.trim());
  } else if (hasInlinePayload) {
    names.push("file.bin");
  }
  return names.slice(0, 10).map((name) => ({ name }));
}

function InspectorField({
  fieldKey,
  field,
  node,
  dbOptions,
  kvDbOptions,
  storageOptions,
  secretOptions,
  identityOptions,
  discordIdentityOptions,
  telegramIdentityOptions,
  pipelineOptions,
  onChange,
  onExpand,
  onCode,
}: {
  fieldKey: string;
  field: import("@/types").FieldDef;
  node: GraphNode;
  dbOptions: DropdownOption[];
  kvDbOptions: DropdownOption[];
  storageOptions: DropdownOption[];
  secretOptions: DropdownOption[];
  identityOptions: { value: string; label: string }[];
  discordIdentityOptions: { value: string; label: string }[];
  telegramIdentityOptions: { value: string; label: string }[];
  pipelineOptions: DropdownOption[];
  onChange: (key: string, value: unknown) => void;
  onExpand: (key: string, label: string) => void;
  onCode: () => void;
}) {
  const { t } = useTranslation();
  const raw = node.values[fieldKey];
  const [jsonDraft, setJsonDraft] = useState<string | null>(null);
  const jsonError =
    field.type === "json" && jsonDraft !== null && !isValidJson(jsonDraft);

  useEffect(() => setJsonDraft(null), [fieldKey]);

  /* structured editors for known complex config kinds */
  if (field.kind === "field-outputs" || field.kind === "object-fields") {
    const isPath = field.kind === "field-outputs";
    return (
      <NamedFieldsEditor
        label={field.label}
        raw={raw}
        secondKey={isPath ? "path" : "key"}
        secondLabel={isPath ? t("editor.fieldPath") : t("editor.objectKey")}
        onChange={(next) => onChange(fieldKey, next)}
      />
    );
  }

  /* structured editors restored from the previous UI */  if (field.kind === "http-headers") {
    return <HeadersEditor label={field.label} value={raw} onChange={(next) => onChange(fieldKey, next)} />;
  }

  if (field.kind === "html-extractions") {
    return <HtmlExtractionsEditor value={raw} onChange={(next) => onChange(fieldKey, next)} />;
  }

  /* KV visual editors: lists, hash fields, scored entries, command arguments */
  if (field.kind === "kv-string-list") {
    return (
      <KVStringListEditor
        label={field.label}
        value={raw}
        placeholder={field.placeholder}
        onChange={(next) => onChange(fieldKey, next)}
      />
    );
  }

  if (field.kind === "kv-hash-fields") {
    return <KVHashFieldsEditor label={field.label} value={raw} onChange={(next) => onChange(fieldKey, next)} />;
  }

  if (field.kind === "kv-scored-entries") {
    return <KVScoredEntriesEditor label={field.label} value={raw} onChange={(next) => onChange(fieldKey, next)} />;
  }

  if (field.kind === "kv-arguments") {
    return <KVArgumentsEditor label={field.label} value={raw} onChange={(next) => onChange(fieldKey, next)} />;
  }

  /* wire-type picker (Type Assert, JS output contracts) */
  if (field.kind === "type-spec") {
    let spec: TypeSpec | undefined;
    if (typeof raw === "string") {
      try {
        spec = JSON.parse(raw) as TypeSpec;
      } catch {
        spec = undefined;
      }
    } else if (raw && typeof raw === "object") {
      spec = raw as TypeSpec;
    }
    return (
      <label className="block">
        <span className="mb-1 block">
          <Label text={field.label} required={field.required} />
        </span>
        <TypeSpecField
          value={spec}
          onChange={(next) => onChange(fieldKey, next)}
        />
      </label>
    );
  }

  if (field.kind === "json-schema") {
    return <SchemaEditor value={raw} onChange={(next) => onChange(fieldKey, next)} />;
  }
  if (field.kind === "route-options") {
    return <RouteOptionsEditor value={raw} onChange={(next) => onChange(fieldKey, next)} />;
  }
  if (field.kind === "switch-cases") {
    return (
      <SwitchCasesEditor
        value={raw}
        legacyOptions={node.values.options}
        onChange={(next) => onChange(fieldKey, next)}
      />
    );
  }
  /* visual grid editor for the Form node's layout */
  if (field.kind === "form-builder") {
    return (
      <div className="flex items-center justify-between gap-2 rounded-md border border-ink-700/70 bg-ink-850 px-2.5 py-[7px]">
        <Label text={field.label} required={field.required} />
        <FormBuilderEditor value={raw} onChange={(next) => onChange(fieldKey, next)} />
      </div>
    );
  }

  /* visual image document editor for the Draw Image node */
  if (field.kind === "image-editor") {
    return (
      <div className="space-y-1.5">
        <Label text={field.label} required={field.required} />
        <DrawImageEditor value={raw} onChange={(next) => onChange(fieldKey, next)} />
      </div>
    );
  }

  /* visual embed editor for the Discord Send Message node */
  if (field.kind === "embed-editor") {
    return (
      <EmbedEditor
        value={raw}
        onChange={(next) => onChange(fieldKey, next)}
        previewContent={typeof node.values.message === "string" ? node.values.message : ""}
        previewAttachments={embedPreviewAttachments(node)}
      />
    );
  }

  if (field.type === "toggle") {
    return (
      <div className="flex items-center justify-between rounded-md border border-ink-700/70 bg-ink-850 px-2.5 py-[7px]">
        <span className="flex flex-col">
          <Label text={field.label} required={field.required} />
          <span className="text-[11px] text-fg-faint">
            {raw ? t("editor.enabled") : t("editor.disabled")}
          </span>
        </span>
        <Toggle on={Boolean(raw)} onChange={(nv) => onChange(fieldKey, nv)} />
      </div>
    );
  }

  if (field.type === "select") {
    const optionMap: Record<string, DropdownOption[] | undefined> = {
      databases: dbOptions,
      "kv-databases": kvDbOptions,
      storages: storageOptions,
      secrets: secretOptions,
      "twitch-identity": identityOptions,
      "discord-identity": discordIdentityOptions,
      "telegram-identity": telegramIdentityOptions,
      pipelines: pipelineOptions,
    };
    const options =
      (field.dynamic ? optionMap[field.dynamic] : undefined) ??
      (field.options ?? []).map((o) => ({ value: o.value, label: o.label }));
    return (
      <label className="block">
        <span className="mb-1 block">
          <Label text={field.label} required={field.required} />
        </span>
        <Dropdown
          value={String(raw ?? "")}
          options={options}
          placeholder={field.dynamic === "pipelines" ? t("editor.selectPipeline") : undefined}
          onChange={(nv) => onChange(fieldKey, nv)}
        />
        {field.key === "pipelineId" && (
          <span className="mt-1 block text-[10.5px] text-fg-faint">{t("editor.pipelinePinOverride")}</span>
        )}
      </label>
    );
  }

  if (field.type === "code-js" || field.type === "code-sql") {
    return (
      <label className="block">
        <span className="mb-1 flex items-center justify-between">
          <Label text={field.label} required={field.required} />
          <button
            type="button"
            onClick={(e) => {
              e.preventDefault();
              onCode();
            }}
            className="flex items-center gap-1 rounded px-1 py-0.5 text-fg-faint transition hover:bg-ink-750 hover:text-fg-muted"
          >
            <Icon name="Braces" className="h-3 w-3" />
            <span className="text-[10px]">{t("textEditor.expand")}</span>
          </button>
        </span>
        <textarea
          rows={4}
          readOnly
          value={String(raw ?? "")}
          onFocus={onCode}
          className={cn(inputCls, "cursor-pointer resize-none font-mono text-[11.5px]")}
        />
      </label>
    );
  }

  return (
    <label className="block">
      <span className="mb-1 flex items-center justify-between">
        <Label text={field.label} required={field.required} />
        {(field.type === "textarea" || field.type === "text" || field.type === "json") && (
          <button
            type="button"
            onClick={(e) => {
              e.preventDefault();
              onExpand(fieldKey, field.label);
            }}
            className="flex items-center gap-1 rounded px-1 py-0.5 text-fg-faint transition hover:bg-ink-750 hover:text-fg-muted"
          >
            <Icon name="Expand" className="h-3 w-3" />
            <span className="text-[10px]">{t("textEditor.expand")}</span>
          </button>
        )}
      </span>
      {field.type === "textarea" || field.type === "json" ? (
        <>
          <textarea
            rows={field.type === "json" ? 5 : 3}
            value={field.type === "json" ? (jsonDraft ?? stringifyValue(raw)) : String(raw ?? "")}
            placeholder={field.placeholder}
            onChange={(e) => {
              if (field.type === "json") {
                setJsonDraft(e.target.value);
              } else {
                onChange(fieldKey, e.target.value);
              }
            }}
            onBlur={() => {
              if (field.type === "json" && jsonDraft !== null && isValidJson(jsonDraft)) {
                try {
                  onChange(fieldKey, JSON.parse(jsonDraft));
                } catch {
                  /* guarded by isValidJson */
                }
              }
            }}
            className={cn(inputCls, "resize-y leading-relaxed", field.type === "json" && "font-mono text-[11.5px]")}
          />
          {jsonError && <span className="mt-1 block text-[11px] text-danger-fg">{t("variables.defaultInvalidJson")}</span>}
        </>
      ) : field.type === "number" ? (
        <input
          type="number"
          value={Number.isFinite(Number(raw)) ? Number(raw) : ""}
          placeholder={field.placeholder}
          onChange={(e) => onChange(fieldKey, Number(e.target.value))}
          className={inputCls}
        />
      ) : (
        <input
          type="text"
          value={String(raw ?? "")}
          placeholder={field.placeholder}
          onChange={(e) => onChange(fieldKey, e.target.value)}
          className={inputCls}
        />
      )}
    </label>
  );
}

function isValidJson(text: string): boolean {
  if (!text.trim()) return true;
  try {
    JSON.parse(text);
    return true;
  } catch {
    return false;
  }
}

/* ------------------------------------------------------------------ */
/*  Named typed fields editor (field-outputs / object-fields)          */
/* ------------------------------------------------------------------ */

interface NamedFieldEntry {
  id: string;
  label: string;
  path: string;
  key: string;
  dataType: string;
}

function parseNamedFields(raw: unknown): NamedFieldEntry[] {
  let list: unknown = raw;
  if (typeof raw === "string") {
    try { list = JSON.parse(raw); } catch { return []; }
  }
  if (!Array.isArray(list)) return [];
  return list
    .filter((p): p is Record<string, unknown> => typeof p === "object" && p !== null)
    .map((p, i) => ({
      id: typeof p.id === "string" ? p.id : `field_${i + 1}`,
      label: typeof p.label === "string" ? p.label : "",
      path: typeof p.path === "string" ? p.path : "",
      key: typeof p.key === "string" ? p.key : "",
      dataType: typeof p.dataType === "string" ? p.dataType : "any",
    }));
}

function serializeNamedFields(entries: NamedFieldEntry[]): unknown[] {
  return entries.map((e, i) => {
    const id = /^[A-Za-z_][A-Za-z0-9_]*$/.test(e.label.replace(/\s+/g, "_"))
      ? e.label.replace(/\s+/g, "_")
      : `field_${i + 1}`;
    return {
      id,
      label: e.label.trim() || id,
      ...(e.path ? { path: e.path } : {}),
      ...(e.key ? { key: e.key } : {}),
      dataType: e.dataType,
    };
  });
}

const NAMED_FIELD_TYPES = ["any", "text", "number", "boolean", "object", "list", "bytes"] as const;

function NamedFieldsEditor({
  label,
  raw,
  secondKey,
  secondLabel,
  onChange,
}: {
  label: string;
  raw: unknown;
  secondKey: string;
  secondLabel: string;
  onChange: (next: unknown) => void;
}) {
  const { t } = useTranslation();
  const entries = parseNamedFields(raw);

  const commit = (next: NamedFieldEntry[]) => onChange(serializeNamedFields(next));
  const patch = (i: number, p: Partial<NamedFieldEntry>) =>
    commit(entries.map((e, j) => (j === i ? { ...e, ...p } : e)));

  return (
    <div className="space-y-2">
      <Label text={label} />
      {entries.map((e, i) => (
        <div key={`${e.id}-${i}`} className="space-y-1 rounded-md border border-ink-700/70 bg-ink-850/50 p-2">
          <div className="flex items-center gap-1.5">
            <input
              value={e.label}
              placeholder={t("editor.fieldName")}
              onChange={(ev) => patch(i, { label: ev.target.value })}
              className="h-7 min-w-0 flex-1 rounded-md border border-ink-700 bg-ink-900 px-2 text-[12px] text-fg"
            />
            <Dropdown
              compact
              value={e.dataType}
              onChange={(v) => patch(i, { dataType: v })}
              className="h-7 shrink-0 w-[80px] text-[11px]"
              options={NAMED_FIELD_TYPES.map((tp) => ({ value: tp, label: tp }))}
            />
            <button
              onClick={() => commit(entries.filter((_, j) => j !== i))}
              aria-label={t("common.delete")}
              className="grid h-7 w-7 shrink-0 place-items-center rounded-md text-fg-faint hover:bg-danger/15 hover:text-danger-fg"
            >
              <Icon name="Trash2" className="h-3.5 w-3.5" />
            </button>
          </div>
          <input
            value={secondKey === "path" ? e.path : e.key}
            placeholder={secondLabel}
            onChange={(ev) => patch(i, secondKey === "path" ? { path: ev.target.value } : { key: ev.target.value })}
            className="h-6 w-full rounded-md border border-ink-700 bg-ink-900 px-2 font-mono text-[11px] text-fg-subtle"
          />
        </div>
      ))}
      {entries.length === 0 && (
        <p className="rounded-md border border-dashed border-ink-700 px-2.5 py-2 text-[11px] text-fg-faint">
          {t("editor.noFields")}
        </p>
      )}
      <button
        onClick={() => commit([...entries, { id: `field_${entries.length + 1}`, label: "", path: "", key: "", dataType: "text" }])}
        className="flex h-7 w-full items-center justify-center gap-1.5 rounded-md border border-ink-700 bg-ink-850 px-2 text-[11.5px] text-fg-muted hover:bg-ink-750"
      >
        <Icon name="Plus" className="h-3.5 w-3.5" />
        {t("editor.addField")}
      </button>
    </div>
  );
}

/* ---------- execution log ---------- */

function LogBody({
  log,
  executions,
  onSelect,
}: {
  log: LogEntry[];
  executions: Execution[];
  onSelect: (execution: Execution) => void;
}) {
  const { t } = useTranslation();
  const [selectedExecution, setSelectedExecution] = useState<string>("");
  const [expandedId, setExpandedId] = useState<string | null>(null);
  /* the entry whose input/output is open in the full-screen JSON viewer */
  const [viewerEntry, setViewerEntry] = useState<LogEntry | null>(null);
  /* scoped to the unique entry (node + start time) so repeated runs of the
     same node highlight individually, never as one group */
  const [hoveredEntryId, setHoveredEntryId] = useState<string | null>(null);
  /* log rows by entry id — timeline clicks scroll to their row */
  const entryRefs = useRef(new Map<string, HTMLLIElement>());
  const scrollToEntry = (id: string) => {
    setHoveredEntryId(id); // keep the target marked after the pointer leaves the strip
    entryRefs.current.get(id)?.scrollIntoView({ behavior: "smooth", block: "nearest" });
  };

  const executionOptions = useMemo(
    () => [
      { value: "", label: t("editor.pickExecution") },
      ...executions.slice(0, 30).map((e) => ({
        value: e.id,
        label: `${new Intl.DateTimeFormat(undefined, { timeStyle: "medium" }).format(new Date(e.startedAt))} · ${t(`runStatus.${e.status}`)}`,
        icon: e.status === "completed" ? "Check" : e.status === "failed" ? "AlertTriangle" : "Activity",
      })),
    ],
    [executions, t],
  );

  /* timeline window: earliest start → latest finish across all entries */
  const timed = log.filter((l) => l.startedAt && l.finishedAt);

  const statusColor = (s: string) =>
    s === "completed" ? "var(--status-success)" : s === "failed" ? "var(--status-danger)" : s === "running" ? "var(--status-info)" : "var(--fg-faint)";

  return (
    <div className="fade-in flex min-h-0 flex-1 flex-col overflow-y-auto">
      <div className="flex shrink-0 items-center gap-2 border-b border-seam px-3 py-2">
        <Icon name="Clock" className="h-3.5 w-3.5 text-fg-faint" />
        <Dropdown
          compact
          value={selectedExecution}
          className="min-w-0 flex-1"
          onChange={(id) => {
            setSelectedExecution(id);
            const hit = executions.find((e) => e.id === id);
            if (hit) onSelect(hit);
          }}
          options={executionOptions}
        />
      </div>

      {/* timeline */}
      {timed.length > 1 && (() => {
        const totalMs = timed.reduce((s, l) => s + l.ms, 0);
        return (
        <div className="shrink-0 border-b border-seam px-3 pb-2 pt-2.5">
          <p className="mb-1.5 text-[10px] font-medium uppercase tracking-[0.09em] text-fg-faint">{t("editor.timeline")}</p>
          <div className="relative flex h-7 w-full gap-px overflow-hidden rounded-md bg-ink-950/60">
            {(() => {
              /* sequential waterfall: segments in execution order, width
                 proportional to duration. Sub-millisecond runs truncate to
                 0 ms, so an all-zero measurement stretches evenly instead
                 of collapsing into minimum strips; every other layout is
                 renormalized to fill exactly 100% of the track. */
              const MIN_PCT = 2;
              const sorted = [...timed].sort(
                (a, b) => Date.parse(a.startedAt!) - Date.parse(b.startedAt!),
              );
              let weights = sorted.map((l) => Math.max(l.ms, 0));
              if (!weights.some((w) => w > 0)) weights = sorted.map(() => 1);
              const weightSum = weights.reduce((s, w) => s + w, 0);
              /* visible minimum, capped so many tiny nodes can still share the track */
              const minPct = Math.min(MIN_PCT, 100 / sorted.length);
              const clamped = weights.map((w) => Math.max((w / weightSum) * 100, minPct));
              const scale = 100 / clamped.reduce((s, w) => s + w, 0);
              return sorted.map((l, i) => {
                const width = clamped[i] * scale;
                const hovered = hoveredEntryId === l.id;
                return (
                  <div
                    key={`tl-${l.id}`}
                    role="button"
                    tabIndex={-1}
                    title={`${l.node} · ${formatDuration(l.ms)}`}
                    onClick={() => scrollToEntry(l.id)}
                    onMouseEnter={() => setHoveredEntryId(l.id)}
                    onMouseLeave={() => setHoveredEntryId(null)}
                    className="h-full cursor-pointer transition-opacity"
                    style={{
                      /* grow ratios (not %) so the 1px gaps are subtracted
                         first and segments exactly fill the track */
                      flexGrow: Math.max(width, 0.0001),
                      flexBasis: 0,
                      background: statusColor(l.status),
                      opacity: hovered ? 1 : 0.55,
                      borderRadius: 2,
                    }}
                  />
                );
              });
            })()}
          </div>
          <div className="mt-0.5 flex justify-between font-mono text-[9px] text-fg-faint">
            <span>0ms</span>
            <span>{totalMs > 0 && totalMs < 1 ? "<1ms" : formatDuration(totalMs)}</span>
          </div>
        </div>
        );
      })()}

      {/* log entries */}
      <ul className="min-h-0 flex-1">
        {log.length === 0 && (
          <p className="px-3 py-6 text-center text-[12px] leading-relaxed text-fg-faint">{t("editor.runToInspect")}</p>
        )}
        {log.map((l) => {
          const isHovered = hoveredEntryId === l.id;
          const isExpanded = expandedId === l.id;
          const hasData = (l.input !== undefined && l.input !== null) || (l.output !== undefined && l.output !== null);
          return (
            <li
              key={l.id}
              ref={(el) => {
                if (el) entryRefs.current.set(l.id, el);
                else entryRefs.current.delete(l.id);
              }}
              onMouseEnter={() => setHoveredEntryId(l.id)}
              onMouseLeave={() => setHoveredEntryId(null)}
              className={cn(
                "group border-b border-seam/70 px-3 py-2 transition",
                isHovered ? "bg-ink-800" : "hover:bg-ink-850",
              )}
            >
              <div className="flex items-center gap-1">
                <button
                  onClick={() => setExpandedId(isExpanded ? null : l.id)}
                  className="min-w-0 flex-1 text-left"
                >
                  <div className="flex items-center gap-2">
                    <Icon
                      name="ChevronRight"
                      className={cn("h-3 w-3 shrink-0 text-fg-faint transition-transform", isExpanded && "rotate-90")}
                    />
                    <Dot
                      tone={
                        l.status === "completed" ? "done"
                        : l.status === "running" ? "running"
                        : l.status === "failed" ? "error"
                        : "idle"
                      }
                    />
                    <span className="truncate text-[12.5px] font-medium text-fg">{l.node}</span>
                    <span
                      className={cn(
                        "ml-auto font-mono text-[10.5px]",
                        l.status === "completed" && "text-success-fg/80",
                        l.status === "running" && "text-fg",
                        l.status === "skipped" && "text-fg-faint",
                        l.status === "failed" && "text-danger-fg",
                      )}
                    >
                      {l.ms}ms
                    </span>
                  </div>
                </button>
                {hasData && (
                  <Tooltip content={t("jsonViewer.inspect")} side="left" delay={200}>
                    <button
                      onClick={() => setViewerEntry(l)}
                      className="grid h-6 w-6 shrink-0 place-items-center rounded-md text-fg-faint opacity-0 transition hover:bg-ink-750 hover:text-fg focus-visible:opacity-100 group-hover:opacity-100"
                    >
                      <Icon name="Braces" className="h-3.5 w-3.5" />
                    </button>
                  </Tooltip>
                )}
              </div>
              {l.error && (
                <pre className="mt-1.5 max-h-[90px] overflow-auto whitespace-pre-wrap rounded-md border border-danger/20 bg-danger/5 px-2 py-1.5 font-mono text-[10px] text-danger-fg">
                  {l.error}
                </pre>
              )}

              {isExpanded && (
                <div className="mt-1.5 space-y-1.5 pl-7">
                  {hasData && (
                    <button
                      onClick={() => setViewerEntry(l)}
                      className="flex h-6 w-full items-center justify-center gap-1.5 rounded-md border border-ink-700 bg-ink-850 text-[10.5px] font-medium text-fg-subtle transition hover:border-ink-600 hover:bg-ink-750 hover:text-fg"
                    >
                      <Icon name="Braces" className="h-3 w-3" />
                      {t("jsonViewer.inspect")}
                    </button>
                  )}
                  {l.input !== undefined && l.input !== null && (
                    <div>
                      <p className="mb-0.5 text-[10px] font-medium uppercase tracking-[0.09em] text-fg-faint">{t("editor.entryInput")}</p>
                      <pre className="max-h-[180px] overflow-auto whitespace-pre-wrap rounded-md border border-ink-700 bg-ink-950/60 px-2.5 py-2 font-mono text-[10.5px] leading-[1.6] text-fg-subtle">
                        {formatJson(l.input)}
                      </pre>
                    </div>
                  )}
                  {l.output !== undefined && l.output !== null && (
                    <div>
                      <p className="mb-0.5 text-[10px] font-medium uppercase tracking-[0.09em] text-fg-faint">{t("editor.entryOutput")}</p>
                      <pre className="max-h-[180px] overflow-auto whitespace-pre-wrap rounded-md border border-ink-700 bg-ink-950/60 px-2.5 py-2 font-mono text-[10.5px] leading-[1.6] text-fg-subtle">
                        {formatJson(l.output)}
                      </pre>
                    </div>
                  )}
                  {!hasData && (
                    <p className="text-[11px] italic text-fg-faint">{t("editor.noEntryData")}</p>
                  )}
                </div>
              )}
            </li>
          );
        })}
      </ul>

      {viewerEntry && <JsonViewerModal entry={viewerEntry} onClose={() => setViewerEntry(null)} />}
    </div>
  );
}

/** Pretty-prints a value as JSON; returns the string as-is on failure. */
function formatJson(value: unknown): string {
  try {
    return JSON.stringify(value, null, 2) ?? String(value);
  } catch {
    return String(value);
  }
}

