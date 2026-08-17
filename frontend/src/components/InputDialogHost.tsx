import { useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { Button } from '@/components/ui/button'
import { useTranslation } from 'react-i18next'
import { useInputDialogStore } from '@/stores/input-dialog'
import { desktop } from '@/lib/bridge'
import type { InputDialogResponse } from '@/lib/types'

/**
 * Renders the one shared input dialog modal above all application views.
 * The Go side raises a request through a Wails event; this host resolves it
 * by calling the bound ResolveInputDialog method, which unblocks the
 * pipeline execution waiting on the dialog service.
 */
export function InputDialogHost() {
  const { t } = useTranslation()
  const request = useInputDialogStore((state) => state.request)
  const respond = useInputDialogStore((state) => state.respond)
  const [value, setValue] = useState('')
  const [error, setError] = useState<string | undefined>(undefined)
  const inputRef = useRef<HTMLInputElement>(null)
  const continueRef = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    if (!request) {
      setValue('')
      setError(undefined)
      return
    }
    setValue('')
    setError(undefined)
    const focusTimer = window.setTimeout(() => {
      // Prefer focusing the Continue button so Enter submits immediately;
      // fall back to the input field for screen-reader navigation.
      continueRef.current?.focus()
    }, 0)
    return () => window.clearTimeout(focusTimer)
  }, [request])

  // The Go side manages cancellation through context; the React layer only
  // reports an explicit Cancel click. Escape on the overlay treats the
  // interaction as cancelled too.
  const respondAndNotify = (response: InputDialogResponse) => {
    if (request) {
      void desktop.resolveInputDialog(request.id, response)
    }
    respond(response)
  }

  const handleContinue = () => {
    if (!request) return
    const trimmed = value.trim()
    if (request.inputType === 'number') {
      if (trimmed === '') {
        setError(t('inputDialog.errorEmpty', 'Enter a value to continue.'))
        return
      }
      if (Number.isNaN(Number(trimmed))) {
        setError(t('inputDialog.errorNumber', 'Enter a valid number.'))
        return
      }
    }
    respondAndNotify({ canceled: false, value })
  }

  if (!request) return null

  const titleID = 'input-dialog-title'
  const messageID = 'input-dialog-message'
  const continueLabel = request.continueLabel || t('inputDialog.continue', 'Continue')
  const cancelLabel = request.cancelLabel || t('inputDialog.cancel', 'Cancel')
  const isNumber = request.inputType === 'number'

  return createPortal(
    <div
      className="fixed inset-0 z-[110] flex items-center justify-center bg-black/70 p-5 backdrop-blur-[2px]"
      onPointerDown={(event) => {
        if (event.target === event.currentTarget) respondAndNotify({ canceled: true, value: '' })
      }}
    >
      <section
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleID}
        aria-describedby={request.message ? messageID : undefined}
        className="w-full max-w-md rounded-xl border border-zinc-700 bg-zinc-950 shadow-2xl shadow-black/70"
      >
        <div className="border-b border-zinc-800 px-5 py-4">
          <h2 id={titleID} className="text-sm font-semibold text-zinc-100">{request.title}</h2>
          {request.message ? (
            <p id={messageID} className="mt-1.5 whitespace-pre-line text-sm leading-5 text-zinc-400">{request.message}</p>
          ) : null}
        </div>
        <div className="px-5 py-4">
          <label className="mb-1.5 block text-xs font-medium text-zinc-400" htmlFor="input-dialog-field">
            {request.label || t('inputDialog.value', 'Value')}
          </label>
          <input
            id="input-dialog-field"
            ref={inputRef}
            type={isNumber ? 'number' : 'text'}
            value={value}
            placeholder={request.placeholder}
            onChange={(event) => { setValue(event.target.value); setError(undefined) }}
            onKeyDown={(event) => {
              if (event.key === 'Enter') {
                event.preventDefault()
                handleContinue()
              }
            }}
            className="flex h-9 w-full rounded-md border border-zinc-700 bg-zinc-950 px-2.5 text-sm text-zinc-100 outline-none placeholder:text-zinc-600 focus:border-zinc-500 focus:ring-2 focus:ring-white/10"
            aria-invalid={Boolean(error)}
          />
          {error ? <p className="mt-1.5 text-xs text-red-400">{error}</p> : null}
        </div>
        <div className="flex justify-end gap-2 px-5 py-4">
          <Button variant="outline" onClick={() => respondAndNotify({ canceled: true, value: '' })}>
            {cancelLabel}
          </Button>
          <Button ref={continueRef} onClick={handleContinue}>
            {continueLabel}
          </Button>
        </div>
      </section>
    </div>,
    document.body,
  )
}
