/* Drives the ChatView rename scenario: the model's rename_conversation tool
   fires mid-turn and the backend pushes chat.conversation.updated. Asserts
   the header and the conversation list retitle LIVE (no page reopen), before
   the turn even finishes. */

import { createRequire } from "node:module";

const require = createRequire(import.meta.url);

function loadPlaywright(): { chromium: { launch(options?: Record<string, unknown>): Promise<any> } } {
  try {
    return require("playwright");
  } catch {
    return require("/home/z/.npm-global/lib/node_modules/playwright");
  }
}

const PAGE_URL = "file:///home/z/my-project/scripts/chat-transcript-live.html";
const VIEWPORT = { width: 1100, height: 760 };

let failures = 0;

function check(label: string, actual: unknown, expected: unknown): void {
  const ok = actual === expected;
  if (!ok) failures += 1;
  console.log(`${ok ? "PASS" : "FAIL"}  ${label}${ok ? "" : ` — actual: ${JSON.stringify(actual)}, expected: ${JSON.stringify(expected)}`}`);
}

async function headerTitle(page: any): Promise<string | null> {
  return page.evaluate(() => {
    const headings = [...document.querySelectorAll("h2")];
    const header = headings.find((el) => el.className.includes("truncate"));
    return header ? header.textContent : null;
  });
}

async function listTitles(page: any): Promise<string[]> {
  return page.evaluate(() =>
    [...document.querySelectorAll('span[class*="text-[12.5px]"]')].map((el) => (el as HTMLElement).textContent ?? "").filter(Boolean),
  );
}

async function run(): Promise<void> {
  const { chromium } = loadPlaywright();
  const browser = await chromium.launch();
  const page = await browser.newPage({ viewport: VIEWPORT });
  await page.goto(`${PAGE_URL}?case=rename`);
  await page.waitForTimeout(900);

  check("header shows the pre-rename title", await headerTitle(page), "New chat");
  const beforeList = await listTitles(page);
  check("list shows the pre-rename title", beforeList.some((t) => t === "New chat"), true);

  await page.evaluate(() => {
    const harness = (window as unknown as Record<string, unknown>).__chatHarness as { sendMessage(t: string): Promise<void> };
    return harness.sendMessage("What is the weather in Munich?");
  });

  // Mid-turn: the rename tool has already run (~250ms) and the reply is
  // still streaming — the UI must already show the new title.
  await page.waitForTimeout(330);
  check("header retitled live, mid-turn (no reopen)", await headerTitle(page), "Weather in Munich");
  const midList = await listTitles(page);
  check("list retitled live, mid-turn", midList.some((t) => t === "Weather in Munich"), true);
  await page.screenshot({ path: "/home/z/my-project/scripts/chat-rename-midturn.png", fullPage: false });

  // After completion everything must still be consistent.
  await page.waitForTimeout(1600);
  check("header keeps the new title after completion", await headerTitle(page), "Weather in Munich");
  const doneList = await listTitles(page);
  check("list keeps the new title after completion", doneList.some((t) => t === "Weather in Munich"), true);

  const transcript = await page.evaluate(() => {
    const container = document.querySelector(".max-w-\\[640px\\]");
    return container ? container.textContent?.includes("18 degrees") : false;
  });
  check("transcript contains the final streamed reply", transcript, true);
  await page.screenshot({ path: "/home/z/my-project/scripts/chat-rename-after.png", fullPage: false });

  await browser.close();
  console.log(failures === 0 ? `\nAll rename-harness checks passed.` : `\n${failures} check(s) FAILED`);
  if (failures > 0) process.exit(1);
}

run().catch((error) => {
  console.error(error);
  process.exit(1);
});
