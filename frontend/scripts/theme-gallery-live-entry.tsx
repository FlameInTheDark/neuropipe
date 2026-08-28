/**
 * Task 25 live entry — theme gallery over REAL app primitives
 * (ui.tsx, primitives/styles.ts, lib/pins.ts, MarkdownRenderer) with a
 * Dark/Light switch driven by the real theme store. Both themes are
 * screenshotted and reviewed.
 */
import { createRoot } from "react-dom/client";
import { useThemeStore, initTheme } from "../src/stores/theme";
import { Button, IconButton, Badge, Dot, Toggle, Empty, Divider, Panel, PanelHeader } from "../src/components/ui";
import { Toaster } from "../src/components/layout/Toaster";
import type { Toast } from "../src/hooks/useToast";
import { surface, control, chip, text } from "../src/components/primitives/styles";
import { pinPalette, ASSIGNABLE_PIN_TYPES } from "../src/lib/pins";
import { MarkdownRenderer } from "../src/components/MarkdownRenderer";
import { useState } from "react";

const MD = `# Heading one
## Heading two
A paragraph with **bold**, *italic*, a [link](https://example.com) and \`inline code\`.

\`\`\`js
// comment
const pins = { tool: "keyword", text: "string", count: 42, flag: true };
function run(pins) { return pins.tool + " → " + pins.count; }
\`\`\`

> A blockquote across the markdown surface.

| Pin | Type | Note |
| --- | --- | --- |
| tool | exec | violet |
| text | data | pink |
`;

function Swatch({ label, color, tall }: { label: string; color: string; tall?: boolean }) {
  return (
    <div className="flex items-center gap-2">
      <span
        className={tall ? "h-4 w-8 shrink-0 rounded-sm border border-ink-700" : "h-4 w-4 shrink-0 rounded-full"}
        style={{ background: color, borderColor: color }}
      />
      <span className="font-mono text-[10px] text-fg-subtle">{label}</span>
    </div>
  );
}

function Gallery() {
  const theme = useThemeStore((s) => s.theme);
  const setTheme = useThemeStore((s) => s.setTheme);
  const [toggle, setToggle] = useState(true);
  const [input, setInput] = useState("");
  const toast: Toast = { id: 1, text: "Settings saved.", icon: "Check" };

  return (
    <div className="min-h-screen bg-ink-1000 p-5" data-testid="gallery">
      {/* theme switch toolbar */}
      <div className="sticky top-0 z-10 mb-4 flex items-center gap-2 rounded-xl border border-ink-700 bg-ink-900/95 p-2 backdrop-blur">
        <span className="px-1 text-[11px] font-medium tracking-[0.08em] text-fg-subtle uppercase">
          Neuropipe color system — live gallery
        </span>
        <div className="ml-auto flex items-center gap-1 rounded-md border border-ink-700 bg-ink-850 p-0.5">
          {(["dark", "light"] as const).map((t) => (
            <button
              key={t}
              onClick={() => setTheme(t)}
              aria-pressed={theme === t}
              className={
                "h-6 rounded px-2.5 text-[11px] font-medium transition " +
                (theme === t ? "bg-ink-50 text-fg-onEmphasis" : "text-fg-subtle hover:text-fg")
              }
            >
              {t}
            </button>
          ))}
        </div>
      </div>

      <div className="grid grid-cols-2 gap-4">
        {/* buttons + toggles */}
        <section className={surface.panel + " p-3"}>
          <p className={text.eyebrow}>Buttons</p>
          <div className="mt-2 flex flex-wrap items-center gap-2">
            <Button variant="primary" icon="Play">Run pipeline</Button>
            <Button variant="solid" icon="Save">Solid</Button>
            <Button variant="ghost" icon="Settings2">Ghost</Button>
            <Button variant="primary" disabled>Disabled</Button>
            <IconButton icon="X" label="close" />
            <IconButton icon="Check" label="ok" active />
            <Toggle on={toggle} onChange={setToggle} />
            <Toggle on={!toggle} onChange={setToggle} />
          </div>
        </section>

        {/* badges + dots + status */}
        <section className={surface.panel + " p-3"}>
          <p className={text.eyebrow}>Status — Linear hexes</p>
          <div className="mt-2 flex flex-wrap items-center gap-2">
            <Badge tone="ok">ok · completed</Badge>
            <Badge tone="warn">warn · queued</Badge>
            <Badge tone="run">run · active</Badge>
            <Badge tone="muted">muted</Badge>
            <Dot tone="done" /> <Dot tone="running" /> <Dot tone="queued" /> <Dot tone="error" /> <Dot tone="idle" />
          </div>
          <div className="mt-3 flex flex-wrap gap-2 text-[11px]">
            <span className="rounded-md border border-success/30 bg-success/10 px-2 py-1 text-success-fg">success #27a644</span>
            <span className="rounded-md border border-warning/30 bg-warning/10 px-2 py-1 text-warning-fg">warning #f0bf00</span>
            <span className="rounded-md border border-danger/30 bg-danger/10 px-2 py-1 text-danger-fg">danger #eb5757</span>
            <span className="rounded-md border border-info/30 bg-info/10 px-2 py-1 text-info-fg">info #4ea7fc</span>
          </div>
        </section>

        {/* inputs */}
        <section className={surface.panel + " p-3"}>
          <p className={text.eyebrow}>Controls</p>
          <div className="mt-2 flex flex-col gap-2">
            <input
              className={control.input}
              placeholder="Standard input — type here"
              value={input}
              onChange={(e) => setInput(e.target.value)}
            />
            <input className={control.inputSm} placeholder="Compact input" />
            <textarea className={control.textarea} placeholder="Multi-line input" rows={2} />
            <div className={control.segment}>
              <button className="h-6 rounded bg-ink-50 px-2.5 text-[11px] font-medium text-fg-onEmphasis">List</button>
              <button className="h-6 rounded px-2.5 text-[11px] text-fg-subtle">Grouped</button>
            </div>
            <div className="flex gap-2">
              <span className={chip.muted}>chip.muted</span>
              <span className={chip.mono}>chip.mono</span>
              <span className={chip.kbd}>⌘K</span>
            </div>
          </div>
        </section>

        {/* text hierarchy + wells */}
        <section className={surface.panel + " p-3"}>
          <p className={text.eyebrow}>Foreground tokens</p>
          <div className="mt-2 flex flex-col gap-1.5">
            <p className="text-[13px] text-fg">fg — default body text on panels</p>
            <p className="text-[13px] text-fg-muted">fg-muted — secondary emphasis</p>
            <p className="text-[13px] text-fg-subtle">fg-subtle — labels and metadata</p>
            <p className="text-[13px] text-fg-faint">fg-faint — hints and placeholders</p>
            <p className="text-[13px] text-fg-onEmphasis">
              <span className="rounded bg-ink-50 px-1.5 py-0.5">fg-onEmphasis on ink-50</span>
            </p>
            <div className={surface.well + " mt-1 p-2"}>
              <p className={text.hint}>A well (surface.well) with a hint inside — inset wells stay recessed in both themes.</p>
            </div>
          </div>
        </section>

        {/* pin palette */}
        <section className={surface.panel + " p-3"}>
          <p className={text.eyebrow}>Pin data-type palette</p>
          <div className="mt-2 grid grid-cols-3 gap-1.5">
            {(["exec", "tool", "text", "number", "boolean", "array", "map", "object", "any"] as const).map((t) => (
              <Swatch key={t} label={`${t} · ${pinPalette(t).name}`} color={pinPalette(t).dot} />
            ))}
          </div>
          <div className="mt-2 flex flex-wrap gap-1.5">
            {ASSIGNABLE_PIN_TYPES.map((t) => (
              <span key={t} className="rounded px-1.5 py-0.5 font-mono text-[10px]" style={{ color: pinPalette(t).label }}>
                {t}
              </span>
            ))}
          </div>
        </section>

        {/* overlay + panel header + empty + toast */}
        <section className={surface.panel + " p-3"}>
          <p className={text.eyebrow}>Surfaces</p>
          <div className="mt-2 relative h-44 rounded-lg border border-seam bg-ink-1000 p-3">
            <Panel className="absolute inset-x-3 top-3 bottom-16 rounded-lg border border-ink-700">
              <PanelHeader title="PanelHeader" icon="Activity" right={<Badge tone="ok">live</Badge>} />
              <div className="flex flex-1 items-center justify-center">
                <Empty icon="Workflow" text="Empty state with icon" />
              </div>
            </Panel>
            <div className={surface.overlay + " absolute inset-x-6 bottom-2 flex items-center gap-2 p-2"}>
              <Dot tone="running" />
              <span className="text-[11.5px] text-fg-muted">surface.overlay — floating menu surface</span>
              <Divider />
              <span className="font-mono text-[10px] text-fg-faint">seam-x demo</span>
            </div>
          </div>
          <div className="pointer-events-none fixed bottom-4 left-1/2 -translate-x-1/2">
            <Toaster toast={toast} />
          </div>
        </section>

        {/* markdown + hljs */}
        <section className={surface.panel + " p-3"}>
          <p className={text.eyebrow}>Markdown + code highlighting</p>
          <div className="mt-2 max-h-[420px] overflow-y-auto pr-1">
            <MarkdownRenderer text={MD} />
          </div>
        </section>
      </div>
    </div>
  );
}

/* identical boot path to main.tsx: apply the persisted theme pre-render */
initTheme();

createRoot(document.getElementById("root")!).render(<Gallery />);
