import { useMemo } from "react";
import { Button } from "./ui";
import { Dropdown } from "./Dropdown";
import { TextInput } from "./primitives/Field";
import { Toggle } from "./ui";
import {
  routeOptionsFromValue,
  switchConfigFromValue,
  type RouteOptionValue,
  type SwitchCaseValue,
  type SwitchComparator,
  type SwitchConfigValue,
} from "@/lib/blueprint-dynamic-pins";
import { ask } from "@/stores/confirmation";
import { useTranslation } from "react-i18next";

/* ------------------------------------------------------------------ */
/* Schema editor (Structured Extract "Fields to extract")              */
/* ------------------------------------------------------------------ */

type SchemaType = "object" | "array" | "string" | "number" | "boolean";

interface SchemaProperty {
  name: string;
  type: SchemaType;
  required: boolean;
  description: string;
}

interface ObjectSchemaValue {
  type: SchemaType;
  properties: SchemaProperty[];
}

const schemaTypes: { value: SchemaType; labelKey: string }[] = [
  { value: "object", labelKey: "editor.object" },
  { value: "array", labelKey: "editor.list" },
  { value: "string", labelKey: "editor.text" },
  { value: "number", labelKey: "editor.number" },
  { value: "boolean", labelKey: "editor.boolean" },
];

function parseStructuredValue(value: unknown): unknown {
  if (typeof value !== "string") return value;
  try {
    return JSON.parse(value);
  } catch {
    return undefined;
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

export function schemaFromValue(value: unknown): ObjectSchemaValue {
  const parsed = parseStructuredValue(value);
  if (!isRecord(parsed)) return { type: "object", properties: [] };
  const type = schemaTypes.some((item) => item.value === parsed.type)
    ? (parsed.type as SchemaType)
    : "object";
  const required = new Set(
    Array.isArray(parsed.required)
      ? parsed.required.filter((item): item is string => typeof item === "string")
      : [],
  );
  const properties = isRecord(parsed.properties)
    ? Object.entries(parsed.properties).map(([name, property]) => ({
        name,
        type:
          isRecord(property) && schemaTypes.some((item) => item.value === property.type)
            ? (property.type as SchemaType)
            : ("string" as SchemaType),
        required: required.has(name),
        description:
          isRecord(property) && typeof property.description === "string"
            ? property.description
            : "",
      }))
    : [];
  return { type, properties };
}

export function schemaToValue(schema: ObjectSchemaValue): Record<string, unknown> {
  const properties = Object.fromEntries(
    schema.properties
      .filter((property) => property.name.trim())
      .map((property) => {
        const description = property.description.trim();
        return [
          property.name.trim(),
          { type: property.type, ...(description ? { description } : {}) },
        ];
      }),
  );
  if (schema.type !== "object") return { type: schema.type };
  const required = schema.properties
    .filter((property) => property.required && property.name.trim())
    .map((property) => property.name.trim());
  return { type: "object", properties, ...(required.length > 0 ? { required } : {}) };
}

export function SchemaEditor({
  value,
  onChange,
}: {
  value: unknown;
  onChange: (value: Record<string, unknown>) => void;
}) {
  const { t } = useTranslation();
  const schemaTypeOptions = useMemo(
    () =>
      schemaTypes.map((item) => ({
        value: item.value,
        label: t(item.labelKey),
      })),
    [t],
  );
  const schema = schemaFromValue(value);
  const update = (patch: Partial<ObjectSchemaValue>) => {
    onChange(schemaToValue({ ...schema, ...patch }));
  };

  return (
    <div className="space-y-3 rounded-md border border-ink-700 bg-ink-900/30 p-3">
      <div className="flex items-center justify-between gap-3">
        <span className="text-xs font-medium text-ink-300">{t("editor.responseShape")}</span>
        <Dropdown
          className="w-36 shrink-0"
          value={schema.type}
          onChange={(type) => update({ type: type as SchemaType })}
          options={schemaTypeOptions}
        />
      </div>
      {schema.type === "object" ? (
        <>
          <div className="flex items-center justify-between">
            <span className="text-xs font-medium text-ink-200">{t("editor.fields")}</span>
            <span className="text-[10px] text-ink-600">{schema.properties.length}</span>
          </div>
          {schema.properties.map((property, index) => (
            <article key={`${property.name}-${index}`} className="rounded-md border border-ink-700 bg-ink-950/60 p-2.5">
              <div className="mb-2 flex items-center justify-between">
                <span className="text-[10px] font-semibold uppercase tracking-[0.12em] text-ink-600">
                  {t("editorExtra.field")} {index + 1}
                </span>
                <Button
                  variant="ghost"
                  icon="Trash2"
                  onClick={() => update({ properties: schema.properties.filter((_, current) => current !== index) })}
                  className="h-6 px-2 text-ink-500 hover:text-rose-300"
                >
                  {t("common.delete")}
                </Button>
              </div>
              <label className="block text-[11px] font-medium text-ink-300">
                {t("editorExtra.fieldName")}
                <TextInput
                  value={property.name}
                  onChange={(v) => updateProperty(index, { name: v })}
                  placeholder={t("editorExtra.exampleFieldName")}
                />
              </label>
              <label className="mt-2 block text-[11px] font-medium text-ink-300">
                {t("editor.guidance")} <span className="font-normal text-ink-600">({t("editor.optional")})</span>
                <textarea
                  value={property.description}
                  onChange={(event) => updateProperty(index, { description: event.target.value })}
                  placeholder={t("editor.fieldGuidancePlaceholder")}
                  className="mt-1 min-h-16 w-full resize-y rounded-md border border-ink-700 bg-ink-950 px-2.5 py-2 text-xs leading-5 text-ink-100 outline-none placeholder:text-ink-600 transition focus:border-ink-500"
                />
              </label>
              <div className="mt-2 flex items-end justify-between gap-3">
                <label className="min-w-0 flex-1 text-[11px] font-medium text-ink-300">
                  {t("editor.valueType")}
                  <Dropdown
                    value={property.type}
                    onChange={(type) => updateProperty(index, { type: type as SchemaType })}
                    options={schemaTypeOptions.filter((item) => item.value !== "object")}
                  />
                </label>
                <div className="flex h-8 shrink-0 items-center gap-2 rounded-md border border-ink-700 bg-ink-900/50 px-2">
                  <span className="text-[11px] text-ink-300">{t("editor.required")}</span>
                  <Toggle on={property.required} onChange={(required) => updateProperty(index, { required })} />
                </div>
              </div>
            </article>
          ))}
          <Button
            variant="ghost"
            icon="Plus"
            onClick={() => {
              const existing = new Set(schema.properties.map((property) => property.name));
              let index = schema.properties.length + 1;
              let name = `field${index}`;
              while (existing.has(name)) {
                index += 1;
                name = `field${index}`;
              }
              update({
                properties: [...schema.properties, { name, type: "string", required: true, description: "" }],
              });
            }}
          >
            {t("editor.addField")}
          </Button>
        </>
      ) : (
        <p className="rounded-md border border-ink-700 bg-ink-950/60 px-2.5 py-2 text-[11px] leading-4 text-ink-500">
          {t("editor.schemaScalarHint", { type: t(schemaTypes.find((s) => s.value === schema.type)?.labelKey ?? "editor.text") })}
        </p>
      )}
    </div>
  );

  function updateProperty(index: number, patch: Partial<SchemaProperty>) {
    update({
      properties: schema.properties.map((property, current) =>
        current === index ? { ...property, ...patch } : property,
      ),
    });
  }
}

/* ------------------------------------------------------------------ */
/* Route options editor (LLM Choice options)                           */
/* ------------------------------------------------------------------ */

function nextRouteOptionID(options: readonly RouteOptionValue[]): string {
  let index = options.length + 1;
  let id = `option-${index}`;
  const used = new Set(options.map((option) => option.id));
  while (used.has(id)) {
    index += 1;
    id = `option-${index}`;
  }
  return id;
}

export function RouteOptionsEditor({
  value,
  onChange,
}: {
  value: unknown;
  onChange: (value: RouteOptionValue[]) => void;
}) {
  const { t } = useTranslation();
  const options = routeOptionsFromValue(value);
  const duplicateIDs = options.filter(
    (option, index) => options.findIndex((candidate) => candidate.id === option.id) !== index,
  );
  const update = (index: number, patch: Partial<RouteOptionValue>) => {
    onChange(options.map((option, current) => (current === index ? { ...option, ...patch } : option)));
  };

  return (
    <div className="space-y-3 rounded-md border border-ink-700 bg-ink-900/30 p-3">
      <p className="text-[11px] leading-4 text-ink-500">{t("editorExtra.optionHelp")}</p>
      {options.map((option, index) => (
        <article key={`${option.id}-${index}`} className="rounded-md border border-ink-700 bg-ink-950/60 p-2.5">
          <div className="mb-2 flex items-center justify-between">
            <div>
              <span className="text-[10px] font-semibold uppercase tracking-[0.12em] text-ink-600">
                {t("editorExtra.option")} {index + 1}
              </span>
              <p className="mt-0.5 text-[10px] text-ink-600">{t("editorExtra.optionCreatesOutput")}</p>
            </div>
            <Button
              variant="ghost"
              icon="Trash2"
              onClick={() => onChange(options.filter((_, current) => current !== index))}
              className="h-6 px-2 text-ink-500 hover:text-rose-300"
            >
              {t("common.delete")}
            </Button>
          </div>
          <label className="block text-[11px] font-medium text-ink-300">
            {t("editor.outputId")}
            <TextInput
              value={option.id}
              onChange={(v) => update(index, { id: v })}
              placeholder={t("editorExtra.exampleOptionID")}
            />
          </label>
          <label className="mt-2 block text-[11px] font-medium text-ink-300">
            {t("editor.displayName")}
            <TextInput
              value={option.label}
              onChange={(v) => update(index, { label: v })}
              placeholder={t("editorExtra.exampleOptionName")}
            />
          </label>
          <label className="mt-2 block text-[11px] font-medium text-ink-300">
            {t("editor.guidance")} <span className="font-normal text-ink-600">({t("editor.optional")})</span>
            <textarea
              value={option.description}
              onChange={(event) => update(index, { description: event.target.value })}
              placeholder={t("editor.guidancePlaceholder")}
              className="mt-1 min-h-16 w-full resize-y rounded-md border border-ink-700 bg-ink-950 px-2.5 py-2 text-xs leading-5 text-ink-100 outline-none placeholder:text-ink-600 transition focus:border-ink-500"
            />
          </label>
        </article>
      ))}
      {duplicateIDs.length > 0 ? (
        <p className="text-[11px] text-rose-300">{t("editor.optionIdsUnique")}</p>
      ) : null}
      <Button
        variant="ghost"
        icon="Plus"
        onClick={() => {
          const id = nextRouteOptionID(options);
          onChange([
            ...options,
            { id, label: `${t("editorExtra.option")} ${options.length + 1}`, description: "" },
          ]);
        }}
      >
        {t("editor.addOption")}
      </Button>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* Switch cases editor                                                 */
/* ------------------------------------------------------------------ */

const comparatorValueTypes: Record<SwitchComparator, SwitchCaseValue["valueType"][]> = {
  equals: ["text", "number", "boolean"],
  not_equals: ["text", "number", "boolean"],
  contains: ["text"],
  starts_with: ["text"],
  ends_with: ["text"],
  greater_than: ["number"],
  greater_than_or_equal: ["number"],
  less_than: ["number"],
  less_than_or_equal: ["number"],
};

function nextCaseID(cases: readonly SwitchCaseValue[]): string {
  const used = new Set(cases.map((item) => item.id));
  for (let index = cases.length + 1; ; index += 1) {
    const id = `case-${index}`;
    if (!used.has(id)) return id;
  }
}

function valueForType(
  value: SwitchCaseValue["value"],
  valueType: SwitchCaseValue["valueType"],
): SwitchCaseValue["value"] {
  if (valueType === "boolean") return value === true;
  if (valueType === "number") {
    return typeof value === "number" && Number.isFinite(value) ? value : 0;
  }
  return typeof value === "string" ? value : String(value);
}

function normalizeCases(cases: readonly SwitchCaseValue[], comparator: SwitchComparator) {
  const allowedTypes = comparatorValueTypes[comparator];
  return cases.map((item) => {
    const valueType = allowedTypes.includes(item.valueType) ? item.valueType : allowedTypes[0];
    return { ...item, valueType, value: valueForType(item.value, valueType) };
  });
}

export function SwitchCasesEditor({
  value,
  legacyOptions,
  onChange,
}: {
  value: unknown;
  legacyOptions?: unknown;
  onChange: (value: SwitchConfigValue) => void;
}) {
  const { t } = useTranslation();
  const configuration = switchConfigFromValue(value, legacyOptions);
  const comparatorOptions = useMemo(
    () =>
      [
        "equals",
        "not_equals",
        "contains",
        "starts_with",
        "ends_with",
        "greater_than",
        "greater_than_or_equal",
        "less_than",
        "less_than_or_equal",
      ].map((comparator) => ({
        value: comparator,
        label: t(`switchCases.comparators.${comparator}`),
      })),
    [t],
  );
  const update = (patch: Partial<SwitchConfigValue>) => onChange({ ...configuration, ...patch });
  const updateCase = (index: number, patch: Partial<SwitchCaseValue>) =>
    update({
      cases: configuration.cases.map((item, current) => (current === index ? { ...item, ...patch } : item)),
    });
  const moveCase = (index: number, direction: -1 | 1) => {
    const target = index + direction;
    if (target < 0 || target >= configuration.cases.length) return;
    const cases = [...configuration.cases];
    [cases[index], cases[target]] = [cases[target], cases[index]];
    update({ cases });
  };
  const removeCase = async (index: number) => {
    const item = configuration.cases[index];
    if (!item || configuration.cases.length <= 1) return;
    const confirmed = await ask({
      title: t("switchCases.deleteTitle"),
      description: t("switchCases.deleteDescription", { name: item.label }),
      confirmLabel: t("switchCases.deleteConfirm"),
    });
    if (!confirmed) return;
    update({ cases: configuration.cases.filter((_, current) => current !== index) });
  };
  const allowedTypes = comparatorValueTypes[configuration.comparator];
  const valueTypeOptions = allowedTypes.map((valueType) => ({
    value: valueType,
    label: t(`editor.${valueType}`),
  }));

  return (
    <div className="space-y-2.5 rounded-md border border-ink-700 bg-ink-900/30 p-2.5">
      <label className="block">
        <span className="mb-1 block text-[10px] font-medium text-ink-500">{t("switchCases.comparator")}</span>
        <Dropdown
          value={configuration.comparator}
          onChange={(next) => {
            const comparator = next as SwitchComparator;
            update({ comparator, cases: normalizeCases(configuration.cases, comparator) });
          }}
          options={comparatorOptions}
        />
      </label>
      {configuration.cases.map((item, index) => (
        <article key={item.id} className="rounded-md border border-ink-700 bg-ink-950/60 p-2">
          <div className="mb-2 flex items-center justify-between gap-2">
            <span className="text-[11px] font-medium text-ink-200">{t("switchCases.case", { index: index + 1 })}</span>
            <div className="flex items-center gap-0.5">
              <Button
                variant="ghost"
                icon="ChevronUp"
                disabled={index === 0}
                onClick={() => moveCase(index, -1)}
              >
                {""}
              </Button>
              <Button
                variant="ghost"
                icon="ChevronDown"
                disabled={index === configuration.cases.length - 1}
                onClick={() => moveCase(index, 1)}
              >
                {""}
              </Button>
              <Button
                variant="ghost"
                icon="Trash2"
                disabled={configuration.cases.length <= 1}
                onClick={() => void removeCase(index)}
                className="text-ink-500 hover:text-rose-300"
              >
                {""}
              </Button>
            </div>
          </div>
          <div className="grid grid-cols-1 gap-2">
            <label className="block">
              <span className="mb-1 block text-[10px] font-medium text-ink-500">{t("switchCases.value")}</span>
              {item.valueType === "boolean" ? (
                <Dropdown
                  value={String(item.value)}
                  onChange={(next) => updateCase(index, { value: next === "true" })}
                  options={[
                    { value: "true", label: t("switchCases.true") },
                    { value: "false", label: t("switchCases.false") },
                  ]}
                />
              ) : (
                <TextInput
                  value={String(item.value)}
                  onChange={(v) =>
                    updateCase(index, {
                      value: item.valueType === "number" ? (v === "" ? "" : Number(v)) : v,
                    })
                  }
                />
              )}
            </label>
            {allowedTypes.length > 1 ? (
              <label className="block">
                <span className="mb-1 block text-[10px] font-medium text-ink-500">{t("editor.valueType")}</span>
                <Dropdown
                  value={item.valueType}
                  onChange={(next) => {
                    const valueType = next as SwitchCaseValue["valueType"];
                    updateCase(index, { valueType, value: valueForType(item.value, valueType) });
                  }}
                  options={valueTypeOptions}
                />
              </label>
            ) : null}
            <label className="block">
              <span className="mb-1 block text-[10px] font-medium text-ink-500">{t("editor.pinName")}</span>
              <TextInput
                value={item.label}
                onChange={(v) => updateCase(index, { label: v })}
                placeholder={t("switchCases.pinNamePlaceholder")}
              />
            </label>
          </div>
        </article>
      ))}
      <Button
        variant="ghost"
        icon="Plus"
        onClick={() => {
          const id = nextCaseID(configuration.cases);
          update({
            cases: [
              ...configuration.cases,
              {
                id,
                label: t("switchCases.caseName", { index: configuration.cases.length + 1 }),
                valueType: allowedTypes[0],
                value: valueForType("", allowedTypes[0]),
              },
            ],
          });
        }}
      >
        {t("switchCases.addCase")}
      </Button>
    </div>
  );
}
