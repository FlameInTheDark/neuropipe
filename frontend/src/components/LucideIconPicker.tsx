import {
  useDeferredValue,
  useEffect,
  useMemo,
  useState,
  type ComponentType,
} from "react";
import dynamicIconImports from "lucide-react/dynamicIconImports";
import { Check, ChevronDown, Palette, Search, Workflow } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Tooltip } from "@/components/ui/tooltip";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { cn } from "@/lib/utils";

type IconComponent = ComponentType<{ className?: string; strokeWidth?: number }>;

const maxVisibleIcons = 120;
const iconColorChoices = ["#f4f4f5", "#18181b", "#a78bfa", "#60a5fa", "#34d399", "#fbbf24", "#fb7185"];
const iconBackgroundChoices = ["#27272a", "#ffffff", "#2e1065", "#172554", "#052e16", "#422006", "#4c0519"];

function formatIconName(name: string) {
  return name
    .split("-")
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join(" ");
}

/** Renders a named Lucide icon while keeping the rest of the icon set lazy. */
export function LucideIcon({
  name,
  className,
  strokeWidth,
}: {
  name?: string;
  className?: string;
  strokeWidth?: number;
}) {
  const [Icon, setIcon] = useState<IconComponent>(
    () => Workflow as unknown as IconComponent,
  );

  useEffect(() => {
    const loader = name
      ? dynamicIconImports[name as keyof typeof dynamicIconImports]
      : undefined;
    if (!loader) {
      setIcon(() => Workflow as unknown as IconComponent);
      return;
    }
    let active = true;
    void loader()
      .then((module) => {
        if (active) setIcon(() => module.default as unknown as IconComponent);
      })
      .catch(() => {
        if (active) setIcon(() => Workflow as unknown as IconComponent);
      });
    return () => {
      active = false;
    };
  }, [name]);

  return <Icon className={className} strokeWidth={strokeWidth} />;
}

/** Searchable picker backed by every icon exported by Lucide. */
export function LucideIconPicker({
  value,
  onValueChange,
  label = "Pipeline icon",
  disabled = false,
  className,
  iconColor,
  iconBackground,
}: {
  value?: string;
  onValueChange: (value: string) => void;
  label?: string;
  disabled?: boolean;
  className?: string;
  iconColor?: string;
  iconBackground?: string;
}) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const deferredQuery = useDeferredValue(query.trim().toLowerCase());
  const icons = useMemo(
    () =>
      Object.keys(dynamicIconImports)
        .filter((name) => name.includes(deferredQuery))
        .sort()
        .slice(0, maxVisibleIcons),
    [deferredQuery],
  );
  const selected = value || "workflow";

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <Tooltip content={label} side="bottom">
        <PopoverTrigger asChild>
          <Button
            type="button"
            variant="outline"
            size="sm"
            aria-label={label}
            disabled={disabled}
            className={cn("gap-1.5 px-2", className)}
            style={{ color: iconColor, backgroundColor: iconBackground }}
          >
            <LucideIcon name={selected} className="size-3.5" />
            <ChevronDown className="size-3 text-zinc-500" />
          </Button>
        </PopoverTrigger>
      </Tooltip>
      <PopoverContent
        align="start"
        sideOffset={6}
        className="w-[22rem] p-3"
        onOpenAutoFocus={(event) => event.preventDefault()}
      >
        <div className="relative">
          <Search className="pointer-events-none absolute left-2.5 top-2.5 size-3.5 text-zinc-600" />
          <Input
            autoFocus
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            className="h-8 pl-8 text-xs"
            placeholder="Search all Lucide icons"
          />
        </div>
        <p className="mt-2 text-[10px] leading-4 text-zinc-600">
          {deferredQuery
            ? `${icons.length} matching icons`
            : `Showing ${Math.min(icons.length, maxVisibleIcons)} icons — search to find any Lucide icon.`}
        </p>
        <div className="muted-scroll mt-2 grid max-h-72 grid-cols-[repeat(7,minmax(0,1fr))] gap-1 overflow-x-hidden overflow-y-auto pr-1">
          {icons.map((icon) => {
            const active = icon === selected;
            return (
              <Tooltip key={icon} content={formatIconName(icon)} side="top">
                <button
                  type="button"
                  aria-label={`Use ${formatIconName(icon)} icon`}
                  aria-pressed={active}
                  onClick={() => {
                    onValueChange(icon);
                    setOpen(false);
                  }}
                  className={cn(
                    "relative flex size-10 items-center justify-center rounded-md transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/50",
                    active
                      ? "bg-white text-zinc-950"
                      : "bg-zinc-900 text-zinc-400 hover:bg-zinc-800 hover:text-zinc-100",
                  )}
                >
                  <LucideIcon name={icon} className="size-4 shrink-0" strokeWidth={1.8} />
                  {active ? <Check className="absolute right-1 top-1 size-2.5" /> : null}
                </button>
              </Tooltip>
            );
          })}
        </div>
        {icons.length === 0 ? (
          <p className="py-6 text-center text-xs text-zinc-500">
            No Lucide icon matches “{query}”.
          </p>
        ) : null}
      </PopoverContent>
    </Popover>
  );
}

function ColorChoices({
  label,
  value,
  values,
  onChange,
}: {
  label: string;
  value?: string;
  values: string[];
  onChange: (value: string) => void;
}) {
  const selected = value || values[0];
  const [draft, setDraft] = useState(selected);
  useEffect(() => setDraft(selected), [selected]);
  const commit = () => {
    const next = draft.trim() || values[0];
    setDraft(next);
    if (next !== selected) onChange(next);
  };
  return (
    <section>
      <p className="mb-1.5 text-[10px] font-semibold uppercase tracking-[.12em] text-zinc-600">{label}</p>
      <div className="flex flex-wrap gap-1.5">
        {values.map((color) => <Tooltip key={color} content={color} side="top"><button type="button" aria-label={`${label}: ${color}`} aria-pressed={selected.toLowerCase() === color.toLowerCase()} onClick={() => onChange(color)} className={cn("flex size-7 items-center justify-center rounded-md border transition-transform hover:scale-105 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/50", selected.toLowerCase() === color.toLowerCase() ? "border-white" : "border-zinc-700")} style={{ backgroundColor: color }}>
          {selected.toLowerCase() === color.toLowerCase() ? <Check className={cn("size-3", color === "#ffffff" || color === "#f4f4f5" ? "text-zinc-900" : "text-white")} /> : null}
        </button></Tooltip>)}
      </div>
      <Input value={draft} onChange={(event) => setDraft(event.target.value)} onBlur={commit} onKeyDown={(event) => { if (event.key === "Enter") { event.preventDefault(); event.currentTarget.blur(); } }} className="mt-2 h-8 font-mono text-[11px]" placeholder={values[0]} aria-label={`${label} value`} />
    </section>
  );
}

/** Palette control for the foreground and backing surface of a workspace icon. */
export function IconAppearancePicker({
  iconColor,
  iconBackground,
  onIconColorChange,
  onIconBackgroundChange,
  label = "Icon appearance",
  disabled = false,
}: {
  iconColor?: string;
  iconBackground?: string;
  onIconColorChange: (value: string) => void;
  onIconBackgroundChange: (value: string) => void;
  label?: string;
  disabled?: boolean;
}) {
  return (
    <Popover>
      <Tooltip content={label} side="bottom">
        <PopoverTrigger asChild>
          <Button type="button" variant="outline" size="sm" aria-label={label} disabled={disabled} className="px-2" style={{ color: iconColor || "#f4f4f5", backgroundColor: iconBackground || "#27272a" }}>
            <Palette className="size-3.5" />
          </Button>
        </PopoverTrigger>
      </Tooltip>
      <PopoverContent align="start" sideOffset={6} className="w-64 space-y-4 p-3">
        <ColorChoices label="Icon color" value={iconColor} values={iconColorChoices} onChange={onIconColorChange} />
        <ColorChoices label="Background" value={iconBackground} values={iconBackgroundChoices} onChange={onIconBackgroundChange} />
      </PopoverContent>
    </Popover>
  );
}
