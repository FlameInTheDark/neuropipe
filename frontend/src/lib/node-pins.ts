import type { DataType, NodePort } from "@/lib/types";

// Pin colours are a graph-wide visual language: data values use their type,
// while white pins and wires always denote Blueprint execution flow.
export function dataPinColor(dataType?: DataType): string {
  switch (dataType) {
    case "text":
      return "#e879f9";
    case "number":
      return "#86efac";
    case "boolean":
      return "#f87171";
    case "object":
      return "#60a5fa";
    case "list":
      return "#facc15";
    default:
      return "#a1a1aa";
  }
}

export function nodePinColor(
  pin: Pick<NodePort, "kind" | "dataType" | "color">,
): string {
  return pin.kind === "exec" ? "#fafafa" : dataPinColor(pin.dataType);
}
