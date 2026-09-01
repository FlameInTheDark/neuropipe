/* Headless positioning check for the searchable model-selection dropdown.
   Opens the live page built by render-model-dropdown-live.mts in Chromium at
   a 960x640 viewport (editor-like: the Dropdown lives in a right-hand
   inspector panel) and asserts the opened menu — [role="listbox"], a
   position:fixed portal element — stays fully inside the viewport:

   - case=wide   : long model keys make the menu much wider than its anchor;
                   the menu must clamp to the right screen edge.
   - case=async  : the model list arrives 500ms AFTER the menu opens; the menu
                   must re-clamp when the list grows.
   - case=bottom : the anchor sits near the viewport bottom; the menu must
                   flip upward and stay inside vertically too.

   Also types a search query on the wide case and re-checks the bounds.

   Run: npx tsx scripts/verify-model-dropdown-position.mts
   (after:  npx tsx scripts/render-model-dropdown-live.mts  and a fresh
    `npx vite build` so dist/index.html carries the compiled CSS) */

import { createRequire } from "node:module";

const require = createRequire(import.meta.url);

/* playwright is not a frontend dependency; prefer the workspace copy if one
   ever appears, else fall back to the globally installed one. */
function loadPlaywright(): {
  chromium: { launch(options?: Record<string, unknown>): Promise<any> };
} {
  try {
    return require("playwright");
  } catch {
    return require("/home/z/.npm-global/lib/node_modules/playwright");
  }
}

const PAGE_URL = "file:///home/z/my-project/scripts/model-dropdown-live.html";
const VIEWPORT = { width: 960, height: 640 };

let failed = 0;
let passed = 0;
function check(name: string, ok: boolean, detail?: string) {
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${ok || !detail ? "" : ` — ${detail}`}`);
  if (ok) passed++;
  else failed++;
}

type Rect = { left: number; top: number; right: number; bottom: number; width: number; height: number };

async function measureMenu(page: any): Promise<Rect | null> {
  return page.evaluate(() => {
    const el = document.querySelector('[role="listbox"]');
    if (!el) return null;
    const r = el.getBoundingClientRect();
    return {
      left: r.left,
      top: r.top,
      right: r.right,
      bottom: r.bottom,
      width: r.width,
      height: r.height,
    };
  });
}

function assertInsideViewport(name: string, rect: Rect, label: string) {
  check(
    `${label}: menu left edge on screen`,
    rect.left >= 0,
    `left=${rect.left.toFixed(1)}`,
  );
  check(
    `${label}: menu right edge on screen (reported bug)`,
    rect.right <= VIEWPORT.width,
    `right=${rect.right.toFixed(1)} > viewport ${VIEWPORT.width}`,
  );
  check(`${label}: menu top edge on screen`, rect.top >= 0, `top=${rect.top.toFixed(1)}`);
  check(
    `${label}: menu bottom edge on screen`,
    rect.bottom <= VIEWPORT.height,
    `bottom=${rect.bottom.toFixed(1)} > viewport ${VIEWPORT.height}`,
  );
  console.log(
    `      ${name}: rect ${rect.left.toFixed(0)},${rect.top.toFixed(0)} ${rect.width.toFixed(0)}x${rect.height.toFixed(0)} (right ${rect.right.toFixed(0)}, bottom ${rect.bottom.toFixed(0)})`,
  );
}

async function openFirstDropdown(page: any) {
  await page.locator('[aria-haspopup="listbox"]').first().click();
}

async function run() {
  const { chromium } = loadPlaywright();
  const browser = await chromium.launch();
  const page = await browser.newPage({ viewport: VIEWPORT });

  /* ---- case=wide: menu wider than the anchor from the start ---- */
  await page.goto(`${PAGE_URL}?case=wide`);
  await page.waitForFunction(() => (window as unknown as Record<string, unknown>).__ready === true);
  await openFirstDropdown(page);
  /* let the menu-in entry animation finish (0.13s) before measuring */
  await page.waitForTimeout(350);
  let rect = await measureMenu(page);
  check("wide: menu is open", rect !== null, "no [role=listbox] found");
  if (rect) assertInsideViewport("wide", rect, "wide: opened");

  /* typing narrows the list; the menu must stay in bounds after re-layout */
  await page.locator('[role="listbox"] input').fill("llama");
  await page.waitForTimeout(250);
  rect = await measureMenu(page);
  if (rect) assertInsideViewport("wide-filtered", rect, "wide: after typing a query");
  check(
    "wide: query filtered the list",
    await page.locator('[role="option"]').count() > 0,
  );

  /* ---- case=async: options arrive 500ms after the menu opened ---- */
  await page.goto(`${PAGE_URL}?case=async`);
  await page.waitForFunction(() => (window as unknown as Record<string, unknown>).__ready === true);
  await openFirstDropdown(page);
  /* wait past the 500ms options arrival + animation */
  await page.waitForTimeout(900);
  rect = await measureMenu(page);
  check("async: menu is open", rect !== null, "no [role=listbox] found");
  if (rect) assertInsideViewport("async", rect, "async: after options arrived");
  check(
    "async: full model list rendered",
    (await page.locator('[role="option"]').count()) >= 40,
  );

  /* ---- case=bottom: anchor at the bottom, menu must flip upward ---- */
  await page.goto(`${PAGE_URL}?case=bottom`);
  await page.waitForFunction(() => (window as unknown as Record<string, unknown>).__ready === true);
  /* the second Model picker is pushed to the panel bottom */
  await page.locator('[aria-haspopup="listbox"]').nth(1).click();
  await page.waitForTimeout(350);
  rect = await measureMenu(page);
  check("bottom: menu is open", rect !== null, "no [role=listbox] found");
  if (rect) {
    assertInsideViewport("bottom", rect, "bottom: flipped open");
    const anchor = await page.evaluate(() => {
      const btns = document.querySelectorAll('[aria-haspopup="listbox"]');
      return btns[1].getBoundingClientRect().top;
    });
    check(
      "bottom: menu flipped above the anchor",
      rect.top < anchor,
      `menu top ${rect.top.toFixed(0)} >= anchor top ${anchor.toFixed(0)}`,
    );
  }

  await browser.close();

  console.log(`\n${passed} passed, ${failed} failed`);
  process.exit(failed > 0 ? 1 : 0);
}

run().catch((err) => {
  console.error(err);
  process.exit(1);
});
