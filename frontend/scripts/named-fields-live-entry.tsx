/* Live harness for the named typed-fields editor focus regression
   (Build Object / Break Object). Renders the REAL NamedFieldsEditor with a
   parent that round-trips every change through state — exactly what the
   Inspector does — then types into the label and key inputs character by
   character with real DOM events. If any keystroke loses focus or changes
   the row's pin id, window.__results reports it. */

import { useState } from "react";
import { createRoot } from "react-dom/client";
import "../src/i18n";
import { NamedFieldsEditor } from "../src/components/Inspector";

function Root() {
  const [fields, setFields] = useState<unknown>([
    { id: "field_1", label: "name", key: "user.name", dataType: "text" },
    { id: "field_2", label: "age", key: "user.age", dataType: "number" },
  ]);
  return (
    <div className="h-screen w-screen overflow-auto bg-ink-1000 p-8 text-fg">
      <div className="mx-auto max-w-[480px] space-y-4">
        <h1 className="text-[16px] font-semibold">Named fields editor focus harness</h1>
        <div id="editor">
          <NamedFieldsEditor
            label="Fields"
            raw={fields}
            secondKey="key"
            secondLabel="Object key"
            onChange={(next) => setFields(next)}
          />
        </div>
        <pre id="serialized" className="rounded-lg border border-ink-700 bg-ink-900 p-3 font-mono text-[11px] text-fg-subtle">
          {JSON.stringify(fields, null, 2)}
        </pre>
      </div>
    </div>
  );
}

const container = document.getElementById("root");
if (!container) throw new Error("#root missing");
createRoot(container).render(<Root />);

/* ---- typing simulation runs below after first paint ---- */
void (async () => {
  await new Promise((resolve) => setTimeout(resolve, 300));
  const results = {
    labelFocusLostAt: null as string | null,
    keyFocusLostAt: null as string | null,
    idsAfter: [] as string[],
    labelAfter: "",
    keyAfter: "",
  };
  const labelInput = document.querySelector<HTMLInputElement>('#editor input[placeholder="Field name"]');
  const keyInput = document.querySelector<HTMLInputElement>('#editor input[placeholder="Object key"]');
  if (!labelInput || !keyInput) {
    window.__results = { error: "inputs not found", labelInput: !!labelInput, keyInput: !!keyInput };
    return;
  }

  // type "username" into the label field, one char per commit
  const target = "username";
  labelInput.focus();
  for (let i = 0; i < target.length; i++) {
    const next = target.slice(0, i + 1);
    const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, "value")?.set;
    setter?.call(labelInput, next);
    labelInput.dispatchEvent(new Event("input", { bubbles: true }));
    await new Promise((resolve) => setTimeout(resolve, 20));
    if (document.activeElement !== labelInput) {
      results.labelFocusLostAt = next;
      labelInput.focus();
    }
  }

  // type "profile.displayName" into the object-key field
  const keyTarget = "profile.displayName";
  keyInput.focus();
  for (let i = 0; i < keyTarget.length; i++) {
    const next = keyTarget.slice(0, i + 1);
    const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, "value")?.set;
    setter?.call(keyInput, next);
    keyInput.dispatchEvent(new Event("input", { bubbles: true }));
    await new Promise((resolve) => setTimeout(resolve, 20));
    if (document.activeElement !== keyInput) {
      results.keyFocusLostAt = next;
      keyInput.focus();
    }
  }

  await new Promise((resolve) => setTimeout(resolve, 200));
  const serialized = JSON.parse(document.getElementById("serialized")?.textContent ?? "[]") as Array<{ id: string; label: string; key: string }>;
  results.idsAfter = serialized.map((e) => e.id);
  results.labelAfter = serialized[0]?.label ?? "";
  results.keyAfter = serialized[0]?.key ?? "";
  window.__results = results;
})();
