/* Browser entry for the copy-path live test: mounts the REAL JsonViewerModal
   client-side (the Copied section override only registers via useEffect, so
   SSR cannot exercise it) with a clipboard stub that records every write to
   window.__clipboardWrites. window.__swapPrimitive() re-renders with a
   primitive input payload so the { value } wrap row's path can be verified
   (it must copy "$", not "$.value"). Bundled to IIFE by render-copypath-live.mts
   and inlined into a standalone page next to the app's compiled CSS. */

import React from "react";
import { createRoot } from "react-dom/client";
import { JsonViewerModal } from "../src/components/JsonViewerModal";
import type { LogEntry } from "../src/types";

/* deterministic clipboard capture — installed BEFORE any component can click */
const writes: string[] = [];
(window as unknown as Record<string, unknown>).__clipboardWrites = writes;
const stub = {
  writeText: (text: string) => {
    writes.push(text);
    return Promise.resolve();
  },
};
try {
  Object.defineProperty(Navigator.prototype, "clipboard", {
    configurable: true,
    get: () => stub,
  });
} catch {
  (navigator as unknown as Record<string, unknown>).clipboard = stub;
}

const entry: LogEntry = {
  id: "exec-copypath-1",
  node: "HTTP Request",
  type: "action:http-request",
  status: "completed",
  ms: 128,
  time: new Date("2026-08-28T10:14:03Z").toISOString(),
  input: {
    url: "https://api.example.com/v2/orders",
    headers: { authorization: "Bearer token", accept: "application/json", "x-request-id": "abc-123" },
    query: { page: 2, limit: 100 },
  },
  output: {
    items: [
      { id: 1, customer: { name: "Ada" } },
      { id: 2, customer: { name: "Bob" } },
    ],
    ok: true,
  },
};

const el = document.getElementById("root");
if (!el) throw new Error("#root missing");
const root = createRoot(el);
const render = (current: LogEntry) => root.render(<JsonViewerModal entry={current} onClose={() => {}} />);
render(entry);
(window as unknown as Record<string, unknown>).__ready = true;
/* re-render with a primitive input payload: the pane wraps it as { value },
   and that row's copy-path must yield "$" */
(window as unknown as Record<string, unknown>).__swapPrimitive = () => {
  writes.length = 0;
  render({ ...entry, input: "just a plain string" });
};
