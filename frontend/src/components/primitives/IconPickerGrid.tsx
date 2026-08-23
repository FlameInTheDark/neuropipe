import { Icon } from "../icons";
import { cn } from "../../utils/cn";

const DEFAULT_CHOICES = [
  "Play", "Bot", "Database", "FileText", "Globe", "Radio",
  "Binary", "Cable", "Sparkles", "Zap", "Clock", "Braces",
  "Grid2x2", "LayoutGrid", "Activity", "HardDrive",
];

/** Grid of selectable icons, shared by the deck and key editors. */
export function IconPickerGrid({
  value,
  onChange,
  choices = DEFAULT_CHOICES,
  columns = 8,
}: {
  value: string;
  onChange: (icon: string) => void;
  choices?: readonly string[];
  columns?: number;
}) {
  return (
    <div
      className="grid gap-1.5 rounded-lg border border-ink-700 bg-ink-950/60 p-2"
      style={{ gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))` }}
    >
      {choices.map((ic) => (
        <button
          key={ic}
          type="button"
          onClick={() => onChange(ic)}
          className={cn(
            "grid aspect-square place-items-center rounded-md border transition",
            value === ic
              ? "border-ink-300 bg-ink-750 text-ink-50"
              : "border-transparent text-ink-400 hover:border-ink-600 hover:bg-ink-850 hover:text-ink-100",
          )}
        >
          <Icon name={ic} className="h-4 w-4" />
        </button>
      ))}
    </div>
  );
}
