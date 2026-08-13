import { useLayoutEffect, useRef, useState, type CSSProperties, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { cn } from '@/lib/utils'

type TooltipSide = 'top' | 'right' | 'bottom' | 'left'
type TooltipAlign = 'start' | 'center' | 'end'

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
  const [position, setPosition] = useState<CSSProperties>()
  const triggerRef = useRef<HTMLSpanElement>(null)
  const tooltipRef = useRef<HTMLSpanElement>(null)
  const layout = wrap
    ? 'w-max max-w-[min(22rem,calc(100vw-1rem))] whitespace-normal break-words'
    : 'w-max max-w-[calc(100vw-1rem)] whitespace-nowrap'
  const typography = size === 'body'
    ? 'text-xs font-normal leading-5'
    : 'text-[10px] font-medium leading-normal'
  useLayoutEffect(() => {
    if (!open) {
      setPosition(undefined)
      return
    }
    const update = () => {
      const trigger = triggerRef.current?.getBoundingClientRect()
      const tooltip = tooltipRef.current?.getBoundingClientRect()
      if (!trigger || !tooltip) return
      const gutter = 8
      const gap = 6
      let left = trigger.left + trigger.width / 2 - tooltip.width / 2
      let top = trigger.top - tooltip.height - gap
      if (side === 'bottom') top = trigger.bottom + gap
      if (side === 'right') {
        left = trigger.right + gap
        top = trigger.top + (trigger.height - tooltip.height) / 2
      }
      if (side === 'left') {
        left = trigger.left - tooltip.width - gap
        top = trigger.top + (trigger.height - tooltip.height) / 2
      }
      if (align === 'start' && (side === 'top' || side === 'bottom')) left = trigger.left
      if (align === 'end' && (side === 'top' || side === 'bottom')) left = trigger.right - tooltip.width
      if (align === 'start' && (side === 'left' || side === 'right')) top = trigger.top
      if (align === 'end' && (side === 'left' || side === 'right')) top = trigger.bottom - tooltip.height
      left = Math.max(gutter, Math.min(left, window.innerWidth - tooltip.width - gutter))
      top = Math.max(gutter, Math.min(top, window.innerHeight - tooltip.height - gutter))
      setPosition({ left, top })
    }
    update()
    window.addEventListener('resize', update)
    window.addEventListener('scroll', update, true)
    return () => {
      window.removeEventListener('resize', update)
      window.removeEventListener('scroll', update, true)
    }
  }, [align, content, open, side, size, wrap])
  return <span
    ref={triggerRef}
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
    {createPortal(<span ref={tooltipRef} role="tooltip" className={cn('pointer-events-none fixed z-[150] rounded border border-zinc-700 bg-zinc-900 px-2 py-1 text-zinc-200 shadow-lg transition-opacity duration-150', open ? 'visible opacity-100' : 'invisible opacity-0', layout, typography, className)} style={position}>
      {content}
    </span>, document.body)}
  </span>
}
