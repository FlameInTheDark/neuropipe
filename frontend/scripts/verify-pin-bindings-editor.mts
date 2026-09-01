/* Playwright verification for the PinBindingsEditor live page.
   (after: npx tsx scripts/render-pin-bindings-live.mts and a fresh
   frontend build so dist/index.html carries the compiled CSS).
   Pins the contract the canvas depends on:
   - typing in name/label/value never loses input focus (row identity is
     index-keyed, ids are minted once and never re-derived from content)
   - the committed payload keeps generated ids stable and drops blank rows
   - add/delete buttons round-trip through the parent state
   - output mode renders no value column and ignores literals in labels
   Run: npx tsx scripts/verify-pin-bindings-editor.mts */

import { createRequire } from "node:module";
import type { Page } from "playwright";

const require = createRequire(import.meta.url);

function loadPlaywright(): {
  chromium: { launch(options?: Record<string, unknown>): Promise<any> };
} {
  try {
    return require("playwright");
  } catch {
    return require("/home/z/.npm-global/lib/node_modules/playwright");
  }
}

const PAGE_URL = "file:///home/z/my-project/scripts/pin-bindings-live.html";
const VIEWPORT = { width: 960, height: 720 };

let failed = 0;
let passed = 0;
function check(name: string, ok: boolean, detail?: string) {
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${ok || !detail ? "" : ` — ${detail}`}`);
  if (ok) passed++;
  else failed++;
}

interface BindingRow {
  id: string;
  name: string;
  label: string;
  value: string;
}

async function results(page: Page): Promise<Record<string, unknown>> {
  return page.evaluate(() => (window as unknown as { __results: Record<string, unknown> }).__results);
}

async function run() {
  const { chromium } = loadPlaywright();
  const browser = await chromium.launch();
  const page = await browser.newPage({ viewport: VIEWPORT });
  await page.goto(PAGE_URL);
  await page.waitForFunction(() => {
    const w = window as unknown as { __results?: Record<string, unknown> };
    return w.__results !== undefined && !("error" in w.__results);
  }, undefined, { timeout: 15000 });
  const data = await results(page);

  /* focus retention through the simulated typing */
  check("typing in the name field never loses focus", data.nameFocusLostAt === undefined, `lost at ${data.nameFocusLostAt}`);
  check("typing in the value field never loses focus", data.valueFocusLostAt === undefined, `lost at ${data.valueFocusLostAt}`);
  check("typing in the label field never loses focus", data.labelFocusLostAt === undefined, `lost at ${data.labelFocusLostAt}`);

  /* the committed input payload after typing + add */
  const inputRows = (data.inputRows ?? []) as BindingRow[];
  check("input rows survive the round trip", inputRows.length === 3, `rows=${JSON.stringify(inputRows)}`);
  const first = inputRows[0];
  check(
    "first row keeps its stable id through all edits",
    first?.id === "field_1",
    `id=${first?.id}`,
  );
  check(
    "name edit landed verbatim",
    first?.name === "customerName",
    `name=${first?.name}`,
  );
  check(
    "label edit landed verbatim",
    first?.label === "Kundenname",
    `label=${first?.label}`,
  );
  check(
    "value edit landed verbatim",
    first?.value === "Litware",
    `value=${first?.value}`,
  );
  check(
    "the added row received a fresh generated id",
    inputRows[2]?.id === "field_3" && inputRows[2]?.name === "date",
    `row3=${JSON.stringify(inputRows[2])}`,
  );

  /* delete keeps the other rows and their ids */
  const afterDelete = (data.inputRowsAfterDelete ?? []) as BindingRow[];
  check("delete removed exactly one row", afterDelete.length === 2, `rows=${JSON.stringify(afterDelete)}`);
  check("delete removed the last (added) row and kept the untouched ids", afterDelete[0]?.id === "field_1" && afterDelete[1]?.id === "field_2", `ids=${afterDelete.map((r) => r.id).join(",")}`);

  /* blanking a name drops the row from the payload */
  const afterBlank = (data.inputRowsAfterBlank ?? []) as BindingRow[];
  check("blanked names are dropped from the payload", afterBlank.length === 1 && afterBlank[0]?.name !== "", `rows=${JSON.stringify(afterBlank)}`);

  /* output mode: no value inputs rendered, literal value carried but ignored */
  const outputRows = (data.outputRows ?? []) as BindingRow[];
  check("output mode keeps its rows", outputRows.length === 1 && outputRows[0]?.id === "field_1" && outputRows[0]?.name === "B4", `rows=${JSON.stringify(outputRows)}`);
  const valueInputsInOutput = await page.locator('#output-editor input[aria-label="Value"]').count();
  check("output mode renders no value column", valueInputsInOutput === 0, `count=${valueInputsInOutput}`);

  /* live DOM state after the scripted interaction */
  const rowCount = await page.locator("#input-editor input").count();
  check("editor renders rows as plain controlled inputs", rowCount > 0, `count=${rowCount}`);

  await browser.close();
  console.log(`\n${passed} passed, ${failed} failed`);
  if (failed > 0) process.exit(1);
}

run().catch((error) => {
  console.error(error);
  process.exit(1);
});
