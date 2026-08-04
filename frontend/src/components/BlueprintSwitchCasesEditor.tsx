import { useMemo } from "react";
import { ArrowDown, ArrowUp, Plus, Trash2 } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Tooltip } from "@/components/ui/tooltip";
import {
  switchConfigFromValue,
  type SwitchCaseValue,
  type SwitchComparator,
  type SwitchConfigValue,
} from "@/lib/blueprint-dynamic-pins";
import { useConfirmationStore } from "@/stores/confirmation";

const comparatorValueTypes: Record<
  SwitchComparator,
  SwitchCaseValue["valueType"][]
> = {
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

function nextCaseID(cases: readonly SwitchCaseValue[]) {
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

function normalizeCases(
  cases: readonly SwitchCaseValue[],
  comparator: SwitchComparator,
) {
  const allowedTypes = comparatorValueTypes[comparator];
  return cases.map((item) => {
    const valueType = allowedTypes.includes(item.valueType)
      ? item.valueType
      : allowedTypes[0];
    return { ...item, valueType, value: valueForType(item.value, valueType) };
  });
}

export function BlueprintSwitchCasesEditor({
  value,
  legacyOptions,
  onChange,
}: {
  value: unknown;
  legacyOptions?: unknown;
  onChange: (value: SwitchConfigValue) => void;
}) {
  const { t } = useTranslation();
  const ask = useConfirmationStore((state) => state.ask);
  const configuration = switchConfigFromValue(value, legacyOptions);
  const comparatorOptions = useMemo(
    () => [
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
  const update = (patch: Partial<SwitchConfigValue>) =>
    onChange({ ...configuration, ...patch });
  const updateCase = (index: number, patch: Partial<SwitchCaseValue>) =>
    update({
      cases: configuration.cases.map((item, current) =>
        current === index ? { ...item, ...patch } : item,
      ),
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
    update({
      cases: configuration.cases.filter((_, current) => current !== index),
    });
  };
  const allowedTypes = comparatorValueTypes[configuration.comparator];
  const valueTypeOptions = allowedTypes.map((valueType) => ({
    value: valueType,
    label: t(`editor.${valueType}`),
  }));

  return (
    <div className="space-y-2.5 rounded-md border border-zinc-800 bg-zinc-900/30 p-2.5">
      <label className="block">
        <span className="mb-1 block text-[10px] font-medium text-zinc-500">
          {t("switchCases.comparator")}
        </span>
        <Select
          value={configuration.comparator}
          onValueChange={(next) => {
            const comparator = next as SwitchComparator;
            update({
              comparator,
              cases: normalizeCases(configuration.cases, comparator),
            });
          }}
          options={comparatorOptions}
          ariaLabel={t("switchCases.comparator")}
        />
      </label>
      {configuration.cases.map((item, index) => (
        <article
          key={item.id}
          className="rounded-md border border-zinc-800 bg-zinc-950/60 p-2"
        >
          <div className="mb-2 flex items-center justify-between gap-2">
            <span className="text-[11px] font-medium text-zinc-300">
              {t("switchCases.case", { index: index + 1 })}
            </span>
            <div className="flex items-center gap-0.5">
              <Tooltip content={t("switchCases.moveUp")} side="top" wrap={false}>
                <Button
                  type="button"
                  size="sm"
                  variant="ghost"
                  className="size-6 p-0"
                  aria-label={t("switchCases.moveUp")}
                  disabled={index === 0}
                  onClick={() => moveCase(index, -1)}
                >
                  <ArrowUp className="size-3.5" />
                </Button>
              </Tooltip>
              <Tooltip content={t("switchCases.moveDown")} side="top" wrap={false}>
                <Button
                  type="button"
                  size="sm"
                  variant="ghost"
                  className="size-6 p-0"
                  aria-label={t("switchCases.moveDown")}
                  disabled={index === configuration.cases.length - 1}
                  onClick={() => moveCase(index, 1)}
                >
                  <ArrowDown className="size-3.5" />
                </Button>
              </Tooltip>
              <Tooltip content={t("editorActions.delete")} side="top" wrap={false}>
                <Button
                  type="button"
                  size="sm"
                  variant="ghost"
                  className="size-6 p-0 text-zinc-500 hover:text-red-300"
                  aria-label={t("editorActions.delete")}
                  disabled={configuration.cases.length <= 1}
                  onClick={() => void removeCase(index)}
                >
                  <Trash2 className="size-3.5" />
                </Button>
              </Tooltip>
            </div>
          </div>
          <div className="grid grid-cols-1 gap-2">
            <label className="block">
              <span className="mb-1 block text-[10px] font-medium text-zinc-500">
                {t("switchCases.value")}
              </span>
              {item.valueType === "boolean" ? (
                <Select
                  value={String(item.value)}
                  onValueChange={(next) => updateCase(index, { value: next === "true" })}
                  options={[
                    { value: "true", label: t("switchCases.true") },
                    { value: "false", label: t("switchCases.false") },
                  ]}
                  ariaLabel={`${t("switchCases.case", { index: index + 1 })} ${t("switchCases.value")}`}
                />
              ) : (
                <Input
                  type={item.valueType === "number" ? "number" : "text"}
                  inputMode={item.valueType === "number" ? "decimal" : undefined}
                  value={String(item.value)}
                  onChange={(event) =>
                    updateCase(index, {
                      value:
                        item.valueType === "number"
                          ? event.target.value === ""
                            ? ""
                            : Number(event.target.value)
                          : event.target.value,
                    })
                  }
                  aria-label={`${t("switchCases.case", { index: index + 1 })} ${t("switchCases.value")}`}
                />
              )}
            </label>
            {allowedTypes.length > 1 ? (
              <label className="block">
                <span className="mb-1 block text-[10px] font-medium text-zinc-500">
                  {t("editor.valueType")}
                </span>
                <Select
                  value={item.valueType}
                  onValueChange={(next) => {
                    const valueType = next as SwitchCaseValue["valueType"];
                    updateCase(index, {
                      valueType,
                      value: valueForType(item.value, valueType),
                    });
                  }}
                  options={valueTypeOptions}
                  ariaLabel={`${t("switchCases.case", { index: index + 1 })} ${t("editor.valueType")}`}
                />
              </label>
            ) : null}
            <label className="block">
              <span className="mb-1 block text-[10px] font-medium text-zinc-500">
                {t("editor.pinName")}
              </span>
              <Input
                value={item.label}
                onChange={(event) => updateCase(index, { label: event.target.value })}
                placeholder={t("switchCases.pinNamePlaceholder")}
                aria-label={`${t("switchCases.case", { index: index + 1 })} ${t("editor.pinName")}`}
              />
            </label>
          </div>
        </article>
      ))}
      <Button
        type="button"
        variant="ghost"
        size="sm"
        onClick={() => {
          const id = nextCaseID(configuration.cases);
          update({
            cases: [
              ...configuration.cases,
              {
                id,
                label: t("switchCases.caseName", {
                  index: configuration.cases.length + 1,
                }),
                valueType: allowedTypes[0],
                value: valueForType("", allowedTypes[0]),
              },
            ],
          });
        }}
      >
        <Plus className="size-3.5" />
        {t("switchCases.addCase")}
      </Button>
    </div>
  );
}
