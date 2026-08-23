import type { MenuItem } from "../../components/ContextMenu";
import type { GraphNode, GroupColor, NodeGroup } from "@/types";
/**
 * Builds the canvas context menus.
 * Pure factory functions so the menu structure lives next to the graph
 * domain instead of bloating the app shell.
 */

export interface NodeMenuActions {
  duplicate: () => void;
  copy: (id: string) => void;
  clearPortLinks: (nodeId: string, portId: string) => void;
  remove: () => void;
}

export function buildNodeMenu(node: GraphNode | undefined, t: (key: string) => string, a: NodeMenuActions): MenuItem[] {
  if (!node) return [];
  return [
    { label: t("editor.menuDuplicate"), icon: "Copy", hint: "⌘D", onSelect: () => setTimeout(a.duplicate, 0), disabled: node.locked },
    { label: t("editor.menuCopy"), icon: "Braces", onSelect: () => a.copy(node.id) },
    { type: "sep" },
    { label: t("editor.menuClearLinks"), icon: "X", onSelect: () => a.clearPortLinks(node.id, "") },
    { type: "sep" },
    {
      label: t("editor.menuDelete"),
      icon: "Trash2",
      hint: "⌫",
      danger: true,
      disabled: node.locked,
      onSelect: () => setTimeout(a.remove, 0),
    },
  ];
}

export function buildEdgeMenu(
  edgeId: string,
  removeEdge: (id: string) => void,
  t: (key: string) => string,
  insertReroute?: (edgeId: string, at: { x: number; y: number }) => void,
  at?: { x: number; y: number },
): MenuItem[] {
  return [
    ...(insertReroute
      ? ([
          { label: t("editor.addReroute"), icon: "Crosshair", onSelect: () => insertReroute(edgeId, at ?? { x: 0, y: 0 }) },
          { type: "sep" },
        ] as MenuItem[])
      : []),
    { label: t("canvas.removeConnection"), icon: "Trash2", danger: true, onSelect: () => removeEdge(edgeId) },
  ];
}

export function buildPortMenu(
  nodeId: string,
  portId: string,
  clearPortLinks: (nodeId: string, portId: string) => void,
  t: (key: string) => string,
): MenuItem[] {
  return [
    { label: t("editor.menuClearPortLinks"), icon: "X", onSelect: () => clearPortLinks(nodeId, portId) },
  ];
}

/* ------------------------------------------------------------------ */
/* multi-selection bulk actions                                        */
/* ------------------------------------------------------------------ */

export interface MultiMenuActions {
  count: number;
  group: () => void;
  duplicate: () => void;
  align: (axis: "left" | "right" | "top" | "bottom" | "hcenter" | "vcenter") => void;
  distribute: (axis: "h" | "v") => void;
  remove: () => void;
  clear: () => void;
}

export function buildMultiMenu(a: MultiMenuActions, t: (key: string, vars?: Record<string, unknown>) => string): MenuItem[] {
  return [
    { label: t("editor.menuGroup", { count: a.count }), icon: "Boxes", hint: "⌘G", onSelect: () => setTimeout(a.group, 0) },
    { label: t("editor.duplicateSelection"), icon: "Copy", hint: "⌘D", onSelect: () => setTimeout(a.duplicate, 0) },
    { type: "sep" },
    { label: t("editor.alignLeft"), icon: "AlignLeft", onSelect: () => a.align("left") },
    { label: t("editor.alignHCenter"), icon: "AlignCenterHorizontal", onSelect: () => a.align("hcenter") },
    { label: t("editor.alignRight"), icon: "AlignRight", onSelect: () => a.align("right") },
    { label: t("editor.alignTop"), icon: "AlignStartVertical", onSelect: () => a.align("top") },
    { label: t("editor.alignVCenter"), icon: "AlignCenterVertical", onSelect: () => a.align("vcenter") },
    { label: t("editor.alignBottom"), icon: "AlignEndVertical", onSelect: () => a.align("bottom") },
    { label: t("editor.distributeH"), icon: "Columns3", disabled: a.count < 3, onSelect: () => a.distribute("h") },
    { label: t("editor.distributeV"), icon: "Rows3", disabled: a.count < 3, onSelect: () => a.distribute("v") },
    { type: "sep" },
    { label: t("editor.deselectAll"), icon: "X", hint: "Esc", onSelect: () => a.clear() },
    { label: t("editor.deleteNodes", { count: a.count }), icon: "Trash2", hint: "⌫", danger: true, onSelect: () => setTimeout(a.remove, 0) },
  ];
}

/* ------------------------------------------------------------------ */
/* group frame actions                                                 */
/* ------------------------------------------------------------------ */

export interface GroupMenuActions {
  selectMembers: (id: string) => void;
  /** Focuses the frame's own title input rather than opening a browser prompt. */
  beginRename: (id: string) => void;
  setColor: (id: string, color: GroupColor) => void;
  ungroup: (id: string) => void;
}

const COLOR_LABEL_KEYS: Record<GroupColor, string> = {
  slate: "editor.colorSlate",
  violet: "editor.colorViolet",
  emerald: "editor.colorEmerald",
  amber: "editor.colorAmber",
  sky: "editor.colorSky",
  rose: "editor.colorRose",
};

export function buildGroupMenu(
  group: NodeGroup | undefined,
  a: GroupMenuActions,
  t: (key: string, vars?: Record<string, unknown>) => string,
): MenuItem[] {
  if (!group) return [];
  return [
    { label: t("editor.selectNodesInGroup"), icon: "Boxes", onSelect: () => a.selectMembers(group.id) },
    { label: t("editor.renameGroup"), icon: "Pencil", hint: "F2", onSelect: () => a.beginRename(group.id) },
    { type: "sep" },
    ...(Object.keys(COLOR_LABEL_KEYS) as GroupColor[]).map((color) => ({
      label: t(COLOR_LABEL_KEYS[color]),
      icon: "CircleDot",
      checked: group.color === color,
      onSelect: () => a.setColor(group.id, color),
    })),
    { type: "sep" },
    { label: t("editor.ungroup"), icon: "Trash2", danger: true, onSelect: () => a.ungroup(group.id) },
  ];
}


/* ------------------------------------------------------------------ */
/* sticky note actions                                                 */
/* ------------------------------------------------------------------ */

export interface CommentMenuActions {
  beginRename: (id: string) => void;
  setColor: (id: string, color: GroupColor) => void;
  remove: (id: string) => void;
}

const COMMENT_COLOR_KEYS: Record<GroupColor, string> = {
  slate: "editor.colorSlate",
  violet: "editor.colorViolet",
  emerald: "editor.colorEmerald",
  amber: "editor.colorAmber",
  sky: "editor.colorSky",
  rose: "editor.colorRose",
};

export function buildCommentMenu(
  comment: { id: string; color: string } | undefined,
  a: CommentMenuActions,
  t: (key: string, vars?: Record<string, unknown>) => string,
): MenuItem[] {
  if (!comment) return [];
  return [
    { label: t("editor.editNote"), icon: "Pencil", onSelect: () => a.beginRename(comment.id) },
    { type: "sep" },
    ...(Object.keys(COMMENT_COLOR_KEYS) as GroupColor[]).map((color) => ({
      label: t(COMMENT_COLOR_KEYS[color]),
      icon: "CircleDot",
      checked: comment.color === color,
      onSelect: () => a.setColor(comment.id, color),
    })),
    { type: "sep" },
    { label: t("editor.deleteNote"), icon: "Trash2", danger: true, hint: "⌫", onSelect: () => a.remove(comment.id) },
  ];
}
