import type { DataType, TypeFieldSpec, TypeSpec } from "@/lib/types";

/** The V3 contract equivalent of retained V2 display data types. */
export function typeSpecFromDataType(dataType: DataType): TypeSpec {
  switch (dataType) {
    case "text": return { kind: "string" };
    case "number": return { kind: "float" };
    case "boolean": return { kind: "bool" };
    case "object": return { kind: "record" };
    case "list": return { kind: "list", element: { kind: "any" } };
    default: return { kind: "any" };
  }
}

/** Go-style assignment for Blueprint V3: no implicit conversion or any narrowing. */
export function isTypeAssignable(source?: TypeSpec, target?: TypeSpec): boolean {
  if (!source || !target) return false;
  if (target.kind === "any") return true;
  if (source.kind === "any" || source.kind !== target.kind) return false;
  if (target.kind === "list") return invariant(source.element, target.element);
  if (target.kind === "map") return invariant(source.key, target.key) && invariant(source.value, target.value);
  if (target.kind === "record") {
    if (target.name) return source.name === target.name;
    return (target.fields ?? []).every((field) => {
      const actual = findField(source.fields, field.name);
      return field.optional ? !actual || isTypeAssignable(actual.type, field.type) : !!actual && isTypeAssignable(actual.type, field.type);
    });
  }
  return true;
}

function invariant(source?: TypeSpec, target?: TypeSpec) {
  return !!source && !!target && isTypeAssignable(source, target) && isTypeAssignable(target, source);
}
function findField(fields: TypeFieldSpec[] | undefined, name: string) {
  return fields?.find((field) => field.name === name || field.id === name);
}
