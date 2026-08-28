import { memo } from "react";
import type { GraphNode, Port } from "../types";
import type { TypeSpec } from "../lib/types";
import { BODY_TOP, HEADER_H, NODE_W, ROW_H } from "../data/graph";
import { pinPalette } from "../lib/pins";
import { mapSpecToPin } from "../lib/adapters";
import { Icon } from "./icons";
import { Tooltip } from "./Tooltip";
import { useTranslation } from "react-i18next";
import { cn } from "../utils/cn";

/* ---- pin tooltip body ---- */

type Translate = ReturnType<typeof useTranslation>["t"];

/** Human-readable type name of a TypeSpec (falls back to "Any"). */
function specTypeLabel(t: Translate, spec?: TypeSpec): string {
  switch (spec?.kind) {
    case "string": return t("pins.type_text");
    case "int":
    case "float": return t("pins.type_number");
    case "bool": return t("pins.type_boolean");
    case "list": return t("pins.type_array");
    case "map": return t("pins.type_map");
    case "record": return spec.name || t("pins.type_object");
    default: return t("pins.type_any");
  }
}

/** Extra structural hint for map/list fields, e.g. "(Text → Any)". */
function specTypeDetail(t: Translate, spec?: TypeSpec): string | undefined {
  if (spec?.kind === "map") {
    return `(${specTypeLabel(t, spec.key)} ${t("pins.mapArrow")} ${specTypeLabel(t, spec.value)})`;
  }
  if (spec?.kind === "list" && spec.element) {
    return `(${specTypeLabel(t, spec.element)})`;
  }
  return undefined;
}

interface StructureRow {
  key: string;
  /** canvas pin type driving the row's dot colour */
  pinType: string;
  typeLabel: string;
  detail?: string;
}

/**
 * Structure rows for object pins. A backend record TypeSpec is authoritative
 * (named fields with full sub-types — e.g. the Discord/Telegram event
 * envelopes); the derived objectFields (spec fields or documented result
 * fields) are the fallback for ports without a record spec.
 */
function structureRows(t: Translate, port: Port): StructureRow[] {
  if (port.spec?.kind === "record" && port.spec.fields?.length) {
    return port.spec.fields.map((f) => ({
      key: f.name,
      pinType: mapSpecToPin(f.type),
      typeLabel: specTypeLabel(t, f.type),
      detail: specTypeDetail(t, f.type),
    }));
  }
  if (port.dataType === "object" && port.objectFields?.length) {
    return port.objectFields.map((f) => ({
      key: f.key,
      pinType: String(f.type),
      typeLabel: t(`pins.type_${f.type}`),
    }));
  }
  return [];
}

function PinTip({ port }: { port: Port }) {
  const { t } = useTranslation();
  const pal = pinPalette(port.dataType);
  const recordName = port.spec?.kind === "record" ? port.spec.name : undefined;
  const rows = structureRows(t, port);
  return (
    <span className="flex flex-col gap-1 py-0.5">
      <span className="flex items-center gap-2">
        <span className="h-2 w-2 rounded-full" style={{ background: pal.dot }} />
        <span className="font-medium text-fg">{port.label}</span>
        <span className="ml-1 rounded bg-ink-800 px-1 py-px font-mono text-[9.5px]" style={{ color: pal.label }}>
          {t(`pins.type_${port.dataType ?? "any"}`)}
        </span>
        {recordName && (
          <span className="rounded bg-ink-800 px-1 py-px font-mono text-[9.5px] text-fg-subtle">{recordName}</span>
        )}
      </span>
      {port.spec?.kind === "map" && (
        <span className="pl-4 font-mono text-[10.5px] text-fg-subtle">
          {t("pins.mapStructure", { key: specTypeLabel(t, port.spec.key), value: specTypeLabel(t, port.spec.value) })}
        </span>
      )}
      {port.dataType === "array" && port.arrayOf && (
        <span className="pl-4 text-[10.5px] text-fg-subtle">
          {t("pins.elementType", { type: t(`pins.type_${port.arrayOf}`) })}
        </span>
      )}
      {rows.length > 0 && (
        <span className="mt-0.5 flex flex-col gap-[2px] pl-4">
          {rows.map((f) => (
            <span key={f.key} className="flex items-center gap-1.5 text-[10.5px]">
              <span className="h-1 w-1 rounded-full" style={{ background: pinPalette(f.pinType as Port["dataType"]).dot }} />
              <span className="font-mono text-fg-muted">{f.key}</span>
              <span className="text-fg-faint">
                {f.typeLabel}
                {f.detail && <span className="font-mono text-[9.5px]"> {f.detail}</span>}
              </span>
            </span>
          ))}
        </span>
      )}
      {port.kind === "exec" && (
        <span className="pl-4 text-[10.5px] text-fg-faint">{t("pins.execHelp")}</span>
      )}
    </span>
  );
}

/* ---- port dot ---- */

function PortDot({
  port,
  dir,
  nodeId,
  connected,
  onPortCtx,
}: {
  port: Port;
  dir: "in" | "out";
  nodeId: string;
  connected: boolean;
  onPortCtx?: (e: React.MouseEvent, portId: string) => void;
}) {
  const pal = pinPalette(port.dataType);
  const isExec = port.kind === "exec";

  return (
    <span
      data-port-node={nodeId}
      data-port-id={port.id}
      data-port-dir={dir}
      data-port-kind={port.kind}
      data-port-type={port.dataType}
      onContextMenu={(e) => {
        e.preventDefault();
        e.stopPropagation();
        onPortCtx?.(e, port.id);
      }}
      className={cn(
        "group/port absolute top-1/2 z-10 grid h-[15px] w-[15px] -translate-y-1/2 place-items-center cursor-crosshair",
        dir === "in" ? "-left-[7.5px]" : "-right-[7.5px]",
      )}
    >
      <Tooltip content={<PinTip port={port} />} side={dir === "in" ? "left" : "right"} delay={350}>
        <span
          className={cn(
            "pointer-events-none border transition-[transform,border-color,background-color] duration-150 ease-out group-hover/port:scale-[1.3]",
            isExec ? "h-[8px] w-[8px] rounded-[2px]" : "h-[7px] w-[7px] rounded-full",
          )}
          style={{
            borderColor: connected ? pal.dot : `color-mix(in srgb, ${pal.dot} 53%, transparent)`,
            background: connected ? pal.bg : "var(--ink-900)",
          }}
        />
      </Tooltip>
    </span>
  );
}

/* ---- node card ---- */

export const NodeCard = memo(function NodeCard({
  node,
  selected,
  primary = false,
  connectedPorts,
  onPointerDown,
  onSelect,
  onContextMenu,
  onPortContextMenu,
}: {
  node: GraphNode;
  selected: boolean;
  /** the multi-selection's primary node gets a stronger outline */
  primary?: boolean;
  connectedPorts: Set<string>;
  onPointerDown: (e: React.PointerEvent, id: string) => void;
  /** selection is owned by pointer-down; kept for API parity (unused) */
  onSelect: (id: string) => void;
  onContextMenu?: (e: React.MouseEvent) => void;
  onPortContextMenu?: (e: React.MouseEvent, portId: string) => void;
}) {
  const rows = Math.max(node.inputs.length, node.outputs.length);
  void onSelect; // selection is owned by pointer-down

  return (
    <div
      data-node={node.id}
      onPointerDown={(e) => onPointerDown(e, node.id)}
      onContextMenu={onContextMenu}
      style={{ left: node.x, top: node.y, width: NODE_W }}
      className={cn(
        "group/node absolute rounded-[10px] border bg-ink-850/95 backdrop-blur-[2px] transition-[border-color,box-shadow]",
        selected && primary
          ? "border-ink-200 shadow-[0_0_0_1px_rgba(236,237,241,0.2),0_12px_28px_-8px_rgba(0,0,0,0.85)]"
          : selected
            ? "border-ink-400 shadow-[0_0_0_1px_rgba(161,161,173,0.09),0_8px_20px_-10px_rgba(0,0,0,0.85)]"
            : "border-ink-700 shadow-[0_8px_20px_-10px_rgba(0,0,0,0.8)] hover:border-ink-600",
      )}
    >
      {/* header */}
      <div
        style={{ height: HEADER_H }}
        className="flex cursor-grab items-center gap-2 border-b border-seam px-2.5 active:cursor-grabbing"
      >
        <span
          className={cn(
            "grid h-[18px] w-[18px] shrink-0 place-items-center rounded-[5px] border",
            selected ? "border-ink-500 bg-ink-700 text-fg" : "border-ink-700 bg-ink-800 text-fg-subtle",
          )}
        >
          <Icon name={node.icon} className="h-[11px] w-[11px]" strokeWidth={2} />
        </span>
        <span className="truncate text-[12.5px] font-medium text-fg">{node.title}</span>
        <span className="ml-auto flex items-center gap-1.5">
          {node.status === "running" && (
            <span className="font-mono text-[9px] tracking-wide text-fg-subtle uppercase">run</span>
          )}
          <span
            className={cn(
              "h-[6px] w-[6px] rounded-full",
              node.status === "done" && "bg-success/90",
              node.status === "running" && "bg-ink-50 pulse-ring",
              node.status === "queued" && "bg-warning/70",
              node.status === "error" && "bg-danger",
              node.status === "idle" && "bg-ink-600",
            )}
          />
        </span>
      </div>

      {/* subtitle */}
      <div
        style={{ height: BODY_TOP }}
        className="flex items-center gap-1.5 px-2.5 font-mono text-[10px] tracking-tight text-fg-faint"
      >
        <span className="truncate">{node.type}</span>
      </div>

      {/* port rows */}
      <div className="relative pb-[10px]">
        {Array.from({ length: rows }).map((_, i) => {
          const inp = node.inputs[i];
          const out = node.outputs[i];
          return (
            <div key={i} style={{ height: ROW_H }} className="relative flex items-center justify-between px-2.5">
              {inp && (
                <PortDot
                  port={inp}
                  dir="in"
                  nodeId={node.id}
                  connected={connectedPorts.has(`${node.id}:${inp.id}:in`)}
                  onPortCtx={onPortContextMenu}
                />
              )}
              {inp ? (
                <Tooltip content={<PinTip port={inp} />} side="left" delay={350} className="min-w-0">
                  <span
                    className={cn(
                      "min-w-0 cursor-default truncate text-[11px] transition-colors",
                      inp.kind === "exec" ? "font-medium text-fg-muted" : "text-fg-subtle",
                      "hover:text-fg",
                    )}
                  >
                    {inp.label}
                  </span>
                </Tooltip>
              ) : (
                <span className="min-w-0 truncate text-[11px] opacity-0">·</span>
              )}

              {out ? (
                <Tooltip content={<PinTip port={out} />} side="right" delay={350} className="min-w-0 justify-end pl-2">
                  <span
                    className={cn(
                      "min-w-0 cursor-default truncate text-right text-[11px] transition-colors",
                      out.kind === "exec" ? "font-medium text-fg-muted" : "text-fg-subtle",
                      "hover:text-fg",
                    )}
                  >
                    {out.label}
                  </span>
                </Tooltip>
              ) : (
                <span className="min-w-0 truncate pl-2 text-right text-[11px] opacity-0">·</span>
              )}
              {out && (
                <PortDot
                  port={out}
                  dir="out"
                  nodeId={node.id}
                  connected={connectedPorts.has(`${node.id}:${out.id}:out`)}
                  onPortCtx={onPortContextMenu}
                />
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
});


