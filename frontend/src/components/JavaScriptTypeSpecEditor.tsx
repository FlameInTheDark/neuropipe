import { Plus, Trash2 } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import type { TypeFieldSpec, TypeKind, TypeSpec } from "@/lib/types";

const kinds: readonly TypeKind[] = [
  "any", "bool", "string", "int", "float", "bytes", "list", "map", "record",
];

function defaultType(kind: TypeKind): TypeSpec {
  switch (kind) {
    case "list": return { kind, element: { kind: "any" } };
    case "map": return { kind, key: { kind: "string" }, value: { kind: "any" } };
    case "record": return { kind, fields: [] };
    default: return { kind };
  }
}

function nextField(fields: readonly TypeFieldSpec[]) {
  const names = new Set(fields.map((field) => field.name || field.id));
  let index = fields.length + 1;
  while (names.has(`field${index}`)) index += 1;
  const name = `field${index}`;
  return { id: name, name, type: { kind: "any" } satisfies TypeSpec };
}

/** Visual editor for the JSON-safe TypeSpec persisted under a JavaScript pin. */
export function JavaScriptTypeSpecEditor({
  value,
  onChange,
  ariaLabel,
  depth = 0,
}: {
  value: TypeSpec;
  onChange: (type: TypeSpec) => void;
  ariaLabel: string;
  depth?: number;
}) {
  const { t } = useTranslation();
  const kind = kinds.includes(value.kind) ? value.kind : "any";
  const options = kinds.map((item) => ({
    value: item,
    label: t(`javascript.types.${item}`),
  }));
  const fields = value.fields ?? [];
  const updateField = (index: number, change: Partial<TypeFieldSpec>) => {
    onChange({
      ...value,
      kind: "record",
      fields: fields.map((field, current) => current === index ? { ...field, ...change } : field),
    });
  };
  const nestedClass = depth > 0 ? "mt-2 border-l border-zinc-800 pl-2.5" : "mt-2";

  return (
    <div className={depth > 0 ? nestedClass : undefined}>
      <Select
        value={kind}
        onValueChange={(next) => onChange(defaultType(next as TypeKind))}
        options={options}
        ariaLabel={ariaLabel}
      />
      {kind === "list" ? (
        <div className={nestedClass}>
          <p className="mb-1 text-[10px] font-medium uppercase tracking-wide text-zinc-500">{t("javascript.listElement")}</p>
          <JavaScriptTypeSpecEditor
            value={value.element ?? { kind: "any" }}
            onChange={(element) => onChange({ kind: "list", element })}
            ariaLabel={`${ariaLabel} ${t("javascript.listElement")}`}
            depth={depth + 1}
          />
        </div>
      ) : null}
      {kind === "map" ? (
        <div className={nestedClass}>
          <p className="mb-2 text-[11px] leading-4 text-zinc-500">{t("javascript.mapStringKeys")}</p>
          <p className="mb-1 text-[10px] font-medium uppercase tracking-wide text-zinc-500">{t("javascript.mapValue")}</p>
          <JavaScriptTypeSpecEditor
            value={value.value ?? { kind: "any" }}
            onChange={(nextValue) => onChange({ kind: "map", key: { kind: "string" }, value: nextValue })}
            ariaLabel={`${ariaLabel} ${t("javascript.mapValue")}`}
            depth={depth + 1}
          />
        </div>
      ) : null}
      {kind === "record" ? (
        <div className={nestedClass}>
          <div className="flex items-center justify-between gap-2">
            <p className="text-[10px] font-medium uppercase tracking-wide text-zinc-500">{t("javascript.recordFields")}</p>
            <Button
              type="button"
              size="sm"
              variant="ghost"
              className="h-7 px-2 text-[11px]"
              onClick={() => onChange({ ...value, kind: "record", fields: [...fields, nextField(fields)] })}
            >
              <Plus className="size-3" /> {t("javascript.addField")}
            </Button>
          </div>
          <div className="mt-2 space-y-2">
            {fields.map((field, index) => (
              <div key={`${field.id}-${index}`} className="rounded border border-zinc-800 bg-zinc-950/60 p-2">
                <div className="flex items-end gap-1.5">
                  <label className="min-w-0 flex-1 text-[10px] font-medium uppercase tracking-wide text-zinc-500">
                    {t("javascript.recordFieldName")}
                    <Input
                      value={field.name || field.id}
                      onChange={(event) => {
                        const name = event.target.value;
                        updateField(index, { id: name, name });
                      }}
                      className="mt-1 h-8 font-mono text-xs"
                      aria-label={`${ariaLabel} ${t("javascript.recordFieldName")} ${index + 1}`}
                    />
                  </label>
                  <button
                    type="button"
                    onClick={() => onChange({ ...value, kind: "record", fields: fields.filter((_, current) => current !== index) })}
                    className="rounded p-1.5 text-zinc-500 hover:bg-zinc-800 hover:text-red-200 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-zinc-400"
                    aria-label={`${t("common.delete")} ${field.name || field.id || index + 1}`}
                  >
                    <Trash2 className="size-3.5" />
                  </button>
                </div>
                <div className="mt-2">
                  <JavaScriptTypeSpecEditor
                    value={field.type}
                    onChange={(type) => updateField(index, { type })}
                    ariaLabel={`${ariaLabel} ${t("javascript.recordFieldName")} ${index + 1} ${t("javascript.type")}`}
                    depth={depth + 1}
                  />
                </div>
                <div className="mt-2 flex items-center justify-between gap-2">
                  <span className="text-[11px] text-zinc-400">{t("javascript.optional")}</span>
                  <Switch
                    checked={field.optional === true}
                    onCheckedChange={(optional) => updateField(index, { optional })}
                    label={`${field.name || field.id || index + 1} ${t("javascript.optional")}`}
                  />
                </div>
              </div>
            ))}
          </div>
        </div>
      ) : null}
    </div>
  );
}
