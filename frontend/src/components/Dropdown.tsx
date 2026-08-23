import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { useTranslation } from "react-i18next";
import { Icon } from "./icons";
import { cn } from "../utils/cn";

export interface DropdownOption {
  value: string;
  label: string;
  icon?: string;
  hint?: string;
}

export function Dropdown({
  value,
  options,
  onChange,
  placeholder,
  className,
  compact,
}: {
  value: string;
  options: DropdownOption[];
  onChange: (v: string) => void;
  placeholder?: string;
  className?: string;
  compact?: boolean;
}) {
  const { t } = useTranslation();
  const btnRef = useRef<HTMLButtonElement>(null);
  const resolvedPlaceholder = placeholder ?? t("common.select");
  const [open, setOpen] = useState(false);
  const current = options.find((o) => o.value === value);

  return (
    <>
      <button
        ref={btnRef}
        type="button"
        aria-haspopup="listbox"
        aria-expanded={open}
        onClick={() => setOpen((o) => !o)}
        onKeyDown={(e) => {
          if (e.key === "ArrowDown" || e.key === "ArrowUp") {
            e.preventDefault();
            setOpen(true);
          }
        }}
        className={cn(
          "flex items-center gap-2 rounded-md border border-ink-700 bg-ink-850 text-left transition",
          compact ? "h-[26px] px-2 text-[11px]" : "h-8 w-full px-2.5 text-[12.5px]",
          open ? "border-ink-500 bg-ink-800" : "hover:border-ink-600 hover:bg-ink-800",
          className,
        )}
      >
        {current?.icon && <Icon name={current.icon} className="h-3.5 w-3.5 shrink-0 text-ink-400" />}
        <span className={cn("min-w-0 flex-1 truncate", current ? "text-ink-50" : "text-ink-500")}>
          {current?.label ?? resolvedPlaceholder}
        </span>
        <Icon
          name="ChevronDown"
          className={cn("h-3.5 w-3.5 shrink-0 text-ink-500 transition-transform duration-150", open && "rotate-180")}
        />
      </button>
      {open && (
        <Menu
          anchorRef={btnRef}
          options={options}
          value={value}
          onPick={(v) => {
            onChange(v);
            setOpen(false);
            btnRef.current?.focus();
          }}
          onClose={() => setOpen(false)}
        />
      )}
    </>
  );
}

function Menu({
  anchorRef,
  options,
  value,
  onPick,
  onClose,
}: {
  anchorRef: React.RefObject<HTMLButtonElement | null>;
  options: DropdownOption[];
  value: string;
  onPick: (v: string) => void;
  onClose: () => void;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const [pos, setPos] = useState<{ left: number; top: number; width: number } | null>(null);
  const [hi, setHi] = useState(() => Math.max(0, options.findIndex((o) => o.value === value)));

  useLayoutEffect(() => {
    const anchor = anchorRef.current;
    const el = ref.current;
    if (!anchor || !el) return;
    const a = anchor.getBoundingClientRect();
    const m = el.getBoundingClientRect();
    let top = a.bottom + 5;
    if (top + m.height > window.innerHeight - 8) top = Math.max(8, a.top - m.height - 5);
    const left = Math.min(Math.max(6, a.left), window.innerWidth - Math.max(m.width, a.width) - 6);
    setPos({ left, top, width: a.width });
  }, [anchorRef]);

  useEffect(() => {
    ref.current?.focus();
    const onDown = (e: PointerEvent) => {
      const t = e.target as Node;
      if (ref.current?.contains(t) || anchorRef.current?.contains(t)) return;
      onClose();
    };
    const onWheel = (e: WheelEvent) => {
      if (!ref.current?.contains(e.target as Node)) onClose();
    };
    window.addEventListener("pointerdown", onDown);
    window.addEventListener("wheel", onWheel, { passive: true });
    window.addEventListener("resize", onClose);
    return () => {
      window.removeEventListener("pointerdown", onDown);
      window.removeEventListener("wheel", onWheel);
      window.removeEventListener("resize", onClose);
    };
  }, [anchorRef, onClose]);

  return createPortal(
    <div
      ref={ref}
      role="listbox"
      tabIndex={-1}
      style={pos ? { left: pos.left, top: pos.top, minWidth: pos.width } : { left: -9999, top: -9999 }}
      onKeyDown={(e) => {
        if (e.key === "Escape") onClose();
        else if (e.key === "ArrowDown") {
          e.preventDefault();
          setHi((h) => Math.min(h + 1, options.length - 1));
        } else if (e.key === "ArrowUp") {
          e.preventDefault();
          setHi((h) => Math.max(h - 1, 0));
        } else if (e.key === "Enter") {
          e.preventDefault();
          const o = options[hi];
          if (o) onPick(o.value);
        }
      }}
      className="timeline-menu fixed z-[90] max-h-[264px] overflow-y-auto rounded-[9px] border border-ink-650 bg-ink-850/95 p-1 shadow-[0_18px_44px_-12px_rgba(0,0,0,0.95),0_0_0_1px_rgba(255,255,255,0.02)_inset] outline-none backdrop-blur-xl"
    >
      {options.map((o, i) => (
        <button
          key={o.value}
          role="option"
          aria-selected={o.value === value}
          onMouseEnter={() => setHi(i)}
          onClick={() => onPick(o.value)}
          className={cn(
            "flex h-7 w-full items-center gap-2.5 rounded-md px-2 text-left text-[12.5px] transition-colors",
            i === hi ? "bg-ink-650/80 text-ink-50" : "text-ink-100",
          )}
        >
          {o.icon ? (
            <Icon name={o.icon} className="h-[14px] w-[14px] shrink-0 text-ink-400" />
          ) : null}
          <span className="min-w-0 truncate">{o.label}</span>
          {o.hint && <span className="ml-auto shrink-0 font-mono text-[10px] text-ink-500">{o.hint}</span>}
          {o.value === value && !o.hint && (
            <Icon name="Check" className="ml-auto h-3.5 w-3.5 shrink-0 text-ink-200" />
          )}
        </button>
      ))}
    </div>,
    document.body,
  );
}


