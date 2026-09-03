/* Builds the chat ToolsPicker live test page: bundles
   scripts/tools-picker-live-entry.tsx to an IIFE with esbuild (real
   react-dom/client + the real ToolsPicker), then inlines the JS and the
   app's compiled CSS (from the single-file dist build) into one standalone
   HTML file. Driven headlessly by verify-tools-picker.mts.
   Output: /home/z/my-project/scripts/tools-picker-live.html */

import { build } from "esbuild";
import { writeFileSync, readFileSync } from "node:fs";
import path from "node:path";

const root = path.resolve(import.meta.dirname, "..");

const result = await build({
  entryPoints: [path.resolve(import.meta.dirname, "tools-picker-live-entry.tsx")],
  bundle: true,
  platform: "browser",
  format: "iife",
  write: false,
  loader: { ".tsx": "tsx", ".ts": "ts" },
  jsx: "automatic",
  alias: { "@": path.resolve(root, "src") },
  define: { "process.env.NODE_ENV": '"production"' },
  logLevel: "silent",
});

const bundle = result.outputFiles?.[0]?.text;
if (!bundle) throw new Error("esbuild produced no output");

/* the single-file dist build carries the full compiled stylesheet inline */
const dist = readFileSync(`${root}/dist/index.html`, "utf8");
const open = dist.lastIndexOf("<style");
const close = dist.lastIndexOf("</style>");
const styles = open >= 0 && close > open ? dist.slice(dist.indexOf(">", open) + 1, close) : "";
if (!styles.includes(".bg-ink-900")) throw new Error("compiled CSS not found in dist/index.html");

const js = bundle.replace(/<\/script>/gi, "<\\/script>");

const page = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Chat tools picker live test</title>
<style>${styles}</style>
</head>
<body style="background:#08080a;margin:0;height:100vh;position:relative">
<div id="root"></div>
<script>${js}</script>
</body>
</html>`;

writeFileSync("/home/z/my-project/scripts/tools-picker-live.html", page);
console.log("wrote /home/z/my-project/scripts/tools-picker-live.html");
