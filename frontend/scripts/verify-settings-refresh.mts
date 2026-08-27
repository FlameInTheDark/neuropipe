// Behavioral check for the dirty-draft identity merge that keeps the
// Discord/Telegram/Twitch settings pages from showing stale bots.
//
// The helper under test lives in src/views/SettingsView.tsx, which cannot be
// imported directly (it pulls the whole React view tree), so it is copied
// verbatim below. The script first asserts the copy is byte-identical
// (modulo whitespace) to the shipped helper so any edit that drifts the two
// apart fails this check loudly.
//
// Run: node --experimental-strip-types scripts/verify-settings-refresh.mts
import { readFileSync } from "node:fs";

interface IdentitySlice<I> {
  identities: I[];
  defaultBotIdentityId?: string;
}

function mergeIdentitySlice<S extends IdentitySlice<{ id: string }>>(draft: S, backend: S): S {
  const draftDefault = draft.defaultBotIdentityId ?? "";
  const backendDefault = backend.defaultBotIdentityId ?? "";
  const draftDefaultValid =
    draftDefault !== "" && backend.identities.some((identity) => identity.id === draftDefault);
  const nextDefault = draftDefaultValid ? draftDefault : backendDefault;
  return {
    ...draft,
    identities: backend.identities,
    defaultBotIdentityId: nextDefault || undefined,
  };
}

/* ---------- copy fidelity ---------- */

function extractFunction(text: string, name: string): string {
  const start = text.indexOf(`function ${name}`);
  if (start < 0) throw new Error(`${name} not found`);
  let depth = 0;
  for (let i = text.indexOf("{", start); i < text.length; i++) {
    if (text[i] === "{") depth++;
    else if (text[i] === "}") {
      depth--;
      if (depth === 0) return text.slice(start, i + 1);
    }
  }
  throw new Error(`unbalanced braces for ${name}`);
}

const normalize = (text: string): string => text.replace(/\s+/g, " ").trim();

const view = readFileSync(new URL("../src/views/SettingsView.tsx", import.meta.url), "utf8");
const self = readFileSync(new URL(import.meta.url), "utf8");
if (normalize(extractFunction(view, "mergeIdentitySlice")) !== normalize(extractFunction(self, "mergeIdentitySlice"))) {
  console.error("FAIL: mergeIdentitySlice in SettingsView.tsx diverged from this script's copy");
  process.exit(1);
}

/* ---------- scenarios ---------- */

interface TestIdentity {
  id: string;
  label: string;
}
interface Slice extends IdentitySlice<TestIdentity> {
  clientId: string;
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

// 1. A bot added server-side appears even while the draft holds unsaved edits.
{
  const merged = mergeIdentitySlice<Slice>(
    { clientId: "user-edit", identities: [{ id: "a", label: "A" }] },
    {
      clientId: "old",
      identities: [
        { id: "a", label: "A" },
        { id: "b", label: "B" },
      ],
    },
  );
  check("added bot flows into dirty draft", merged.identities.length === 2 && merged.identities[1].id === "b");
  check("unsaved sibling edit survives", merged.clientId === "user-edit");
}

// 2. A bot removed server-side disappears; the backend's default rotation
//    wins because the draft default now dangles.
{
  const merged = mergeIdentitySlice<Slice>(
    { clientId: "c", defaultBotIdentityId: "gone", identities: [{ id: "gone", label: "G" }, { id: "a", label: "A" }] },
    { clientId: "c", defaultBotIdentityId: "a", identities: [{ id: "a", label: "A" }] },
  );
  check("removed bot disappears", merged.identities.length === 1 && merged.identities[0].id === "a");
  check("dangling default adopts backend rotation", merged.defaultBotIdentityId === "a");
}

// 3. A valid user-picked default survives a backend that rotated elsewhere.
{
  const merged = mergeIdentitySlice<Slice>(
    {
      clientId: "c",
      defaultBotIdentityId: "a",
      identities: [{ id: "a", label: "A" }],
    },
    {
      clientId: "c",
      defaultBotIdentityId: "b",
      identities: [
        { id: "a", label: "A" },
        { id: "b", label: "B" },
      ],
    },
  );
  check("user default choice survives", merged.defaultBotIdentityId === "a");
}

// 4. First bot auto-assigned server-side flows into a draft without default.
{
  const merged = mergeIdentitySlice<Slice>(
    { clientId: "c", identities: [] },
    { clientId: "c", defaultBotIdentityId: "a", identities: [{ id: "a", label: "A" }] },
  );
  check("backend default adopted when draft has none", merged.defaultBotIdentityId === "a");
  check("identities replaced wholesale", merged.identities.length === 1);
}

// 5. Dangling default with no backend default clears instead of dangling.
{
  const merged = mergeIdentitySlice<Slice>(
    { clientId: "c", defaultBotIdentityId: "x", identities: [{ id: "x", label: "X" }] },
    { clientId: "c", identities: [] },
  );
  check("dangling default cleared when backend has none", merged.defaultBotIdentityId === undefined);
}

if (failures > 0) {
  console.error(`\n${failures} check(s) failed`);
  process.exit(1);
}
console.log("\nALL PASSED");
