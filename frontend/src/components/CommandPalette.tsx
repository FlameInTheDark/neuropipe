import { useEffect, useMemo, useRef, useState } from "react";
import { Icon } from "./icons";
import { useTranslation } from "react-i18next";
import { cn } from "../utils/cn";

export interface Command {
  id: string;
  label: string;
  icon: string;
  group: string;
  hint?: string;
  run: () => void;
}

export function CommandPalette({
  open,
  commands,
  onClose,
}: {
  open: boolean;
  commands: Command[];
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const [q, setQ] = useState(``);
  const [i, setI] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLUListElement>(null);

  useEffect(() => {
    if (open) {
      setQ("");
      setI(0);
      requestAnimationFrame(() => inputRef.current?.focus());
    }
  }, [open]);

  /* keep the keyboard-highlighted command inside the scrolled list */
  useEffect(() => {
    const el = listRef.current?.children[i] as HTMLElement | undefined;
    el?.scrollIntoView({ block: "nearest" });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [i, open]);

  const results = useMemo(() => {
    const s = q.trim().toLowerCase();
    return commands.filter((c) => !s || c.label.toLowerCase().includes(s) || c.group.toLowerCase().includes(s));
  }, [q, commands]);

  if (!open) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-start justify-center bg-black/60 pt-[14vh] backdrop-blur-[2px]"
      onClick={onClose}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        onKeyDown={(e) => {
          if (e.key === "ArrowDown") {
            e.preventDefault();
            setI((v) => Math.min(v + 1, results.length - 1));
          } else if (e.key === "ArrowUp") {
            e.preventDefault();
            setI((v) => Math.max(v - 1, 0));
          } else if (e.key === "Enter") {
            results[i]?.run();
            onClose();
          }
        }}
        className="pop-in w-[540px] max-w-[92vw] overflow-hidden rounded-xl border border-ink-650 bg-ink-900 shadow-[0_30px_80px_-20px_rgba(0,0,0,0.9)]"
      >
        <div className="flex h-11 items-center gap-2.5 border-b border-seam px-3.5">
          <Icon name="Search" className="h-4 w-4 text-ink-500" />
          <input
            ref={inputRef}
            value={q}
            onChange={(e) => {
              setQ(e.target.value);
              setI(0);
            }}
            placeholder={t("palette.search")}
            className="flex-1 bg-transparent text-[13.5px] text-ink-50 placeholder:text-ink-500"
          />
          <kbd className="rounded border border-ink-700 bg-ink-850 px-1.5 py-0.5 font-mono text-[10px] text-ink-500">
            esc
          </kbd>
        </div>
        <ul ref={listRef} className="max-h-[320px] overflow-y-auto p-1.5">
          {results.length === 0 && (
            <li className="px-3 py-6 text-center text-[12.5px] text-ink-500">{t("palette.noMatches")}</li>
          )}
          {results.map((c, idx) => (
            <li key={c.id}>
              <button
                onMouseEnter={() => setI(idx)}
                onClick={() => {
                  c.run();
                  onClose();
                }}
                className={cn(
                  "flex w-full items-center gap-2.5 rounded-lg px-2.5 py-2 text-left transition",
                  idx === i ? "bg-ink-750 text-ink-50" : "text-ink-200 hover:bg-ink-850",
                )}
              >
                <span className="grid h-6 w-6 place-items-center rounded-md border border-ink-700 bg-ink-850">
                  <Icon name={c.icon} className="h-3.5 w-3.5" />
                </span>
                <span className="text-[13px]">{c.label}</span>
                <span className="ml-auto text-[11px] text-ink-500">{c.hint ?? c.group}</span>
              </button>
            </li>
          ))}
        </ul>
        <div className="flex items-center gap-3 border-t border-seam px-3 py-1.5 text-[10.5px] text-ink-500">
          <span className="flex items-center gap-1">
            <kbd className="rounded border border-ink-700 bg-ink-850 px-1 font-mono">↑↓</kbd>
            {t("palette.navigate")}
          </span>
          <span className="flex items-center gap-1">
            <kbd className="rounded border border-ink-700 bg-ink-850 px-1 font-mono">↵</kbd>
            {t("palette.run")}
          </span>
        </div>
      </div>
    </div>
  );
}


