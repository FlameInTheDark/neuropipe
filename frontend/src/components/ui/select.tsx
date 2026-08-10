import { useEffect, useLayoutEffect, useRef, useState, type KeyboardEvent as ReactKeyboardEvent } from 'react'
import { createPortal } from 'react-dom'
import { Check, ChevronDown } from 'lucide-react'
import { cn } from '@/lib/utils'

export interface SelectOption {
  value: string
  label: string
  description?: string
  disabled?: boolean
}

interface SelectProps {
  options: readonly SelectOption[]
  value: string
  onValueChange: (value: string) => void
  placeholder?: string
  disabled?: boolean
  id?: string
  className?: string
  menuClassName?: string
  menuPlacement?: 'bottom' | 'top'
  ariaLabel?: string
}

interface ListboxPosition {
  bottom?: number
  left: number
  maxHeight: number
  top?: number
  width: number
}

// Select is a keyboard-accessible in-app listbox. Native Windows select menus
// are rendered by the OS and cannot reliably follow Neuropipe's dark theme.
export function Select({ options, value, onValueChange, placeholder = 'Select an option', disabled = false, id, className, menuClassName, menuPlacement = 'bottom', ariaLabel }: SelectProps) {
  const [open, setOpen] = useState(false)
  const [listboxPosition, setListboxPosition] = useState<ListboxPosition>()
  const listboxRef = useRef<HTMLDivElement>(null)
  const rootRef = useRef<HTMLDivElement>(null)
  const selected = options.find((option) => option.value === value)

  useEffect(() => {
    if (!open) return
    const closeOutside = (event: PointerEvent) => {
      if (!rootRef.current?.contains(event.target as Node) && !listboxRef.current?.contains(event.target as Node)) setOpen(false)
    }
    window.addEventListener('pointerdown', closeOutside)
    return () => window.removeEventListener('pointerdown', closeOutside)
  }, [open])

  useLayoutEffect(() => {
    if (!open) {
      setListboxPosition(undefined)
      return
    }
    const updatePosition = () => {
      const bounds = rootRef.current?.getBoundingClientRect()
      if (!bounds) return
      const gutter = 8
      const preferredMaxHeight = 256
      const availableAbove = Math.max(0, bounds.top - gutter)
      const availableBelow = Math.max(0, window.innerHeight - bounds.bottom - gutter)
      const above = menuPlacement === 'top'
        ? availableAbove >= 96 || availableAbove >= availableBelow
        : availableBelow < 96 && availableAbove > availableBelow
      const availableHeight = above ? availableAbove : availableBelow
      const width = Math.min(bounds.width, Math.max(0, window.innerWidth - gutter * 2))
      setListboxPosition({
        bottom: above ? window.innerHeight - bounds.top + 4 : undefined,
        left: Math.max(gutter, Math.min(bounds.left, window.innerWidth - width - gutter)),
        maxHeight: Math.min(preferredMaxHeight, availableHeight),
        top: above ? undefined : bounds.bottom + 4,
        width,
      })
    }
    updatePosition()
    window.addEventListener('resize', updatePosition)
    window.addEventListener('scroll', updatePosition, true)
    const observer = new ResizeObserver(updatePosition)
    if (rootRef.current) observer.observe(rootRef.current)
    return () => {
      window.removeEventListener('resize', updatePosition)
      window.removeEventListener('scroll', updatePosition, true)
      observer.disconnect()
    }
  }, [menuPlacement, open])

  const choose = (option: SelectOption) => {
    if (option.disabled) return
    onValueChange(option.value)
    setOpen(false)
  }

  const move = (direction: 1 | -1) => {
    const enabled = options.filter((option) => !option.disabled)
    if (enabled.length === 0) return
    const currentIndex = enabled.findIndex((option) => option.value === value)
    const nextIndex = currentIndex < 0 ? (direction === 1 ? 0 : enabled.length - 1) : (currentIndex + direction + enabled.length) % enabled.length
    choose(enabled[nextIndex])
  }

  const onKeyDown = (event: ReactKeyboardEvent<HTMLButtonElement>) => {
    if (event.key === 'Escape') {
      setOpen(false)
      return
    }
    if (event.key === 'ArrowDown') {
      event.preventDefault()
      if (open) move(1)
      else setOpen(true)
      return
    }
    if (event.key === 'ArrowUp') {
      event.preventDefault()
      if (open) move(-1)
      else setOpen(true)
      return
    }
    if (event.key === 'Home') {
      event.preventDefault()
      const first = options.find((option) => !option.disabled)
      if (first) choose(first)
      return
    }
    if (event.key === 'End') {
      event.preventDefault()
      const last = [...options].reverse().find((option) => !option.disabled)
      if (last) choose(last)
    }
  }

  return <div ref={rootRef} className={cn('relative min-w-0', className)}>
    <button id={id} type="button" className="flex h-8 w-full items-center gap-2 rounded-md border border-zinc-700 bg-zinc-950 px-2.5 text-left text-sm text-zinc-200 shadow-inner shadow-black/20 transition hover:border-zinc-600 hover:bg-zinc-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-zinc-500 disabled:cursor-not-allowed disabled:opacity-50" aria-label={ariaLabel} aria-haspopup="listbox" aria-expanded={open} disabled={disabled} onClick={() => setOpen((current) => !current)} onKeyDown={onKeyDown}>
      <span className={cn('min-w-0 flex-1 truncate', !selected && 'text-zinc-500')}>{selected?.label ?? placeholder}</span>
      <ChevronDown aria-hidden className={cn('size-4 shrink-0 text-zinc-500 transition-transform', open && 'rotate-180')} />
    </button>
    {open && listboxPosition ? createPortal(<div ref={listboxRef} role="listbox" aria-label={ariaLabel ?? placeholder} className={cn('fixed z-[130] overflow-y-auto rounded-md border border-zinc-700 bg-zinc-950 p-1 shadow-2xl shadow-black/50', menuClassName)} style={{ bottom: listboxPosition.bottom, left: listboxPosition.left, maxHeight: listboxPosition.maxHeight, top: listboxPosition.top, width: listboxPosition.width }}>{options.length === 0 ? <p className="px-2.5 py-3 text-xs text-zinc-500">No options available.</p> : options.map((option) => <button key={option.value} type="button" role="option" aria-selected={option.value === value} disabled={option.disabled} onClick={() => choose(option)} className={cn('flex w-full items-center gap-2 rounded px-2.5 py-2 text-left hover:bg-zinc-800 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-zinc-500 disabled:cursor-not-allowed disabled:opacity-40', option.value === value && 'bg-zinc-800/80')}><span className="min-w-0 flex-1"><span className="block truncate text-xs font-medium text-zinc-200">{option.label}</span>{option.description ? <span className="block truncate pt-0.5 font-mono text-[10px] text-zinc-500">{option.description}</span> : null}</span>{option.value === value ? <Check aria-hidden className="size-3.5 shrink-0 text-zinc-300" /> : null}</button>)}</div>, document.body) : null}
  </div>
}
