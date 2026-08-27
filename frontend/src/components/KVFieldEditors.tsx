import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Icon } from "./icons";
import { Dropdown, type DropdownOption } from "./Dropdown";
import { Toggle } from "./ui";
import { pinColor } from "../lib/pins";
import { cn } from "../utils/cn";
import {
  buildKvArgumentPayload,
  buildKvHashFieldsPayload,
  buildKvScoredEntriesPayload,
  buildKvStringListPayload,
  parseKvArguments,
  parseKvHashFields,
  parseKvScoredEntries,
  parseKvStringList,
  type KvArgKind,
  type KvArgumentRow,
} from "../lib/kv-editors";

/* Shared visual language with HeadersEditor: mono inputs, dashed add button,
   count badge. Row state is kept locally so blank mid-edit rows survive the
   round-trip through the persisted (filtered) payload. */

const inputCls =
  "h-7 min-w-0 flex-1 rounded-md border border-ink-700 bg-ink-900 px-2 font-mono text-[11.5px] text-ink-100 transition focus:border-ink-500 focus:outline-none";

function EditorLabel({ children, count }: { children: React.ReactNode; count?: number }) {
  return (
    <div className="flex items-center justify-between">
      <span className="block text-[11px] font-medium text-ink-300">{children}</span>
      {count !== undefined && <span className="font-mono text-[10px] text-ink-600">{count}</span>}
    </div>
  );
}

function ColumnHeads({ left, right }: { left: string; right?: string }) {
  return (
    <div className="flex items-center gap-1.5 px-0.5 text-[10px] tracking-wide text-ink-600 uppercase">
      <span className="flex-1">{left}</span>
      {right !== undefined ? (
        <span className="flex-1">{right}</span>
      ) : (
        <span className="w-7 shrink-0" />
      )}
    </div>
  );
}

function AddButton({ label, onClick }: { label: string; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="flex h-7 w-full items-center justify-center gap-1.5 rounded-md border border-dashed border-ink-700 bg-ink-850/50 px-2 text-[11.5px] text-ink-300 transition hover:border-ink-500 hover:bg-ink-750"
    >
      <Icon name="Plus" className="h-3.5 w-3.5" />
      {label}
    </button>
  );
}

function EmptyHint({ text }: { text: string }) {
  return (
    <p className="rounded-md border border-dashed border-ink-700 px-2.5 py-2 text-[11px] text-ink-500">{text}</p>
  );
}

/**
 * Draft-row editor state. The component keeps its own row list so partially
 * filled rows stay editable, while `onChange` only ever receives the clean
 * payload (blank rows filtered). External value changes — undo, another
 * editor instance — are adopted only when they do not match what this editor
 * last emitted, so typing never fights the persisted config.
 */
function useDraftRows<T>(
  value: unknown,
  parse: (raw: unknown) => T[],
  build: (rows: T[]) => unknown,
  onChange: (next: unknown) => void,
) {
  const [rows, setRows] = useState<T[]>(() => parse(value));
  const lastEmitted = useRef<string | null>(null);
  const incoming = JSON.stringify(parse(value));

  useEffect(() => {
    if (lastEmitted.current !== null && incoming !== lastEmitted.current) {
      setRows(parse(value));
      lastEmitted.current = null;
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [incoming]);

  const commit = (next: T[]) => {
    setRows(next);
    const payload = build(next);
    lastEmitted.current = JSON.stringify(parse(payload));
    onChange(payload);
  };

  return { rows, commit };
}

/* ---------------- string list editor ---------------- */

/** One value per row — the visual replacement for hand-typed JSON arrays. */
export function KVStringListEditor({
  label,
  value,
  onChange,
  placeholder,
}: {
  label: string;
  value: unknown;
  onChange: (next: unknown) => void;
  placeholder?: string;
}) {
  const { t } = useTranslation();
  const { rows, commit } = useDraftRows(value, parseKvStringList, buildKvStringListPayload, onChange);

  return (
    <div className="space-y-2">
      <EditorLabel count={rows.filter((row) => row.trim() !== "").length}>{label}</EditorLabel>
      {rows.length > 0 && <ColumnHeads left={t("editor.kvValueColumn")} />}
      {rows.map((row, index) => (
        <div key={index} className="flex items-center gap-1.5">
          <input
            value={row}
            onChange={(e) => commit(rows.map((r, i) => (i === index ? e.target.value : r)))}
            placeholder={placeholder ?? t("editor.kvValuePlaceholder")}
            aria-label={t("editor.kvValueColumn")}
            className={inputCls}
          />
          <button
            type="button"
            onClick={() => commit(rows.filter((_, i) => i !== index))}
            aria-label={t("common.delete")}
            className="grid h-7 w-7 shrink-0 place-items-center rounded-md text-ink-500 transition hover:bg-rose-500/15 hover:text-rose-300"
          >
            <Icon name="Trash2" className="h-3.5 w-3.5" />
          </button>
        </div>
      ))}
      {rows.length === 0 && <EmptyHint text={t("editor.kvNoValues")} />}
      <AddButton label={t("editor.kvAddValue")} onClick={() => commit([...rows, ""])} />
    </div>
  );
}

/* ---------------- hash fields editor ---------------- */

/** Field/value rows — the visual replacement for hand-typed JSON objects. */
export function KVHashFieldsEditor({
  label,
  value,
  onChange,
}: {
  label: string;
  value: unknown;
  onChange: (next: unknown) => void;
}) {
  const { t } = useTranslation();
  const { rows, commit } = useDraftRows(value, parseKvHashFields, buildKvHashFieldsPayload, onChange);

  return (
    <div className="space-y-2">
      <EditorLabel count={rows.filter((row) => row.field.trim() !== "").length}>{label}</EditorLabel>
      {rows.length > 0 && (
        <ColumnHeads left={t("editor.kvFieldColumn")} right={t("editor.kvValueColumn")} />
      )}
      {rows.map((row, index) => (
        <div key={`${index}-${row.field}`} className="flex items-center gap-1.5">
          <input
            value={row.field}
            onChange={(e) => commit(rows.map((r, i) => (i === index ? { ...r, field: e.target.value } : r)))}
            placeholder="email"
            aria-label={t("editor.kvFieldColumn")}
            className={inputCls}
          />
          <input
            value={row.value}
            onChange={(e) => commit(rows.map((r, i) => (i === index ? { ...r, value: e.target.value } : r)))}
            placeholder="ada@example.com"
            aria-label={t("editor.kvValueColumn")}
            className={inputCls}
          />
          <button
            type="button"
            onClick={() => commit(rows.filter((_, i) => i !== index))}
            aria-label={t("common.delete")}
            className="grid h-7 w-7 shrink-0 place-items-center rounded-md text-ink-500 transition hover:bg-rose-500/15 hover:text-rose-300"
          >
            <Icon name="Trash2" className="h-3.5 w-3.5" />
          </button>
        </div>
      ))}
      {rows.length === 0 && <EmptyHint text={t("editor.kvNoFields")} />}
      <AddButton label={t("editor.kvAddField")} onClick={() => commit([...rows, { field: "", value: "" }])} />
    </div>
  );
}

/* ---------------- scored entries editor ---------------- */

/** Member/score rows for sorted sets — no more hand-typed {member, score} JSON. */
export function KVScoredEntriesEditor({
  label,
  value,
  onChange,
}: {
  label: string;
  value: unknown;
  onChange: (next: unknown) => void;
}) {
  const { t } = useTranslation();
  const { rows, commit } = useDraftRows(value, parseKvScoredEntries, buildKvScoredEntriesPayload, onChange);

  return (
    <div className="space-y-2">
      <EditorLabel count={rows.filter((row) => row.member.trim() !== "").length}>{label}</EditorLabel>
      {rows.length > 0 && (
        <ColumnHeads left={t("editor.kvMemberColumn")} right={t("editor.kvScoreColumn")} />
      )}
      {rows.map((row, index) => (
        <div key={`${index}-${row.member}`} className="flex items-center gap-1.5">
          <input
            value={row.member}
            onChange={(e) => commit(rows.map((r, i) => (i === index ? { ...r, member: e.target.value } : r)))}
            placeholder="ada"
            aria-label={t("editor.kvMemberColumn")}
            className={inputCls}
          />
          <input
            value={row.score}
            onChange={(e) => commit(rows.map((r, i) => (i === index ? { ...r, score: e.target.value } : r)))}
            placeholder="100"
            aria-label={t("editor.kvScoreColumn")}
            className={cn(inputCls, "w-20 flex-none")}
          />
          <button
            type="button"
            onClick={() => commit(rows.filter((_, i) => i !== index))}
            aria-label={t("common.delete")}
            className="grid h-7 w-7 shrink-0 place-items-center rounded-md text-ink-500 transition hover:bg-rose-500/15 hover:text-rose-300"
          >
            <Icon name="Trash2" className="h-3.5 w-3.5" />
          </button>
        </div>
      ))}
      {rows.length === 0 && <EmptyHint text={t("editor.kvNoEntries")} />}
      <AddButton label={t("editor.kvAddEntry")} onClick={() => commit([...rows, { member: "", score: "" }])} />
    </div>
  );
}

/* ---------------- command arguments editor ---------------- */

const ARG_KIND_TO_PIN: Record<KvArgKind, string> = {
  any: "any",
  string: "text",
  int: "number",
  float: "number",
  bool: "boolean",
  list: "array",
  map: "map",
};

/**
 * Typed argument builder for the generic KV Command node — the analogue of
 * the SQL node's parameter panel, available directly in the inspector. Every
 * change re-derives the node's dynamic input pins through the backend
 * resolver.
 */
export function KVArgumentsEditor({
  label,
  value,
  onChange,
}: {
  label: string;
  value: unknown;
  onChange: (next: unknown) => void;
}) {
  const { t } = useTranslation();
  const { rows, commit } = useDraftRows(value, parseKvArguments, buildKvArgumentPayload, onChange);
  const patch = (index: number, next: Partial<KvArgumentRow>) =>
    commit(rows.map((row, i) => (i === index ? { ...row, ...next } : row)));

  const typeOptions: DropdownOption[] = [
    { value: "string", label: t("editor.kvTypeText") },
    { value: "int", label: t("editor.kvTypeInt") },
    { value: "float", label: t("editor.kvTypeFloat") },
    { value: "bool", label: t("editor.kvTypeBool") },
    { value: "list", label: t("editor.kvTypeList") },
    { value: "map", label: t("editor.kvTypeMap") },
    { value: "any", label: t("editor.kvTypeAny") },
  ];

  return (
    <div className="space-y-2">
      <EditorLabel count={rows.length}>{label}</EditorLabel>
      {rows.map((row, index) => (
        <div
          key={index}
          className="space-y-1.5 rounded-md border border-ink-700/70 bg-ink-850/60 p-2"
        >
          <div className="flex items-center gap-1.5">
            <span
              className="h-2 w-2 shrink-0 rounded-full"
              style={{ background: pinColor(ARG_KIND_TO_PIN[row.kind]) }}
            />
            <input
              value={row.name}
              onChange={(e) => patch(index, { name: e.target.value })}
              placeholder="key"
              aria-label={t("editor.kvArgName")}
              className={cn(inputCls, "flex-[2]")}
            />
            <button
              type="button"
              onClick={() => commit(rows.filter((_, i) => i !== index))}
              aria-label={t("common.delete")}
              className="grid h-7 w-7 shrink-0 place-items-center rounded-md text-ink-500 transition hover:bg-rose-500/15 hover:text-rose-300"
            >
              <Icon name="Trash2" className="h-3.5 w-3.5" />
            </button>
          </div>
          <div className="flex items-center gap-1.5">
            <Dropdown
              value={row.kind}
              options={typeOptions}
              onChange={(kind) => patch(index, { kind: kind as KvArgKind })}
              compact
              className="flex-1"
            />
            <label className="flex h-[26px] flex-1 cursor-pointer items-center justify-end gap-1.5 text-[11px] text-ink-400">
              <span className="truncate">{t("editor.kvArgRequired")}</span>
              <Toggle on={row.required} onChange={(required) => patch(index, { required })} />
            </label>
          </div>
          <input
            value={row.label}
            onChange={(e) => patch(index, { label: e.target.value })}
            placeholder={t("editor.kvArgLabel")}
            aria-label={t("editor.kvArgLabel")}
            className={cn(inputCls, "font-sans")}
          />
        </div>
      ))}
      {rows.length === 0 && <EmptyHint text={t("editor.kvNoArguments")} />}
      <AddButton
        label={t("editor.kvAddArgument")}
        onClick={() => commit([...rows, { name: `arg_${rows.length + 1}`, label: "", kind: "string", required: true }])}
      />
    </div>
  );
}
