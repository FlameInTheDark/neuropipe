import type { ReactNode } from "react";
import { cn } from "../utils/cn";
import { Icon } from "./icons";

export function ViewShell({
  title,
  subtitle,
  actions,
  toolbar,
  children,
  padded = true,
}: {
  title: string;
  subtitle?: string;
  actions?: ReactNode;
  toolbar?: ReactNode;
  children: ReactNode;
  padded?: boolean;
}) {
  return (
    <section className="fade-in flex h-full flex-col overflow-hidden">
      <header className="flex h-11 shrink-0 items-center gap-2.5 border-b border-seam px-4">
        <h1 className="text-[13.5px] font-semibold tracking-tight text-fg">{title}</h1>
        {subtitle && (
          <>
            <span className="h-3 w-px bg-ink-700" />
            <span className="truncate text-[11.5px] text-fg-faint">{subtitle}</span>
          </>
        )}
        <div className="ml-auto flex shrink-0 items-center gap-1.5">{actions}</div>
      </header>
      {toolbar && (
        <div className="flex h-11 shrink-0 items-center gap-2 border-b border-seam px-4">{toolbar}</div>
      )}
      <div className={cn("min-h-0 flex-1 overflow-y-auto", padded && "p-4")}>{children}</div>
    </section>
  );
}

export function SearchInput({
  value,
  onChange,
  placeholder,
  className,
}: {
  value: string;
  onChange: (v: string) => void;
  placeholder: string;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "flex h-8 items-center gap-2 rounded-md border border-ink-700 bg-ink-850 px-2.5 transition focus-within:border-ink-500",
        className,
      )}
    >
      <Icon name="Search" className="h-3.5 w-3.5 shrink-0 text-fg-faint" />
      <input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className="min-w-0 flex-1 bg-transparent text-[12.5px] text-fg placeholder:text-fg-faint"
      />
      {value && (
        <button onClick={() => onChange("")} className="text-fg-faint hover:text-fg-muted">
          <Icon name="X" className="h-3.5 w-3.5" />
        </button>
      )}
    </div>
  );
}

export function Card({
  className,
  children,
  onClick,
  onContextMenu,
  hoverable,
}: {
  className?: string;
  children: ReactNode;
  onClick?: () => void;
  onContextMenu?: (e: React.MouseEvent) => void;
  hoverable?: boolean;
}) {
  const Tag = onClick || onContextMenu ? "button" : "div";
  return (
    <Tag
      onClick={onClick}
      onContextMenu={onContextMenu}
      className={cn(
        "rounded-xl border border-ink-700/80 bg-ink-850/60 text-left transition",
        (hoverable || onClick || onContextMenu) && "hover:border-ink-500 hover:bg-ink-800/70",
        className,
      )}
    >
      {children}
    </Tag>
  );
}

export function SectionTitle({ children, right }: { children: ReactNode; right?: ReactNode }) {
  return (
    <div className="mb-2 flex items-center gap-2">
      <h2 className="text-[10.5px] font-medium tracking-[0.09em] text-fg-subtle uppercase">{children}</h2>
      <div className="ml-auto">{right}</div>
    </div>
  );
}

export function StatusPill({ status }: { status: string }) {
  const map: Record<string, string> = {
    completed: "bg-success/10 text-success-fg/90",
    published: "bg-success/10 text-success-fg/90",
    connected: "bg-success/10 text-success-fg/90",
    running: "bg-ink-50/12 text-fg",
    queued: "bg-warning/10 text-warning-fg/90",
    draft: "bg-ink-750 text-fg-subtle",
    idle: "bg-ink-750 text-fg-subtle",
    failed: "bg-danger/10 text-danger-fg/90",
    error: "bg-danger/10 text-danger-fg/90",
  };
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 rounded px-1.5 py-[2px] font-mono text-[10px] tracking-tight",
        map[status] ?? "bg-ink-750 text-fg-subtle",
      )}
    >
      {status === "running" && <span className="h-1.5 w-1.5 rounded-full bg-ink-50 pulse-ring" />}
      {status}
    </span>
  );
}

export function EmptyState({ icon, title, hint }: { icon: string; title: string; hint?: string }) {
  return (
    <div className="flex flex-col items-center justify-center gap-2 px-6 py-16 text-center">
      <span className="grid h-10 w-10 place-items-center rounded-xl border border-ink-700 bg-ink-850 text-fg-subtle">
        <Icon name={icon} className="h-4 w-4" />
      </span>
      <p className="text-[13px] font-medium text-fg">{title}</p>
      {hint && <p className="max-w-[280px] text-[11.5px] leading-relaxed text-fg-faint">{hint}</p>}
    </div>
  );
}
