import { useMemo } from "react";
import { Plus, Trash2, WandSparkles } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Tooltip } from "@/components/ui/tooltip";
import {
  fieldOutputsFromValue,
  htmlExtractionsFromValue,
  nextMappingID,
  objectFieldsFromValue,
  type FieldOutputValue,
  type HtmlExtractionValue,
  type HtmlReturnMode,
  type ObjectFieldValue,
} from "@/lib/blueprint-dynamic-pins";
import type { DataField, DataType } from "@/lib/types";

const dataTypes: readonly DataType[] = [
  "any",
  "text",
  "number",
  "boolean",
  "object",
  "list",
];

function MappingShell({ children }: { children: React.ReactNode }) {
  return (
    <div className="space-y-2 rounded-md border border-zinc-800 bg-zinc-900/30 p-2.5">
      {children}
    </div>
  );
}

function MappingHeader({
  title,
  index,
  canRemove,
  onRemove,
}: {
  title: string;
  index: number;
  canRemove: boolean;
  onRemove: () => void;
}) {
  const { t } = useTranslation();
  return (
    <div className="mb-2 flex items-center justify-between gap-2">
      <span className="text-[11px] font-medium text-zinc-300">
        {title} {index + 1}
      </span>
      <button
        type="button"
        className="rounded p-1 text-zinc-500 hover:bg-zinc-800 hover:text-zinc-200 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-zinc-400 disabled:cursor-not-allowed disabled:opacity-40"
        aria-label={t("editorActions.delete")}
        disabled={!canRemove}
        onClick={onRemove}
      >
        <Trash2 className="size-3.5" />
      </button>
    </div>
  );
}

function MappingTextField({
  label,
  value,
  placeholder,
  ariaLabel,
  mono = false,
  onChange,
}: {
  label: string;
  value: string;
  placeholder: string;
  ariaLabel: string;
  mono?: boolean;
  onChange: (value: string) => void;
}) {
  return (
    <label className="mb-2 block">
      <span className="mb-1 block text-[10px] font-medium text-zinc-500">
        {label}
      </span>
      <Input
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
        aria-label={ariaLabel}
        className={mono ? "font-mono text-xs" : undefined}
      />
    </label>
  );
}

function DataTypeField({
  title,
  index,
  value,
  onChange,
}: {
  title: string;
  index: number;
  value: DataType;
  onChange: (value: DataType) => void;
}) {
  const { t } = useTranslation();
  const options = useMemo(
    () => dataTypes.map((value) => ({ value, label: t(`editor.${value}`) })),
    [t],
  );
  return (
    <label className="block">
      <span className="mb-1 block text-[10px] font-medium text-zinc-500">
        {t("editor.dataType")}
      </span>
      <Select
        value={value}
        onValueChange={(dataType) => onChange(dataType as DataType)}
        options={options}
        ariaLabel={`${title} ${index + 1} ${t("editor.dataType")}`}
      />
    </label>
  );
}

export function FieldOutputsEditor({
  value,
  sourceFields,
  onChange,
}: {
  value: unknown;
  sourceFields: readonly DataField[];
  onChange: (value: FieldOutputValue[]) => void;
}) {
  const { t } = useTranslation();
  const outputs = fieldOutputsFromValue(value);
  const update = (index: number, patch: Partial<FieldOutputValue>) => {
    onChange(outputs.map((item, current) => (current === index ? { ...item, ...patch } : item)));
  };
  const sourceMapping = (field: DataField) => ({
    label: field.label?.trim() || field.path.split(".").at(-1) || t("editor.output"),
    path: field.path,
    dataType: field.dataType,
  });
  const useSourceField = (field: DataField) => {
    const next = sourceMapping(field);
    const target = outputs.findIndex((output) => !output.path || output.path === "value");
    if (target >= 0) {
      update(target, next);
      return;
    }
    onChange([...outputs, { id: nextMappingID(outputs), ...next }]);
  };
  const configureKnownFields = () => {
    let next = [...outputs];
    for (const field of sourceFields) {
      const mapping = sourceMapping(field);
      const existing = next.findIndex((output) => output.path === mapping.path);
      if (existing >= 0) continue;
      const placeholder = next.findIndex(
        (output) => !output.path || output.path === "value",
      );
      if (placeholder >= 0) {
        next[placeholder] = { ...next[placeholder], ...mapping };
      } else {
        next = [...next, { id: nextMappingID(next), ...mapping }];
      }
    }
    onChange(next);
  };
  return (
    <MappingShell>
      {sourceFields.length > 0 ? (
        <div className="rounded-md border border-zinc-800 bg-zinc-950/60 p-2">
          <div className="flex items-center justify-between gap-2">
            <p className="text-[10px] font-semibold uppercase tracking-[.12em] text-zinc-500">
              {t("editor.availableFromSource")}
            </p>
            <Tooltip content={t("objectMappings.autoConfigure")} side="bottom" align="end" wrap={false}>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className="size-7 shrink-0 p-0"
                aria-label={t("objectMappings.autoConfigure")}
                onClick={configureKnownFields}
              >
                <WandSparkles className="size-3.5" />
              </Button>
            </Tooltip>
          </div>
          <div className="mt-1.5 flex flex-wrap gap-1">
            {sourceFields.map((field) => (
              <Tooltip key={field.path} content={field.description || field.path} side="top" align="start">
                <button type="button" onClick={() => useSourceField(field)} className="inline-flex max-w-full items-center gap-1 rounded border border-zinc-700 bg-zinc-900 px-1.5 py-1 font-mono text-[10px] text-zinc-300 hover:border-zinc-500 hover:bg-zinc-800 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-zinc-400">
                  <span className="truncate">{field.path}</span>
                  <span className="shrink-0 font-sans text-zinc-500">{t(`editor.${field.dataType}`)}</span>
                </button>
              </Tooltip>
            ))}
          </div>
        </div>
      ) : null}
      {outputs.map((output, index) => (
        <div key={output.id} className="rounded-md border border-zinc-800 bg-zinc-950/60 p-2">
          <MappingHeader title={t("editor.output")} index={index} canRemove={outputs.length > 1} onRemove={() => onChange(outputs.filter((_, current) => current !== index))} />
          <MappingTextField label={t("editor.pinName")} value={output.label} placeholder={t("editor.output")} ariaLabel={`${t("editor.output")} ${index + 1} ${t("editor.pinName")}`} onChange={(label) => update(index, { label })} />
          <MappingTextField label={t("editor.fieldPath")} value={output.path} placeholder={t("editorExtra.exampleFieldPath")} ariaLabel={`${t("editor.output")} ${index + 1} ${t("editor.fieldPath")}`} mono onChange={(path) => update(index, { path })} />
          <DataTypeField title={t("editor.output")} index={index} value={output.dataType} onChange={(dataType) => update(index, { dataType })} />
        </div>
      ))}
      <Button type="button" variant="ghost" size="sm" onClick={() => onChange([...outputs, { id: nextMappingID(outputs), label: `${t("editor.output")} ${outputs.length + 1}`, path: "", dataType: "any" }])}>
        <Plus className="size-3.5" />
        {t("objectMappings.addOutput")}
      </Button>
    </MappingShell>
  );
}

export function ObjectFieldsEditor({
  value,
  onChange,
}: {
  value: unknown;
  onChange: (value: ObjectFieldValue[]) => void;
}) {
  const { t } = useTranslation();
  const fields = objectFieldsFromValue(value);
  const update = (index: number, patch: Partial<ObjectFieldValue>) => {
    onChange(fields.map((item, current) => (current === index ? { ...item, ...patch } : item)));
  };
  return (
    <MappingShell>
      {fields.map((field, index) => (
        <div key={field.id} className="rounded-md border border-zinc-800 bg-zinc-950/60 p-2">
          <MappingHeader title={t("editor.input")} index={index} canRemove={fields.length > 1} onRemove={() => onChange(fields.filter((_, current) => current !== index))} />
          <MappingTextField label={t("editor.pinName")} value={field.label} placeholder={t("editor.input")} ariaLabel={`${t("editor.input")} ${index + 1} ${t("editor.pinName")}`} onChange={(label) => update(index, { label })} />
          <MappingTextField label={t("objectMappings.objectKey")} value={field.key} placeholder={t("objectMappings.exampleObjectKey")} ariaLabel={`${t("editor.input")} ${index + 1} ${t("objectMappings.objectKey")}`} mono onChange={(key) => update(index, { key })} />
          <DataTypeField title={t("editor.input")} index={index} value={field.dataType} onChange={(dataType) => update(index, { dataType })} />
        </div>
      ))}
      <Button type="button" variant="ghost" size="sm" onClick={() => onChange([...fields, { id: nextMappingID(fields), label: `${t("editor.input")} ${fields.length + 1}`, key: "", dataType: "any" }])}>
        <Plus className="size-3.5" />
        {t("objectMappings.addInput")}
      </Button>
    </MappingShell>
  );
}

export function HtmlExtractionsEditor({
  value,
  onChange,
}: {
  value: unknown;
  onChange: (value: HtmlExtractionValue[]) => void;
}) {
  const { t } = useTranslation();
  const extractions = htmlExtractionsFromValue(value);
  const update = (index: number, patch: Partial<HtmlExtractionValue>) => {
    onChange(extractions.map((item, current) => (current === index ? { ...item, ...patch } : item)));
  };
  const modeOptions = useMemo(
    () => (["text", "html", "attribute"] as const).map((mode) => ({
      value: mode,
      label: t(`htmlExtractions.mode_${mode}`),
    })),
    [t],
  );
  return (
    <MappingShell>
      {extractions.map((extraction, index) => (
        <div key={extraction.id} className="rounded-md border border-zinc-800 bg-zinc-950/60 p-2">
          <MappingHeader title={t("htmlExtractions.extraction")} index={index} canRemove={extractions.length > 1} onRemove={() => onChange(extractions.filter((_, current) => current !== index))} />
          <MappingTextField label={t("editor.pinName")} value={extraction.label} placeholder={t("editor.output")} ariaLabel={`${t("htmlExtractions.extraction")} ${index + 1} ${t("editor.pinName")}`} onChange={(label) => update(index, { label })} />
          <MappingTextField label={t("htmlExtractions.selector")} value={extraction.selector} placeholder={t("htmlExtractions.exampleSelector")} ariaLabel={`${t("htmlExtractions.extraction")} ${index + 1} ${t("htmlExtractions.selector")}`} mono onChange={(selector) => update(index, { selector })} />
          <label className="mb-2 block">
            <span className="mb-1 block text-[10px] font-medium text-zinc-500">
              {t("htmlExtractions.returnValue")}
            </span>
            <Select
              value={extraction.mode}
              onValueChange={(mode) => update(index, { mode: mode as HtmlReturnMode })}
              options={modeOptions}
              ariaLabel={`${t("htmlExtractions.extraction")} ${index + 1} ${t("htmlExtractions.returnValue")}`}
            />
          </label>
          {extraction.mode === "attribute" ? (
            <MappingTextField label={t("htmlExtractions.attributeName")} value={extraction.attribute} placeholder="href" ariaLabel={`${t("htmlExtractions.extraction")} ${index + 1} ${t("htmlExtractions.attributeName")}`} mono onChange={(attribute) => update(index, { attribute })} />
          ) : null}
          <div className="flex h-9 items-center justify-between rounded-md border border-zinc-800 bg-zinc-900/40 px-2.5">
            <span className="text-xs text-zinc-500">
              {t("htmlExtractions.returnAll")}
            </span>
            <Switch
              checked={extraction.returnAll}
              onCheckedChange={(checked) => update(index, { returnAll: checked })}
              label={t("htmlExtractions.returnAll")}
            />
          </div>
          <p className="mt-1.5 text-[10px] text-zinc-600">
            {extraction.returnAll ? t("editor.list") : t("editor.text")}
          </p>
        </div>
      ))}
      <Button type="button" variant="ghost" size="sm" onClick={() => onChange([...extractions, { id: nextMappingID(extractions), label: `${t("editor.output")} ${extractions.length + 1}`, selector: "", mode: "text", attribute: "", returnAll: false }])}>
        <Plus className="size-3.5" />
        {t("htmlExtractions.addExtraction")}
      </Button>
    </MappingShell>
  );
}
