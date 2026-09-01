/* Headless regression test for the installed-runtime dropdown on the REAL
   RuntimePanel (page built by render-runtime-panel-live.mts).

   The reported bug: the user installed b10205 (settings pin runtimeVersion
   b10205, mode cuda — exactly what InstallLlamaRuntime writes), but the
   "Installed runtime" dropdown showed b10724, the newest GitHub release:
   option values ended in ":auto", the configured value ended in ":cuda", so
   no option matched and the field fell back to the FIRST option — the newest
   release. A second face of the same bug: with no version configured the
   field silently showed the newest release as if it were installed.

   Scenarios (see runtime-panel-bridge-stub.ts):
   - pinned          : the field shows b10205 · CUDA, the menu leads with the
                       installed entry, and b10724 is only an option below it.
   - unconfigured    : the field shows the placeholder, never b10724.
   - offline         : a failed release lookup leaves the installed entry as
                       the only option, banner included.
   - pick-uninstalled: picking b10724 patches the draft to version b10724
                       with mode auto.
   - pick-installed  : picking b10206 (cuda installed) while the draft pins
                       cuda keeps the mode — no silent reset.

   Run: npx tsx scripts/verify-runtime-dropdown.mts
   (after: npx tsx scripts/render-runtime-panel-live.mts and a fresh
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

const PAGE_URL = "file:///home/z/my-project/scripts/runtime-panel-live.html";
const VIEWPORT = { width: 960, height: 720 };

let failed = 0;
let passed = 0;
function check(name: string, ok: boolean, detail?: string) {
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${ok || !detail ? "" : ` — ${detail}`}`);
  if (ok) passed++;
  else failed++;
}

/* The first listbox button on the page is the installed-runtime picker (the
 * panel's only earlier dropdown is the same field; the model picker renders
 * after it). */
async function runtimeFieldText(page: any): Promise<string> {
  return page.locator('[aria-haspopup="listbox"]').first().innerText();
}

async function openRuntimeMenu(page: any) {
  await page.locator('[aria-haspopup="listbox"]').first().click();
  await page.waitForSelector('[role="listbox"]');
  await page.waitForTimeout(200); /* menu entry animation */
}

async function pickOption(page: any, version: string) {
  await openRuntimeMenu(page);
  await page.locator('[role="option"]', { hasText: version }).first().click();
  await page.waitForTimeout(150);
}

async function patchedRuntime(page: any): Promise<Record<string, unknown>> {
  return page.evaluate(() => (window as unknown as Record<string, unknown>).__patched as Record<string, unknown>);
}

async function gotoScenario(page: any, scenario: string) {
  await page.goto(`${PAGE_URL}?case=${scenario}`);
  await page.waitForFunction(() => (window as unknown as Record<string, unknown>).__ready === true);
  /* the panel fires 4 bridge calls on mount; wait until they all settle so
   * assertions never race the async data load */
  await page.waitForFunction(
    () => {
      const w = window as unknown as Record<string, number>;
      return (w.__pending ?? 1) === 0 && (w.__bridgeCalls ?? 0) >= 4;
    },
    undefined,
    { timeout: 5000 },
  );
  await page.waitForTimeout(150); /* React commit */
}

async function run() {
  const { chromium } = loadPlaywright();
  const browser = await chromium.launch();
  const page = await browser.newPage({ viewport: VIEWPORT });

  /* ---- case=pinned: the reported bug ---- */
  await gotoScenario(page, "pinned");
  let text = await runtimeFieldText(page);
  check("pinned: field shows the installed version, not the newest release", text.includes("b10205"), `text="${text}"`);
  check("pinned: field names the installed CUDA build", text.includes("CUDA"), `text="${text}"`);
  check("pinned: field never claims b10724", !text.includes("b10724"), `text="${text}"`);
  const chips = await page.locator("span.font-mono").filter({ hasText: "b10205" }).count();
  check("pinned: installed chip row shows b10205", chips > 0);
  await openRuntimeMenu(page);
  const firstOption = await page.locator('[role="option"]').first().innerText();
  check("pinned: menu leads with the installed entry", firstOption.includes("b10205") && firstOption.includes("CUDA"), `first="${firstOption}"`);
  const optionCount = await page.locator('[role="option"]').count();
  check("pinned: menu merges installed + releases (4 options)", optionCount === 4, `count=${optionCount}`);
  const selectedCount = await page.locator('[role="option"][aria-selected="true"]').count();
  const selectedText = selectedCount > 0 ? await page.locator('[role="option"][aria-selected="true"]').first().innerText() : "";
  check("pinned: the configured version is the selected option", selectedCount === 1 && selectedText.includes("b10205"), `selected="${selectedText}"`);
  await page.keyboard.press("Escape");

  /* ---- case=unconfigured: placeholder instead of the newest release ---- */
  await gotoScenario(page, "unconfigured");
  text = await runtimeFieldText(page);
  check("unconfigured: field shows the placeholder", text.includes("Select a runtime"), `text="${text}"`);
  check("unconfigured: field never claims the newest release", !text.includes("b10724"), `text="${text}"`);
  const unconfiguredChips = await page.locator("span.font-mono").filter({ hasText: "b10205" }).count();
  check("unconfigured: installed runtime is still visible as a chip", unconfiguredChips > 0);

  /* ---- case=offline: only installed entries remain ---- */
  await gotoScenario(page, "offline");
  text = await runtimeFieldText(page);
  check("offline: field keeps showing the installed version", text.includes("b10205"), `text="${text}"`);
  check("offline: offline banner explains the failed lookup", (await page.locator("text=Installed runtimes are still shown below").count()) > 0);
  await openRuntimeMenu(page);
  const offlineCount = await page.locator('[role="option"]').count();
  check("offline: menu offers exactly the installed runtime", offlineCount === 1, `count=${offlineCount}`);
  await page.keyboard.press("Escape");

  /* ---- case=pick-uninstalled: switch to a release that is not installed ---- */
  await gotoScenario(page, "pick-uninstalled");
  await pickOption(page, "b10724");
  let patched = await patchedRuntime(page);
  check("pick-uninstalled: draft switches to b10724", patched.runtimeVersion === "b10724", JSON.stringify(patched));
  check("pick-uninstalled: mode resolves to auto (no installed build to pin)", patched.mode === "auto", JSON.stringify(patched));
  text = await runtimeFieldText(page);
  check("pick-uninstalled: field now shows b10724", text.includes("b10724"), `text="${text}"`);

  /* ---- case=pick-installed: keep the pinned mode ---- */
  await gotoScenario(page, "pick-installed");
  await pickOption(page, "b10206");
  patched = await patchedRuntime(page);
  check("pick-installed: draft switches to b10206", patched.runtimeVersion === "b10206", JSON.stringify(patched));
  check("pick-installed: pinned cuda mode survives the switch", patched.mode === "cuda", JSON.stringify(patched));
  text = await runtimeFieldText(page);
  check("pick-installed: field shows b10206 with its installed builds", text.includes("b10206") && text.includes("CUDA"), `text="${text}"`);

  await browser.close();

  console.log(`\n${passed} passed, ${failed} failed`);
  if (failed > 0) process.exit(1);
}

run().catch((error) => {
  console.error(error);
  process.exit(1);
});
