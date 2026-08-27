// Source-fidelity check for the execution-log JSON viewer entry points: the
// hover icon button on each log row, the "Inspect data" button in the expanded
// entry, and the modal wiring in Inspector.tsx LogBody. The modal itself is
// exercised by scripts/run-jsonviewer-modal-test.cjs; this script pins the
// wiring so refactors can't silently drop the buttons (e.g. by re-nesting a
// button inside the row's expand button, which is invalid HTML).
//
// Run: node --experimental-strip-types scripts/verify-log-viewer-buttons.mts
import { readFileSync } from "node:fs";

const inspector = readFileSync(new URL("../src/components/Inspector.tsx", import.meta.url), "utf8");
const modal = readFileSync(new URL("../src/components/JsonViewerModal.tsx", import.meta.url), "utf8");

function fail(message: string): never {
  console.error(`FAIL: ${message}`);
  process.exit(1);
}

/* ---------- modal integration ---------- */

if (!inspector.includes(`import { JsonViewerModal } from "./JsonViewerModal";`))
  fail("Inspector.tsx no longer imports JsonViewerModal");

if (!inspector.includes("const [viewerEntry, setViewerEntry] = useState<LogEntry | null>(null);"))
  fail("LogBody lost the viewerEntry state");

if (!inspector.includes("{viewerEntry && <JsonViewerModal entry={viewerEntry} onClose={() => setViewerEntry(null)} />}"))
  fail("LogBody no longer renders the JsonViewerModal");

/* ---------- entry points: both buttons open the same modal ---------- */

const opens = inspector.match(/onClick=\{\(\) => setViewerEntry\(l\)\}/g) ?? [];
if (opens.length !== 2)
  fail(`expected exactly 2 setViewerEntry(l) openers (row hover + expanded area), found ${opens.length}`);

/* row button must be gated on hasData so entries without payloads don't show it */
if (!inspector.includes("const hasData = (l.input !== undefined && l.input !== null) || (l.output !== undefined && l.output !== null);"))
  fail("hasData guard (null-aware) is missing");

if (!inspector.includes("{hasData && (\n                  <Tooltip content={t(\"jsonViewer.inspect\")} side=\"left\" delay={200}>"))
  fail("row hover inspect button with left tooltip is missing");

if (!inspector.includes("{hasData && (\n                    <button\n                      onClick={() => setViewerEntry(l)}\n                      className=\"flex h-6 w-full"))
  fail("expanded-area Inspect data button is missing");

/* the inspect button must sit OUTSIDE the expand <button> — nested buttons are
   invalid HTML and break click handling; the restructure splits the header row
   into siblings inside a flex wrapper */
const headerRow = inspector.match(/<div className="flex items-center gap-1">[\s\S]*?<\/button>\s*\{hasData &&/);
if (!headerRow) fail("header row restructure missing: inspect button must be a sibling of the expand button");

/* hover-reveal: the row button fades in on row hover via the li's group class */
if (!inspector.includes('"group border-b border-seam/70 px-3 py-2 transition"'))
  fail("log row li lost its group class (hover reveal target)");
if (!inspector.includes("opacity-0 transition hover:bg-ink-750 hover:text-ink-50 focus-visible:opacity-100 group-hover:opacity-100"))
  fail("row inspect button lost its hover/focus reveal classes");

/* ---------- modal contract the wiring depends on ---------- */

if (!modal.includes("export function JsonViewerModal({ entry, onClose }: { entry: LogEntry; onClose: () => void })"))
  fail("JsonViewerModal signature changed — Inspector wiring needs entry + onClose");

/* primitive roots must be wrapped: the library splits bare strings into
   characters and throws on null roots */
if (!modal.includes("isStructured(value) ? value : { value }"))
  fail("primitive-root wrap guard missing in JsonViewerModal");

/* escape closes the modal */
if (!modal.includes('if (e.key === "Escape") onClose();'))
  fail("Escape-to-close handler missing in JsonViewerModal");

/* ---------- tall-payload containment (scroll regression) ----------
   The panes live in a CSS grid row. Without an explicit constrained row the
   implicit auto row grows to content height, so tall JSON trees blow past the
   panes' overflow-auto: no scrollbar, data painted over the footer and clipped
   by the modal edge. grid-rows-1 (= repeat(1, minmax(0,1fr))) pins the row to
   the body height; min-h-0 on the pane section removes the grid item's
   content-based minimum so it can shrink to the track. */
if (!modal.includes("grid min-h-0 flex-1 grid-cols-2 grid-rows-1 divide-x divide-seam"))
  fail("modal body grid lost grid-rows-1 — tall JSON will overflow instead of scrolling");
if (!modal.includes('<section className="flex min-h-0 min-w-0 flex-col">'))
  fail("DataPane section lost min-h-0 — grid item can force the row past its track");

/* ---------- per-node copy-path action ----------
   The built-in hover copy icon is replaced via the library's Copied section
   render override with two app-styled buttons: copy element (forwards the
   library's own onClick + data-copied feedback) and copy JSON path (built
   from the node's full key chain). Pins the contract the live test relies on. */
if (!modal.includes("<JsonView.Copied render={renderNodeTools} />"))
  fail("JsonView no longer registers the Copied section render override (per-node tools)");

if (!modal.includes('props.onClick as unknown as React.MouseEventHandler<HTMLButtonElement>'))
  fail("copy-element button no longer forwards the library's built-in copy handler");

if (!modal.includes('testId="copy-path"'))
  fail("copy-path button (data-testid=copy-path) missing");

if (!modal.includes("e.stopPropagation();"))
  fail("copy-path click no longer stops propagation — clicks would fold object rows");

if (!modal.includes('jsonPathToString(result.keys)'))
  fail("copy-path no longer builds the path from the node's key chain");

if (!modal.includes('const path = result.keys?.length ? jsonPathToString(result.keys) : "";'))
  fail("copy-path root guard missing — root node (empty path) must not render the button");

/* ---------- per-node tooltips: app Tooltip component, not native title ----------
   The node buttons sit inside the pane's overflow-auto, where an OS-styled
   native title is visually off-theme (every other button in the modal uses the
   app Tooltip). Pins: NodeToolButton wraps in <Tooltip>, keeps aria-label for
   accessibility, and sets NO native title attribute; the copy-path tooltip
   previews the exact path in mono (capped so the nowrap bubble can never
   overflow the viewport — the Bubble only clamps position, not width). */
const nodeBtnStart = modal.indexOf("function NodeToolButton");
const nodeBtnEnd = modal.indexOf("function CopyPathButton");
if (nodeBtnStart < 0 || nodeBtnEnd <= nodeBtnStart)
  fail("NodeToolButton component missing from JsonViewerModal.tsx");
const nodeBtn = modal.slice(nodeBtnStart, nodeBtnEnd);

if (!nodeBtn.includes('<Tooltip content={tooltip ?? label} side="bottom" delay={200}>'))
  fail("NodeToolButton no longer wraps its button in the app Tooltip component");

if (nodeBtn.includes(" title="))
  fail("NodeToolButton still sets a native title attribute — replace with the app Tooltip");

if (!nodeBtn.includes("aria-label={label}"))
  fail("NodeToolButton lost aria-label — the tooltip is visual-only, the button needs an accessible name");

if (!modal.includes('className="max-w-[420px] truncate font-mono text-sky-300/90"'))
  fail("copy-path tooltip no longer previews the path (mono, viewport-safe truncation)");

/* path builder must live in lib and handle identifiers, indices, quoted keys */
const jsonPath = readFileSync(new URL("../src/lib/jsonPath.ts", import.meta.url), "utf8");
if (!jsonPath.includes("export function jsonPathToString"))
  fail("jsonPathToString no longer exported from src/lib/jsonPath.ts");

/* i18n keys must exist in all four locales */
for (const locale of ["en", "de", "fr", "ru"]) {
  const file = readFileSync(new URL(`../src/i18n/${locale}.ts`, import.meta.url), "utf8");
  if (!file.includes("copyElement:")) fail(`jsonViewer.copyElement missing in ${locale}.ts`);
  if (!file.includes("copyPath:")) fail(`jsonViewer.copyPath missing in ${locale}.ts`);
}

console.log("ALL PASSED");
