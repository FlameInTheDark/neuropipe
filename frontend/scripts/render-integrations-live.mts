/* Builds the Integrations-view live page: bundles scripts/integrations-live-entry.tsx
   to an IIFE with esbuild (real react-dom/client + the real IntegrationsView,
   the three integration panels, and DialogHosts), aliasing "@/lib/bridge" to
   the mock so no Wails runtime is needed. The app's compiled CSS comes from
   the single-file dist build.
   Output: /home/z/my-project/scripts/integrations-live.html */

import { build } from "esbuild";
import { writeFileSync, readFileSync } from "node:fs";
import path from "node:path";

const root = path.resolve(import.meta.dirname, "..");

const result = await build({
  entryPoints: [path.resolve(import.meta.dirname, "integrations-live-entry.tsx")],
  bundle: true,
  platform: "browser",
  format: "iife",
  write: false,
  loader: { ".tsx": "tsx", ".ts": "ts" },
  jsx: "automatic",
  alias: {
    "@/lib/bridge": path.resolve(import.meta.dirname, "integrations-bridge-mock.mts"),
    "@": path.resolve(root, "src"),
  },
  define: { "process.env.NODE_ENV": '"production"' },
  logLevel: "silent",
});

const bundle = result.outputFiles?.[0]?.text;
if (!bundle) throw new Error("esbuild produced no output");

const dist = readFileSync(`${root}/dist/index.html`, "utf8");
const open = dist.lastIndexOf("<style");
const close = dist.lastIndexOf("</style>");
const styles = open >= 0 && close > open ? dist.slice(dist.indexOf(">", open) + 1, close) : "";
if (!styles.includes(".bg-ink-900")) throw new Error("compiled CSS not found in dist/index.html");

const js = bundle.replace(/<\/script>/gi, "<\\/script>");

const page = `<!doctype html>
<html lang="en" data-theme="dark">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Integrations view live test</title>
<style>${styles}</style>
</head>
<body>
<div id="root"></div>
<script>${js}</script>
</body>
</html>`;

const out = "/home/z/my-project/scripts/integrations-live.html";
writeFileSync(out, page);
console.log(`wrote ${out} (${(page.length / 1024).toFixed(0)} KB)`);
