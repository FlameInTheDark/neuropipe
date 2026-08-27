/* Renders JsonViewerModal to a standalone static HTML page using the app's
   compiled CSS (extracted from the single-file dist build) so the design can
   be reviewed in a browser. Output: /home/z/my-project/scripts/jsonviewer-preview.html */

import "./dom-stub.mts";

import React from "react";
import { renderToString } from "react-dom/server";
import { writeFileSync, readFileSync } from "node:fs";
import { JsonViewerModal } from "../src/components/JsonViewerModal";
import type { LogEntry } from "../src/types";

const entry: LogEntry = {
  id: "exec-9f2c",
  node: "HTTP Request",
  type: "action:http-request",
  status: "failed",
  ms: 342,
  time: new Date("2026-08-28T10:14:03Z").toISOString(),
  error: 'request failed: Get "https://api.example.com/v2/orders?page=2": dial tcp 93.184.216.34:443: connect: connection refused',
  input: {
    url: "https://api.example.com/v2/orders",
    method: "GET",
    headers: {
      authorization: "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.payload.signature",
      accept: "application/json",
      "x-request-id": "req-8f31-0a2b",
    },
    query: { page: 2, limit: 100, sort: "-created_at" },
    timeout: 30,
  },
  output: {
    items: [
      { id: 1001, total: 59.99, currency: "USD", customer: { id: 42, name: "Ada Lovelace", email: "ada@example.com" } },
      { id: 1002, total: 12.5, currency: "USD", customer: { id: 43, name: "Bob Ross", email: "bob@example.com" } },
      { id: 1003, total: 240, currency: "EUR", customer: { id: 44, name: "Carol Danvers", email: "carol@example.com" } },
    ],
    page: 2,
    total: 1284,
    ok: true,
    cached: false,
  },
};

const body = renderToString(React.createElement(JsonViewerModal, { entry, onClose: () => {} }));

/* the single-file dist build carries the full compiled stylesheet inline
   (resolved from the frontend working directory). The JS bundle contains a
   highlight.js grammar regex that looks like a <style> tag, so grab the LAST
   real style element by position and validate it carries compiled app CSS. */
const dist = readFileSync(`${process.cwd()}/dist/index.html`, "utf8");
const open = dist.lastIndexOf("<style");
const close = dist.lastIndexOf("</style>");
const styles = open >= 0 && close > open ? dist.slice(dist.indexOf(">", open) + 1, close) : "";
if (!styles.includes(".bg-ink-900")) throw new Error("compiled CSS not found in dist/index.html");

const page = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>JsonViewerModal preview</title>
<style>${styles}</style>
</head>
<body style="background:#08080a">
${body}
</body>
</html>`;

/* primary output lives outside the repo to keep the working tree clean;
   falls back to the frontend dir when that path isn't writable */
const out = (() => {
  try {
    writeFileSync("/home/z/my-project/scripts/jsonviewer-preview.html", page);
    return "/home/z/my-project/scripts/jsonviewer-preview.html";
  } catch {
    return `${process.cwd()}/jsonviewer-preview.html`;
  }
})();
writeFileSync(out, page);
console.log(`wrote ${out}`);