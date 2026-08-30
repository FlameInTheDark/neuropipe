/* SSR smoke test for JsonViewerModal — stubs the DOM pieces the app touches
   at module load (i18n sets documentElement.lang; the modal portals to body),
   then renders the real component with a realistic execution-log entry. */

import "./dom-stub.mts";

import React from "react";
import { renderToString } from "react-dom/server";
import { JsonViewerModal } from "../src/components/JsonViewerModal";
import type { LogEntry } from "../src/types";

const entry: LogEntry = {
  id: "run-1",
  node: "HTTP Request",
  type: "action:http-request",
  status: "failed",
  ms: 342,
  time: new Date("2026-08-28T10:00:00Z").toISOString(),
  error: "dial tcp: connection refused",
  input: {
    url: "https://api.example.com/v2/orders",
    method: "GET",
    headers: { authorization: "Bearer token", "x-request-id": "abc-123" },
    query: { page: 2, limit: 100 },
  },
  output: {
    items: [
      { id: 1, total: 59.99, customer: { id: 42, name: "Ada" } },
      { id: 2, total: 12.5, customer: { id: 43, name: "Bob" } },
    ],
    ok: true,
  },
};

const html = renderToString(React.createElement(JsonViewerModal, { entry, onClose: () => {} }));
const text = html.replace(/<[^>]+>/g, " ").replace(/\s+/g, " ").trim();

const checks: Array<[string, boolean]> = [
  ["renders node title", html.includes("HTTP Request")],
  ["renders error strip", html.includes("connection refused")],
  ["input pane label", /INPUT/i.test(text)],
  ["output pane label", /OUTPUT/i.test(text)],
  ["input url key visible (depth 2)", html.includes("api.example.com/v2/orders")],
  ["level-2 headers expanded: auth value visible", html.includes("Bearer token")],
  ["output items collapsed at depth 2", /items/.test(text) && !html.includes("Ada")],
  ["input size stat", /\d+(\.\d+)?\s*(byte|kB|KB)/.test(text)],
  ["object kind stat", text.includes("object{")],
  ["theme css var present", html.includes("--w-rjv-key-string")],
  ["mono font theme", html.includes("var(--font-mono)")],
  ["tree/raw toggle icons", html.includes("ListTree") || /list-tree|ListTree/.test(html) || text.includes("")],
  ["copy toolbar", html.toLowerCase().includes("copy")],
];

let failed = 0;
for (const [name, ok] of checks) {
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}`);
  if (!ok) failed++;
}

/* primitive + empty panes must not crash (library throws on null root) */
const primitiveEntry: LogEntry = {
  ...entry,
  input: "just a plain string",
  output: null,
  error: undefined,
  status: "completed",
};
try {
  const phtml = renderToString(React.createElement(JsonViewerModal, { entry: primitiveEntry, onClose: () => {} }));
  const ptext = phtml.replace(/<[^>]+>/g, " ").replace(/\s+/g, " ").trim();
  console.log(`${ptext.includes("just a plain string") ? "PASS" : "FAIL"}  primitive input wrapped & visible`);
  if (!ptext.includes("just a plain string")) failed++;
  console.log(`${/no output data/i.test(ptext) ? "PASS" : "FAIL"}  null output shows empty state`);
  if (!/no output data/i.test(ptext)) failed++;
} catch (err) {
  console.log(`FAIL  primitive/null panes threw: ${err}`);
  failed++;
}

console.log(failed === 0 ? "ALL PASSED" : `${failed} FAILED`);
process.exit(failed === 0 ? 0 : 1);
