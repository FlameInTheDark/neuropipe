/**
 * Builds a JSONPath expression from a `@uiw/react-json-view` node key chain
 * (`keys` = full path from the tree root to the node, array indices as
 * numbers). The result is rooted at `$` and uses the classic Goessner
 * notation the Query JSON node evaluates:
 *
 *   []                                   -> "$"
 *   ["headers", "authorization"]         -> "$.headers.authorization"
 *   ["items", 0, "customer", "name"]     -> "$.items[0].customer.name"
 *   ["weird key"]                        -> "$[\"weird key\"]"
 *
 * Bare identifier keys become `.name` segments; array indices and keys that
 * aren't valid identifiers use the bracket form (quoted where needed), which
 * every JSONPath implementation accepts. The output pastes straight into the
 * JSON path field of the Query JSON node.
 */
const BARE_KEY = /^[A-Za-z_$][A-Za-z0-9_$]*$/;

export function jsonPathToString(keys: ReadonlyArray<string | number>): string {
  return keys.reduce<string>((acc, key) => {
    if (typeof key === "number") return `${acc}[${key}]`;
    if (BARE_KEY.test(key)) return `${acc}.${key}`;
    // non-identifier key (spaces, dots, leading digits…) -> quoted brackets
    return `${acc}[${JSON.stringify(key)}]`;
  }, "$");
}
