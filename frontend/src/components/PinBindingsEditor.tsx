import { useTranslation } from "react-i18next";
import { Icon } from "./icons";
import { cn } from "../utils/cn";
import {
  AddButton,
  EditorLabel,
  EmptyHint,
  inputCls,
  useDraftRows,
} from "./KVFieldEditors";
import {
  buildPinBindingsPayload,
  nextPinBindingID,
  parsePinBindings,
  type PinBindingRow,
} from "../lib/document-pins";

/**
 * Pin-row editor for the Documents nodes' dynamic value pins — the visual
 * replacement for hand-typed JSON binding lists. Every change re-derives the
 * node's dynamic input or output pins through the resolver twins, exactly
 * like the KV Command arguments editor.
 *
 * Input mode rows carry a literal Value column ("constant used when the pin
 * is not wired"); output pins have no value, so the column is hidden.
 */
export function PinBindingsEditor({
  label,
  value,
  onChange,
  mode,
  placeholder,
}: {
  label: string;
  value: unknown;
  onChange: (next: unknown) => void;
  mode: "input" | "output";
  placeholder?: string;
}) {
  const { t } = useTranslation();
  const { rows, commit } = useDraftRows(value, parsePinBindings, buildPinBindingsPayload, onChange);
  const patch = (index: number, next: Partial<PinBindingRow>) =>
    commit(rows.map((row, i) => (i === index ? { ...row, ...next } : row)));

  return (
    <div className="space-y-2">
      <EditorLabel count={rows.length}>{label}</EditorLabel>
      {rows.map((row, index) => (
        // index-keyed: content-derived keys remount the row mid-keystroke and
        // steal focus from the input being edited
        <div
          key={index}
          className="space-y-1.5 rounded-md border border-ink-700/70 bg-ink-850/60 p-2"
        >
          <div className="flex items-center gap-1.5">
            <input
              value={row.name}
              onChange={(e) => patch(index, { name: e.target.value })}
              placeholder={placeholder ?? (mode === "output" ? "B4" : "customer")}
              aria-label={t("editor.pinNameColumn")}
              className={cn(inputCls, "flex-[2]")}
            />
            <button
              type="button"
              onClick={() => commit(rows.filter((_, i) => i !== index))}
              aria-label={t("common.delete")}
              className="grid h-7 w-7 shrink-0 place-items-center rounded-md text-fg-faint transition hover:bg-danger/15 hover:text-danger-fg"
            >
              <Icon name="Trash2" className="h-3.5 w-3.5" />
            </button>
          </div>
          <div className="flex items-center gap-1.5">
            <input
              value={row.label}
              onChange={(e) => patch(index, { label: e.target.value })}
              placeholder={t("editor.pinLabelPlaceholder")}
              aria-label={t("editor.kvFieldColumn")}
              className={cn(inputCls, "font-sans", mode === "input" ? "flex-1" : "flex-[2]")}
            />
            {mode === "input" && (
              <input
                value={row.value}
                onChange={(e) => patch(index, { value: e.target.value })}
                placeholder={t("editor.pinValuePlaceholder")}
                aria-label={t("editor.kvValueColumn")}
                className={cn(inputCls, "flex-1")}
              />
            )}
          </div>
        </div>
      ))}
      {rows.length === 0 && <EmptyHint text={t("editor.pinNoRows")} />}
      <AddButton
        label={mode === "output" ? t("editor.pinAddOutputRow") : t("editor.pinAddRow")}
        onClick={() =>
          commit([...rows, { id: nextPinBindingID(rows), name: "", label: "", value: "" }])
        }
      />
    </div>
  );
}
