import type { ReactNode } from "react";
import { cn } from "../utils/cn";
import { Icon } from "./icons";
import { Tooltip } from "./Tooltip";

/* ---------- Panel shell ---------- */

export function Panel({ className, children }: { className?: string; children: ReactNode }) {
  return (
    <section className={cn("flex min-h-0 min-w-0 flex-col overflow-hidden bg-ink-900", className)}>
      {children}
    </section>
  );
}

export function PanelHeader({
  title,
  icon,
  right,
  className,
}: {
  title: string;
  icon?: string;
  right?: ReactNode;
  className?: string;
}) {
  return (
    <header
      className={cn(
        "flex h-9 shrink-0 items-center gap-2 border-b border-seam px-3 text-ink-200",
        className,
      )}
    >
      {icon && <Icon name={icon} className="h-3.5 w-3.5 text-ink-400" />}
      <span className="text-[11px] font-medium tracking-[0.08em] uppercase">{title}</span>
      <div className="ml-auto flex items-center gap-1">{right}</div>
    </header>
  );
}

/* ---------- Buttons ---------- */

export function IconButton({
  icon,
  label,
  active,
  onClick,
  size = "md",
  className,
}: {
  icon: string;
  label: string;
  active?: boolean;
  onClick?: () => void;
  size?: "sm" | "md";
  className?: string;
}) {
  return (
    <Tooltip content={label} side="bottom">
      <button
        aria-label={label}
        onClick={onClick}
        className={cn(
          "group relative grid place-items-center rounded-md text-ink-300 transition",
          "hover:bg-ink-700/70 hover:text-ink-50 active:scale-[0.94]",
          size === "sm" ? "h-6 w-6" : "h-7 w-7",
          active && "bg-ink-700 text-ink-50",
          className,
        )}
      >
        <Icon name={icon} className={size === "sm" ? "h-3.5 w-3.5" : "h-[15px] w-[15px]"} />
      </button>
    </Tooltip>
  );
}

export function Button({
  children,
  icon,
  variant = "ghost",
  onClick,
  className,
  shortcut,
  spin,
  disabled,
}: {
  children: ReactNode;
  icon?: string;
  variant?: "primary" | "solid" | "ghost";
  onClick?: () => void;
  className?: string;
  shortcut?: string;
  spin?: boolean;
  disabled?: boolean;
}) {
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      className={cn(
        "inline-flex h-7 shrink-0 items-center gap-1.5 whitespace-nowrap rounded-md px-2.5 text-[12.5px] font-medium transition active:scale-[0.97]",
        variant === "primary" &&
          "bg-ink-50 text-ink-950 shadow-[0_1px_0_0_rgba(255,255,255,0.4)_inset] hover:bg-white",
        variant === "solid" && "bg-ink-700 text-ink-50 hover:bg-ink-650",
        variant === "ghost" && "text-ink-200 hover:bg-ink-750 hover:text-ink-50",
        disabled && "cursor-not-allowed bg-ink-800 text-ink-500 hover:bg-ink-800 active:scale-100",
        className,
      )}
    >
      {icon && <Icon name={icon} className={cn("h-[14px] w-[14px]", spin && "animate-spin")} />}
      {children}
      {shortcut && (
        <kbd className="ml-1 rounded border border-ink-600 bg-ink-800/80 px-1 font-mono text-[10px] text-ink-400">
          {shortcut}
        </kbd>
      )}
    </button>
  );
}

/* ---------- Bits ---------- */

export function Badge({
  children,
  tone = "muted",
  className,
}: {
  children: ReactNode;
  tone?: "muted" | "ok" | "run" | "warn";
  className?: string;
}) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1 rounded px-1.5 py-[1px] font-mono text-[10px] tracking-tight",
        tone === "muted" && "bg-ink-750 text-ink-300",
        tone === "ok" && "bg-emerald-400/10 text-emerald-300/90",
        tone === "run" && "bg-ink-50/10 text-ink-50",
        tone === "warn" && "bg-amber-400/10 text-amber-300/90",
        className,
      )}
    >
      {children}
    </span>
  );
}

export function Dot({ tone = "idle", className }: { tone?: string; className?: string }) {
  return (
    <span
      className={cn(
        "h-1.5 w-1.5 shrink-0 rounded-full",
        tone === "done" && "bg-emerald-400",
        tone === "running" && "bg-ink-50 pulse-ring",
        tone === "queued" && "bg-amber-400/80",
        tone === "error" && "bg-rose-400",
        tone === "idle" && "bg-ink-500",
        className,
      )}
    />
  );
}

export function Toggle({ on, onChange, disabled }: { on: boolean; onChange: (v: boolean) => void; disabled?: boolean }) {
  return (
    <button
      onClick={() => !disabled && onChange(!on)}
      disabled={disabled}
      aria-pressed={on}
      className={cn(
        "relative h-[18px] w-[32px] shrink-0 rounded-full transition-colors",
        on ? "bg-ink-50" : "bg-ink-600",
        disabled && "cursor-not-allowed opacity-40",
      )}
    >
      <span
        className={cn(
          "absolute top-[2px] h-[14px] w-[14px] rounded-full transition-all",
          on ? "left-[16px] bg-ink-950" : "left-[2px] bg-ink-300",
        )}
      />
    </button>
  );
}

export function Divider({ className }: { className?: string }) {
  return <div className={cn("h-4 w-px bg-ink-700", className)} />;
}

export function Empty({ icon, text }: { icon: string; text: string }) {
  return (
    <div className="flex flex-col items-center gap-2 px-6 py-10 text-center">
      <Icon name={icon} className="h-5 w-5 text-ink-500" />
      <p className="text-[12px] leading-relaxed text-ink-400">{text}</p>
    </div>
  );
}

