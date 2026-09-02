/* Verification for the SI byte-size formatter used by the models catalog.
   The managed-llama Models tab and model file dropdowns showed ICU's compact
   byte rendering ("53.8B byte") instead of human sizes. formatBytes now
   delegates to the pure formatByteSize helper (decimal SI steps, one
   fraction digit, locale-aware digits), so a 53.8 GB GGUF reads "53.8 GB"
   (and "53,8 GB" in German).
   Part 1 pins the wiring: format.ts delegates with the app locale and no
   longer uses ICU's unit/compact byte style; the models catalog, installed
   model rows, and the JSON viewer still render through formatBytes.
   Part 2 exercises the real formatter in node: exact renderings across
   units, boundaries, locales, and degenerate inputs.
   Run: npx tsx scripts/verify-format-bytes.mts */

import { readFileSync } from "node:fs";

import { formatByteSize } from "../src/lib/format-bytes";

let failed = 0;
let passed = 0;
function check(name: string, ok: boolean, detail?: string) {
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${ok || !detail ? "" : ` — ${detail}`}`);
  if (ok) passed++;
  else failed++;
}

const format = readFileSync(new URL("../src/lib/format.ts", import.meta.url), "utf8");
const settings = readFileSync(new URL("../src/views/SettingsView.tsx", import.meta.url), "utf8");
const jsonViewer = readFileSync(new URL("../src/components/JsonViewerModal.tsx", import.meta.url), "utf8");

check("format.ts delegates to formatByteSize with the app locale",
  /import \{ formatByteSize \} from "@\/lib\/format-bytes";/.test(format) &&
  /return formatByteSize\(bytes, i18n\.resolvedLanguage \?\? "en"\);/.test(format));
check("the ICU unit/compact byte style is gone", !/unit: "byte"/.test(format));
check("model file dropdown sizes render through formatBytes",
  /formatBytes\(f\.size\)/.test(settings));
check("installed model rows render through formatBytes",
  /formatBytes\(model\.size\)\} · \{formatDateTime/.test(settings));
check("model detail size badges render through formatBytes",
  /px-2 py-1">\{formatBytes\(model\.size\)\}<\/span>/.test(settings));
check("model table rows size-guard through formatBytes",
  /model\.size > 0 \? formatBytes\(model\.size\) : ""/.test(settings));
check("JSON viewer stats render through formatBytes",
  /formatBytes\(stats\.bytes\)/.test(jsonViewer));

const cases: Array<[bytes: number, locale: string, want: string, name: string]> = [
  [53_800_000_000, "en", "53.8 GB", "the reported 53.8B byte case reads as GB"],
  [890, "en", "890 B", "bytes under a kilobyte stay plain"],
  [0, "en", "0 B", "zero renders as 0 B"],
  [-1024, "en", "0 B", "negative counts render as 0 B"],
  [Number.NaN, "en", "0 B", "NaN renders as 0 B"],
  [1024, "en", "1 KB", "a kibibyte of bytes is 1 KB in SI steps"],
  [1_048_576, "en", "1 MB", "a mebibyte reads as 1 MB"],
  [1_234_567, "en", "1.2 MB", "one fraction digit"],
  [2_500_000_000, "en", "2.5 GB", "multi-GB model files"],
  [5_000_000_000_000, "en", "5 TB", "terabyte tier"],
  [999_999_999, "en", "1 GB", "boundary rounds up into the next unit"],
  [53_800_000_000, "de", "53,8 GB", "German decimal comma"],
  [53_800_000_000, "ru", "53,8 GB", "Russian decimal comma"],
  [730_000_000, "fr", "730 MB", "French grouping"],
];
for (const [bytes, locale, want, name] of cases) {
  const got = formatByteSize(bytes, locale);
  check(name, got === want, `formatByteSize(${bytes}, ${locale}) = ${got}, want ${want}`);
}

console.log(`\n${passed} passed, ${failed} failed`);
if (failed > 0) process.exit(1);
