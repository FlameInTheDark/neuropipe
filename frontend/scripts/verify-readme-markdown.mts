/* Verification for the managed-llama models detail pane layout + markdown
   README. The catalog detail used to dump the Hugging Face model card into a
   mono <pre> and scrolled the whole pane body (with a nested 420px readme
   scroller) — two nested scrollbars on one column.
   The pane is now a flex stack: the model header (name, stats, HF link) and
   the install-package card are pinned at the top, and the README card
   stretches to fill all remaining height with the single internal scroll.
   The README renders through the shared MarkdownRenderer (react-markdown +
   GFM + highlight.js) — the same engine as the Reports page and text-editor
   preview.
   Part 1 pins the wiring: header and install sections are shrink-0 (never
   scroll away), the old pane-body scroller is gone (no double scroll), the
   readme area is the flex-1 scroller that fills the remaining height, the
   card stretches with it, images are contained, and the old plain-text
   <pre> dump is gone.
   Part 2 renders a realistic HF-style model card (headings, bold, GFM
   table, list, link, blockquote, python fence) through the real component
   to string and asserts markdown actually renders — not raw source text.
   Run: npx tsx scripts/verify-readme-markdown.mts */

import { readFileSync } from "node:fs";

import "./dom-stub.mts";

import React from "react";
import { renderToString } from "react-dom/server";

import { MarkdownRenderer } from "../src/components/MarkdownRenderer";

let failed = 0;
let passed = 0;
function check(name: string, ok: boolean, detail?: string) {
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${ok || !detail ? "" : ` — ${detail}`}`);
  if (ok) passed++;
  else failed++;
}

/* ------------------------------------------------------------------ */
/* Part 1: wiring pinned in source                                     */
/* ------------------------------------------------------------------ */

const settings = readFileSync(new URL("../src/views/SettingsView.tsx", import.meta.url), "utf8");

check("SettingsView imports the shared MarkdownRenderer",
  /import \{ MarkdownRenderer \} from "\.\.\/components\/MarkdownRenderer";/.test(settings));
check("model README renders through MarkdownRenderer",
  /<MarkdownRenderer text=\{detail\.readme\} className="max-w-none" \/>/.test(settings));
check("the old plain-text mono <pre> readme dump is gone",
  !/font-mono text-\[11px\] leading-relaxed whitespace-pre-wrap text-fg-subtle/.test(settings));
check("model header is pinned and never scrolls away",
  /shrink-0 border-b border-seam p-4/.test(settings));
check("install package section is pinned below the model header",
  /<div className="shrink-0 p-4">/.test(settings));
check("the old pane-body scroller is gone (no double scroll)",
  !/min-h-0 flex-1 overflow-y-auto p-4/.test(settings) &&
  !/max-h-\[420px\]/.test(settings));
check("readme area stretches to fill the remaining height",
  /flex min-h-0 flex-1 flex-col p-4 pt-0/.test(settings) &&
  /min-h-0 flex-1 overflow-y-auto pr-1/.test(settings));
check("the readme card itself stretches with the area",
  /flex min-h-0 flex-1 flex-col rounded-xl border border-ink-700\/80 bg-ink-850\/40 p-3\.5/.test(settings));
check("model-card images are contained, not layout-breaking",
  /\[&_img\]:h-auto \[&_img\]:max-w-full \[&_img\]:rounded-lg/.test(settings));

/* ------------------------------------------------------------------ */
/* Part 2: real render of an HF-style model card                       */
/* ------------------------------------------------------------------ */

const sampleReadme = [
  "# Meta Llama 3.3 70B Instruct — GGUF",
  "",
  "Quantized GGUF files of `meta-llama/Llama-3.3-70B-Instruct` for use with **llama.cpp**.",
  "",
  "## Download",
  "",
  "| File | Quant | Size |",
  "|------|-------|------|",
  "| Llama-3.3-70B-Q4_K_M.gguf | 4-bit | 42.5 GB |",
  "",
  "- Requires `llama.cpp` b4181 or newer",
  "- [Original model card](https://huggingface.co/meta-llama/Llama-3.3-70B-Instruct)",
  "",
  "> Metadata is embedded in every GGUF file.",
  "",
  "```python",
  'from llama_cpp import Llama',
  'llm = Llama(model_path="Llama-3.3-70B-Q4_K_M.gguf")',
  "```",
].join("\n");

const html = renderToString(
  React.createElement(MarkdownRenderer, { text: sampleReadme, className: "max-w-none" }),
);

check("heading renders as a real <h1>, not a # line",
  /<h1[^>]*>/.test(html) && html.includes("Meta Llama 3.3 70B Instruct"));
check("bold renders as <strong>, not literal asterisks",
  /<strong[^>]*>/.test(html) && html.includes("llama.cpp") && !html.includes("**"));
check("GFM table renders as <table>",
  /<table[^>]*>/.test(html) && html.includes("42.5 GB"));
check("links render with target=_blank like the rest of the app",
  /href="https:\/\/huggingface\.co\/meta-llama\/Llama-3\.3-70B-Instruct"/.test(html) &&
  /target="_blank"/.test(html));
check("code fence highlights as python",
  /language-python/.test(html) && /hljs/.test(html));
check("blockquote renders as <blockquote>",
  /<blockquote[^>]*>/.test(html));
check("list renders as <ul> with items",
  /<ul[^>]*>/.test(html) && html.includes("b4181 or newer"));
check("raw markdown source does not leak through",
  !html.includes("## ") && !html.includes("**") && !html.includes("|------"));
check("max-w-none overrides the 740px article measure via tailwind-merge",
  html.includes("max-w-none") && !html.includes("max-w-[740px]"));

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
