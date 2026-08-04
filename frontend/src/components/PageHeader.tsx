import type { ReactNode } from "react";
import { CircleHelp } from "lucide-react";
import { Tooltip } from '@/components/ui/tooltip';

interface PageHeaderProps {
  eyebrow?: string;
  title: string;
  description?: string;
  titleAccessory?: ReactNode;
  actions?: ReactNode;
  compact?: boolean;
}

function HeaderHelp({ text }: { text: string }) {
  return <Tooltip content={text} side="right" align="start" size="body" className="w-72 px-3 py-2 text-zinc-300">
    <button
        type="button"
        className="flex size-5 items-center justify-center rounded-full text-zinc-600 transition-colors hover:bg-zinc-800 hover:text-zinc-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/40"
        aria-label="About this page"
      >
        <CircleHelp className="size-3.5" />
    </button>
  </Tooltip>;
}

export function PageHeader({
  title,
  description,
  titleAccessory,
  actions,
}: PageHeaderProps) {
  return (
    <header className="flex h-16 shrink-0 items-center justify-between gap-6 border-b border-zinc-800 px-8">
      <div className="flex min-w-0 items-center gap-2">
        <h1 className="truncate text-base font-semibold tracking-tight">
          {title}
        </h1>
        {description ? <HeaderHelp text={description} /> : null}
        {titleAccessory}
      </div>
      {actions && (
        <div className="title-no-drag flex shrink-0 items-center gap-2">
          {actions}
        </div>
      )}
    </header>
  );
}
