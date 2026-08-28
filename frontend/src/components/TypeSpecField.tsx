import { useTranslation } from "react-i18next";
import type { TypeSpec } from "@/lib/types";
import type { PinDataType } from "@/types";
import { Dropdown } from "./Dropdown";
import { cn } from "../utils/cn";

/**
 * Recursive wire-type editor mirroring the Go `domain.TypeSpec` contract:
 * any · bool · string(text) · int · float · bytes · list<…> · map<string,…> · record(object).
 * Renders one compact dropdown per nesting level, e.g.
 *   [list] < [map] < [string , [any] > >
 * Map keys are fixed to text — the backend only accepts comparable keys and
 * the JavaScript runtime further restricts them to strings.
 */

export const MAX_SPEC_DEPTH = 4;

/** Canonical display token for a spec kind. */
export function specKindToken(kind?: string): string {
  switch (kind) {
    case "string": return "text";
    case "int": return "int";
    case "float": return "float";
    case "bool": return "bool";
    case "bytes": return "bytes";
    case "list": return "list";
    case "map": return "map";
    case "record": return "object";
    default: return "any";
  }
}

/** Coarse pin colour/validation type for a token. */
export function tokenToPinDataType(token: string): PinDataType {
  switch (token) {
    case "text": return "text";
    case "int":
    case "float": return "number";
    case "bool": return "boolean";
    case "list": return "array";
    case "map":
    case "object": return "object";
    default: return "any";
  }
}

/** Top-level token of a spec (for dots, validation fallbacks). */
export function specTopToken(spec?: TypeSpec): string {
  return specKindToken(spec?.kind);
}

const BASE_OPTIONS: { value: string; label: string }[] = [
  { value: "any", label: "any" },
  { value: "string", label: "text" },
  { value: "int", label: "int" },
  { value: "float", label: "float" },
  { value: "bool", label: "bool" },
  { value: "bytes", label: "bytes" },
  { value: "list", label: "list<…>" },
  { value: "map", label: "map<string,…>" },
  { value: "record", label: "object" },
];

function kindFromToken(token: string): TypeSpec {
  switch (token) {
    case "string": return { kind: "string" };
    case "int": return { kind: "int" };
    case "float": return { kind: "float" };
    case "bool": return { kind: "bool" };
    case "bytes": return { kind: "bytes" };
    case "list": return { kind: "list", element: { kind: "any" } };
    case "map": return { kind: "map", key: { kind: "string" }, value: { kind: "any" } };
    case "record": return { kind: "record" };
    default: return { kind: "any" };
  }
}

export function formatSpec(spec?: TypeSpec): string {
  if (!spec) return "any";
  const k = specKindToken(spec.kind);
  if (spec.kind === "list") return `${k}<${formatSpec(spec.element)}>`;
  if (spec.kind === "map") return `${k}<string, ${formatSpec(spec.value)}>`;
  return k;
}

export function TypeSpecField({
  value,
  onChange,
  depth = 0,
  className,
  allowAny = true,
  compact = false,
}: {
  value?: TypeSpec;
  onChange: (next: TypeSpec) => void;
  depth?: number;
  className?: string;
  /** tool contracts forbid `any` (backend validateToolType) */
  allowAny?: boolean;
  /** inline chip sizing used inside code-editor rows; inspectors use full size */
  compact?: boolean;
}) {
  const { t } = useTranslation();
  const spec = value ?? { kind: "any" as const };
  const nested = depth < MAX_SPEC_DEPTH;

  const options = allowAny
    ? BASE_OPTIONS
    : spec.kind === "any"
      ? [{ value: "any", label: "any (!)" }, ...BASE_OPTIONS.filter((o) => o.value !== "any")]
      : BASE_OPTIONS.filter((o) => o.value !== "any");

  const setKind = (kind: string) => {
    // keep the previous inner spec when staying within the same container kind
    if (kind === "list" && spec.kind === "list") return onChange(spec);
    if (kind === "map" && spec.kind === "map") return onChange({ ...spec, key: spec.key ?? { kind: "string" } });
    onChange(kindFromToken(kind));
  };

  return (
    <div className={cn("flex min-w-0 flex-wrap items-center gap-0.5", className)}>
      <Dropdown
        compact={compact}
        value={spec.kind}
        onChange={setKind}
        className={
          compact
            ? "h-4 shrink-0 px-1 font-mono text-[9px] [&>svg]:h-2 [&>svg]:w-2"
            : "h-8 min-w-[150px] shrink-0 px-2.5 text-[12px]"
        }
        options={options}
      />
      {nested && spec.kind === "list" && (
        <>
          <span aria-hidden className={cn("leading-none text-fg-faint", compact ? "font-mono text-[9px]" : "text-[11px]")}>&lt;</span>
          <TypeSpecField
            value={spec.element ?? { kind: "any" }}
            onChange={(element) => onChange({ ...spec, element })}
            depth={depth + 1}
            compact={compact}
          />
          <span aria-hidden className={cn("self-end leading-none text-fg-faint", compact ? "font-mono text-[9px]" : "text-[11px]")}>&gt;</span>
        </>
      )}
      {nested && spec.kind === "map" && (
        <>
          <span aria-hidden className={cn("leading-none text-fg-faint", compact ? "font-mono text-[9px]" : "text-[11px]")}>&lt;{t("typeField.textKey")},</span>
          <TypeSpecField
            value={spec.value ?? { kind: "any" }}
            onChange={(value) => onChange({ ...spec, value })}
            depth={depth + 1}
            compact={compact}
          />
          <span aria-hidden className={cn("self-end leading-none text-fg-faint", compact ? "font-mono text-[9px]" : "text-[11px]")}>&gt;</span>
        </>
      )}
      {(!nested || spec.kind === "record") && spec.kind === "record" && (
        <span aria-hidden className="font-mono text-[9px] leading-none text-fg-faint">{"{}"}</span>
      )}
    </div>
  );
}

