import { useMemo, useState } from "react";
import { Button, Toggle } from "./ui";
import { Dropdown } from "./Dropdown";
import { TextInput } from "./primitives/Field";
import {
  htmlExtractionsFromValue,
  nextMappingID,
  type HtmlExtractionValue,
} from "@/lib/blueprint-dynamic-pins";
import { useTranslation } from "react-i18next";

/**
 * Structured editor for the HTML Extract node's extractions list: CSS
 * selector, output label, return mode and per-extraction options. Replaces
 * the raw JSON textarea from the previous UI generation.
 */
export function HtmlExtractionsEditor({
  value,
  onChange,
}: {
  value: unknown;
  onChange: (next: unknown) => void;
}) {
  const { t } = useTranslation();
  const items = useMemo(() => htmlExtractionsFromValue(value), [value]);
  /** local text drafts so typing stays responsive while config round-trips */
  const [edits, setEdits] = useState<Record<string, string>>({});
  const editText = (key: string, v: string) => setEdits((e) => ({ ...e, [key]: v }));
  const text = (id: string, prop: string, fallback: string) => edits[`${id}:${prop}`] ?? fallback;

  const update = (index: number, patch: Partial<HtmlExtractionValue>) => {
    const next = items.map((item, current) => (current === index ? { ...item, ...patch } : item));
    onChange(next);
  };

  const add = () => {
    const item: HtmlExtractionValue = {
      id: nextMappingID(items),
      label: t("editor.htmlExtraction", { index: items.length + 1 }),
      selector: "",
      mode: "text",
      attribute: "",
      returnAll: false,
    };
    onChange([...items, item]);
  };

  const modeOptions = [
    { value: "text", label: t("editor.htmlModeText") },
    { value: "html", label: t("editor.htmlModeHtml") },
    { value: "attribute", label: t("editor.htmlModeAttribute") },
  ];

  if (items.length === 0) {
    return (
      <div className="space-y-2 rounded-md border border-ink-700 bg-ink-900/30 p-3">
        <p className="text-[11px] leading-4 text-ink-500">{t("editor.htmlEmptyHint")}</p>
        <Button variant="ghost" icon="Plus" onClick={add}>
          {t("editor.addExtraction")}
        </Button>
      </div>
    );
  }

  return (
    <div className="space-y-2 rounded-md border border-ink-700 bg-ink-900/30 p-3">
      {items.map((item, index) => (
        <article key={item.id} className="rounded-md border border-ink-700 bg-ink-950/60 p-2.5">
          <div className="mb-2 flex items-center justify-between gap-2">
            <span className="font-mono text-[10px] uppercase tracking-[0.12em] text-ink-600">{item.id}</span>
            <Button
              variant="ghost"
              icon="Trash2"
              onClick={() => onChange(items.filter((_, current) => current !== index))}
              className="h-6 px-2 text-[11px] text-ink-500 hover:text-rose-300"
            >
              {t("common.delete")}
            </Button>
          </div>

          <label className="block text-[11px] font-medium text-ink-300">
            {t("editor.htmlLabel")}
            <TextInput
              value={text(item.id, "label", item.label)}
              onChange={(v) => {
                editText(`${item.id}:label`, v);
                update(index, { label: v });
              }}
              placeholder={t("editor.htmlLabelPlaceholder")}
            />
          </label>

          <label className="mt-2 block text-[11px] font-medium text-ink-300">
            {t("editor.htmlSelector")}
            <TextInput
              mono
              value={text(item.id, "selector", item.selector)}
              onChange={(v) => {
                editText(`${item.id}:selector`, v);
                update(index, { selector: v });
              }}
              placeholder={t("editor.htmlSelectorPlaceholder")}
            />
          </label>

          <div className="mt-2 flex items-end gap-3">
            <label className="min-w-0 flex-1 text-[11px] font-medium text-ink-300">
              {t("editor.htmlMode")}
              <Dropdown
                value={item.mode}
                onChange={(mode) => update(index, { mode: mode as HtmlExtractionValue["mode"], attribute: mode === "attribute" ? item.attribute : "" })}
                options={modeOptions}
              />
            </label>
            <label className="flex h-8 shrink-0 items-center gap-2 rounded-md border border-ink-700 bg-ink-900/50 px-2">
              <span className="text-[11px] text-ink-300">{t("editor.htmlReturnAll")}</span>
              <Toggle on={item.returnAll} onChange={(on) => update(index, { returnAll: on })} />
            </label>
          </div>

          {item.mode === "attribute" ? (
            <label className="mt-2 block text-[11px] font-medium text-ink-300">
              {t("editor.htmlAttribute")}
              <TextInput
                mono
                value={text(item.id, "attribute", item.attribute)}
                onChange={(v) => {
                  editText(`${item.id}:attribute`, v);
                  update(index, { attribute: v });
                }}
                placeholder={t("editor.htmlAttributePlaceholder")}
              />
            </label>
          ) : null}
        </article>
      ))}
      <Button variant="ghost" icon="Plus" onClick={add}>
        {t("editor.addExtraction")}
      </Button>
    </div>
  );
}
