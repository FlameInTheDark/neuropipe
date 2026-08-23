import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { createPortal } from "react-dom";
import { Icon } from "../icons";
import { cn } from "../../utils/cn";
import { surface } from "./styles";
import { useDismissKeys } from "../../hooks/useDismissKeys";

export type ModalSize = "sm" | "md" | "lg" | "full";

const SIZES: Record<ModalSize, string> = {
  sm: "w-full max-w-[440px]",
  md: "w-full max-w-[720px]",
  lg: "w-full max-w-[min(96vw,1200px)] h-[min(92vh,900px)]",
  full: "w-full max-w-[min(98vw,1500px)] h-[min(96vh,960px)]",
};

/**
 * Single modal shell used by every dialog in the app.
 * Replaces the three hand-rolled portal+backdrop+header+footer
 * implementations that previously lived in TextEditorModal,
 * CodeEditorModal and WorkViews.
 */
export function Modal({
  title,
  icon,
  size = "sm",
  badge,
  headerExtra,
  toolbar,
  footer,
  children,
  onClose,
  bodyClassName,
}: {
  title: ReactNode;
  icon?: string;
  size?: ModalSize;
  /** small element rendered right of the title (e.g. "Unsaved") */
  badge?: ReactNode;
  /** controls rendered at the right edge of the header */
  headerExtra?: ReactNode;
  /** optional secondary bar under the header */
  toolbar?: ReactNode;
  footer?: ReactNode;
  children: ReactNode;
  onClose: () => void;
  bodyClassName?: string;
}) {
  const { t } = useTranslation();
  useDismissKeys(onClose);

  return createPortal(
    <div
      className="fixed inset-0 z-[70] flex items-center justify-center bg-black/70 p-4 backdrop-blur-[3px]"
      onClick={onClose}
    >
      <div
        className={cn("pop-in flex flex-col overflow-hidden", surface.overlay, SIZES[size])}
        onClick={(e) => e.stopPropagation()}
      >
        <header className="flex h-11 shrink-0 items-center gap-2.5 border-b border-seam px-4">
          {icon && <Icon name={icon} className="h-4 w-4 shrink-0 text-ink-400" />}
          <h2 className="truncate text-[13px] font-semibold text-ink-50">{title}</h2>
          {badge}
          <div className="ml-auto flex items-center gap-1">
            {headerExtra}
            <button
              onClick={onClose}
              aria-label={t("common.close")}
              className="grid h-7 w-7 place-items-center rounded-md text-ink-400 transition hover:bg-ink-800 hover:text-ink-50"
            >
              <Icon name="X" className="h-4 w-4" />
            </button>
          </div>
        </header>

        {toolbar}

        <div className={cn("min-h-0 flex-1", bodyClassName ?? "space-y-3 overflow-y-auto p-4")}>
          {children}
        </div>

        {footer && (
          <footer className="flex h-11 shrink-0 items-center gap-2 border-t border-seam px-4">
            {footer}
          </footer>
        )}
      </div>
    </div>,
    document.body,
  );
}

/** Standard cancel/confirm pair — previously copy-pasted into every dialog. */
export function ModalActions({
  onCancel,
  onConfirm,
  confirmLabel,
  cancelLabel,
  disabled,
}: {
  onCancel: () => void;
  onConfirm: () => void;
  confirmLabel?: string;
  cancelLabel?: string;
  disabled?: boolean;
}) {
  const { t } = useTranslation();
  const resolvedCancel = cancelLabel ?? t("common.cancel");
  const resolvedConfirm = confirmLabel ?? t("common.save");
  return (
    <div className="ml-auto flex items-center gap-2">
      <button
        onClick={onCancel}
        className="h-7 rounded-md border border-ink-700 bg-ink-850 px-3 text-[11.5px] text-ink-200 transition hover:bg-ink-750"
      >
        {resolvedCancel}
      </button>
      <button
        onClick={onConfirm}
        disabled={disabled}
        className={cn(
          "h-7 rounded-md px-3 text-[11.5px] font-medium transition",
          disabled
            ? "cursor-not-allowed bg-ink-800 text-ink-500"
            : "bg-ink-50 text-ink-950 hover:bg-white",
        )}
      >
        {resolvedConfirm}
      </button>
    </div>
  );
}

