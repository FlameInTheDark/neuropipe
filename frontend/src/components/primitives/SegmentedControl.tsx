import { Icon } from "../icons";
import { cn } from "../../utils/cn";
import { control } from "./styles";

export interface Segment<T extends string> {
  value: T;
  label: string;
  icon?: string;
}

/**
 * Tab-strip / filter toggle.
 * Replaces the six hand-rolled copies of this markup across the views.
 */
export function SegmentedControl<T extends string>({
  value,
  segments,
  onChange,
  size = "md",
  className,
}: {
  value: T;
  segments: readonly Segment<T>[];
  onChange: (v: T) => void;
  size?: "sm" | "md";
  className?: string;
}) {
  return (
    <div className={cn(control.segment, className)}>
      {segments.map((s) => {
        const active = s.value === value;
        return (
          <button
            key={s.value}
            onClick={() => onChange(s.value)}
            className={cn(
              "flex shrink-0 items-center gap-1.5 rounded transition",
              size === "sm" ? "h-[22px] px-2 text-[11px]" : "h-6 px-2.5 text-[11.5px]",
              active ? "bg-ink-700 text-fg" : "text-fg-subtle hover:text-fg",
            )}
          >
            {s.icon && <Icon name={s.icon} className="h-3 w-3 shrink-0" />}
            {s.label}
          </button>
        );
      })}
    </div>
  );
}
