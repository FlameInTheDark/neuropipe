/* Verification for the function-editor trigger filter: trigger nodes must not
   appear in the node library or the canvas add-node picker while a function is
   open, and both the frontend add/paste paths and the backend draft-save path
   reject them outright.

   Part 1 runs the REAL buildLibrary / isTriggerDefinition / localizeNodeDefinitions
   against synthetic definitions (DOM stub keeps the i18n import happy).
   Part 2 pins the wiring in App.tsx / useWorkspace.ts / useGraphEditor.ts, the
   i18n keys, and the backend guards, so a refactor cannot silently drop them.

   Run: npx tsx scripts/verify-function-trigger-filter.mts */

import "./dom-stub.mts";
import { readFileSync } from "node:fs";
import { buildLibrary, isTriggerDefinition } from "../src/lib/adapters";
import { localizeNodeDefinitions } from "../src/lib/node-catalog";
import type { NodeDefinition } from "../src/lib/types";

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

function def(partial: Partial<NodeDefinition> & { type: string; category: string; label: string }): NodeDefinition {
  return {
    description: "",
    icon: "Boxes",
    color: "#fff",
    mode: "impure",
    inputs: [],
    outputs: [],
    fields: [],
    capabilities: [],
    defaultConfig: {},
    source: "builtin",
    ...partial,
  } as NodeDefinition;
}

/* mirrors the real catalog: legacy triggers use the trigger: prefix, module
   triggers use provider prefixes and the TriggerKind flag */
const definitions: NodeDefinition[] = [
  def({ type: "trigger:button", category: "Triggers", label: "Button Trigger", triggerKind: "button" }),
  def({ type: "trigger:custom", category: "Triggers", label: "Plugin Trigger" }), // prefix-only, no flag
  def({ type: "twitch:event", category: "Twitch", label: "Twitch Event Trigger", triggerKind: "twitch", mode: "event" }),
  def({ type: "discord:event", category: "Discord", label: "Discord Event Trigger", triggerKind: "discord", mode: "event" }),
  def({ type: "telegram:event", category: "Telegram", label: "Telegram Event Trigger", triggerKind: "telegram", mode: "event" }),
  def({ type: "kv:subscribe", category: "KV Store", label: "KV Subscribe Trigger", triggerKind: "kvsubscribe", mode: "event" }),
  def({ type: "action:http-request", category: "Actions", label: "HTTP Request" }),
  def({ type: "plugin:helper", category: "Actions", label: "Plugin Helper", source: "plugin" }),
  def({ type: "function:entry", category: "Functions", label: "Function Entry", mode: "event" }),
];

const types = (library: ReturnType<typeof buildLibrary>) => library.flatMap((c) => c.items.map((i) => i.type ?? ""));

const pipelineLib = types(buildLibrary(definitions, []));
const functionLib = types(buildLibrary(definitions, [], { excludeTriggers: true }));

/* pipeline library: unchanged behavior — module triggers stay visible */
check(
  "pipeline library keeps module triggers (twitch/discord/telegram/kv)",
  ["twitch:event", "discord:event", "telegram:event", "kv:subscribe"].every((t) => pipelineLib.includes(t)),
  `got [${pipelineLib.join(", ")}]`,
);
check(
  "pipeline library hides legacy trigger:* nodes",
  !pipelineLib.some((t) => t.startsWith("trigger:")),
);
check(
  "pipeline library hides function boundary nodes",
  !pipelineLib.includes("function:entry"),
);

/* function library: every trigger gone, everything else intact */
const allTriggers = ["trigger:button", "trigger:custom", "twitch:event", "discord:event", "telegram:event", "kv:subscribe"];
check(
  "function library excludes every trigger (flagged AND prefix-only)",
  allTriggers.every((t) => !functionLib.includes(t)),
  `still present: [${allTriggers.filter((t) => functionLib.includes(t)).join(", ")}]`,
);
check(
  "function library keeps non-trigger nodes",
  ["action:http-request", "plugin:helper"].every((t) => functionLib.includes(t)),
);

/* empty categories must collapse instead of rendering a dead group header */
const triggerOnlyCategory = buildLibrary(
  [def({ type: "twitch:event", category: "Twitch", label: "Twitch Event Trigger", triggerKind: "twitch", mode: "event" })],
  [],
  { excludeTriggers: true },
);
check("function library drops categories that become empty", triggerOnlyCategory.length === 0, `got ${triggerOnlyCategory.length} categories`);

/* isTriggerDefinition edge cases */
check("isTriggerDefinition(undefined) === false", isTriggerDefinition(undefined) === false);
check("isTriggerDefinition: flagged without prefix", isTriggerDefinition({ type: "kv:subscribe", triggerKind: "kvsubscribe" }) === true);
check("isTriggerDefinition: prefix without flag", isTriggerDefinition({ type: "trigger:custom" }) === true);
check("isTriggerDefinition: plain node", isTriggerDefinition({ type: "action:http-request" }) === false);
check("isTriggerDefinition: function boundary is not a trigger", isTriggerDefinition({ type: "function:entry" }) === false);

/* localization must preserve the flag — it rebuilds the definition object */
const localized = localizeNodeDefinitions(
  [def({ type: "twitch:event", category: "Twitch", label: "Twitch Event Trigger", triggerKind: "twitch", mode: "event" })],
  "de",
);
check(
  "localizeNodeDefinitions preserves triggerKind",
  localized[0]?.triggerKind === "twitch",
  `got ${String(localized[0]?.triggerKind)}`,
);

/* ---------- part 2: source-fidelity pins ---------- */

function read(rel: string): string {
  return readFileSync(new URL(rel, import.meta.url), "utf8");
}

const adapters = read("../src/lib/adapters.ts");
const workspace = read("../src/features/workspace/useWorkspace.ts");
const app = read("../src/App.tsx");
const editor = read("../src/features/graph/useGraphEditor.ts");

/* the filter helper is exported from adapters so the graph editor reuses it */
check("adapters exports isTriggerDefinition", adapters.includes("export function isTriggerDefinition"));
check("buildLibrary accepts excludeTriggers option", adapters.includes("opts?: { excludeTriggers?: boolean }"));
check(
  "buildLibrary applies the trigger filter",
  adapters.includes("(opts?.excludeTriggers && isTriggerDefinition(def))"),
);

/* the workspace exposes a dedicated function-mode library */
check(
  "useWorkspace builds functionLibrary with excludeTriggers",
  workspace.includes("buildLibrary(definitions, [], { excludeTriggers: true })"),
);
check("useWorkspace exports functionLibrary", /^\s{4}functionLibrary,$/m.test(workspace));

/* the editor receives the filtered library only in function mode */
check(
  "App passes functionLibrary when editorKind is function",
  app.includes('library={nav.editorKind === "function" ? workspace.functionLibrary : workspace.library}'),
);

/* addNode: last line of defense for drags/stale pickers */
check(
  "addNode rejects triggers in function mode",
  editor.includes('if (mode === "function" && isTriggerDefinition(definition)) {') &&
    editor.includes('notify(i18n.t("editor.triggerInFunction"), "AlertTriangle");'),
);
check(
  "addNode lists mode in its hook deps",
  /\[definitionIndex, mode, notify, touch\]/.test(editor),
);

/* paste: nodes copied in a pipeline session must not smuggle triggers in */
check(
  "pasteClipboard filters triggers in function mode",
  editor.includes('mode === "function" ? source.filter((n) => !isTriggerDefinition(definitionIndex[n.type])) : source'),
);
check(
  "pasteClipboard notifies when triggers were dropped",
  editor.includes('if (allowed.length < source.length) notify(i18n.t("editor.triggerInFunction"), "AlertTriangle");'),
);

/* i18n keys must exist in all four locales */
for (const locale of ["en", "de", "fr", "ru"]) {
  const file = read(`../src/i18n/${locale}.ts`);
  check(`editor.triggerInFunction present in ${locale}.ts`, file.includes("triggerInFunction:"));
}

/* backend guards: draft save rejects triggers; invariant test exists */
const desktop = read("../../internal/app/desktop.go");
check(
  "SaveFunction validates triggers before persisting",
  desktop.includes("if err := validateFunctionTriggers(function, d.registry); err != nil {"),
);
check(
  "validateFunctionTriggers checks the TriggerKind flag",
  desktop.includes('if definition.TriggerKind != "" {'),
);
const validationTest = read("../../internal/app/function_validation_test.go");
check("backend test TestValidateFunctionTriggers exists", validationTest.includes("func TestValidateFunctionTriggers("));
const catalogTest = read("../../internal/catalog/trigger_flag_test.go");
check("catalog invariant test TestTriggerFlagMatchesEventMode exists", catalogTest.includes("func TestTriggerFlagMatchesEventMode("));

/* ---------- verdict ---------- */

if (failures > 0) {
  console.error(`\n${failures} check(s) FAILED`);
  process.exit(1);
}
console.log("\nALL PASSED");
