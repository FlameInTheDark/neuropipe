/* Browser entry for the ChatView transcript live harness: mounts the REAL
   ChatView against the faithful chat-bridge-mock (multi-turn conversation:
   plain turns plus one tool round) and exposes scenario hooks via ?case=:

   - loaded : the stored transcript as the user sees it when reopening a chat
              (expect one bubble per turn, tool cards inline)
   - send   : types a message into the composer, presses Enter, and lets the
              mocked backend replay the full event sequence (user row,
              streamed tokens, persisted reply) — the exact moment the user
              reported "all turns in one chat element"

   Bundled to IIFE by render-chat-transcript-live.mts. */

import React from "react";
import { createRoot } from "react-dom/client";
import { I18nextProvider } from "react-i18next";
import i18n from "../src/i18n";
import ChatView from "../src/views/ChatView";

const caseName = new URLSearchParams(window.location.search).get("case") ?? "loaded";

function Page() {
  return (
    <div className="h-screen w-screen overflow-hidden bg-ink-950">
      <ChatView />
    </div>
  );
}

const root = createRoot(document.getElementById("root")!);
root.render(
  <React.StrictMode>
    <I18nextProvider i18n={i18n}>
      <Page />
    </I18nextProvider>
  </React.StrictMode>,
);

/* Driver hook: for the send scenario, type into the composer and submit. */
(window as unknown as Record<string, unknown>).__chatHarness = {
  caseName,
  ready: true,
  async sendMessage(text: string) {
    const textarea = document.querySelector("textarea");
    if (!textarea) throw new Error("composer textarea not found");
    const setter = Object.getOwnPropertyDescriptor(window.HTMLTextAreaElement.prototype, "value")!.set!;
    setter.call(textarea, text);
    textarea.dispatchEvent(new Event("input", { bubbles: true }));
    await new Promise((resolve) => setTimeout(resolve, 50));
    textarea.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", code: "Enter", bubbles: true }));
  },
};
