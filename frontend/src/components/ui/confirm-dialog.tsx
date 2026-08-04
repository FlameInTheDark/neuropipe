import { useEffect, useId, useRef } from 'react'
import { createPortal } from 'react-dom'
import { AlertTriangle, Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'

interface ConfirmDialogProps {
  open: boolean
  title: string
  description: string
  confirmLabel: string
  pending?: boolean
  onConfirm: () => void
  onOpenChange: (open: boolean) => void
}

/**
 * Application-owned confirmation surface. It deliberately avoids browser
 * browser-native dialogs so destructive actions retain Neuropipe's visual
 * language and keyboard behavior.
 */
export function ConfirmDialog({ open, title, description, confirmLabel, pending = false, onConfirm, onOpenChange }: ConfirmDialogProps) {
  const titleID = useId()
  const descriptionID = useId()
  const confirmRef = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    if (!open) return
    const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : undefined
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape' && !pending) onOpenChange(false)
    }
    window.addEventListener('keydown', onKeyDown)
    const focusTimer = window.setTimeout(() => confirmRef.current?.focus(), 0)
    return () => {
      window.removeEventListener('keydown', onKeyDown)
      window.clearTimeout(focusTimer)
      previousFocus?.focus()
    }
  }, [onOpenChange, open, pending])

  if (!open) return null

  return createPortal(
    <div
      className="fixed inset-0 z-[100] flex items-center justify-center bg-black/70 p-5 backdrop-blur-[2px]"
      onPointerDown={(event) => { if (event.target === event.currentTarget && !pending) onOpenChange(false) }}
    >
      <section
        role="alertdialog"
        aria-modal="true"
        aria-labelledby={titleID}
        aria-describedby={descriptionID}
        className="w-full max-w-md rounded-xl border border-zinc-700 bg-zinc-950 shadow-2xl shadow-black/70"
      >
        <div className="flex items-start gap-3 border-b border-zinc-800 px-5 py-4">
          <span className="mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-md border border-red-500/20 bg-red-500/10 text-red-300">
            <AlertTriangle className="size-4" />
          </span>
          <div>
            <h2 id={titleID} className="text-sm font-semibold text-zinc-100">{title}</h2>
            <p id={descriptionID} className="mt-1.5 text-sm leading-5 text-zinc-400">{description}</p>
          </div>
        </div>
        <div className="flex justify-end gap-2 px-5 py-4">
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={pending}>Cancel</Button>
          <Button ref={confirmRef} variant="danger" onClick={onConfirm} disabled={pending}>
            {pending && <Loader2 className="size-3.5 animate-spin" />}
            {confirmLabel}
          </Button>
        </div>
      </section>
    </div>,
    document.body,
  )
}
