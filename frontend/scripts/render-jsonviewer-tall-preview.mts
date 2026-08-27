/* Renders JsonViewerModal with a TALL payload (hundreds of visible tree rows)
   into two standalone pages — "fixed" (current component) and "broken" (the
   grid-rows-1 row constraint stripped, reproducing the pre-fix DOM) — so the
   scroll regression can be verified in a real browser.
   Output: /home/z/my-project/scripts/jsonviewer-tall-{fixed,broken}.html */

import "./dom-stub.mts";

import React from "react";
import { renderToString } from "react-dom/server";
import { writeFileSync, readFileSync } from "node:fs";
import { JsonViewerModal } from "../src/components/JsonViewerModal";
import type { LogEntry } from "../src/types";

/* ~230 visible rows per pane at the default collapsed=2 depth: 40 depth-1
   objects × (1 key row + 5 child rows) + root scalar rows. Far taller than
   the ~800px pane, so the panes MUST scroll after the fix. */
const tallInput: Record<string, unknown> = {
  source: "discord",
  channel_id: "1098427362547",
  guild: "NeuroPipe Community",
  triggered_by: "webhook.message.created",
};
for (let i = 0; i < 40; i++) {
  tallInput[`field_${String(i).padStart(2, "0")}`] = {
    kind: "payload",
    seq: i,
    label: `payload segment ${i}`,
    weight: i * 1.5,
    active: i % 2 === 0,
  };
}

const tallOutput: Record<string, unknown> = {
  status: "completed",
  emitted: 40,
  duration_ms: 2873,
};
for (let i = 0; i < 40; i++) {
  tallOutput[`result_${String(i).padStart(2, "0")}`] = {
    ok: true,
    score: 0.5 + i / 100,
    note: `processed segment ${i} without errors`,
    tries: (i % 3) + 1,
    cached: i % 4 === 0,
  };
}

const entry: LogEntry = {
  id: "exec-tall-1",
  node: "Webhook Receive",
  type: "trigger:webhook",
  status: "completed",
  ms: 2873,
  time: new Date("2026-08-28T10:14:03Z").toISOString(),
  input: tallInput,
  output: tallOutput,
};

const body = renderToString(React.createElement(JsonViewerModal, { entry, onClose: () => {} }));

const FIXED_GRID = "grid min-h-0 flex-1 grid-cols-2 grid-rows-1 divide-x divide-seam";
const BROKEN_GRID = "grid min-h-0 flex-1 grid-cols-2 divide-x divide-seam";
const FIXED_SECTION = "flex min-h-0 min-w-0 flex-col";
const OLD_SECTION = "flex min-w-0 flex-col";
if (!body.includes(FIXED_GRID)) throw new Error("fixed grid classes not found in render output");
if (!body.includes(FIXED_SECTION)) throw new Error("fixed section classes not found in render output");
/* broken = row constraint stripped only; old = BOTH pre-fix classes stripped
   (exactly the DOM users of the shipped build saw) */
const brokenBody = body.replace(FIXED_GRID, BROKEN_GRID);
const oldBody = body.replace(FIXED_GRID, BROKEN_GRID).replace(FIXED_SECTION, OLD_SECTION);

/* the single-file dist build carries the full compiled stylesheet inline
   (resolved from the frontend working directory). The JS bundle contains a
   highlight.js grammar regex that looks like a <style> tag, so grab the LAST
   real style element by position and validate it carries compiled app CSS. */
const dist = readFileSync(`${process.cwd()}/dist/index.html`, "utf8");
const open = dist.lastIndexOf("<style");
const close = dist.lastIndexOf("</style>");
const styles = open >= 0 && close > open ? dist.slice(dist.indexOf(">", open) + 1, close) : "";
if (!styles.includes(".bg-ink-900")) throw new Error("compiled CSS not found in dist/index.html");
if (!styles.includes(".grid-rows-1")) throw new Error("grid-rows-1 missing from compiled CSS — rebuild dist");

const page = (inner: string) => `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>JsonViewerModal tall-payload preview</title>
<style>${styles}</style>
</head>
<body style="background:#08080a">
${inner}
</body>
</html>`;

writeFileSync("/home/z/my-project/scripts/jsonviewer-tall-fixed.html", page(body));
writeFileSync("/home/z/my-project/scripts/jsonviewer-tall-broken.html", page(brokenBody));
writeFileSync("/home/z/my-project/scripts/jsonviewer-tall-old.html", page(oldBody));
console.log("wrote /home/z/my-project/scripts/jsonviewer-tall-fixed.html");
console.log("wrote /home/z/my-project/scripts/jsonviewer-tall-broken.html");
console.log("wrote /home/z/my-project/scripts/jsonviewer-tall-old.html");
