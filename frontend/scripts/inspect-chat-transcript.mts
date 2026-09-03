/* Drives the ChatView transcript live page and inspects how turns render:
   counts transcript bubbles, captures screenshots for the loaded state and
   the send flow (typing a message). */

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

async function inspectTranscript(page: any, label: string): Promise<void> {
  const info = await page.evaluate(() => {
    const containers = document.querySelectorAll(".max-w-\\[640px\\]");
    const root = containers[0];
    if (!root) return { error: "transcript container not found" };
    // direct children of the transcript column
    const children = [...root.children].map((el) => {
      const html = el as HTMLElement;
      const isUser = html.className.includes("justify-end");
      const bubble = html.querySelector(".rounded-2xl");
      const text = (bubble?.textContent ?? html.textContent ?? "").slice(0, 80);
      return {
        cls: html.className.slice(0, 60),
        isUser,
        hasBubble: Boolean(bubble),
        text,
      };
    });
    return { count: children.length, children };
  });
  console.log(`\n=== ${label} ===`);
  if (info.error) {
    console.log("ERROR:", info.error);
    return;
  }
  console.log(`direct transcript children: ${info.count}`);
  for (const [index, child] of (info.children ?? []).entries()) {
    console.log(`  [${index}] user=${child.isUser ? "yes" : "no "} bubble=${child.hasBubble ? "yes" : "no "} :: ${JSON.stringify(child.text)}`);
  }
}

async function run(): Promise<void> {
  const { chromium } = loadPlaywright();
  const browser = await chromium.launch();

  /* case 1: loaded conversation */
  const page = await browser.newPage({ viewport: VIEWPORT });
  await page.goto(`${PAGE_URL}?case=loaded`);
  await page.waitForTimeout(900);
  await inspectTranscript(page, "loaded case (existing multi-turn conversation)");
  await page.screenshot({ path: "/home/z/my-project/scripts/chat-loaded.png", fullPage: false });

  /* case 2: type a message and watch what appears while sending + after */
  await page.__chatHarness;
  await page.evaluate(() => (window as unknown as Record<string, unknown>).__chatHarness);
  const canSend = await page.evaluate(() => {
    const harness = (window as unknown as Record<string, unknown>).__chatHarness as { sendMessage(t: string): Promise<void> } | undefined;
    return Boolean(harness);
  });
  console.log("\nharness hook present:", canSend);
  if (canSend) {
    await page.evaluate(() => {
      const harness = (window as unknown as Record<string, unknown>).__chatHarness as { sendMessage(t: string): Promise<void> };
      return harness.sendMessage("What is my busiest hour?");
    });
    await page.waitForTimeout(400);
    await inspectTranscript(page, "while streaming (400ms after send)");
    await page.screenshot({ path: "/home/z/my-project/scripts/chat-streaming.png", fullPage: false });
    await page.waitForTimeout(1800);
    await inspectTranscript(page, "after completion (~2.2s after send)");
    await page.screenshot({ path: "/home/z/my-project/scripts/chat-after-send.png", fullPage: false });
  }

  /* case 3: brand-new chat — the first message creates the conversation,
     exactly the flow the user described ("when I type message in the chat") */
  const fresh = await browser.newPage({ viewport: VIEWPORT });
  await fresh.goto(`${PAGE_URL}?case=new`);
  await fresh.waitForTimeout(700);
  await inspectTranscript(fresh, "new chat (empty state before first message)");
  await fresh.screenshot({ path: "/home/z/my-project/scripts/chat-new-empty.png", fullPage: false });
  await fresh.evaluate(() => {
    const harness = (window as unknown as Record<string, unknown>).__chatHarness as { sendMessage(t: string): Promise<void> };
    return harness.sendMessage("Help me build a morning report automation");
  });
  await fresh.waitForTimeout(500);
  await inspectTranscript(fresh, "new chat while streaming");
  await fresh.screenshot({ path: "/home/z/my-project/scripts/chat-new-streaming.png", fullPage: false });
  await fresh.waitForTimeout(1800);
  await inspectTranscript(fresh, "new chat after completion");
  await fresh.screenshot({ path: "/home/z/my-project/scripts/chat-new-after.png", fullPage: false });

  await browser.close();
}

run().catch((error) => {
  console.error(error);
  process.exit(1);
});
