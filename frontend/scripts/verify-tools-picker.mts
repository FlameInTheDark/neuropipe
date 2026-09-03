/* Headless behavior check for the chat ToolsPicker (search + toggles).
   Opens the live page built by render-tools-picker-live.mts in Chromium and
   asserts:

   - the compact trigger renders the enabled count badge
   - the portal menu opens inside the viewport with one row per published
     tool plus one "unavailable" row for the enabled-but-deleted ID
   - typing filters rows by name and description
   - clicking a row (or Enter on the highlighted row) toggles it and the
     count badge follows
   - Escape closes the menu, second Escape after clearing a query
   - the footer hint is present
   - case=bottom: the anchor at the viewport bottom flips the menu upward

   Run: npx tsx scripts/verify-tools-picker.mts
   (after:  npx tsx scripts/render-tools-picker-live.mts  and a fresh
    `npx vite build` so dist/index.html carries the compiled CSS) */

import { createRequire } from "node:module";

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

const PAGE_URL = "file:///home/z/my-project/scripts/tools-picker-live.html";
const VIEWPORT = { width: 960, height: 640 };

let failed = 0;
let passed = 0;
function check(name: string, ok: boolean, detail?: string) {
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${ok || !detail ? "" : ` — ${detail}`}`);
  if (ok) passed++;
  else failed++;
}

async function enabledState(page: any): Promise<string[]> {
  return page.evaluate(() => (window as unknown as Record<string, unknown>).__enabled as string[]);
}

async function measureMenu(page: any): Promise<{ top: number; bottom: number } | null> {
  return page.evaluate(() => {
    const el = document.querySelector('[role="listbox"]');
    if (!el) return null;
    const r = el.getBoundingClientRect();
    return { top: r.top, bottom: r.bottom };
  });
}

async function run() {
  const { chromium } = loadPlaywright();
  const browser = await chromium.launch();
  const page = await browser.newPage({ viewport: VIEWPORT });
  await page.goto(`${PAGE_URL}?case=default`);
  await page.waitForFunction(() => (window as unknown as Record<string, unknown>).__ready === true);

  /* trigger: label + count badge for the two pre-enabled IDs */
  check(
    "trigger shows the enabled count",
    (await page.locator('[aria-haspopup="listbox"]').textContent())?.includes("2") === true,
    "badge missing",
  );

  /* open and inspect rows: 5 published tools + 1 unavailable */
  await page.locator('[aria-haspopup="listbox"]').click();
  await page.waitForTimeout(350);
  check("menu opened with options", (await page.locator('[role="option"]').count()) === 5);
  check(
    "unavailable row rendered for the deleted function",
    (await page.locator('[role="listbox"] [data-idx]').count()) === 6,
    "missing row not rendered",
  );
  const rect = await measureMenu(page);
  check("menu is inside the viewport", rect !== null && rect.top >= 0 && rect.bottom <= VIEWPORT.height);

  /* search filters by name and by description */
  await page.locator('[role="listbox"] input').fill("forecast");
  await page.waitForTimeout(200);
  check("search by description filters to one row", (await page.locator('[role="option"]').count()) === 1);
  await page.locator('[role="listbox"] input').fill("sql");
  await page.waitForTimeout(200);
  check("search by name filters to one row", (await page.locator('[role="option"]').count()) === 1);

  /* Escape with a query clears the query first, second Escape closes */
  await page.locator('[role="listbox"] input').press("Escape");
  await page.waitForTimeout(200);
  check("first Escape cleared the query", (await page.locator('[role="option"]').count()) === 5);
  await page.locator('[role="listbox"]').press("Escape");
  await page.waitForTimeout(200);
  check("second Escape closed the menu", (await page.locator('[role="listbox"]').count()) === 0);

  /* click-toggle: enabling Calendar events grows the badge to 3 */
  await page.locator('[aria-haspopup="listbox"]').click();
  await page.waitForTimeout(300);
  await page.locator('[role="option"]').nth(1).click();
  await page.waitForTimeout(200);
  const enabled = await enabledState(page);
  check(
    "row click toggles the function on",
    enabled.includes("fn-calendar") && enabled.length === 3,
    `enabled = ${JSON.stringify(enabled)}`,
  );
  check(
    "trigger badge follows the toggle",
    (await page.locator('[aria-haspopup="listbox"]').textContent())?.includes("3") === true,
  );

  /* keyboard: ArrowDown moves the highlight onto the SQL row; Enter
     toggles it on, a second Enter toggles it back off */
  await page.locator('[role="listbox"]').press("ArrowDown");
  await page.locator('[role="listbox"]').press("Enter");
  await page.waitForTimeout(200);
  let keyboardEnabled = await enabledState(page);
  check(
    "ArrowDown + Enter toggles the highlighted row on",
    keyboardEnabled.includes("fn-sql") && keyboardEnabled.length === 4,
    `enabled = ${JSON.stringify(keyboardEnabled)}`,
  );
  await page.locator('[role="listbox"]').press("Enter");
  await page.waitForTimeout(200);
  keyboardEnabled = await enabledState(page);
  check(
    "Enter toggles the highlighted row off",
    !keyboardEnabled.includes("fn-sql") && keyboardEnabled.length === 3,
    `enabled = ${JSON.stringify(keyboardEnabled)}`,
  );

  /* the unavailable row cannot be toggled */
  await page.locator('[role="listbox"]').press("End");
  await page.locator('[role="listbox"]').press("Enter");
  await page.waitForTimeout(150);
  check(
    "unavailable row ignores Enter",
    (await enabledState(page)).includes("fn-gone"),
    "missing ID was dropped or changed",
  );

  /* footer hint present */
  check(
    "footer hint rendered",
    (await page.locator('[role="listbox"] p').last().textContent())?.length !== undefined,
  );

  /* outside click closes */
  await page.mouse.click(500, 300);
  await page.waitForTimeout(200);
  check("outside click closed the menu", (await page.locator('[role="listbox"]').count()) === 0);

  /* case=bottom: composer at the bottom edge, menu flips upward */
  await page.goto(`${PAGE_URL}?case=bottom`);
  await page.waitForFunction(() => (window as unknown as Record<string, unknown>).__ready === true);
  await page.locator('[aria-haspopup="listbox"]').click();
  await page.waitForTimeout(350);
  const bottomRect = await measureMenu(page);
  const anchorTop = await page.evaluate(() => document.querySelector('[aria-haspopup="listbox"]')!.getBoundingClientRect().top);
  check(
    "bottom: menu flipped above the anchor",
    bottomRect !== null && bottomRect.top < anchorTop,
    `menu top ${bottomRect?.top.toFixed(0)} >= anchor top ${anchorTop.toFixed(0)}`,
  );

  await browser.close();
  console.log(`\n${passed} passed, ${failed} failed`);
  if (failed > 0) process.exit(1);
}

run().catch((err) => {
  console.error(err);
  process.exit(1);
});
