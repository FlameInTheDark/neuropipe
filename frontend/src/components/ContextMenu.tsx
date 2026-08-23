import { createContext, useCallback, useContext, useEffect, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { Icon } from "./icons";
import { cn } from "../utils/cn";

export interface MenuItem {
  type?: "item" | "sep";
  label?: string;
  icon?: string;
  hint?: string;
  danger?: boolean;
  disabled?: boolean;
  done?: boolean;
  checked?: boolean;
  onSelect?: () => void;
}

interface MenuState {
  x: number;
  y: number;
  items: MenuItem[];
}

const Ctx = createContext<(e: { preventDefault: () => void; clientX: number; clientY: number }, items: MenuItem[]) => void>(() => {});

export function useCtxMenu() {
  return useContext(Ctx);
}

export function ContextTrigger({
  items,
  children,
  className,
}: {
  items: MenuItem[];
  children: React.ReactNode;
  className?: string;
}) {
  const open = useContext(Ctx);
  return (
    <div className={className} onContextMenu={(e) => open(e, items)}>
      {children}
    </div>
  );
}

/* ---------------------------------------------------------------- */

export function ContextMenuProvider({ children, onAnyAction }: { children: React.ReactNode; onAnyAction?: () => void }) {
  const [menu, setMenu] = useState<MenuState | null>(null);
  const [glyph, setGlyph] = useState<{ x: number; y: number; icon?: string } | null>(null);
  const menuRef = useRef<HTMLDivElement>(null);

  const open = useCallback(
    (e: { preventDefault: () => void; clientX: number; clientY: number }, items: MenuItem[]) => {
      e.preventDefault();
      setMenu({ x: e.clientX, y: e.clientY, items });
    },
    [],
  );

  const close = useCallback(() => setMenu(null), []);

  const action = useCallback(
    (item: MenuItem) => {
      setGlyph({ x: menu?.x ?? 0, y: menu?.y ?? 0, icon: item.icon ?? "Check" });
      window.setTimeout(() => setGlyph(null), 260);
      onAnyAction?.();
    },
    [menu, onAnyAction],
  );

  /* dismiss */
  useEffect(() => {
    if (!menu) return;
    const onDown = (e: PointerEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) close();
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") close();
      if (menuRef.current && (e.key === "ArrowDown" || e.key === "ArrowUp" || e.key === "Enter")) {
        const btns = [...menuRef.current.querySelectorAll<HTMLButtonElement>("button:not(:disabled)")];
        const cur = btns.indexOf(document.activeElement as HTMLButtonElement);
        if (e.key === "ArrowDown") (btns[cur + 1] ?? btns[0])?.focus();
        if (e.key === "ArrowUp") (btns[cur - 1] ?? btns[btns.length - 1])?.focus();
        if (e.key === "Enter" && cur >= 0) btns[cur]?.click();
      }
    };
    window.addEventListener("pointerdown", onDown);
    window.addEventListener("keydown", onKey);
    window.addEventListener("wheel", close, { passive: true });
    window.addEventListener("resize", close);
    return () => {
      window.removeEventListener("pointerdown", onDown);
      window.removeEventListener("keydown", onKey);
      window.removeEventListener("wheel", close);
      window.removeEventListener("resize", close);
    };
  }, [menu, close]);

  return (
    <Ctx.Provider value={open}>
      {children}
      {menu && createPortal(<Menu menu={menu} menuRef={menuRef} close={close} action={action} />, document.body)}
      {glyph && (
        <div
          className="action-glyph pointer-events-none fixed z-[70] rounded-full border border-ink-500/60 bg-ink-800 p-1.5 text-ink-100"
          style={{ left: glyph.x - 26, top: glyph.y + 14 }}
        >
          <Icon name={glyph.icon ?? "Check"} className="h-3 w-3" />
        </div>
      )}
    </Ctx.Provider>
  );
}

function Menu({
  menu,
  menuRef,
  close,
  action,
}: {
  menu: MenuState;
  menuRef: React.RefObject<HTMLDivElement | null>;
  close: () => void;
  action: (item: MenuItem) => void;
}) {
  const [pos, setPos] = useState<{ left: number; top: number; origin: string }>({
    left: menu.x,
    top: menu.y,
    origin: "top left",
  });

  useLayoutEffect(() => {
    const el = menuRef.current;
    if (!el) return;
    const { innerWidth, innerHeight } = window;
    const r = el.getBoundingClientRect();
    let left = menu.x;
    let top = menu.y;
    let origin = "top left";
    if (left + r.width > innerWidth - 8) left = menu.x - r.width;
    if (top + r.height > innerHeight - 8) top = menu.y - r.height;
    if (left !== menu.x && top !== menu.y) origin = "bottom right";
    else if (left !== menu.x) origin = "top right";
    else if (top !== menu.y) origin = "bottom left";
    setPos({ left: Math.max(8, left), top: Math.max(8, top), origin });
  }, [menu, menuRef]);

  /* focus first item for keyboard nav */
  useEffect(() => {
    menuRef.current?.querySelector<HTMLButtonElement>("button:not(:disabled)")?.focus();
  }, [menuRef]);

  return (
    <div
      ref={menuRef}
      role="menu"
      style={{ left: pos.left, top: pos.top, transformOrigin: pos.origin }}
      className="timeline-menu fixed z-[60] min-w-[200px] rounded-[9px] border border-ink-650 bg-ink-850/95 p-1 shadow-[0_18px_44px_-12px_rgba(0,0,0,0.95),0_0_0_1px_rgba(255,255,255,0.02)_inset] backdrop-blur-xl"
      onContextMenu={(e) => e.preventDefault()}
    >
      {menu.items.map((item, i) =>
        item.type === "sep" ? (
          <div key={i} className="mx-1.5 my-1 h-px bg-ink-700/80" />
        ) : (
          <button
            key={i}
            role="menuitem"
            disabled={item.disabled}
            onClick={() => {
              close();
              if (item.onSelect) {
                item.onSelect();
                action(item);
              }
            }}
            className={cn(
              "flex h-7 w-full items-center gap-2.5 rounded-md px-2 text-left text-[12.5px] outline-none transition-colors",
              item.danger
                ? "text-rose-300 hover:bg-rose-500/15 focus:bg-rose-500/15"
                : item.disabled
                  ? "cursor-not-allowed text-ink-600"
                  : "text-ink-100 hover:bg-ink-650/80 focus:bg-ink-650/80",
            )}
          >
            {item.icon ? (
              <Icon
                name={item.icon}
                className={cn("h-[14px] w-[14px] shrink-0", item.disabled ? "text-ink-600" : item.danger ? "text-rose-300/80" : "text-ink-400")}
              />
            ) : (
              <span className="w-[14px] shrink-0" />
            )}
            <span className="min-w-0 truncate">{item.label}</span>
            {item.checked && <Icon name="Check" className="ml-auto h-3.5 w-3.5 shrink-0 text-ink-200" />}
            {item.hint && <span className="ml-auto shrink-0 font-mono text-[10px] text-ink-500">{item.hint}</span>}
          </button>
        ),
      )}
    </div>
  );
}
