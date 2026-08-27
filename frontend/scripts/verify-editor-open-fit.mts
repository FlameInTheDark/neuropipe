// Behavioral check for the editor-open auto-fit ordering that keeps the
// pipeline editor from opening with the graph off-screen.
//
// The real effect lives in src/App.tsx and cannot be executed here (it needs
// the React tree), so this script mirrors its guard semantics and asserts the
// shipped source still contains the exact gating conditions (copy fidelity).
// The scenario machine replays the async open sequence — nav sets the editing
// target optimistically, the backend load lands later — and asserts the fit
// fires only once the loaded graph is the one the editor is showing.
//
// Run: node --experimental-strip-types scripts/verify-editor-open-fit.mts
import { readFileSync } from "node:fs";

/* ---------- copy fidelity: the shipped effect must keep the gating ---------- */

const app = readFileSync(new URL("../src/App.tsx", import.meta.url), "utf8");

function fail(message: string): never {
  console.error(`FAIL: ${message}`);
  process.exit(1);
}

const effectNeedles = [
  "if (!nav.inEditor || !editorTargetId || editorTargetId !== loadedGraphId) return;",
  "const loadedGraphId = graph.pipeline?.id ?? graph.fn?.id ?? null;",
  "[nav.inEditor, editorTargetId, loadedGraphId]",
];
for (const needle of effectNeedles) {
  if (!app.includes(needle)) {
    fail(`App.tsx auto-fit effect no longer contains: ${needle}`);
  }
}

/* ---------- scenario machine ---------- */
//
// Mirror of the effect: it runs when any of (inEditor, editorTargetId,
// loadedGraphId) changes, and fires the fit only when the editor is open and
// the graph loaded into the editor IS the graph nav is showing. The nodes the
// fit would see are whatever setNodes last committed.

interface EditorState {
  inEditor: boolean;
  editorTargetId: string | null; // nav's optimistic target
  loadedGraphId: string | null; // graph.pipeline?.id ?? graph.fn?.id ?? null
  nodes: { id: string }[]; // graph.nodes committed state
  userPanned: boolean;
}

function shouldFitFiring(state: EditorState): boolean {
  if (!state.inEditor) return false;
  if (!state.editorTargetId) return false;
  if (state.editorTargetId !== state.loadedGraphId) return false;
  return true; // the rAF then calls fitRef.current?.()
}

let failures = 0;
function check(name: string, condition: boolean) {
  if (!condition) {
    console.error(`FAIL: ${name}`);
    failures++;
  } else {
    console.log(`ok: ${name}`);
  }
}

function replay(name: string, steps: Array<(state: EditorState) => void>) {
  const state: EditorState = {
    inEditor: false,
    editorTargetId: null,
    loadedGraphId: null,
    nodes: [],
    userPanned: false,
  };
  let fitFirings = 0;
  let fitSawEmptyNodes = false;
  let fitSawForeignNodes = false;
  const deps = (s: EditorState) => `${s.inEditor}|${s.editorTargetId}|${s.loadedGraphId}`;
  let prevDeps = deps(state);
  const fire = () => {
    if (shouldFitFiring(state)) {
      fitFirings++;
      if (state.nodes.length === 0) fitSawEmptyNodes = true;
      if (state.loadedGraphId !== null && state.nodes.some((n) => !n.id.startsWith(state.loadedGraphId!))) {
        fitSawForeignNodes = true;
      }
    }
  };
  // every step is a render; the effect re-runs only when its deps changed
  for (const step of steps) {
    step(state);
    const nextDeps = deps(state);
    if (nextDeps !== prevDeps) {
      prevDeps = nextDeps;
      fire();
    }
  }
  return { name, fitFirings, fitSawEmptyNodes, fitSawForeignNodes, state };
}

// 1. The reported bug: open a pipeline whose nodes load asynchronously. The
//    optimistic nav target lands first (empty editor), the backend fetch and
//    setNodes/setPipeline land later. The fit must fire exactly once — AFTER
//    the load commit — and never against empty nodes.
{
  const r = replay("open pipeline, async load", [
    (s) => {
      s.inEditor = true;
      s.editorTargetId = "pipe-1"; // nav.doOpenPipeline sets this synchronously
    },
    // ...backend getPipeline in flight; several renders happen with no nodes...
    (s) => s,
    (s) => s,
    (s) => {
      // loadPipeline commit: setNodes + setPipeline batched atomically
      s.nodes = [{ id: "pipe-1:n1" }, { id: "pipe-1:n2" }];
      s.loadedGraphId = "pipe-1";
    },
    (s) => s, // post-load render (reresolve pin updates etc.)
  ]);
  check(`${r.name}: fit fires exactly once`, r.fitFirings === 1);
  check(`${r.name}: fit never ran against empty nodes`, !r.fitSawEmptyNodes);
  check(`${r.name}: fit never ran against foreign nodes`, !r.fitSawForeignNodes);
}

// 2. Close the editor, reopen the SAME pipeline: close() nulls the loaded
//    graph, so the reopen must fit again (not skip because ids repeat).
{
  const r = replay("close then reopen same pipeline", [
    (s) => {
      s.inEditor = true;
      s.editorTargetId = "pipe-1";
    },
    (s) => {
      s.nodes = [{ id: "pipe-1:n1" }];
      s.loadedGraphId = "pipe-1";
    },
    (s) => {
      // graph.close(): nodes cleared, pipeline nulled
      s.inEditor = false;
      s.editorTargetId = null;
      s.loadedGraphId = null;
      s.nodes = [];
    },
    (s) => {
      s.inEditor = true;
      s.editorTargetId = "pipe-1";
    },
    (s) => {
      s.nodes = [{ id: "pipe-1:n1" }];
      s.loadedGraphId = "pipe-1";
    },
  ]);
  check(`${r.name}: fit fires once per open session`, r.fitFirings === 2);
}

// 3. Deep-link swap: open pipeline B while pipeline A is already on screen.
//    The old nodes are still committed when nav's target flips to B — the fit
//    must NOT frame A's stale nodes; it fires only after B loads.
{
  const r = replay("swap pipelines while editing", [
    (s) => {
      s.inEditor = true;
      s.editorTargetId = "pipe-a";
    },
    (s) => {
      s.nodes = [{ id: "pipe-a:n1" }];
      s.loadedGraphId = "pipe-a";
    },
    (s) => {
      s.editorTargetId = "pipe-b"; // nav swaps target optimistically
    },
    (s) => s, // A's nodes still on screen while B is fetched
    (s) => {
      s.nodes = [{ id: "pipe-b:n1" }];
      s.loadedGraphId = "pipe-b";
    },
  ]);
  check(`${r.name}: fit fires once per loaded graph (A then B)`, r.fitFirings === 2);
  check(`${r.name}: fit never framed the stale graph`, !r.fitSawForeignNodes);
}

// 4. After the open-fit, editing must not keep re-fitting: adding nodes or
//    panning around changes nodes/view state but none of the effect deps.
{
  const r = replay("no refit while editing", [
    (s) => {
      s.inEditor = true;
      s.editorTargetId = "pipe-1";
    },
    (s) => {
      s.nodes = [{ id: "pipe-1:n1" }];
      s.loadedGraphId = "pipe-1";
    },
    (s) => {
      s.nodes = [...s.nodes, { id: "pipe-1:n2" }]; // user adds a node
    },
    (s) => {
      s.userPanned = true; // user pans away — view must be respected
    },
    (s) => {
      s.nodes = s.nodes.slice(0, 1); // user deletes a node
    },
  ]);
  check(`${r.name}: fit fired exactly once (on load)`, r.fitFirings === 1);
}

// 5. Load failure (legacy schema): nodes stay empty, loadedGraphId never
//    matches — no fit, no crash; the legacy error state renders instead.
{
  const r = replay("legacy schema load error", [
    (s) => {
      s.inEditor = true;
      s.editorTargetId = "pipe-legacy";
    },
    (s) => {
      s.nodes = []; // loadPipeline's legacy branch clears nodes, sets loadError
      s.loadedGraphId = null;
    },
    (s) => s,
  ]);
  check(`${r.name}: fit never fires`, r.fitFirings === 0);
}

// 6. Contrast — the previous gating (fire whenever the editor opens) failed
//    scenario 1: the effect fired while nodes were still empty and nothing
//    re-fired after the load landed.
{
  const oldShouldFire = (state: EditorState) => state.inEditor && state.editorTargetId !== null;
  const sequence: Array<(state: EditorState) => void> = [
    (s) => {
      s.inEditor = true;
      s.editorTargetId = "pipe-1";
    },
    () => undefined,
    (s) => {
      s.nodes = [{ id: "pipe-1:n1" }];
      s.loadedGraphId = "pipe-1";
    },
  ];
  const state: EditorState = {
    inEditor: false,
    editorTargetId: null,
    loadedGraphId: null,
    nodes: [],
    userPanned: false,
  };
  let firedOnEmpty = false;
  let firedAfterLoad = false;
  // the OLD effect's deps were [editorTargetId, nav.inEditor] — loadedGraphId
  // was not a dependency, which is why nothing re-fired once the load landed
  let prevDeps = `${state.inEditor}|${state.editorTargetId}`;
  for (const step of sequence) {
    step(state);
    const nextDeps = `${state.inEditor}|${state.editorTargetId}`;
    if (nextDeps === prevDeps) continue;
    prevDeps = nextDeps;
    if (oldShouldFire(state)) {
      if (state.nodes.length === 0) firedOnEmpty = true;
      else firedAfterLoad = true;
    }
  }
  check("old behavior demonstrably fired on empty nodes (bug reproduced)", firedOnEmpty && !firedAfterLoad);
}

if (failures > 0) {
  console.error(`\n${failures} check(s) failed`);
  process.exit(1);
}
console.log("\nAll editor-open fit checks passed");
