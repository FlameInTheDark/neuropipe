/* Regression tests for the named typed-fields editor identity rules
   (Build Object "object-fields" / Break Object "field-outputs").

   Bug being pinned down: the old serializer re-derived each field's id from
   its label on every commit, so typing one character in the label input
   changed the row's React key → React remounted the row → the input lost
   focus after every keystroke. Worse, the id IS the dynamic pin id, so every
   keystroke also renamed the canvas pin and dropped its wires.

   The contract under test: ids are minted once (when a row is added or first
   parsed) and then stay frozen through label/key/path/dataType edits.
   Run: npx tsx scripts/test-named-fields.mts */

import {
  parseNamedFields,
  serializeNamedFields,
  uniqueFieldID,
  validFieldID,
} from "../src/lib/named-fields";

let failed = 0;
let passed = 0;
function check(name: string, ok: boolean, detail?: string) {
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${ok || !detail ? "" : ` — ${detail}`}`);
  if (ok) passed++;
  else failed++;
}

/* --- the reported bug: typing in the label must not churn the id --- */

const savedConfig = [
  { id: "field_1", label: "name", key: "user.name", dataType: "text" },
  { id: "field_2", label: "age", key: "user.age", dataType: "number" },
];

let entries = parseNamedFields(savedConfig);
const idsBefore = entries.map((e) => e.id);

/* simulate typing "username" into the first row's label, one keystroke per
   commit — exactly what the controlled input does via patch() */
for (const typed of ["u", "us", "use", "user", "usern", "userna", "usernam", "username"]) {
  entries = parseNamedFields(
    serializeNamedFields(entries.map((e, i) => (i === 0 ? { ...e, label: typed } : e))),
  );
}
const idsAfterTyping = entries.map((e) => e.id);
check(
  "label typing never changes ids (focus + wires survive)",
  JSON.stringify(idsBefore) === JSON.stringify(idsAfterTyping),
  `${idsBefore.join(",")} -> ${idsAfterTyping.join(",")}`,
);
check("label edit lands", entries[0].label === "username" && entries[0].key === "user.name");
check("sibling rows untouched", entries[1].label === "age" && entries[1].dataType === "number");

/* --- editing the object key / path also must not touch the id --- */

const keyed = parseNamedFields(
  serializeNamedFields(entries.map((e, i) => (i === 0 ? { ...e, key: "profile.displayName" } : e))),
);
check("key edits keep ids stable", keyed[0].id === entries[0].id && keyed[0].key === "profile.displayName");

/* --- new rows get unique, collision-free ids --- */

check("fresh id skips used ones", uniqueFieldID(new Set(["field_1", "field_2"])) === "field_3");
check("fresh id fills gaps", uniqueFieldID(new Set(["field_1", "field_3"])) === "field_2");
check("fresh id always valid", validFieldID(uniqueFieldID(new Set(["field_1"]))));

/* --- malformed / legacy payloads are healed, not crashed on --- */

const noIds = parseNamedFields([{ label: "a", key: "a", dataType: "any" }]);
check("missing ids get index fallback", noIds[0].id === "field_1");

const healed = serializeNamedFields([
  { id: "field_1", label: "x", path: "", key: "x", dataType: "any" },
  { id: "field_1", label: "y", path: "", key: "y", dataType: "any" }, // duplicate
  { id: "has space", label: "z", path: "", key: "z", dataType: "any" }, // invalid pin id
] as never);
const healedIds = healed.map((e) => (e as { id: string }).id);
check(
  "duplicate and invalid ids are healed uniquely",
  healedIds[0] === "field_1" && healedIds[1] === "field_2" && healedIds[2] === "field_3" && new Set(healedIds).size === 3,
  `got ${healedIds.join(",")}`,
);

const legacyLabelDerived = parseNamedFields([{ id: "old_label_id", label: "renamed now", key: "k", dataType: "text" }]);
const legacyRoundTrip = parseNamedFields(serializeNamedFields(legacyLabelDerived));
check(
  "legacy label-derived ids are preserved (not regenerated)",
  legacyRoundTrip[0].id === "old_label_id" && legacyRoundTrip[0].label === "renamed now",
);

/* --- empty labels keep falling back to the id (backend mirrors this) --- */

const cleared = parseNamedFields(
  serializeNamedFields([{ id: "field_1", label: "", path: "", key: "k", dataType: "text" }]),
);
check("empty label falls back to id", cleared[0].label === "field_1");

/* --- string-typed configs (legacy JSON string storage) parse fine --- */

const stringForm = parseNamedFields(JSON.stringify(savedConfig));
check("JSON-string configs parse", stringForm.length === 2 && stringForm[0].id === "field_1");

if (failed > 0) {
  console.log(`\n${failed} failed`);
  process.exit(1);
}
console.log(`\n${passed} passed, ${failed} failed`);
