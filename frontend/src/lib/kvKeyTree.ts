/* Key-tree grouping for the KV browser's Keys tab.
   Keys are split on a caller-supplied separator (default ":") into a
   collapsible namespace tree: "test:0" becomes folder "test" containing leaf
   "0". Pure and dependency-free so it is trivially unit-testable; the caller
   passes KVKey records (or anything structurally compatible). */

export interface KVKeyLike {
  name: string;
  type: string;
  ttl: number;
  encoding?: string;
  size?: number;
}

export interface KeyTreeFolder {
  kind: "folder";
  /** Display segment (may be "" for empty segments like "a::b"). */
  name: string;
  /** Full key prefix this folder stands for, segments joined by the separator. */
  path: string;
  /** Number of keys anywhere inside this subtree. */
  count: number;
  children: KeyTreeNode[];
}

export interface KeyTreeLeaf {
  kind: "leaf";
  /** Display segment — the FULL key name for root-level leaves. */
  name: string;
  key: KVKeyLike;
}

export type KeyTreeNode = KeyTreeFolder | KeyTreeLeaf;

const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: "base" });

/**
 * Groups scanned keys into a sorted tree.
 *
 * - An empty/null separator degenerates to a flat, alphabetically sorted list
 *   of root leaves (every key is its own segment).
 * - Sorting is folders-first, then numeric-aware alphabetical ("user:2" sorts
 *   before "user:10").
 * - Duplicate key names (SCAN cursors may revisit a key) are emitted once.
 * - A key that is also a prefix of other keys coexists with its folder: the
 *   leaf renders alongside the folder at the same level.
 */
export function buildKeyTree(keys: readonly KVKeyLike[], separator: string): KeyTreeNode[] {
  const sep = typeof separator === "string" && separator.length > 0 ? separator : null;
  const root: KeyTreeNode[] = [];
  const folders = new Map<string, KeyTreeFolder>();
  const seen = new Set<string>();

  const childrenOf = (parentPath: string | null): KeyTreeNode[] =>
    parentPath === null ? root : (folders.get(parentPath)?.children ?? root);

  const ensureFolder = (parentPath: string | null, name: string): KeyTreeFolder => {
    const path = parentPath === null ? name : parentPath + sep + name;
    let folder = folders.get(path);
    if (!folder) {
      folder = { kind: "folder", name, path, count: 0, children: [] };
      folders.set(path, folder);
      childrenOf(parentPath).push(folder);
    }
    return folder;
  };

  for (const key of keys) {
    if (seen.has(key.name)) continue;
    seen.add(key.name);

    const segments = sep ? key.name.split(sep) : [key.name];
    let parentPath: string | null = null;
    for (let i = 0; i < segments.length - 1; i += 1) {
      const folder = ensureFolder(parentPath, segments[i]);
      folder.count += 1;
      parentPath = folder.path;
    }
    childrenOf(parentPath).push({ kind: "leaf", name: segments[segments.length - 1], key });
  }

  const sortNodes = (nodes: KeyTreeNode[]): void => {
    nodes.sort((a, b) => {
      if (a.kind !== b.kind) return a.kind === "folder" ? -1 : 1;
      return collator.compare(a.name, b.name);
    });
    for (const node of nodes) if (node.kind === "folder") sortNodes(node.children);
  };
  sortNodes(root);

  return root;
}
