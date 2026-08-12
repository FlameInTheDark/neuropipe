import type { ReactNode } from "react";

import { BlueprintPinRow } from "@/components/BlueprintPinRow";
import type { NodePort } from "@/lib/types";
import { cn } from "@/lib/utils";

export function BlueprintNodeCard({
  label,
  icon,
  summary,
  inputs,
  outputs,
  selected = false,
  tone = "border-zinc-700",
  status,
}: {
  label: string;
  icon?: ReactNode;
  summary?: ReactNode;
  inputs: NodePort[];
  outputs: NodePort[];
  selected?: boolean;
  tone?: string;
  status?: ReactNode;
}) {
  return (
    <div
      className={cn(
        "min-w-52 rounded-lg border bg-zinc-950 shadow-xl transition-colors",
        selected ? "border-zinc-300 ring-2 ring-white/10" : tone,
      )}
    >
      <div className="flex items-center gap-2 border-b border-zinc-800 px-3 py-2">
        {icon ? (
          <div className="flex size-5 shrink-0 items-center justify-center rounded bg-zinc-800">
            {icon}
          </div>
        ) : null}
        <span className="min-w-0 max-w-32 flex-1 truncate text-xs font-medium text-zinc-100">
          {label}
        </span>
        {status}
      </div>
      {summary ? (
        <div className="px-3 py-2 text-[11px] text-zinc-500">{summary}</div>
      ) : null}
      <div className="grid grid-cols-2 border-t border-zinc-800">
        <div className="border-r border-zinc-800 py-1">
          {inputs.map((input) => (
            <BlueprintPinRow key={input.id} pin={input} target />
          ))}
        </div>
        <div className="py-1">
          {outputs.map((output) => (
            <BlueprintPinRow key={output.id} pin={output} />
          ))}
        </div>
      </div>
    </div>
  );
}
