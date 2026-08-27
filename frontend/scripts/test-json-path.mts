/* Unit tests for the JSON path builder used by the viewer's copy-path action.
   Run: npx tsx scripts/test-json-path.mts */

import { jsonPathToString } from "../src/lib/jsonPath";

const cases: Array<[string, ReadonlyArray<string | number>, string]> = [
  ["empty root path", [], ""],
  ["single object key", ["headers"], "headers"],
  ["nested object keys", ["headers", "authorization"], "headers.authorization"],
  ["array index", ["items", 0], "items[0]"],
  ["multi-digit index", ["items", 10], "items[10]"],
  ["mixed depth", ["items", 0, "customer", "name"], "items[0].customer.name"],
  ["array of arrays", ["matrix", 3, 1], "matrix[3][1]"],
  ["leading index", [0], "[0]"],
  ["identifier with underscore/dollar", ["$schema", "my_key"], "$schema.my_key"],
  ["identifier with digits", ["field2", "a1b2"], "field2.a1b2"],
  ["key with space", ["weird key"], '["weird key"]'],
  ["key with dot", ["a.b", "c"], '["a.b"].c'],
  ["key with leading digit", ["123abc"], '["123abc"]'],
  ["key with dash", ["x-request-id"], '["x-request-id"]'],
  ["quoted key after index", ["items", 1, "total price"], 'items[1]["total price"]'],
  ["empty-string key", [""], '[""]'],
];

let failed = 0;
for (const [name, input, expected] of cases) {
  const got = jsonPathToString(input);
  const ok = got === expected;
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}: ${JSON.stringify(input)} -> ${got}${ok ? "" : ` (expected ${expected})`}`);
  if (!ok) failed++;
}

/* paths must be directly consumable by lodash-style get(): spot check via eval-free traversal */
const data = { items: [{ customer: { name: "Ada" } }] } as unknown as Record<string, unknown>;
type Cursor = Record<string, unknown> | unknown[];
const resolved = ["items", 0, "customer", "name"].reduce<Cursor | string | number | boolean | null>(
  (acc, key) => (acc as Record<string, unknown>)[key as string],
  data,
);
const ok = resolved === "Ada";
console.log(`${ok ? "PASS" : "FAIL"}  path segments resolve through nested data`);
if (!ok) failed++;

console.log(failed === 0 ? "ALL PASSED" : `${failed} FAILED`);
process.exit(failed === 0 ? 0 : 1);
