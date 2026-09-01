/* Verification for the searchable model-selection dropdowns.
   Part 1 unit-tests the pure filter (src/lib/dropdownFilter.ts) — label, hint,
   and VALUE matching (model keys), case-insensitivity, trim, no-mutation.
   Part 2 pins the wiring: the AI-node inspector llm-model picker and the
   settings ProviderPanel default-model picker render the Dropdown searchable
   with the shared "Search models…" placeholder, the menu grows taller and
   keeps keyboard navigation usable on huge lists (Home/End, scroll-into-view,
   live match counter), and the pre-existing searchable usage (Discord
   ApplicationCommands) is not regressed.
   Part 3 checks the i18n key ships in all four locales.
   Run: npx tsx scripts/verify-model-search.mts */

import { readFileSync } from "node:fs";
import { filterDropdownOptions, type DropdownOption } from "../src/lib/dropdownFilter";

let failed = 0;
let passed = 0;
function check(name: string, ok: boolean, detail?: string) {
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${ok || !detail ? "" : ` — ${detail}`}`);
  if (ok) passed++;
  else failed++;
}

/* ------------------------------------------------------------------ */
/* Part 1: filterDropdownOptions behavior                              */
/* ------------------------------------------------------------------ */

const models: DropdownOption[] = [
  { value: "", label: "Provider default · claude-sonnet-4-5", hint: "" },
  { value: "claude-sonnet-4-5-20250929", label: "Claude Sonnet 4.5" },
  { value: "claude-opus-4-1-20250805", label: "Claude Opus 4.1" },
  { value: "claude-haiku-4-5", label: "Claude Haiku 4.5" },
  { value: "gpt-4o", label: "gpt-4o" },
  { value: "gpt-4o-mini", label: "gpt-4o-mini" },
  { value: "chatgpt-4o-latest", label: "chatgpt-4o-latest" },
  { value: "text-embedding-3-large", label: "text-embedding-3-large" },
  { value: "llama3.3:70b", label: "Llama 3.3 70B", hint: "local" },
];

const values = (opts: DropdownOption[]) => opts.map((o) => o.value);

/* an empty or whitespace-only query keeps the full list (menu open, no filter) */
check("empty query returns all options", filterDropdownOptions(models, "").length === models.length);
check("whitespace query returns all options", filterDropdownOptions(models, "   ").length === models.length);

/* label matching is case-insensitive and substring-based. Note the provider-
   default sentinel carries the resolved model in its label, so a query that
   names that model keeps the sentinel reachable — by design. */
check(
  "label match is case-insensitive; default sentinel stays reachable (sonnet)",
  JSON.stringify(values(filterDropdownOptions(models, "sonnet"))) ===
    JSON.stringify(["", "claude-sonnet-4-5-20250929"]),
);
check(
  "label match catches default label + all three Claude titles",
  filterDropdownOptions(models, "Claude").length === 4,
  `got ${filterDropdownOptions(models, "Claude").map((o) => o.value).join(", ")}`,
);

/* VALUE matching — the point of the feature: find a model by its key while
   the label is a display title, e.g. the date-stamped Anthropic model ids */
check(
  "value match: search by model key fragment (20250929)",
  JSON.stringify(values(filterDropdownOptions(models, "20250929"))) === JSON.stringify(["claude-sonnet-4-5-20250929"]),
);
check(
  "value match: search by model key prefix (claude-) also sees the sentinel label",
  filterDropdownOptions(models, "claude-").length === 4,
);
check(
  "value match: gpt-4o keys collapse label+value hits to one entry each",
  JSON.stringify(values(filterDropdownOptions(models, "gpt-4o"))) ===
    JSON.stringify(["gpt-4o", "gpt-4o-mini", "chatgpt-4o-latest"]),
);

/* hint matching still works (local-tagged Ollama models) */
check(
  "hint match: 'local' finds the Ollama entry",
  JSON.stringify(values(filterDropdownOptions(models, "local"))) === JSON.stringify(["llama3.3:70b"]),
);

/* no matches yield an empty list (menu shows the no-matches note) */
check("no match returns empty list", filterDropdownOptions(models, "gemini").length === 0);

/* the provider-default sentinel drops out when the query names another model */
const withFilter = filterDropdownOptions(models, "opus");
check("default sentinel is filtered out by a foreign query", !withFilter.some((o) => o.value === ""));
check(
  "foreign query still finds the model by label",
  JSON.stringify(values(withFilter)) === JSON.stringify(["claude-opus-4-1-20250805"]),
);

/* filtering must not mutate the caller's option list */
const snapshot = JSON.stringify(models);
filterDropdownOptions(models, "gpt");
check("filter does not mutate the input options", JSON.stringify(models) === snapshot);

/* a big synthetic list stays exact: 100 keys, prefix search narrows to 12 */
const big: DropdownOption[] = Array.from({ length: 100 }, (_, i) => ({
  value: `provider-model-${String(i).padStart(3, "0")}`,
  label: `Model ${i}`,
}));
check("big list unfiltered length is 100", filterDropdownOptions(big, "").length === 100);
check(
  "big list prefix search narrows to model-01* (10 hits)",
  filterDropdownOptions(big, "model-01").length === 10,
);
check(
  "big list numeric-suffix search hits one key (provider-model-042)",
  filterDropdownOptions(big, "042").length === 1,
);

/* ------------------------------------------------------------------ */
/* Part 2: wiring pins (source fidelity)                               */
/* ------------------------------------------------------------------ */

const dropdown = readFileSync(new URL("../src/components/Dropdown.tsx", import.meta.url), "utf8");
const inspector = readFileSync(new URL("../src/components/Inspector.tsx", import.meta.url), "utf8");
const settings = readFileSync(new URL("../src/views/SettingsView.tsx", import.meta.url), "utf8");
const appCommands = readFileSync(new URL("../src/components/discord/ApplicationCommands.tsx", import.meta.url), "utf8");

/* the filter lives in a JSX-free lib module and is re-exported for callers */
check(
  "Dropdown imports the filter from lib/dropdownFilter",
  dropdown.includes('import { filterDropdownOptions, type DropdownOption } from "../lib/dropdownFilter";'),
);
check("Dropdown re-exports filterDropdownOptions", dropdown.includes("export { filterDropdownOptions };"));
check("Dropdown re-exports the DropdownOption type", dropdown.includes("export type { DropdownOption };"));
check(
  "Menu filters through filterDropdownOptions only when searchable",
  dropdown.includes("() => (searchable ? filterDropdownOptions(options, query) : options),"),
);

/* long-list ergonomics */
check(
  "searchable menu is taller (340px vs 264px)",
  dropdown.includes('searchable ? "max-h-[340px]" : "max-h-[264px]"'),
);
check("Home key jumps to the first option", /e\.key === "Home"/.test(dropdown));
check("End key jumps to the last option", /e\.key === "End"/.test(dropdown));
check("options carry data-idx for scroll-into-view", dropdown.includes("data-idx={i}"));
check(
  "highlighted option is scrolled into view",
  dropdown.includes('el?.scrollIntoView({ block: "nearest" });'),
);
check(
  "search row shows a live match counter",
  dropdown.includes("{filtered.length}/{options.length}"),
);
check(
  "counter is announced politely to screen readers",
  dropdown.includes('aria-live="polite"'),
);

/* the AI-node inspector model picker is searchable with the shared placeholder */
check(
  "inspector llm-model picker renders searchable with searchModels placeholder",
  /field\.kind === "llm-model"[\s\S]{0,2500}searchable[\s\S]{0,300}searchPlaceholder=\{t\("common\.searchModels"\)\}/.test(
    inspector,
  ),
);
check(
  "inspector keeps the saved-but-unconfigured model option",
  inspector.includes("modelOptions.push({ value: current,"),
);
check(
  "inspector provider picker still routes provider change to model clear",
  inspector.includes('if (String(node.values.model ?? "") !== "") onChange("model", "");'),
);

/* the settings ProviderPanel default-model picker is searchable too */
check(
  "ProviderPanel default-model picker renders searchable",
  /provider\.defaultModelLabel[\s\S]{0,600}searchable/.test(settings),
);
check(
  "ProviderPanel uses the shared searchModels placeholder",
  /provider\.defaultModelLabel[\s\S]{0,600}searchPlaceholder=\{t\("common\.searchModels"\)\}/.test(settings),
);

/* pre-existing searchable dropdown (Discord servers) is untouched */
check("ApplicationCommands dropdown stays searchable", /\bsearchable\b/.test(appCommands));

/* positioning: the menu must stay inside the viewport even when anchored in
   the editor's right-side inspector with a long model list. Pinned after the
   reported "goes off the right side of the screen" bug; behavior is covered
   headlessly by verify-model-dropdown-position.mts. */
check(
  "menu measured via offsetWidth (immune to the menu-in animation's scale)",
  dropdown.includes("const mw = el.offsetWidth;") && dropdown.includes("const mh = el.offsetHeight;"),
);
check(
  "menu width is hard-capped to the viewport (maxWidth style)",
  dropdown.includes('maxWidth: "calc(100vw - 12px)"'),
);
check(
  "menu width is capped in JS before clamping left",
  dropdown.includes("const width = Math.min(Math.max(mw, a.width), vw - 12);"),
);
check(
  "horizontal clamp uses the scrollbar-free client width",
  dropdown.includes("const vw = document.documentElement.clientWidth;"),
);
check(
  "positioning re-clamps when the option list or filter changes size",
  dropdown.includes("[anchorRef, filtered.length, options.length]"),
);

/* ------------------------------------------------------------------ */
/* Part 3: i18n key in every locale                                    */
/* ------------------------------------------------------------------ */

for (const locale of ["en", "de", "fr", "ru"]) {
  const dict = readFileSync(new URL(`../src/i18n/${locale}.ts`, import.meta.url), "utf8");
  check(`common.searchModels present in ${locale}`, dict.includes('    searchModels: "'));
}

/* ------------------------------------------------------------------ */

console.log(`\n${passed} passed, ${failed} failed`);
process.exit(failed > 0 ? 1 : 0);
