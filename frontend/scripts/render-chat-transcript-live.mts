/* Builds the ChatView transcript live test page: bundles
   scripts/chat-transcript-live-entry.tsx to an IIFE with esbuild (real
   react-dom/client + the real ChatView), aliasing "@/lib/bridge" to the chat
   bridge mock, "@wailsio/runtime" to the Events stand-in, and "@/App" to a
   tiny extractPayload stub so the whole app tree is not bundled. The app's
   compiled CSS comes from the single-file dist build.
   Output: /home/z/my-project/scripts/chat-transcript-live.html */

import { build } from "esbuild";
import { writeFileSync, readFileSync } from "node:fs";
import path from "node:path";

const root = path.resolve(import.meta.dirname, "..");

const result = await build({
  entryPoints: [path.resolve(import.meta.dirname, "chat-transcript-live-entry.tsx")],
  bundle: true,
  platform: "browser",
  format: "iife",
  write: false,
  loader: { ".tsx": "tsx", ".ts": "ts", ".mts": "ts" },
  jsx: "automatic",
  alias: {
    "@/lib/bridge": path.resolve(import.meta.dirname, "chat-bridge-mock.mts"),
    "@wailsio/runtime": path.resolve(import.meta.dirname, "chat-wails-runtime-mock.mts"),
    "@/App": path.resolve(import.meta.dirname, "chat-extract-payload-stub.mts"),
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
<title>Chat transcript live test</title>
<style>${styles}</style>
</head>
<body style="background:#08080a;margin:0">
<div id="root"></div>
<script>${js}</script>
</body>
</html>`;

writeFileSync("/home/z/my-project/scripts/chat-transcript-live.html", page);
console.log("wrote /home/z/my-project/scripts/chat-transcript-live.html");
