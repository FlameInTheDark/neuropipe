/* Regression guard for LLM tool function boundary exec pins.

   The "Big refresh of the UI" refactor made applyFunctionKind /
   applyFunctionInterface strip the exec pin from function:entry /
   function:return whenever kind === "tool". Tool functions always execute as
   an impure subgraph (backend forces tool ⇒ impure), and publish validation
   ("function return must be reachable from Function Entry") plus the runtime
   (child.follow(entryID, "out", …)) traverse exec edges — so tool boundary
   nodes MUST keep exactly one exec pin.

   Part 1 runs the REAL graph-ops helpers against synthetic boundary nodes for
   every function kind. Part 2 pins the call sites so a refactor cannot
   silently re-narrow the condition.

   Run: npx tsx scripts/verify-function-tool-exec-pins.mts */

import { readFileSync } from "node:fs";
import { applyFunctionKind, applyFunctionInterface } from "../src/features/graph/graph-ops";
import type { GraphNode, Port } from "../src/types";

let failures = 0;
function check(name: string, ok: boolean, detail = "") {
  if (ok) {
    console.log(`PASS  ${name}`);
  } else {
    failures += 1;
    console.error(`FAIL  ${name}${detail ? ` — ${detail}` : ""}`);
  }
}

/* ---------- part 1: unit tests on the real code ---------- */

const execOut: Port = { id: "out", label: "Then", kind: "exec" };
const execIn: Port = { id: "in", label: "Exec", kind: "exec" };
const dataOut: Port = { id: "result", label: "Result", kind: "data", dataType: "text" };
const dataIn: Port = { id: "query", label: "Query", kind: "data", dataType: "text" };

function boundaryNode(type: "function:entry" | "function:return"): GraphNode {
  return {
    id: type === "function:entry" ? "entry" : "return",
    type,
    title: type,
    icon: "log-in",
    group: "",
    summary: "",
    x: 0,
    y: 0,
    status: "idle",
    inputs: type === "function:return" ? [execIn, dataIn] : [],
    outputs: type === "function:entry" ? [execOut, dataOut] : [],
    fields: [],
    values: {},
  };
}

for (const kind of ["impure", "tool"] as const) {
  const [entry, ret] = applyFunctionKind([boundaryNode("function:entry"), boundaryNode("function:return")], kind);
  check(
    `applyFunctionKind keeps the exec output on function:entry for ${kind}`,
    entry.outputs.some((p) => p.kind === "exec"),
    JSON.stringify(entry.outputs),
  );
  check(
    `applyFunctionKind keeps the exec input on function:return for ${kind}`,
    ret.inputs.some((p) => p.kind === "exec"),
    JSON.stringify(ret.inputs),
  );
  const [entry2, ret2] = applyFunctionInterface(
    [boundaryNode("function:entry"), boundaryNode("function:return")],
    { kind, inputs: [dataOut], outputs: [dataIn] },
  );
  check(
    `applyFunctionInterface keeps the exec output on function:entry for ${kind}`,
    entry2.outputs.some((p) => p.kind === "exec") && entry2.outputs.some((p) => p.id === "result"),
    JSON.stringify(entry2.outputs),
  );
  check(
    `applyFunctionInterface keeps the exec input on function:return for ${kind}`,
    ret2.inputs.some((p) => p.kind === "exec") && ret2.inputs.some((p) => p.id === "query"),
    JSON.stringify(ret2.inputs),
  );
}

/* pure functions keep their no-exec contract */
const [pureEntry, pureReturn] = applyFunctionKind(
  [boundaryNode("function:entry"), boundaryNode("function:return")],
  "pure",
);
check(
  "applyFunctionKind drops exec pins for pure",
  !pureEntry.outputs.some((p) => p.kind === "exec") && !pureReturn.inputs.some((p) => p.kind === "exec"),
);

/* an exec pin restored for tool must keep its original id so saved wires stay connected */
const restored = applyFunctionInterface([boundaryNode("function:entry")], {
  kind: "tool",
  inputs: [],
  outputs: [],
})[0];
check(
  "applyFunctionInterface preserves the exec pin id (out) for tool",
  restored.outputs.some((p) => p.kind === "exec" && p.id === "out"),
  JSON.stringify(restored.outputs),
);

/* exactly one exec pin survives, even if the graph somehow carries duplicates */
const duplicated = applyFunctionKind(
  [
    {
      ...boundaryNode("function:entry"),
      outputs: [execOut, { ...execOut, id: "out2" }, dataOut],
    },
  ],
  "tool",
)[0];
check(
  "applyFunctionKind collapses duplicate exec pins to one for tool",
  duplicated.outputs.filter((p) => p.kind === "exec").length === 1,
  JSON.stringify(duplicated.outputs),
);

/* ---------- part 2: source-fidelity pins ---------- */

function read(rel: string): string {
  return readFileSync(new URL(rel, import.meta.url), "utf8");
}

const ops = read("../src/features/graph/graph-ops.ts");
check(
  "applyFunctionKind keys exec pins off kind !== 'pure'",
  ops.includes("const withExec = kind !== \"pure\";"),
);
check(
  "applyFunctionInterface keys exec pins off fn.kind !== 'pure'",
  ops.includes("const withExec = fn.kind !== \"pure\";"),
);
check(
  "no 'kind === \"impure\"' exec-pin condition remains in graph-ops",
  !/kind === "impure" \? \[\.\.\.exec/.test(ops),
);

/* the editor must still apply both helpers on load and on kind switch */
const editor = read("../src/features/graph/useGraphEditor.ts");
check(
  "useGraphEditor applies applyFunctionInterface on function load",
  editor.includes("applyFunctionInterface("),
);
check(
  "useGraphEditor applies applyFunctionKind on kind switch",
  editor.includes("applyFunctionKind("),
);

/* ---------- verdict ---------- */

if (failures > 0) {
  console.error(`\n${failures} check(s) FAILED`);
  process.exit(1);
}
console.log("\nALL PASSED");
