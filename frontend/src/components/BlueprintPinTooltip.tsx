import { useTranslation } from "react-i18next";

import { Tooltip } from "@/components/ui/tooltip";
import type { DataType, NodePort, TypeSpec } from "@/lib/types";

function dataTypeLabel(dataType: DataType | undefined, t: (key: string) => string) {
  const key =
    dataType === "text" ||
    dataType === "number" ||
    dataType === "boolean" ||
    dataType === "object" ||
    dataType === "list"
      ? dataType
      : "any";
  return t(`editor.${key}`);
}

function typeLabel(spec: TypeSpec | undefined, dataType: DataType | undefined, t: (key: string) => string): string {
  if (!spec) return dataTypeLabel(dataType, t);
  switch (spec.kind) {
    case "bool": return t("editor.boolean");
    case "string": return t("editor.text");
    case "bytes": return t("editor.bytes");
    case "int":
    case "float": return t("editor.number");
    case "list": return `[${typeLabel(spec.element, undefined, t)}]`;
    case "map": return `map<${typeLabel(spec.key, undefined, t)}, ${typeLabel(spec.value, undefined, t)}>`;
    case "record": return spec.name || t("editor.object");
    default: return t("editor.any");
  }
}

/**
 * Shared Blueprint-canvas pin tooltip. The app tooltip owns its chrome and
 * interaction; this component only supplies the node-specific type metadata.
 */
export function BlueprintPinTooltip({
  pin,
  target = false,
}: {
  pin: NodePort;
  target?: boolean;
}) {
  const { t } = useTranslation();
  const fields = pin.fields ?? [];
  const type = pin.kind === "exec"
    ? t("editor.executionFlow")
    : pin.kind === "tool"
      ? t("functions.tool")
      : typeLabel(pin.type, pin.dataType, t);
  const structuredFields = pin.type?.kind === "record" ? pin.type.fields ?? [] : [];
  const hasDetails = fields.length > 0 || structuredFields.length > 0;

  return (
    <Tooltip
      content={
        <>
          <span className="block">{type}</span>
          {hasDetails ? (
            <span className="mt-1.5 block border-t border-zinc-700 pt-1.5">
              <span className="mb-1 block text-[9px] font-semibold uppercase tracking-[.12em] text-zinc-500">
                {t("editor.knownFields")}
              </span>
              {structuredFields.length > 0 ? structuredFields.map((field) => (
                <span key={field.id} className="mb-1 flex items-baseline justify-between gap-3 last:mb-0">
                  <span className="min-w-0 truncate font-mono text-zinc-100">
                    {field.name || field.id}
                  </span>
                  <span className="shrink-0 text-zinc-500">
                    {typeLabel(field.type, undefined, t)}
                    {field.optional ? ` · ${t("editor.optional")}` : ""}
                  </span>
                </span>
              )) : fields.map((field) => (
                <span key={field.path} className="mb-1 block last:mb-0">
                  <span className="font-mono text-zinc-100">{field.path}</span>{" "}
                  <span className="text-zinc-500">
                    {dataTypeLabel(field.dataType, t)}
                    {field.optional ? ` · ${t("editor.optional")}` : ""}
                  </span>
                  {field.description ? (
                    <span className="block text-zinc-500">{field.description}</span>
                  ) : null}
                </span>
              ))}
            </span>
          ) : null}
        </>
      }
      side="top"
      align={target ? "start" : "end"}
      wrap={hasDetails}
      className={hasDetails ? "w-72 whitespace-normal" : undefined}
    >
      <span
        tabIndex={0}
        className="cursor-help rounded-sm outline-none focus-visible:text-zinc-100"
      >
        {pin.label}
      </span>
    </Tooltip>
  );
}
