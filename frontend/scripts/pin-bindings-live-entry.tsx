/* Live harness for the document value-pins editor. Renders the REAL
   PinBindingsEditor with a parent that round-trips every change through
   state — exactly what the Inspector does — then types into the name, label,
   and value inputs character by character with real DOM events, adds and
   deletes rows, and switches to output mode. window.__results reports focus
   retention, committed payloads, and the generated row ids. */

import { useState } from "react";
import { createRoot } from "react-dom/client";
import "../src/i18n";
import { PinBindingsEditor } from "../src/components/PinBindingsEditor";

function Root() {
  const [inputRows, setInputRows] = useState<unknown>([
    { id: "field_1", name: "customer", label: "Customer", value: "Contoso" },
    { id: "field_2", name: "amount", label: "", value: "" },
  ]);
  const [outputRows, setOutputRows] = useState<unknown>([
    { id: "field_1", name: "B4", label: "Total", value: "ignored-literal" },
  ]);
  return (
    <div className="h-screen w-screen overflow-auto bg-ink-1000 p-8 text-fg">
      <div className="mx-auto max-w-[480px] space-y-4">
        <h1 className="text-[16px] font-semibold">Pin bindings editor harness</h1>
        <div id="input-editor">
          <PinBindingsEditor
            label="Value pins"
            value={inputRows}
            mode="input"
            onChange={(next) => setInputRows(next)}
          />
        </div>
        <div id="output-editor">
          <PinBindingsEditor
            label="Cell pins"
            value={outputRows}
            mode="output"
            onChange={(next) => setOutputRows(next)}
          />
        </div>
        <pre id="serialized" className="rounded-lg border border-ink-700 bg-ink-900 p-3 font-mono text-[11px] text-fg-subtle">
          {JSON.stringify({ input: inputRows, output: outputRows }, null, 2)}
        </pre>
      </div>
    </div>
  );
}

const container = document.getElementById("root");
if (!container) throw new Error("#root missing");
createRoot(container).render(<Root />);

/* ---- typing simulation runs below after first paint ---- */
declare global {
  interface Window {
    __results: Record<string, unknown>;
  }
}

void (async () => {
  await new Promise((resolve) => setTimeout(resolve, 300));
  const results: Record<string, unknown> = {};
  const setValue = (input: HTMLInputElement, next: string) => {
    const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, "value")?.set;
    setter?.call(input, next);
    input.dispatchEvent(new Event("input", { bubbles: true }));
  };
  const type = async (input: HTMLInputElement, text: string, key: string) => {
    input.focus();
    for (let i = 0; i < text.length; i++) {
      const next = text.slice(0, i + 1);
      setValue(input, next);
      await new Promise((resolve) => setTimeout(resolve, 20));
      if (document.activeElement !== input) {
        const lost = results[key] as string | undefined;
        results[key] = lost ? `${lost},${next}` : next;
        input.focus();
      }
    }
  };
  const readSerialized = () =>
    JSON.parse(document.getElementById("serialized")?.textContent ?? "{}") as {
      input: Array<{ id: string; name: string; label: string; value: string }>;
      output: Array<{ id: string; name: string; label: string; value: string }>;
    };

  // 1. type into the first row's name field (focus retention regression)
  const nameInput = document.querySelector<HTMLInputElement>('#input-editor input[aria-label="Name (placeholder, column, or cell)"]');
  if (!nameInput) {
    window.__results = { error: "name input not found" };
    return;
  }
  await type(nameInput, "customerName", "nameFocusLostAt");

  // 2. type into the same row's value field
  const valueInputs = Array.from(document.querySelectorAll<HTMLInputElement>('#input-editor input[aria-label="Value"]'));
  await type(valueInputs[0], "Litware", "valueFocusLostAt");

  // 3. type into the label field
  const labelInputs = Array.from(document.querySelectorAll<HTMLInputElement>('#input-editor input[aria-label="Field"]'));
  await type(labelInputs[0], "Kundenname", "labelFocusLostAt");

  // 4. add a row through the real button, then give it a name
  const addButtons = Array.from(document.querySelectorAll<HTMLButtonElement>("#input-editor button")).filter((b) => b.textContent?.includes("Add value pin"));
  addButtons[0]?.click();
  await new Promise((resolve) => setTimeout(resolve, 100));
  const freshInputs = Array.from(document.querySelectorAll<HTMLInputElement>('#input-editor input[aria-label="Name (placeholder, column, or cell)"]'));
  const fresh = freshInputs[freshInputs.length - 1];
  if (fresh) {
    fresh.focus();
    setValue(fresh, "date");
    await new Promise((resolve) => setTimeout(resolve, 50));
  }

  await new Promise((resolve) => setTimeout(resolve, 200));
  const serialized = readSerialized();
  results.inputRows = serialized.input;
  results.outputRows = serialized.output;

  // 5. delete the second row (Trash buttons come after the name inputs)
  const trashButtons = Array.from(document.querySelectorAll<HTMLButtonElement>('#input-editor button[aria-label="Delete"]'));
  trashButtons[trashButtons.length - 1]?.click();
  await new Promise((resolve) => setTimeout(resolve, 150));
  const afterDelete = readSerialized();
  results.inputRowsAfterDelete = afterDelete.input;

  // 6. blank the last remaining row's name — the payload must drop it
  const lastInput = Array.from(document.querySelectorAll<HTMLInputElement>('#input-editor input[aria-label="Name (placeholder, column, or cell)"]')).pop();
  if (lastInput) {
    lastInput.focus();
    setValue(lastInput, "");
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  const afterBlank = readSerialized();
  results.inputRowsAfterBlank = afterBlank.input;

  window.__results = results;
})();
