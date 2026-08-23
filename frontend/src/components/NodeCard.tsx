import { memo } from "react";
import type { GraphNode, Port } from "../types";
import { BODY_TOP, HEADER_H, NODE_W, ROW_H } from "../data/graph";
import { pinPalette } from "../lib/pins";
import { Icon } from "./icons";
import { Tooltip } from "./Tooltip";
import { useTranslation } from "react-i18next";
import { cn } from "../utils/cn";

/* ---- pin tooltip body ---- */

function PinTip({ port }: { port: Port }) {
  const { t } = useTranslation();
  const pal = pinPalette(port.dataType);
  return (
    <span className="flex flex-col gap-1 py-0.5">
      <span className="flex items-center gap-2">
        <span className="h-2 w-2 rounded-full" style={{ background: pal.dot }} />
        <span className="font-medium text-ink-50">{port.label}</span>
        <span className="ml-1 rounded bg-ink-800 px-1 py-px font-mono text-[9.5px]" style={{ color: pal.label }}>
          {t(`pins.type_${port.dataType ?? "any"}`)}
        </span>
      </span>
      {port.dataType === "array" && port.arrayOf && (
        <span className="pl-4 text-[10.5px] text-ink-400">{t("pins.elementType", { type: String(port.arrayOf) })}</span>
      )}
      {port.dataType === "object" && port.objectFields && port.objectFields.length > 0 && (
        <span className="mt-0.5 flex flex-col gap-[2px] pl-4">
          {port.objectFields.map((f) => (
            <span key={f.key} className="flex items-center gap-1.5 text-[10.5px]">
              <span className="h-1 w-1 rounded-full" style={{ background: pinPalette(f.type as any).dot }} />
              <span className="font-mono text-ink-200">{f.key}</span>
              <span className="text-ink-500">{f.type}</span>
            </span>
          ))}
        </span>
      )}
      {port.kind === "exec" && (
        <span className="pl-4 text-[10.5px] text-ink-500">{t("pins.execHelp")}</span>
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
            borderColor: connected ? pal.dot : `${pal.dot}88`,
            background: connected ? pal.bg : "var(--color-ink-900)",
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
            selected ? "border-ink-500 bg-ink-700 text-ink-50" : "border-ink-700 bg-ink-800 text-ink-300",
          )}
        >
          <Icon name={node.icon} className="h-[11px] w-[11px]" strokeWidth={2} />
        </span>
        <span className="truncate text-[12.5px] font-medium text-ink-50">{node.title}</span>
        <span className="ml-auto flex items-center gap-1.5">
          {node.status === "running" && (
            <span className="font-mono text-[9px] tracking-wide text-ink-400 uppercase">run</span>
          )}
          <span
            className={cn(
              "h-[6px] w-[6px] rounded-full",
              node.status === "done" && "bg-emerald-400/90",
              node.status === "running" && "bg-ink-50 pulse-ring",
              node.status === "queued" && "bg-amber-400/70",
              node.status === "error" && "bg-rose-400",
              node.status === "idle" && "bg-ink-600",
            )}
          />
        </span>
      </div>

      {/* subtitle */}
      <div
        style={{ height: BODY_TOP }}
        className="flex items-center gap-1.5 px-2.5 font-mono text-[10px] tracking-tight text-ink-500"
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
                      inp.kind === "exec" ? "font-medium text-ink-200" : "text-ink-400",
                      "hover:text-ink-50",
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
                      out.kind === "exec" ? "font-medium text-ink-200" : "text-ink-400",
                      "hover:text-ink-50",
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


