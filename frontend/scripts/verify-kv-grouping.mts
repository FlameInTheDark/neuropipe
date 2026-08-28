// Behavioral + source-fidelity check for the KV browser's grouped keys view.
// Part 1 unit-tests the pure tree builder (src/lib/kvKeyTree.ts) — separator
// splitting, nesting, sorting, counts, dedupe, degenerate separators.
// Part 2 pins the wiring in KVBrowser.tsx and the i18n keys so a refactor
// can't silently drop the view toggle, the separator input, or the tree.
//
// Run: npx tsx scripts/verify-kv-grouping.mts
import { readFileSync } from "node:fs";
import { buildKeyTree, type KVKeyLike, type KeyTreeNode } from "../src/lib/kvKeyTree";

let failures = 0;
let checks = 0;

function ok(condition: boolean, message: string): void {
  checks += 1;
  if (!condition) {
    failures += 1;
    console.error(`FAIL: ${message}`);
  }
}

function eq<T>(actual: T, expected: T, message: string): void {
  const a = JSON.stringify(actual);
  const b = JSON.stringify(expected);
  ok(a === b, `${message} — expected ${b}, got ${a}`);
}

function key(name: string, type = "string", ttl = -1): KVKeyLike {
  return { name, type, ttl };
}

function names(nodes: KeyTreeNode[]): string[] {
  return nodes.map((n) => n.name);
}

function folder(nodes: KeyTreeNode[], name: string): KeyTreeNode[] {
  const found = nodes.find((n) => n.kind === "folder" && n.name === name);
  if (!found || found.kind !== "folder") throw new Error(`folder ${name} not found`);
  return found.children;
}

/* ---------- Part 1: buildKeyTree behavior ---------- */

// The user's canonical example: test:0 -> category "test", key "0".
{
  const tree = buildKeyTree([key("test:0")], ":");
  eq(names(tree), ["test"], "test:0 groups under folder test");
  const inside = folder(tree, "test");
  eq(names(inside), ["0"], "leaf shows the segment 0");
  ok(inside[0].kind === "leaf" && inside[0].key.name === "test:0", "leaf carries the full key");
  eq((tree[0] as { count: number }).count, 1, "folder count is 1");
}

// Multi-level nesting: user:profile:42 -> user / profile / 42.
{
  const tree = buildKeyTree([key("user:profile:42")], ":");
  const level2 = folder(folder(tree, "user"), "profile");
  eq(names(level2), ["42"], "three-segment key nests three levels");
  eq((tree[0] as { count: number }).count, 1, "top folder counts nested key");
}

// Custom separators: single char and multi-char.
{
  const slash = buildKeyTree([key("test/0"), key("other")], "/");
  eq(names(slash), ["test", "other"], "slash separator: folder sorts before the lone leaf");
  eq(names(folder(slash, "test")), ["0"], "slash separator groups test/0");

  const dbl = buildKeyTree([key("a::b")], "::");
  eq(names(dbl), ["a"], "multi-char separator groups");
  eq(names(folder(dbl, "a")), ["b"], "multi-char separator splits");
}

// Folder-is-also-a-key: the standalone key coexists with the folder.
{
  const tree = buildKeyTree([key("test"), key("test:0"), key("test:1")], ":");
  const kinds = tree.map((n) => n.kind);
  eq(kinds, ["folder", "leaf"], "folder sorts before the same-named leaf");
  const f = tree[0];
  ok(f.kind === "folder" && f.count === 2, "folder counts its 2 inner keys");
  const leaf = tree[1];
  ok(leaf.kind === "leaf" && leaf.key.name === "test", "the exact key stays reachable as a leaf");
}

// Empty separator degenerates to a flat sorted root of full names.
{
  const tree = buildKeyTree([key("b:1"), key("a:2"), key("c")], "");
  eq(names(tree), ["a:2", "b:1", "c"], "empty separator keeps full names, sorted");
  ok(tree.every((n) => n.kind === "leaf"), "no folders without a separator");
  const nullTree = buildKeyTree([key("x")], "" as unknown as string);
  eq(names(nullTree), ["x"], "no crash on empty separator");
}

// Numeric-aware ordering: user:2 before user:10 inside a folder.
{
  const tree = buildKeyTree([key("user:10"), key("user:2")], ":");
  eq(names(folder(tree, "user")), ["2", "10"], "numeric collation sorts 2 before 10");
}

// Folders sort before leaves at the same level; both alphabetical.
{
  const tree = buildKeyTree([key("zebra"), key("alpha:1"), key("beta:1")], ":");
  eq(names(tree), ["alpha", "beta", "zebra"], "folders first, then leaves");
}

// SCAN revisits: duplicate key names emit a single leaf.
{
  const tree = buildKeyTree([key("dup"), key("dup")], ":");
  eq(names(tree), ["dup"], "duplicate scan results collapse");
}

// Empty segments (keys like ":foo" or "a::b") survive as (empty)-named nodes.
{
  const tree = buildKeyTree([key(":foo")], ":");
  const f = tree[0];
  ok(f.kind === "folder" && f.name === "", "leading separator yields an empty-named root folder");
  eq(names(folder(tree, "")), ["foo"], "the key still nests under it");
}

// Mixed depth inside one namespace + recursive counts.
{
  const tree = buildKeyTree(
    [key("cfg:db:host"), key("cfg:db:port"), key("cfg:debug"), key("cfg")],
    ":",
  );
  const cfg = tree[0];
  ok(cfg.kind === "folder" && cfg.count === 3, "cfg folder counts all 3 nested keys");
  const inner = folder(tree, "cfg");
  eq(names(inner), ["db", "debug"], "nested folder and direct leaf sort together, folders first");
}

// Path reconstruction: folder.path joins segments with the live separator.
{
  const tree = buildKeyTree([key("a:b:c")], ":");
  const ab = folder(tree, "a")[0];
  ok(ab.kind === "folder" && ab.path === "a:b", "nested folder path is prefix-joined");
}

// Leaf metadata flows through untouched.
{
  const tree = buildKeyTree([{ name: "s", type: "set", ttl: 90, size: 12 }], ":");
  const leaf = tree[0];
  ok(
    leaf.kind === "leaf" && leaf.key.type === "set" && leaf.key.ttl === 90 && leaf.key.size === 12,
    "leaf carries type/ttl/size for the row renderer",
  );
}

/* ---------- Part 2: KVBrowser wiring pins ---------- */

const browser = readFileSync(new URL("../src/features/database/KVBrowser.tsx", import.meta.url), "utf8");

ok(browser.includes(`import { buildKeyTree, type KeyTreeNode } from "@/lib/kvKeyTree";`), "KVBrowser imports the tree builder");
ok(browser.includes(`usePersistedChoice<"list" | "grouped">(\n    "neuropipe.kv.keysView.v1",`), "view mode is persisted via prefs");
ok(browser.includes(`usePersistedValue("neuropipe.kv.keysSeparator.v1", ":")`), "separator is persisted with : default");
ok(browser.includes("const tree = useMemo(() => buildKeyTree(keys, separator), [keys, separator]);"), "tree rebuilds from loaded keys + separator only (no refetch)");
ok(browser.includes('{ id: "list", icon: "List", label: t("kv.viewList") }'), "list toggle entry present");
ok(browser.includes('{ id: "grouped", icon: "ListTree", label: t("kv.viewGrouped") }'), "grouped toggle entry present");
ok(browser.includes("aria-pressed={view === entry.id}"), "toggle buttons expose aria-pressed");
ok(browser.includes('view === "grouped" && ('), "separator input renders only in grouped mode");
ok(browser.includes('aria-label={t("kv.separatorLabel")}'), "separator input is labelled");
ok(browser.includes('view === "grouped" ? (\n            <KeyTreeRows'), "grouped mode renders the tree");
ok(browser.includes("onToggle={(path) => setExpanded((prev) => ({ ...prev, [path]: !prev[path] }))}"), "folder toggle flips expansion by path");
ok(browser.includes("aria-expanded={expanded[node.path] ?? false}"), "folder rows expose aria-expanded");
ok(browser.includes(`{node.name || t("kv.emptySegment")}`), "empty segments are labeled");
ok(browser.includes("{node.count}"), "folder rows show the subtree count");
ok(browser.includes('onContextMenu={(e) => onContext(e, node.key)}'), "tree leaves keep the flat list's context menu");
ok(browser.includes('selected === node.key.name && "bg-ink-800/70"'), "tree leaves keep the selection highlight");
ok(browser.includes("keys.map((key) => ("), "flat list rendering retained for list mode");

/* i18n: the four new keys must exist in every locale. */
for (const locale of ["en", "de", "fr", "ru"]) {
  const dict = readFileSync(new URL(`../src/i18n/${locale}.ts`, import.meta.url), "utf8");
  for (const k of ["viewList:", "viewGrouped:", "separatorLabel:", "emptySegment:"]) {
    ok(dict.includes(`    ${k}`), `kv.${k.slice(0, -1)} missing in ${locale}`);
  }
}

/* ---------- verdict ---------- */

if (failures > 0) {
  console.error(`\n${failures}/${checks} checks FAILED`);
  process.exit(1);
}
console.log(`ALL PASSED (${checks} checks)`);
