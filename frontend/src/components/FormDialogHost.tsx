import { useEffect, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { Button } from '@/components/ui/button'
import { useTranslation } from 'react-i18next'
import { useFormDialogStore } from '@/stores/form-dialog'
import { desktop } from '@/lib/bridge'
import type {
  FormDialogField,
  FormDialogRequest,
  FormDialogResponse,
} from '@/lib/types'

type FieldValue = string | number

const emptyValues = (): Record<string, FieldValue> => ({})

function defaultFieldValue(field: FormDialogField): FieldValue {
  if (field.kind === 'input' && field.inputType === 'number') return ''
  return ''
}

/**
 * Renders the one shared form dialog modal above all application views. The
 * Go side raises a request through a Wails event; this host resolves it by
 * calling the bound ResolveFormDialog method, which unblocks the pipeline
 * execution waiting on the form dialog service.
 *
 * The dialog is rendered through a raw portal (not the shadcn/ui Dialog
 * component) to avoid z-index stacking issues with nested modals such as the
 * JavaScript code editor. The grid layout is owned by the form layout stored
 * on the Form node config and is forwarded verbatim through the Wails event.
 */
export function FormDialogHost() {
  const { t } = useTranslation()
  const request = useFormDialogStore((state) => state.request)
  const respond = useFormDialogStore((state) => state.respond)
  const [values, setValues] = useState<Record<string, FieldValue>>(emptyValues)
  const [error, setError] = useState<string | undefined>(undefined)
  const continueRef = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    if (!request) {
      setValues(emptyValues())
      setError(undefined)
      return
    }
    const initial: Record<string, FieldValue> = {}
    for (const item of request.items) {
      if (item.kind === 'text') continue
      initial[item.id] = defaultFieldValue(item)
    }
    setValues(initial)
    setError(undefined)
    const focusTimer = window.setTimeout(() => {
      continueRef.current?.focus()
    }, 0)
    return () => window.clearTimeout(focusTimer)
  }, [request])

  const respondAndNotify = (response: FormDialogResponse) => {
    if (request) {
      void desktop.resolveFormDialog(request.id, response)
    }
    respond(response)
  }

  const handleCancel = () => {
    respondAndNotify({ canceled: true, values: {} })
  }

  const handleSubmit = () => {
    if (!request) return
    for (const item of request.items) {
      if (item.kind !== 'input') continue
      if (item.inputType !== 'number') continue
      const raw = values[item.id]
      const trimmed = String(raw ?? '').trim()
      if (trimmed === '' || Number.isNaN(Number(trimmed))) {
        setError(t('formBuilder.errorNumber', { name: item.label || item.id }))
        return
      }
    }
    const payload: Record<string, string | number> = {}
    for (const item of request.items) {
      if (item.kind === 'text') continue
      const raw = values[item.id]
      if (item.kind === 'input' && item.inputType === 'number') {
        payload[item.id] = Number(String(raw ?? '').trim())
      } else {
        payload[item.id] = String(raw ?? '')
      }
    }
    setError(undefined)
    respondAndNotify({ canceled: false, values: payload })
  }

  const onKeyDown = (event: React.KeyboardEvent<HTMLDivElement>) => {
    if (event.key === 'Escape') {
      event.preventDefault()
      handleCancel()
    }
  }

  const rows = useMemo(() => {
    if (!request) return 0
    return request.items.reduce((max, item) => Math.max(max, item.row + item.rowSpan), 0)
  }, [request])

  if (!request) return null

  const req: FormDialogRequest = request
  const titleID = 'form-dialog-title'
  const messageID = 'form-dialog-message'
  const continueLabel = req.continueLabel || t('formBuilder.submit')
  const cancelLabel = req.cancelLabel || t('formBuilder.cancel')

  return createPortal(
    <div
      className="fixed inset-0 z-[120] flex items-center justify-center bg-black/70 p-5 backdrop-blur-[2px]"
      onPointerDown={(event) => {
        if (event.target === event.currentTarget) handleCancel()
      }}
      onKeyDown={onKeyDown}
    >
      <section
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleID}
        aria-describedby={req.message ? messageID : undefined}
        className="flex max-h-[85vh] w-full max-w-3xl flex-col rounded-xl border border-zinc-700 bg-zinc-950 shadow-2xl shadow-black/70"
      >
        <div className="border-b border-zinc-800 px-6 py-4">
          <h2 id={titleID} className="text-base font-semibold text-zinc-100">{req.title}</h2>
          {req.message ? (
            <p id={messageID} className="mt-1.5 whitespace-pre-line text-sm leading-5 text-zinc-400">{req.message}</p>
          ) : null}
        </div>
        <div className="grid max-h-[60vh] gap-3 overflow-y-auto px-6 py-5" style={{ gridTemplateColumns: 'repeat(4, minmax(0, 1fr))', gridAutoRows: 'minmax(48px, auto)', gridTemplateRows: `repeat(${Math.max(rows, 1)}, auto)` }}>
          {req.items.map((item) => (
            <FormDialogItem
              key={item.id}
              field={item}
              value={values[item.id]}
              errorName={error ? item.label || item.id : undefined}
              onChange={(next) => {
                setValues((current) => ({ ...current, [item.id]: next }))
                setError(undefined)
              }}
            />
          ))}
        </div>
        {error ? (
          <p className="px-6 text-xs text-red-400">{error}</p>
        ) : null}
        <div className="flex justify-end gap-2 border-t border-zinc-800 px-6 py-4">
          <Button variant="outline" onClick={handleCancel}>
            {cancelLabel}
          </Button>
          <Button ref={continueRef} onClick={handleSubmit}>
            {continueLabel}
          </Button>
        </div>
      </section>
    </div>,
    document.body,
  )
}

function FormDialogItem({
  field,
  value,
  errorName,
  onChange,
}: {
  field: FormDialogField
  value: FieldValue | undefined
  errorName?: string
  onChange: (value: FieldValue) => void
}) {
  const { t } = useTranslation()
  const span = Math.min(4, Math.max(1, field.span || 1))
  const rowSpan = Math.max(1, field.rowSpan || 1)
  const style: React.CSSProperties = {
    gridColumn: `${field.col + 1} / span ${span}`,
    gridRow: `${field.row + 1} / span ${rowSpan}`,
  }

  if (field.kind === 'text') {
    return (
      <div style={style} className="flex items-center">
        <p className="text-sm leading-5 text-zinc-300">{field.label}</p>
      </div>
    )
  }

  if (field.kind === 'input') {
    const isNumber = field.inputType === 'number'
    const stringValue = value === undefined ? '' : String(value)
    const showError = Boolean(errorName) && isNumber && (stringValue.trim() === '' || Number.isNaN(Number(stringValue.trim())))
    return (
      <label style={style} className="block">
        <span className="mb-1.5 block text-xs font-medium text-zinc-400">{field.label}</span>
        <input
          type={isNumber ? 'number' : 'text'}
          value={stringValue}
          placeholder={field.placeholder}
          onChange={(event) => onChange(isNumber ? event.target.value : event.target.value)}
          aria-invalid={showError}
          aria-label={field.label}
          className="flex h-9 w-full rounded-md border border-zinc-700 bg-zinc-950 px-2.5 text-sm text-zinc-100 outline-none placeholder:text-zinc-600 focus:border-zinc-500 focus:ring-2 focus:ring-white/10"
        />
        {showError ? (
          <p className="mt-1 text-[11px] text-red-400">{t('formBuilder.errorNumber', { name: field.label || field.id })}</p>
        ) : null}
      </label>
    )
  }

  // dropdown
  const options = Array.isArray(field.options) ? field.options : []
  const stringValue = value === undefined ? '' : String(value)
  return (
    <label style={style} className="block">
      <span className="mb-1.5 block text-xs font-medium text-zinc-400">{field.label}</span>
      <select
        value={stringValue}
        onChange={(event) => onChange(event.target.value)}
        aria-label={field.label}
        className="flex h-9 w-full rounded-md border border-zinc-700 bg-zinc-950 px-2.5 text-sm text-zinc-100 outline-none focus:border-zinc-500 focus:ring-2 focus:ring-white/10"
      >
        {stringValue === '' ? <option value="">{field.placeholder || ''}</option> : null}
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label || option.value}
          </option>
        ))}
      </select>
    </label>
  )
}
