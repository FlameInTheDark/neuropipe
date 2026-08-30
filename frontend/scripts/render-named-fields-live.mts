/* Builds the NamedFieldsEditor live page (real component + compiled CSS).
   Output: /home/z/my-project/scripts/named-fields-live.html */
import { build } from "esbuild";
import { writeFileSync, readFileSync } from "node:fs";
import path from "node:path";

const root = path.resolve(import.meta.dirname, "..");

const result = await build({
  entryPoints: [path.resolve(import.meta.dirname, "named-fields-live-entry.tsx")],
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

const dist = readFileSync(`${root}/dist/index.html`, "utf8");
const open = dist.lastIndexOf("<style");
const close = dist.lastIndexOf("</style>");
const styles = open >= 0 && close > open ? dist.slice(dist.indexOf(">", open) + 1, close) : "";

const js = bundle.replace(/<\/script>/gi, "<\\/script>");
const page = `<!doctype html>
<html lang="en" data-theme="dark">
<head><meta charset="utf-8"><title>Named fields focus test</title><style>${styles}</style></head>
<body><div id="root"></div><script>${js}</script></body>
</html>`;
writeFileSync("/home/z/my-project/scripts/named-fields-live.html", page);
console.log("wrote /home/z/my-project/scripts/named-fields-live.html");
