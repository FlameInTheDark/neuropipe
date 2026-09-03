import type { Edge, FunctionKind, GraphNode, GroupColor, NodeGroup, EditorComment, PinDataType, Port, PortKind } from "../../types";
import { NODE_W, nodeHeight } from "../../data/graph";
import { portKindFromDataType } from "../../lib/pins";

/**
 * Pure graph transformations.
 * No React, no side effects — every function returns a new collection,
 * which keeps them trivially testable and reusable.
 */

export const EXEC_IN: Port = { id: "exec", label: "Exec", kind: "exec", dataType: "exec" };
export const EXEC_OUT: Port = { id: "then", label: "Then", kind: "exec", dataType: "exec" };

/* ------------------------------------------------------------------ */
/*  Groups (UI-only frames)                                            */
/* ------------------------------------------------------------------ */

export const GROUP_HEADER_H = 28;
/** Breathing room left around nodes when a group is created from a selection. */
export const GROUP_PADDING = 26;
export const GROUP_MIN_W = 160;
export const GROUP_MIN_H = 120;

const GROUP_COLORS: GroupColor[] = ["slate", "violet", "emerald", "amber", "sky", "rose"];

/** Outer footprint of a node, accounting for the compact reroute knot. */
export function nodeBox(n: GraphNode, rerouteSize = 14) {
  const w = isReroute(n) ? rerouteSize : NODE_W;
  const h = isReroute(n) ? rerouteSize : nodeHeight(n);
  return { x: n.x, y: n.y, w, h, cx: n.x + w / 2, cy: n.y + h / 2 };
}

/** Wrap the given nodes in a new frame, leaving room for the title bar. */
export function createGroupFromNodes(
  nodes: GraphNode[],
  ids: string[],
  id: string,
  index = 0,
): NodeGroup | null {
  const members = nodes.filter((n) => ids.includes(n.id));
  if (!members.length) return null;

  const boxes = members.map((n) => nodeBox(n));
  const minX = Math.min(...boxes.map((b) => b.x)) - GROUP_PADDING;
  const minY = Math.min(...boxes.map((b) => b.y)) - GROUP_PADDING - GROUP_HEADER_H;
  const maxX = Math.max(...boxes.map((b) => b.x + b.w)) + GROUP_PADDING;
  const maxY = Math.max(...boxes.map((b) => b.y + b.h)) + GROUP_PADDING;

  return {
    id,
    title: `Group ${index + 1}`,
    x: minX,
    y: minY,
    w: Math.max(GROUP_MIN_W, maxX - minX),
    h: Math.max(GROUP_MIN_H, maxY - minY),
    color: GROUP_COLORS[index % GROUP_COLORS.length],
  };
}

/** Ids of every node whose centre falls inside the frame's body. */
export function nodesInGroup(nodes: GraphNode[], group: NodeGroup): string[] {
  const top = group.y + GROUP_HEADER_H;
  return nodes
    .filter((n) => {
      const { cx, cy } = nodeBox(n);
      return cx >= group.x && cx <= group.x + group.w && cy >= top && cy <= group.y + group.h;
    })
    .map((n) => n.id);
}

/** Clamp a resize so a frame can never invert or collapse. */
export function normalizeGroupRect(rect: { x: number; y: number; w: number; h: number }) {
  return {
    x: rect.w < 0 ? rect.x + rect.w : rect.x,
    y: rect.h < 0 ? rect.y + rect.h : rect.y,
    w: Math.max(GROUP_MIN_W, Math.abs(rect.w)),
    h: Math.max(GROUP_MIN_H, Math.abs(rect.h)),
  };
}

/* ------------------------------------------------------------------ */
/*  Sticky notes (UI-only comments)                                    */
/* ------------------------------------------------------------------ */

export const COMMENT_MIN_W = 140;
export const COMMENT_MIN_H = 80;

export function createCommentNode(id: string, x: number, y: number, text = ""): EditorComment {
  return { id, text, x, y, w: 200, h: 120, color: "amber" };
}

export function normalizeCommentRect(rect: { x: number; y: number; w: number; h: number }) {
  return {
    x: rect.w < 0 ? rect.x + rect.w : rect.x,
    y: rect.h < 0 ? rect.y + rect.h : rect.y,
    w: Math.max(COMMENT_MIN_W, Math.abs(rect.w)),
    h: Math.max(COMMENT_MIN_H, Math.abs(rect.h)),
  };
}
/* ------------------------------------------------------------------ */
/*  Reroute nodes                                                      */
/* ------------------------------------------------------------------ */

/** UI-only knot type; persisted as flow:reroute / data:reroute (see adapters). */
export const REROUTE_TYPE = "util.reroute";
export const REROUTE_IN = "in";
export const REROUTE_OUT = "out";

export function isReroute(node: GraphNode | undefined): boolean {
  return node?.type === REROUTE_TYPE;
}

/**
 * A reroute is a pin-sized pass-through used to bend wires.
 * It starts untyped (`any`) and adopts the type of whatever is plugged in.
 */
export function createRerouteNode(id: string, x: number, y: number, dataType: PinDataType = "any"): GraphNode {
  return {
    id,
    type: REROUTE_TYPE,
    title: "",
    icon: "Crosshair",
    group: "Utility",
    summary: "",
    x,
    y,
    status: "idle",
    inputs: [{ id: REROUTE_IN, label: "", kind: portKindFromDataType(dataType), dataType }],
    outputs: [{ id: REROUTE_OUT, label: "", kind: portKindFromDataType(dataType), dataType }],
    fields: [],
    values: {},
  };
}

/** Re-type both pins of a reroute node. */
function retypeReroute(node: GraphNode, dataType: PinDataType): GraphNode {
  const kind = portKindFromDataType(dataType);
  return {
    ...node,
    inputs: [{ id: REROUTE_IN, label: "", kind, dataType }],
    outputs: [{ id: REROUTE_OUT, label: "", kind, dataType }],
  };
}

/**
 * Walk downstream from a reroute, re-typing every reroute in the chain and
 * repainting the edges between them, so a chain adopts its upstream source.
 */
export function propagateRerouteType(
  nodes: GraphNode[],
  edges: Edge[],
  startNodeId: string,
  dataType: PinDataType,
): { nodes: GraphNode[]; edges: Edge[] } {
  let nextNodes = nodes;
  let nextEdges = edges;
  const seen = new Set<string>();
  const queue = [startNodeId];

  while (queue.length) {
    const id = queue.shift()!;
    if (seen.has(id)) continue;
    seen.add(id);

    const node = nextNodes.find((n) => n.id === id);
    if (!isReroute(node) || !node) continue;

    nextNodes = nextNodes.map((n) => (n.id === id ? retypeReroute(n, dataType) : n));

    // repaint outgoing edges and continue through any chained reroutes
    nextEdges = nextEdges.map((e) =>
      e.from.node === id ? { ...e, kind: portKindFromDataType(dataType), dataType } : e,
    );

    for (const e of nextEdges) {
      if (e.from.node === id) queue.push(e.to.node);
    }
  }

  return { nodes: nextNodes, edges: nextEdges };
}

/** Reset a reroute back to untyped once nothing feeds it any more. */
export function clearRerouteTypeIfOrphaned(
  nodes: GraphNode[],
  edges: Edge[],
  nodeId: string,
): { nodes: GraphNode[]; edges: Edge[] } {
  const node = nodes.find((n) => n.id === nodeId);
  if (!isReroute(node)) return { nodes, edges };
  const stillFed = edges.some((e) => e.to.node === nodeId);
  if (stillFed) return { nodes, edges };
  return propagateRerouteType(nodes, edges, nodeId, "any");
}

/**
 * Bridge the wires around a reroute that is being deleted, so the upstream
 * source reconnects straight to every downstream target.
 */
export function bridgeRerouteOnDelete(edges: Edge[], nodeId: string): Edge[] {
  const incoming = edges.find((e) => e.to.node === nodeId);
  const outgoing = edges.filter((e) => e.from.node === nodeId);
  const remaining = edges.filter((e) => e.to.node !== nodeId && e.from.node !== nodeId);
  if (!incoming) return remaining;

  const bridged: Edge[] = outgoing.map((out, i) => ({
    id: `${out.id}-bridged-${i}`,
    from: incoming.from,
    to: out.to,
    kind: incoming.kind,
    dataType: incoming.dataType,
  }));

  return [...remaining, ...bridged];
}

/**
 * An input pin accepts a single wire. Connecting a new source to an
 * already-fed input replaces the old one; outputs fan out freely.
 */
export function detachInput(edges: Edge[], to: { node: string; port: string }): Edge[] {
  return edges.filter((e) => !(e.to.node === to.node && e.to.port === to.port));
}

/** Replace one node, leaving the rest untouched. */
export function patchNode(nodes: GraphNode[], id: string, patch: Partial<GraphNode>): GraphNode[] {
  return nodes.map((n) => (n.id === id ? { ...n, ...patch } : n));
}

/** Merge a single entry into a node''s `values` bag. */
export function setNodeValue(
  nodes: GraphNode[],
  id: string,
  key: string,
  value: unknown,
): GraphNode[] {
  return nodes.map((n) => (n.id === id ? { ...n, values: { ...n.values, [key]: value } } : n));
}

/** Reset every node to idle — used before starting a run. */
export function resetStatuses(nodes: GraphNode[]): GraphNode[] {
  return nodes.map((n) => ({ ...n, status: "idle" as const }));
}

export function findPort(node: GraphNode | undefined, portId: string): Port | undefined {
  if (!node) return undefined;
  return node.outputs.find((p) => p.id === portId) ?? node.inputs.find((p) => p.id === portId);
}

/** Resolve the data-type a new edge should paint itself with. */
export function resolveEdgeDataType(
  nodes: GraphNode[],
  from: { node: string; port: string },
  kind: PortKind,
): PinDataType {
  const port = findPort(nodes.find((n) => n.id === from.node), from.port);
  return port?.dataType ?? (kind === "exec" || kind === "tool" ? kind : "any");
}

export function edgeExists(
  edges: Edge[],
  from: { node: string; port: string },
  to: { node: string; port: string },
): boolean {
  return edges.some(
    (e) =>
      e.from.node === from.node &&
      e.from.port === from.port &&
      e.to.node === to.node &&
      e.to.port === to.port,
  );
}

/** Drop every edge attached to a node (used when the node is deleted). */
export function removeEdgesForNode(edges: Edge[], nodeId: string): Edge[] {
  return edges.filter((e) => e.from.node !== nodeId && e.to.node !== nodeId);
}

/** Drop every edge attached to one specific port. */
export function removeEdgesForPort(edges: Edge[], nodeId: string, portId: string): Edge[] {
  return edges.filter(
    (e) =>
      !(e.from.node === nodeId && e.from.port === portId) &&
      !(e.to.node === nodeId && e.to.port === portId),
  );
}

/** Split a port list into its exec pins and its data pins. */
export function splitPins(ports: Port[]): { exec: Port[]; data: Port[] } {
  return {
    exec: ports.filter((p) => p.kind === "exec"),
    data: ports.filter((p) => p.kind !== "exec"),
  };
}

/**
 * Apply the exec-pin rules for a function kind to both boundary nodes:
 * impure and tool keep exactly one exec in and one exec out (tool functions
 * always execute as an impure subgraph and the backend requires the
 * entry→return exec path to be visible and re-wirable), pure has none.
 * The existing exec pin id is preserved so saved wires stay connected.
 */
export function applyFunctionKind(nodes: GraphNode[], kind: FunctionKind): GraphNode[] {
  const withExec = kind !== "pure";
  return nodes.map((n) => {
    if (n.type === "function:entry") {
      const exec = n.outputs.filter((p) => p.kind === "exec").slice(0, 1);
      const data = n.outputs.filter((p) => p.kind !== "exec");
      return {
        ...n,
        outputs: withExec ? [...exec, ...data] : data,
        values: { ...n.values, functionKind: kind },
      };
    }
    if (n.type === "function:return") {
      const exec = n.inputs.filter((p) => p.kind === "exec").slice(0, 1);
      const data = n.inputs.filter((p) => p.kind !== "exec");
      return {
        ...n,
        inputs: withExec ? [...exec, ...data] : data,
        values: { ...n.values, functionKind: kind },
      };
    }
    return n;
  });
}

/** Overlay pins on the boundary nodes from a saved function contract. */
export function applyFunctionInterface(
  nodes: GraphNode[],
  fn: { kind: FunctionKind; inputs: Port[]; outputs: Port[] },
): GraphNode[] {
  const withExec = fn.kind !== "pure";
  return nodes.map((n) => {
    if (n.type === "function:entry") {
      const exec = n.outputs.filter((p) => p.kind === "exec").slice(0, 1);
      return {
        ...n,
        outputs: withExec ? [...exec, ...fn.inputs] : [...fn.inputs],
        values: { ...n.values, functionKind: fn.kind },
      };
    }
    if (n.type === "function:return") {
      const exec = n.inputs.filter((p) => p.kind === "exec").slice(0, 1);
      // tools expose their declared outputs (JSON object keys fed to the agent)
      const returned = fn.outputs;
      return {
        ...n,
        inputs: withExec ? [...exec, ...returned] : [...returned],
        values: { ...n.values, functionKind: fn.kind },
      };
    }
    return n;
  });
}

/** Offset copy of a node, ready to be appended to the graph. */
export function duplicateNode(node: GraphNode, id: string, offset = 32): GraphNode {
  return { ...node, id, x: node.x + offset, y: node.y + offset, status: "idle" };
}

/** Deterministic-ish unique id from a human label. */
export function makeNodeId(seed: string): string {
  const slug = seed.toLowerCase().replace(/\W+/g, "-").replace(/^-|-$/g, "");
  return `${slug}-${Math.random().toString(36).slice(2, 6)}`;
}


