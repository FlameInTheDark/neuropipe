/* Pure serialization helpers for the named typed-fields editors
   (Build Object's "object-fields" and Break Object's "field-outputs").

   Extracted from Inspector.tsx so the identity rules are unit-testable
   without dragging the whole inspector component tree into tests. */

export interface NamedFieldEntry {
  id: string;
  label: string;
  path: string;
  key: string;
  dataType: string;
}

/** Parses the persisted fields config (array of {id,label,path,key,dataType}). */
export function parseNamedFields(raw: unknown): NamedFieldEntry[] {
  let list: unknown = raw;
  if (typeof raw === "string") {
    try { list = JSON.parse(raw); } catch { return []; }
  }
  if (!Array.isArray(list)) return [];
  return list
    .filter((p): p is Record<string, unknown> => typeof p === "object" && p !== null)
    .map((p, i) => ({
      id: typeof p.id === "string" ? p.id : `field_${i + 1}`,
      label: typeof p.label === "string" ? p.label : "",
      path: typeof p.path === "string" ? p.path : "",
      key: typeof p.key === "string" ? p.key : "",
      dataType: typeof p.dataType === "string" ? p.dataType : "any",
    }));
}

const FIELD_ID_PATTERN = /^[A-Za-z_][A-Za-z0-9_]*$/;

export function validFieldID(id: string): boolean {
  return FIELD_ID_PATTERN.test(id);
}

/** Mints a collision-free `field_N` id for a new row. */
export function uniqueFieldID(used: ReadonlySet<string>): string {
  for (let index = 1; ; index += 1) {
    const id = `field_${index}`;
    if (!used.has(id)) return id;
  }
}

/* Pin identity is minted once when a field is added and then stays frozen:
   re-deriving the id from the label would churn the React key (the row
   remounts and the text input loses focus after every keystroke) and rename
   the dynamic pin behind the user's back (dropping its wires on the canvas). */
export function serializeNamedFields(entries: NamedFieldEntry[]): unknown[] {
  const used = new Set<string>();
  return entries.map((e) => {
    let id = e.id.trim();
    if (!id || !validFieldID(id) || used.has(id)) id = uniqueFieldID(used);
    used.add(id);
    return {
      id,
      label: e.label.trim() || id,
      ...(e.path ? { path: e.path } : {}),
      ...(e.key ? { key: e.key } : {}),
      dataType: e.dataType,
    };
  });
}
