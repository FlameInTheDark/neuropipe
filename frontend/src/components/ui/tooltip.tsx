import { useState, type ReactNode } from 'react'
import { cn } from '@/lib/utils'

type TooltipSide = 'top' | 'right' | 'bottom' | 'left'
type TooltipAlign = 'start' | 'center' | 'end'

const positions: Record<`${TooltipSide}-${TooltipAlign}`, string> = {
  'top-start': 'bottom-full left-0 mb-1',
  'top-center': 'bottom-full left-1/2 mb-1 -translate-x-1/2',
  'top-end': 'bottom-full right-0 mb-1',
  'right-start': 'left-full top-0 ml-1',
  'right-center': 'left-full top-1/2 ml-1 -translate-y-1/2',
  'right-end': 'left-full bottom-0 ml-1',
  'bottom-start': 'top-full left-0 mt-1',
  'bottom-center': 'top-full left-1/2 mt-1 -translate-x-1/2',
  'bottom-end': 'top-full right-0 mt-1',
  'left-start': 'right-full top-0 mr-1',
  'left-center': 'right-full top-1/2 mr-1 -translate-y-1/2',
  'left-end': 'right-full bottom-0 mr-1',
}

interface TooltipProps {
  content: ReactNode
  children: ReactNode
  side?: TooltipSide
  align?: TooltipAlign
  /** Long explanations wrap; short labels can opt into a compact single line. */
  wrap?: boolean
  /** Keeps the shared surface while giving explanatory copy comfortable leading. */
  size?: 'compact' | 'body'
  triggerClassName?: string
  className?: string
}

/** Shared app-styled tooltip. It is the only tooltip chrome used by Neuropipe. */
export function Tooltip({ content, children, side = 'top', align = 'center', wrap = true, size = 'compact', triggerClassName, className }: TooltipProps) {
  const [open, setOpen] = useState(false)
  const [suppressedUntilLeave, setSuppressedUntilLeave] = useState(false)
  const layout = wrap
    ? 'max-w-[min(20rem,calc(100vw-1rem))] whitespace-normal break-words'
    : 'whitespace-nowrap'
  const typography = size === 'body'
    ? 'text-xs font-normal leading-5'
    : 'text-[10px] font-medium leading-normal'
  return <span
    className={cn('relative inline-flex', triggerClassName)}
    onPointerEnter={() => {
      if (!suppressedUntilLeave) setOpen(true)
    }}
    onPointerLeave={() => {
      setOpen(false)
      setSuppressedUntilLeave(false)
    }}
    onFocusCapture={() => {
      if (!suppressedUntilLeave) setOpen(true)
    }}
    onBlurCapture={(event) => {
      if (!event.currentTarget.contains(event.relatedTarget as Node | null)) {
        setOpen(false)
      }
    }}
    onClickCapture={() => {
      setOpen(false)
      setSuppressedUntilLeave(true)
    }}
  >
    {children}
    <span role="tooltip" className={cn('pointer-events-none absolute z-[90] rounded border border-zinc-700 bg-zinc-900 px-2 py-1 text-zinc-200 shadow-lg transition-opacity duration-150', open ? 'visible opacity-100' : 'invisible opacity-0', layout, typography, positions[`${side}-${align}`], className)}>
      {content}
    </span>
  </span>
}
