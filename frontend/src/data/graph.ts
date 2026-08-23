import type { GraphNode } from "@/types";

/* ---- canvas card geometry (no data, pure layout math) ---- */

export const NODE_W = 212;
export const HEADER_H = 34;
export const ROW_H = 22;
export const BODY_TOP = 26;
export const BODY_BOTTOM = 10;

/** card border width — content is inset by this on every side (box-sizing: border-box) */
export const NODE_BORDER = 1;

export function nodeHeight(n: GraphNode) {
  const rows = Math.max(n.inputs.length, n.outputs.length);
  return NODE_BORDER * 2 + HEADER_H + BODY_TOP + rows * ROW_H + BODY_BOTTOM;
}

/** vertical centre of pin row `index`, relative to the node's outer box */
export function portY(index: number) {
  return NODE_BORDER + HEADER_H + BODY_TOP + index * ROW_H + ROW_H / 2;
}

/** horizontal anchor of a pin, relative to the node's outer box */
export function portX(dir: "in" | "out") {
  return dir === "in" ? NODE_BORDER : NODE_W - NODE_BORDER;
}
