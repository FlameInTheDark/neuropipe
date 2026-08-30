/* Builds the CommandField live page (real component + bridge mock + compiled CSS).
   Output: /home/z/my-project/scripts/command-field-live.html */
import { build } from "esbuild";
import { writeFileSync, readFileSync } from "node:fs";
import path from "node:path";

const root = path.resolve(import.meta.dirname, "..");

const result = await build({
  entryPoints: [path.resolve(import.meta.dirname, "command-field-live-entry.tsx")],
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

const js = bundle.replace(/<\/script>/gi, "<\\/script>");
const page = `<!doctype html>
<html lang="en" data-theme="dark">
<head><meta charset="utf-8"><title>Command field live test</title><style>${styles}</style></head>
<body><div id="root"></div><script>${js}</script></body>
</html>`;
writeFileSync("/home/z/my-project/scripts/command-field-live.html", page);
console.log("wrote /home/z/my-project/scripts/command-field-live.html");
