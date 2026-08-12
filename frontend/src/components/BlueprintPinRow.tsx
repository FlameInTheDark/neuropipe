import { Handle, Position } from "@xyflow/react";

import { BlueprintPinTooltip } from "@/components/BlueprintPinTooltip";
import { nodePinColor } from "@/lib/node-pins";
import type { NodePort } from "@/lib/types";
import { cn } from "@/lib/utils";

export function BlueprintPinRow({
  pin,
  target = false,
}: {
  pin: NodePort;
  target?: boolean;
}) {
  const exec = pin.kind === "exec";

  return (
    <div
      className={cn(
        "relative flex min-h-6 items-center px-3 text-[10px]",
        target ? "justify-start text-zinc-400" : "justify-end text-zinc-400",
      )}
    >
      <Handle
        id={pin.id}
        type={target ? "target" : "source"}
        position={target ? Position.Left : Position.Right}
        className={exec ? "!h-3 !w-3 !rounded-sm" : "!size-2.5"}
        style={{
          background: nodePinColor(pin),
          [target ? "left" : "right"]: 0,
        }}
      />
      <BlueprintPinTooltip pin={pin} target={target} />
    </div>
  );
}
