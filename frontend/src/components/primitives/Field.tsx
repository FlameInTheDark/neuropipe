import type { ReactNode } from "react";
import { cn } from "../../utils/cn";
import { control } from "./styles";

/** Labelled form row. Used by the inspector, settings pages and all dialogs. */
export function Field({
  label,
  required,
  hint,
  action,
  className,
  children,
}: {
  label: string;
  required?: boolean;
  hint?: string;
  /** optional control rendered at the right of the label (e.g. "Expand") */
  action?: ReactNode;
  className?: string;
  children: ReactNode;
}) {
  return (
    <label className={cn("block", className)}>
      <span className="mb-1 flex items-center justify-between gap-2">
        <span className="flex items-center gap-1 text-[11.5px] font-medium text-ink-300">
          {label}
          {required && <span className="text-ink-500">*</span>}
        </span>
        {action}
      </span>
      {children}
      {hint && <span className="mt-1 block text-[11px] text-ink-500">{hint}</span>}
    </label>
  );
}

export function TextInput({
  value,
  onChange,
  placeholder,
  type = "text",
  mono,
  size = "md",
  className,
  autoFocus,
  disabled,
}: {
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  type?: "text" | "number" | "password";
  mono?: boolean;
  size?: "sm" | "md";
  className?: string;
  autoFocus?: boolean;
  disabled?: boolean;
}) {
  return (
    <input
      type={type}
      autoFocus={autoFocus}
      disabled={disabled}
      value={value}
      placeholder={placeholder}
      onChange={(e) => onChange(e.target.value)}
      className={cn(size === "sm" ? control.inputSm : control.input, mono && "font-mono text-[11.5px]", className)}
    />
  );
}

export function TextArea({
  value,
  onChange,
  placeholder,
  rows = 3,
  className,
}: {
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  rows?: number;
  className?: string;
}) {
  return (
    <textarea
      rows={rows}
      value={value}
      placeholder={placeholder}
      onChange={(e) => onChange(e.target.value)}
      className={cn(control.textarea, "resize-y", className)}
    />
  );
}

/** Native select styled to match — used for dense rows where Dropdown is too heavy. */
export function SelectInput<T extends string>({
  value,
  options,
  onChange,
  className,
}: {
  value: T;
  options: readonly T[];
  onChange: (v: T) => void;
  className?: string;
}) {
  return (
    <select
      value={value}
      onChange={(e) => onChange(e.target.value as T)}
      className={cn(control.inputSm, "px-1.5 text-[11px] text-ink-200", className)}
    >
      {options.map((o) => (
        <option key={o} value={o}>
          {o}
        </option>
      ))}
    </select>
  );
}

