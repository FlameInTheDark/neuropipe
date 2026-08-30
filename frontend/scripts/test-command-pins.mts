/* Unit tests for the Discord application command trigger's dynamic option
   pins: the frontend twin must grow one typed output pin per stored command
   option (text / number / boolean, mirroring the Go resolver), and the three
   reply nodes must mirror their embed document pins.
   Run: npx tsx scripts/test-command-pins.mts */

import "./dom-stub.mts";
import {
  resolveConfigDrivenInputs,
  resolveConfigDrivenOutputs,
} from "../src/lib/blueprint-dynamic-pins";
import { isBackendResolvedType } from "../src/lib/adapters";
import type { NodeDefinition } from "../src/lib/types";

let failed = 0;
let passed = 0;
function check(name: string, ok: boolean, detail?: string) {
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${ok || !detail ? "" : ` — ${detail}`}`);
  if (ok) passed++;
  else failed++;
}

const triggerDefinition: NodeDefinition = {
  type: "discord:app_command",
  label: "Discord Command Trigger",
  category: "Discord",
  outputs: [
    { id: "out", label: "Start", kind: "exec", direction: "output", color: "#fafafa", maxConnections: 1 },
    { id: "commandName", label: "Command name", kind: "data", direction: "output", dataType: "text" },
  ],
  inputs: [],
  fields: [],
} as unknown as NodeDefinition;

const weatherConfig = {
  command: {
    commandId: "800001",
    commandName: "weather",
    options: [
      { name: "city", type: 3, required: true },
      { name: "days", type: 4, required: false },
      { name: "metric", type: 5, required: false },
      { name: "scale", type: 10, required: false },
      { name: "user", type: 6, required: false },
    ],
  },
};

const outputs = resolveConfigDrivenOutputs(triggerDefinition, weatherConfig);
const byID = new Map(outputs.map((pin) => [pin.id, pin]));

check("trigger keeps its envelope pins", byID.has("commandName") && byID.has("out"));
check("one pin per command option", outputs.length === 2 + 5, `got ${outputs.length}`);
check("text option becomes a text pin", byID.get("city")?.dataType === "text");
check("integer option becomes a number pin", byID.get("days")?.dataType === "number");
check("number option becomes a number pin", byID.get("scale")?.dataType === "number");
check("boolean option becomes a boolean pin", byID.get("metric")?.dataType === "boolean");
check("user option stays text (snowflake)", byID.get("user")?.dataType === "text");
check("required flag survives", byID.get("city")?.required === true && byID.get("days")?.required !== true);
check("integer pin carries the int spec", (byID.get("days")?.type as { kind?: string } | undefined)?.kind === "int");
check("boolean pin carries the bool spec", (byID.get("metric")?.type as { kind?: string } | undefined)?.kind === "bool");

const noSelection = resolveConfigDrivenOutputs(triggerDefinition, {});
check("no command picked keeps only envelope pins", noSelection.length === 2, `got ${noSelection.length}`);

/* the trigger's envelope pins (event, command, INTERACTION, …) exist only in
   the backend-resolved contract: the static catalog carries just Start, and
   the frontend mirror only appends option pins. Without backend resolution
   the Reply/Followup/Edit nodes have no Interaction output to wire from. */
check(
  "command trigger is backend-resolved (interaction pin reaches the canvas)",
  isBackendResolvedType("discord:app_command"),
);
check("discord event trigger stays backend-resolved", isBackendResolvedType("discord:event"));
check("static nodes are not backend-resolved", !isBackendResolvedType("data:build_object"));

const malformed = resolveConfigDrivenOutputs(triggerDefinition, {
  command: { options: [{ name: "", type: 3 }, { name: "ok", type: 3 }, "garbage"] },
});
check("malformed options are skipped safely", malformed.length === 3, `got ${malformed.length}`);

/* the reply nodes mirror their embed document template pins */
const replyDefinition: NodeDefinition = {
  type: "action:discord_reply_command",
  label: "Reply to Command",
  category: "Discord",
  inputs: [
    { id: "interaction", label: "Interaction", kind: "data", direction: "input", dataType: "object" },
    { id: "message", label: "Message", kind: "data", direction: "input", dataType: "text" },
  ],
  outputs: [],
  fields: [],
} as unknown as NodeDefinition;

const replyInputs = resolveConfigDrivenInputs(replyDefinition, {
  embeds: {
    embeds: [{ title: "Weather in {{city}}" }],
    pins: [
      { name: "city", type: "text" },
      { name: "temp", type: "number" },
      { name: "metric", type: "boolean" },
    ],
  },
});
const replyIDs = new Map(replyInputs.map((pin) => [pin.id, pin]));
check("reply node grows embed template pins", replyIDs.has("city") && replyInputs.length === 5, `got ${replyInputs.length}`);
check("embed number pin typed", replyIDs.get("temp")?.dataType === "number");
check("embed boolean pin typed", replyIDs.get("metric")?.dataType === "boolean");

const editInputs = resolveConfigDrivenInputs(
  { ...replyDefinition, type: "action:discord_edit_command_reply" } as NodeDefinition,
  { embeds: { embeds: [{ title: "{{v}}" }], pins: [{ name: "v", type: "text" }] } },
);
check(
  "edit node mirrors embed pins too",
  editInputs.some((pin) => pin.id === "v"),
  `got ${editInputs.map((p) => p.id).join(",")}`,
);

if (failed > 0) {
  console.log(`\n${failed} failed`);
  process.exit(1);
}
console.log(`\n${passed} passed, ${failed} failed`);
