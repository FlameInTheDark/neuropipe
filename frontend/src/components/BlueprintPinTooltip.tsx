import { useTranslation } from "react-i18next";

import { Tooltip } from "@/components/ui/tooltip";
import type { DataType, NodePort } from "@/lib/types";

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
  const type =
    pin.kind === "exec"
      ? t("editor.executionFlow")
      : `${dataTypeLabel(pin.dataType, t)} ${t("editor.data")}`;

  return (
    <Tooltip
      content={
        <>
          <span className="block">{type}</span>
          {fields.length > 0 ? (
            <span className="mt-1.5 block border-t border-zinc-700 pt-1.5">
              <span className="mb-1 block text-[9px] font-semibold uppercase tracking-[.12em] text-zinc-500">
                {t("editor.knownFields")}
              </span>
              {fields.map((field) => (
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
      wrap={fields.length > 0}
      className={fields.length > 0 ? "w-64 whitespace-normal" : undefined}
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
