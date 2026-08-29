/* Unit tests for the bytes pin data-type: the label must survive every
   backend <-> canvas mapping (adapters, type-spec, TypeSpecField tokens,
   the palette), the dynamic-pin twins must mark bytes pins as "bytes", and
   the JS node contract resolver must surface bytes pins as bytes.
   Run: npx tsx scripts/test-bytes-pin-type.mts */

import "./dom-stub.mts";
import {
  mapDataType,
  mapSpecToPin,
  unmapDataType,
  portFromNodePort,
} from "../src/lib/adapters";
import { typeSpecFromDataType, isTypeAssignable } from "../src/lib/type-spec";
import { dataPinColor } from "../src/lib/node-pins";
import { pinPalette, pinColor, ASSIGNABLE_PIN_TYPES, portKindFromDataType } from "../src/lib/pins";
import { tokenToPinDataType } from "../src/components/TypeSpecField";
import { resolveConfigDrivenOutputs } from "../src/lib/blueprint-dynamic-pins";
import { resolveJavaScriptInputs } from "../src/lib/javascript-node";
import type { NodeDefinition, NodePort, Port } from "../src/lib/types";
import type { PinDataType } from "../src/types";

let failed = 0;
let passed = 0;
function check(name: string, ok: boolean, detail?: string) {
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${ok || !detail ? "" : ` — ${detail}`}`);
  if (ok) passed++;
  else failed++;
}

/* ------------------------------------------------------------------ */
/* label round-trips                                                   */
/* ------------------------------------------------------------------ */

check(
  'mapDataType("bytes") stays "bytes"',
  mapDataType("bytes") === "bytes",
);
check(
  'unmapDataType("bytes") stays "bytes"',
  unmapDataType("bytes") === "bytes",
);
check(
  'mapSpecToPin({kind:"bytes"}) is "bytes"',
  mapSpecToPin({ kind: "bytes" }) === "bytes",
);
check(
  'typeSpecFromDataType("bytes") is the bytes contract',
  typeSpecFromDataType("bytes").kind === "bytes",
);
check(
  'tokenToPinDataType("bytes") is "bytes"',
  tokenToPinDataType("bytes") === "bytes",
);
check(
  "bytes is assignable to bytes and any, never text",
  isTypeAssignable({ kind: "bytes" }, { kind: "bytes" })
    && isTypeAssignable({ kind: "bytes" }, { kind: "any" })
    && !isTypeAssignable({ kind: "bytes" }, { kind: "string" })
    && !isTypeAssignable({ kind: "string" }, { kind: "bytes" })
    && !isTypeAssignable({ kind: "any" }, { kind: "bytes" }),
);

/* ------------------------------------------------------------------ */
/* palette                                                             */
/* ------------------------------------------------------------------ */

const pal = pinPalette("bytes");
check(
  "palette resolves the bytes entry with a name",
  pal.name === "Bytes" && pal.dot.includes("--pin-bytes"),
);
check(
  "pinColor falls back gracefully for unknown labels",
  pinColor("bytes") === pal.dot && pinColor("nonsense") === pinPalette("any").dot,
);
check(
  "bytes is an assignable (selectable) pin type",
  ASSIGNABLE_PIN_TYPES.includes("bytes"),
);
check(
  "bytes remains a data port kind",
  portKindFromDataType("bytes") === "data",
);
check(
  "backend mirror colour for bytes is amber",
  dataPinColor("bytes") === "#fbbf24",
);

/* ------------------------------------------------------------------ */
/* port mapping                                                        */
/* ------------------------------------------------------------------ */

const bytesPort: NodePort = {
  id: "data",
  label: "Data",
  kind: "data",
  direction: "input",
  dataType: "bytes",
  type: { kind: "bytes" },
  color: "#fbbf24",
  required: false,
  maxConnections: 1,
};
const port: Port = portFromNodePort(bytesPort);
check(
  "backend bytes NodePort maps to a bytes canvas port",
  port.dataType === "bytes" && port.spec?.kind === "bytes",
);

const legacyPort = portFromNodePort({ ...bytesPort, dataType: "any", type: { kind: "any" } });
check(
  "legacy any pins still map to any (no accidental narrowing)",
  legacyPort.dataType === "any",
);

/* ------------------------------------------------------------------ */
/* dynamic pin twins                                                   */
/* ------------------------------------------------------------------ */

const textBytesDefinition: NodeDefinition = {
  type: "action:file_read",
  label: "Read File",
  category: "Local",
  inputs: [],
  outputs: [
    { id: "result", label: "Result", kind: "data", direction: "output", dataType: "text", type: { kind: "string" } },
  ],
  fields: [],
  defaultConfig: { outputType: "text" },
};

const bytesOutputs = resolveConfigDrivenOutputs(textBytesDefinition, { outputType: "bytes" });
const resolvedPin = bytesOutputs.find((p) => p.id === "result");
check(
  "file_read outputType=bytes resolves a bytes pin",
  resolvedPin?.dataType === "bytes" && resolvedPin?.type?.kind === "bytes",
);

const textOutputs = resolveConfigDrivenOutputs(textBytesDefinition, { outputType: "text" });
const textPin = textOutputs.find((p) => p.id === "result");
check(
  "file_read outputType=text resolves a text pin",
  textPin?.dataType === "text" && textPin?.type?.kind === "string",
);

/* ------------------------------------------------------------------ */
/* JavaScript node contracts                                           */
/* ------------------------------------------------------------------ */

const jsDefinition: NodeDefinition = {
  type: "action:javascript",
  label: "JavaScript",
  category: "Code",
  inputs: [{ id: "code", label: "Code", kind: "data", direction: "input", dataType: "text", type: { kind: "string" } }],
  outputs: [],
  fields: [],
  defaultConfig: {},
};
const jsInputs = resolveJavaScriptInputs(jsDefinition, {
  inputs: [{ id: "picture", label: "Picture", type: { kind: "bytes" }, required: true }],
});
const jsPin = jsInputs.find((p) => p.id === "picture");
check(
  "JS node bytes contract resolves as a bytes pin",
  jsPin?.dataType === "bytes" && jsPin?.type?.kind === "bytes",
);

/* ------------------------------------------------------------------ */
/* coarse canvas compatibility mirror                                  */
/* ------------------------------------------------------------------ */

function typesCompatible(source?: Port, target?: Port): boolean {
  if (!source || !target) return false;
  if (source.kind !== target.kind) return false;
  if (source.kind === "exec") return true;
  if (source.spec && target.spec) return isTypeAssignable(source.spec, target.spec);
  const t = (target.dataType ?? "any") as PinDataType;
  if (t === "any") return true;
  const s = (source.dataType ?? "any") as PinDataType;
  return s === t;
}

const src = (spec?: { kind: string }, dataType?: PinDataType): Port => ({
  id: "v", label: "V", kind: "data", dataType, spec: spec as never,
});
check(
  "canvas mirror: bytes wires to bytes, not to text or any",
  typesCompatible(src({ kind: "bytes" }, "bytes"), src({ kind: "bytes" }, "bytes"))
    && !typesCompatible(src({ kind: "string" }, "text"), src({ kind: "bytes" }, "bytes"))
    && typesCompatible(src({ kind: "bytes" }, "bytes"), src({ kind: "any" }, "any"))
    && !typesCompatible(src({ kind: "any" }, "any"), src({ kind: "bytes" }, "bytes")),
);

/* ------------------------------------------------------------------ */

console.log(`\n${passed} passed, ${failed} failed`);
process.exit(failed ? 1 : 0);
