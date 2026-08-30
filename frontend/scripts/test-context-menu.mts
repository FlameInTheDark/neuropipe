/* Unit tests for the app-wide default-context-menu policy
   (frontend/src/lib/context-menu.ts).

   Product contract: the webview's native context menu must never open in a
   release build — only Neuropipe's own ContextMenu components (which render
   their own portal UI and preventDefault themselves) may appear. Dev builds
   keep the native menu for web inspection (Inspect Element etc.).

   The installer is a pure function of (dev, target); the suite injects a
   recording target instead of a real document, so no DOM stub is needed.
   Run: npx tsx scripts/test-context-menu.mts */

import { installDefaultContextMenuPolicy } from "../src/lib/context-menu";

/** Stand-in for the contextmenu DOM event — only the members the policy and
 *  custom handlers use (preventDefault + the prevented flag). */
type FakeEvent = {
  type: "contextmenu";
  defaultPrevented: boolean;
  preventDefault(): void;
};

function fakeEvent(): FakeEvent {
  const ev = {
    type: "contextmenu" as const,
    defaultPrevented: false,
    preventDefault() {
      ev.defaultPrevented = true;
    },
  };
  return ev;
}

/** Recording document stand-in: addEventListener stores (type, listener,
 *  options) in registration order; the tests replay the listener the same
 *  way the DOM would deliver a bubbling contextmenu event. */
function fakeTarget() {
  const regs: { type: string; listener: (e: FakeEvent) => void; options?: unknown }[] = [];
  return {
    regs,
    target: {
      addEventListener(type: string, listener: (e: FakeEvent) => void, options?: unknown) {
        regs.push({ type, listener, options });
      },
    } as unknown as Document,
  };
}

let failed = 0;
let passed = 0;
function check(name: string, ok: boolean, detail?: string) {
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${ok || !detail ? "" : ` — ${detail}`}`);
  if (ok) passed++;
  else failed++;
}

/* --- dev build: nothing is installed, the native menu stays available --- */
{
  const { regs, target } = fakeTarget();
  installDefaultContextMenuPolicy(true, target);
  check(
    "dev build registers no listener (native menu available)",
    regs.length === 0,
    `got ${regs.length} listener(s)`,
  );
}

/* --- release build: exactly one bubble-phase contextmenu listener --- */
{
  const { regs, target } = fakeTarget();
  installDefaultContextMenuPolicy(false, target);
  check(
    "release build registers exactly one contextmenu listener",
    regs.length === 1 && regs[0].type === "contextmenu",
    JSON.stringify(regs.map((r) => r.type)),
  );
  check(
    "listener is bubble-phase (document runs last, after React handlers)",
    regs.length === 1 && regs[0].options === undefined,
    `options=${JSON.stringify(regs[0]?.options)}`,
  );
}

/* --- release build: a contextmenu event ends up defaultPrevented --- */
{
  const { regs, target } = fakeTarget();
  installDefaultContextMenuPolicy(false, target);
  const ev = fakeEvent();
  regs[0].listener(ev as unknown as Event);
  check("release listener prevents the native menu", ev.defaultPrevented);
}

/* --- custom menus keep working alongside the release policy --- */
{
  const { regs, target } = fakeTarget();
  installDefaultContextMenuPolicy(false, target);

  // a ContextMenu.tsx-style handler attached to an element: preventDefault +
  // open its own portal UI. Element handlers run while the event bubbles up,
  // BEFORE the document-level policy listener.
  let customOpened = false;
  let sawFreshEvent = false;
  const openCustomMenu = (e: { defaultPrevented: boolean; preventDefault(): void }) => {
    sawFreshEvent = !e.defaultPrevented; // handlers must see an unconsumed event
    e.preventDefault();
    customOpened = true;
  };

  const ev = fakeEvent();
  openCustomMenu(ev); // element phase (React root container sits below document)
  regs[0].listener(ev as unknown as Event); // document phase (the policy)
  check(
    "custom menu opens AND the native menu is suppressed",
    customOpened && ev.defaultPrevented,
  );
  check(
    "element handlers see defaultPrevented=false (policy runs after them)",
    sawFreshEvent,
  );
}

/* --- policy never swallows the event: every registered consumer still runs --- */
{
  const { regs, target } = fakeTarget();
  installDefaultContextMenuPolicy(false, target);

  const calls: string[] = [];
  const elementHandler = (e: { preventDefault(): void }) => {
    e.preventDefault();
    calls.push("element");
  };
  const ev = fakeEvent();
  elementHandler(ev);
  regs[0].listener(ev as unknown as Event);
  calls.push("document");
  check(
    "preventDefault never stops propagation (both handlers ran)",
    calls.length === 2 && calls[0] === "element" && calls[1] === "document",
    calls.join(" -> "),
  );
}

console.log(`\n${passed} passed, ${failed} failed`);
process.exit(failed ? 1 : 0);
