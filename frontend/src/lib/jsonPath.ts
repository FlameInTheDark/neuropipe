/**
 * Builds a human-readable path string from a `@uiw/react-json-view` node key
 * chain (`keys` = full path from the tree root to the node, array indices as
 * numbers). Produces dot notation for bare identifier keys and bracket
 * notation for array indices or keys that aren't valid identifiers:
 *
 *   ["headers", "authorization"]        -> "headers.authorization"
 *   ["items", 0, "customer", "name"]    -> "items[0].customer.name"
 *   ["weird key"]                       -> "[\"weird key\"]"
 *
 * The result drops straight into lodash `get`, JSONPath-ish tooling and most
 * expression languages, mirroring the "Copy property path" convention used by
 * browser DevTools.
 */
const BARE_KEY = /^[A-Za-z_$][A-Za-z0-9_$]*$/;

export function jsonPathToString(keys: ReadonlyArray<string | number>): string {
  return keys.reduce<string>((acc, key) => {
    if (typeof key === "number") return `${acc}[${key}]`;
    if (BARE_KEY.test(key)) return acc ? `${acc}.${key}` : key;
    // non-identifier key (spaces, dots, leading digits…) -> quoted brackets
    return `${acc}[${JSON.stringify(key)}]`;
  }, "");
}
