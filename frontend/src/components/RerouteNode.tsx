import { memo } from "react";
import { useTranslation } from "react-i18next";
import type { GraphNode } from "@/types";
import { pinPalette } from "../lib/pins";
import { REROUTE_IN, REROUTE_OUT } from "@/features/graph/graph-ops";
import { Tooltip } from "./Tooltip";
import { cn } from "../utils/cn";

/** Rendered size of a reroute knot (square, centred on its position). */
export const REROUTE_SIZE = 14;

/**
 * A reroute is drawn as a single small knot rather than a node card.
 * Both of its pins sit at the centre so wires enter and leave the same point,
 * which is what makes a reroute chain look like one continuous wire.
 */
export const RerouteNode = memo(function RerouteNode({
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
  primary?: boolean;
  connectedPorts: Set<string>;
  onPointerDown: (e: React.PointerEvent, id: string) => void;
  onSelect: (id: string) => void;
  onContextMenu?: (e: React.MouseEvent) => void;
  onPortContextMenu?: (e: React.MouseEvent, portId: string) => void;
}) {
  const { t } = useTranslation();
  const dataType = node.outputs[0]?.dataType ?? "any";
  const pal = pinPalette(dataType);
  const isExec = dataType === "exec";
  const fed = connectedPorts.has(`${node.id}:${REROUTE_IN}:in`);
  const label = t(`pins.type_${dataType}`);
  void onSelect; // selection is owned by pointer-down

  /** both pins occupy the same hit area at the centre of the knot */
  const pin = (dir: "in" | "out", portId: string) => (
    <span
      data-port-node={node.id}
      data-port-id={portId}
      data-port-dir={dir}
      data-port-kind={isExec ? "exec" : "data"}
      data-port-type={dataType}
      onContextMenu={(e) => {
        e.preventDefault();
        e.stopPropagation();
        onPortContextMenu?.(e, portId);
      }}
      className={cn(
        "absolute top-1/2 h-[15px] w-[10px] -translate-y-1/2 cursor-crosshair",
        dir === "in" ? "left-0 -translate-x-1/2" : "right-0 translate-x-1/2",
      )}
    />
  );

  return (
    <Tooltip content={`${t("editor.reroute")} — ${label}`} side="top">
      <div
        data-node={node.id}
        onPointerDown={(e) => onPointerDown(e, node.id)}
      onContextMenu={onContextMenu}
      style={{
        left: node.x,
        top: node.y,
        width: REROUTE_SIZE,
        height: REROUTE_SIZE,
      }}
      role="button"
      aria-label={`${t("editor.reroute")} — ${label}`}
      className="group/reroute absolute grid cursor-grab place-items-center active:cursor-grabbing"
    >
        <span
          className={cn(
            "pointer-events-none border transition-transform duration-100 group-hover/reroute:scale-125",
            "h-[10px] w-[10px]",
            isExec ? "rounded-[3px]" : "rounded-full",
            selected && primary && "ring-2 ring-ring/80",
            selected && !primary && "ring-2 ring-ring/50",
          )}
          style={{
            borderColor: pal.dot,
            background: fed ? pal.bg : "var(--ink-900)",
          }}
        />
        {pin("in", REROUTE_IN)}
        {pin("out", REROUTE_OUT)}
      </div>
    </Tooltip>
  );
});
