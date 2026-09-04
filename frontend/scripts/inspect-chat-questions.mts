/* Drives the ChatView questions live page (?case=questions) and verifies the
   ask_user_questions stepper end to end: renders the pending form, answers
   one step by option, one by custom text, skips the last, submits, and
   checks the resolved summary card plus the resumed assistant reply. */

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

async function assertTruthy(condition: unknown, message: string): Promise<void> {
  if (!condition) throw new Error(`assertion failed: ${message}`);
  console.log(`  ok: ${message}`);
}

async function setText(page: any, element: HTMLElement, value: string): Promise<void> {
  await page.evaluate(
    ({ element, value }: { element: HTMLElement; value: string }) => {
      const input = element as unknown as HTMLInputElement;
      const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, "value")!.set!;
      setter.call(input, value);
      input.dispatchEvent(new Event("input", { bubbles: true }));
    },
    { element, value },
  );
}

async function run(): Promise<void> {
  const { chromium } = loadPlaywright();
  const browser = await chromium.launch();
  const page = await browser.newPage({ viewport: VIEWPORT });
  page.on("pageerror", (error: Error) => {
    console.error("PAGE ERROR:", error.message);
    process.exitCode = 1;
  });
  await page.goto(`${PAGE_URL}?case=questions`);
  await page.waitForTimeout(1000);

  console.log("\n=== pending question form ===");
  const initial = await page.evaluate(() => {
    const text = (selector: string) => [...document.querySelectorAll(selector)].map((el) => (el as HTMLElement).textContent?.trim() ?? "");
    return {
      title: document.body.textContent?.includes("The assistant has questions") ?? false,
      question1: document.body.textContent?.includes("Which database engine should we use?") ?? false,
      question2: document.body.textContent?.includes("How much data do we expect per month?") ?? false,
      question3: document.body.textContent?.includes("Do you need built-in full-text search?") ?? false,
      optionDescriptions: document.body.textContent?.includes("Relational, strong consistency") ?? false,
      historyCard: document.body.textContent?.includes("Charts or tables?") ?? false,
      historyCustom: document.body.textContent?.includes("CSV and XLSX") ?? false,
      skippedToolCard: document.body.textContent?.includes("Skipped: the turn paused") ?? false,
      dotsIdle: Boolean(document.querySelector(".animate-bounce")),
      progress: text("span").find((value) => value.startsWith("0 of 3")),
      customInputs: document.querySelectorAll('input[placeholder="Or type your own answer…"]').length,
      skipButtons: text("button").filter((value) => value === "Skip").length,
      submitDisabled: (text("button").find((value) => value === "Send answers") !== undefined) ?? false,
    };
  });
  await assertTruthy(initial.title, "form title rendered");
  await assertTruthy(initial.question1 && initial.question2 && initial.question3, "all three steps rendered");
  await assertTruthy(initial.optionDescriptions, "option descriptions rendered");
  await assertTruthy(initial.historyCard && initial.historyCustom, "resolved history card rendered");
  await assertTruthy(initial.skippedToolCard, "sibling skip tool card rendered");
  await assertTruthy(initial.customInputs === 3, `three custom inputs (got ${initial.customInputs})`);
  await assertTruthy(initial.skipButtons === 3, `three skip buttons (got ${initial.skipButtons})`);
  await assertTruthy(initial.progress === "0 of 3 answered", `progress label (got ${initial.progress})`);
  await assertTruthy(!initial.dotsIdle, "no idle typing dots while the form is open");
  await page.screenshot({ path: "/home/z/my-project/scripts/chat-questions-pending.png", fullPage: false });

  console.log("\n=== answering the steps ===");
  const clicked = await page.evaluate(() => {
    // option buttons concatenate label + description, so match by prefix
    const postgres = [...document.querySelectorAll("button")].find((button) => button.textContent?.trim().startsWith("PostgreSQL"));
    if (!postgres) return false;
    postgres.click();
    return true;
  });
  await assertTruthy(clicked, "PostgreSQL option clickable");
  await page.waitForTimeout(200);
  const submitState = await page.evaluate(() => {
    const submit = [...document.querySelectorAll("button")].find((button) => button.textContent?.trim() === "Send answers") as HTMLButtonElement | undefined;
    return { submitDisabledAt1: submit?.hasAttribute("disabled") ?? null };
  });
  await assertTruthy((submitState as Record<string, unknown>).submitDisabledAt1 === true, "submit stays disabled after 1 of 3");

  // custom answer on step 2
  const secondInput = await page.$$('input[placeholder="Or type your own answer…"]');
  await setText(page, secondInput[1], "About 50 GB, mostly JSON");
  await page.waitForTimeout(150);

  // skip step 3 (third Skip button)
  await page.evaluate(() => {
    const skips = [...document.querySelectorAll("button")].filter((button) => button.textContent?.trim() === "Skip");
    (skips[2] as HTMLElement).click();
  });
  await page.waitForTimeout(150);

  const midState = await page.evaluate(() => {
    const progress = [...document.querySelectorAll("span")].map((el) => el.textContent?.trim() ?? "").find((value) => /of 3 answered/.test(value));
    const submit = [...document.querySelectorAll("button")].find((button) => button.textContent?.trim() === "Send answers") as HTMLButtonElement | undefined;
    return { progress, submitDisabled: submit?.hasAttribute("disabled") ?? null };
  });
  await assertTruthy(midState.progress === "3 of 3 answered", `progress after all steps (got ${midState.progress})`);
  await assertTruthy(midState.submitDisabled === false, "submit enabled once every step is decided");
  await page.screenshot({ path: "/home/z/my-project/scripts/chat-questions-answered.png", fullPage: false });

  await page.evaluate(() => {
    const submit = [...document.querySelectorAll("button")].find((button) => button.textContent?.trim() === "Send answers") as HTMLButtonElement;
    submit.click();
  });

  console.log("\n=== after submit ===");
  await page.waitForTimeout(2300);
  const after = await page.evaluate(() => ({
    resolvedCard: document.body.textContent?.includes("PostgreSQL") ?? false,
    customChip: document.body.textContent?.includes("About 50 GB, mostly JSON") ?? false,
    skippedChip: document.body.textContent?.includes("Skipped") ?? false,
    reply: document.body.textContent?.includes("Great choice - PostgreSQL with about 50 GB per month") ?? false,
    formStillOpen: document.body.textContent?.includes("Or type your own answer…") ?? false,
  }));
  await assertTruthy(after.resolvedCard, "resolved summary card shows the chosen option");
  await assertTruthy(after.customChip, "resolved summary card shows the custom answer");
  await assertTruthy(after.skippedChip, "resolved summary card shows the skipped step");
  await assertTruthy(after.reply, "resumed assistant reply rendered");
  await assertTruthy(!after.formStillOpen, "interactive form replaced by the summary");
  await page.screenshot({ path: "/home/z/my-project/scripts/chat-questions-resolved.png", fullPage: false });

  await browser.close();
  console.log("\nquestions harness done");
}

run().catch((error) => {
  console.error(error);
  process.exit(1);
});
