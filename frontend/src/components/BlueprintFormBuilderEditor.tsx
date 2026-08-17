import { useState } from 'react'
import { createPortal } from 'react-dom'
import { ChevronDown, Move, Plus, TextCursorInput, Trash2, Type, X } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { formLayoutFromValue } from '@/lib/blueprint-dynamic-pins'
import type { FormItemValue, FormLayoutValue } from '@/lib/types'
import { useEffect, useMemo } from 'react'

const GRID_COLUMNS = 4
const ROW_HEIGHT = 64
const MIN_ROWS = 6

function nextItemID(items: readonly FormItemValue[]): string {
  const used = new Set(items.map((item) => item.id))
  for (let index = items.length + 1; ; index += 1) {
    const id = `field_${index}`
    if (!used.has(id)) return id
  }
}

function clampSpan(value: number): number {
  return Math.min(GRID_COLUMNS, Math.max(1, value))
}

function clampRowSpan(value: number): number {
  return Math.max(1, value)
}

function findFreeSlot(items: readonly FormItemValue[]): { col: number; row: number } {
  const occupied = new Set<string>()
  for (const item of items) {
    for (let c = 0; c < item.span; c += 1) {
      for (let r = 0; r < item.rowSpan; r += 1) {
        occupied.add(`${item.col + c},${item.row + r}`)
      }
    }
  }
  for (let row = 0; row < 50; row += 1) {
    for (let col = 0; col < GRID_COLUMNS; col += 1) {
      if (!occupied.has(`${col},${row}`)) return { col, row }
    }
  }
  return { col: 0, row: 0 }
}

/**
 * FormBuilderEditor is the inspector entry point: a button that opens a
 * full-screen modal containing the visual grid editor. The modal gives the
 * grid enough space to be usable, unlike the narrow inspector panel.
 */
export function FormBuilderEditor({
  value,
  onChange,
}: {
  value: unknown
  onChange: (value: FormLayoutValue) => void
}) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)

  return (
    <>
      <Button type="button" variant="outline" onClick={() => setOpen(true)}>
        <Move className="size-3.5" />
        {t('formBuilder.layout')}
      </Button>
      {open ? (
        <FormBuilderModal
          value={value}
          onChange={onChange}
          onClose={() => setOpen(false)}
        />
      ) : null}
    </>
  )
}

function FormBuilderModal({
  value,
  onChange,
  onClose,
}: {
  value: unknown
  onChange: (value: FormLayoutValue) => void
  onClose: () => void
}) {
  const { t } = useTranslation()
  const layout = useMemo(() => formLayoutFromValue(value), [value])
  const [selectedID, setSelectedID] = useState<string | undefined>(undefined)
  const [draggedID, setDraggedID] = useState<string | undefined>(undefined)

  // Escape to close
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  const rows = useMemo(() => {
    const used = layout.items.reduce((max, item) => Math.max(max, item.row + item.rowSpan), 0)
    return Math.max(MIN_ROWS, used + 1)
  }, [layout.items])

  const update = (next: FormLayoutValue) => onChange(next)
  const updateItem = (id: string, patch: Partial<FormItemValue>) => {
    const item = layout.items.find((candidate) => candidate.id === id)
    if (!item) return
    const next = { ...item, ...patch }
    // Collision check: if the updated item (with new span/rowSpan/col/row)
    // would overlap another item, abort the update.
    const span = clampSpan(next.span)
    const rowSpan = clampRowSpan(next.rowSpan)
    const col = Math.min(GRID_COLUMNS - span, Math.max(0, next.col))
    const row = Math.max(0, next.row)
    const collides = layout.items.some((other) => {
      if (other.id === id) return false
      for (let c = 0; c < span; c += 1) {
        for (let r = 0; r < rowSpan; r += 1) {
          for (let oc = 0; oc < clampSpan(other.span); oc += 1) {
            for (let or = 0; or < clampRowSpan(other.rowSpan); or += 1) {
              if (col + c === other.col + oc && row + r === other.row + or) {
                return true
              }
            }
          }
        }
      }
      return false
    })
    if (collides) return
    update({
      items: layout.items.map((it) => (it.id === id ? { ...it, ...patch, col, row } : it)),
    })
  }
  const addItem = (kind: FormItemValue['kind']) => {
    const slot = findFreeSlot(layout.items)
    const id = nextItemID(layout.items)
    const item: FormItemValue =
      kind === 'text'
        ? { id, kind, label: t('formBuilder.addText'), col: slot.col, row: slot.row, span: 4, rowSpan: 1 }
        : kind === 'dropdown'
          ? { id, kind, label: t('formBuilder.addDropdown'), col: slot.col, row: slot.row, span: 2, rowSpan: 1, options: [{ value: 'option_1' }] }
          : { id, kind, label: t('formBuilder.addInput'), col: slot.col, row: slot.row, span: 2, rowSpan: 1, inputType: 'text' }
    update({ items: [...layout.items, item] })
    setSelectedID(id)
  }
  const deleteItem = (id: string) => {
    update({ items: layout.items.filter((item) => item.id !== id) })
    if (selectedID === id) setSelectedID(undefined)
  }
  const moveItem = (id: string, col: number, row: number) => {
    const safeCol = Math.min(GRID_COLUMNS - 1, Math.max(0, col))
    const safeRow = Math.max(0, row)
    // Check for collision with other items. If the dragged item would overlap
    // any other item at the target position, abort the move — the item stays
    // where it was. This prevents items from stacking on top of each other.
    const dragged = layout.items.find((item) => item.id === id)
    if (!dragged) return
    const span = clampSpan(dragged.span)
    const rowSpan = clampRowSpan(dragged.rowSpan)
    const collides = layout.items.some((other) => {
      if (other.id === id) return false
      for (let c = 0; c < span; c += 1) {
        for (let r = 0; r < rowSpan; r += 1) {
          for (let oc = 0; oc < clampSpan(other.span); oc += 1) {
            for (let or = 0; or < clampRowSpan(other.rowSpan); or += 1) {
              if (safeCol + c === other.col + oc && safeRow + r === other.row + or) {
                return true
              }
            }
          }
        }
      }
      return false
    })
    if (collides) return
    updateItem(id, { col: safeCol, row: safeRow })
  }
  const addOption = (id: string) => {
    const item = layout.items.find((candidate) => candidate.id === id)
    if (!item) return
    const options = [...(item.options ?? [])]
    const used = new Set(options.map((option) => option.value))
    let index = options.length + 1
    while (used.has(`option_${index}`)) index += 1
    options.push({ value: `option_${index}` })
    updateItem(id, { options })
  }
  const updateOption = (id: string, index: number, patch: Partial<{ value: string; label: string }>) => {
    const item = layout.items.find((candidate) => candidate.id === id)
    if (!item) return
    const options = (item.options ?? []).map((option, current) =>
      current === index ? { ...option, ...patch } : option,
    )
    updateItem(id, { options })
  }
  const removeOption = (id: string, index: number) => {
    const item = layout.items.find((candidate) => candidate.id === id)
    if (!item) return
    const options = (item.options ?? []).filter((_, current) => current !== index)
    updateItem(id, { options })
  }

  const selected = layout.items.find((item) => item.id === selectedID)

  return createPortal(
    <div
      className="fixed inset-0 z-[110] flex items-center justify-center bg-black/70 p-5 backdrop-blur-[2px]"
      onPointerDown={(event) => { if (event.target === event.currentTarget) onClose() }}
    >
      <section
        className="flex max-h-[calc(100vh-40px)] w-full max-w-6xl flex-col overflow-hidden rounded-xl border border-zinc-700 bg-zinc-950 shadow-2xl shadow-black/70"
      >
        {/* Header */}
        <div className="flex items-center justify-between border-b border-zinc-800 px-5 py-3">
          <h2 className="text-sm font-semibold text-zinc-100">{t('formBuilder.layout')}</h2>
          <Button variant="ghost" size="sm" className="size-7 p-0" onClick={onClose} aria-label="Close">
            <X className="size-4" />
          </Button>
        </div>

        {/* Toolbar */}
        <div className="flex flex-wrap items-center gap-1.5 border-b border-zinc-800 px-5 py-2.5">
          <Button type="button" size="sm" variant="outline" onClick={() => addItem('text')}>
            <Type className="size-3.5" />
            {t('formBuilder.addText')}
          </Button>
          <Button type="button" size="sm" variant="outline" onClick={() => addItem('input')}>
            <TextCursorInput className="size-3.5" />
            {t('formBuilder.addInput')}
          </Button>
          <Button type="button" size="sm" variant="outline" onClick={() => addItem('dropdown')}>
            <ChevronDown className="size-3.5" />
            {t('formBuilder.addDropdown')}
          </Button>
        </div>

        {/* Body: grid + side panel */}
        <div className="grid min-h-0 flex-1 grid-cols-1 lg:grid-cols-[1fr_260px]">
          {/* Grid */}
          <div
            className="muted-scroll min-h-0 overflow-auto border-b border-zinc-800 p-4 lg:border-b-0 lg:border-r"
          >
            <div
              className="grid gap-2"
              style={{
                gridTemplateColumns: `repeat(${GRID_COLUMNS}, minmax(0, 1fr))`,
                gridTemplateRows: `repeat(${rows}, ${ROW_HEIGHT}px)`,
              }}
            >
              {Array.from({ length: rows * GRID_COLUMNS }).map((_, index) => {
                const col = index % GRID_COLUMNS
                const row = Math.floor(index / GRID_COLUMNS)
                return (
                  <div
                    key={`cell-${col}-${row}`}
                    onDragOver={(event) => event.preventDefault()}
                    onDrop={(event) => {
                      event.preventDefault()
                      if (draggedID) moveItem(draggedID, col, row)
                      setDraggedID(undefined)
                    }}
                    className="rounded border border-dashed border-zinc-800/70 bg-zinc-900/20"
                    style={{ gridColumn: `${col + 1}`, gridRow: `${row + 1}` }}
                  />
                )
              })}
              {layout.items.map((item) => {
                const isSelected = item.id === selectedID
                return (
                  <button
                    key={item.id}
                    type="button"
                    draggable
                    onDragStart={() => setDraggedID(item.id)}
                    onDragEnd={() => setDraggedID(undefined)}
                    onDragOver={(event) => event.preventDefault()}
                    onDrop={(event) => {
                      event.preventDefault()
                      if (draggedID && draggedID !== item.id) moveItem(draggedID, item.col, item.row)
                      setDraggedID(undefined)
                    }}
                    onClick={() => setSelectedID(item.id)}
                    className={`group flex flex-col gap-1 rounded-md border p-2 text-left transition-colors ${
                      isSelected
                        ? 'border-zinc-500 bg-zinc-800/80 ring-1 ring-zinc-500'
                        : 'border-zinc-700 bg-zinc-900/70 hover:border-zinc-600'
                    }`}
                    style={{
                      gridColumn: `${item.col + 1} / span ${clampSpan(item.span)}`,
                      gridRow: `${item.row + 1} / span ${clampRowSpan(item.rowSpan)}`,
                    }}
                    aria-label={item.label || item.id}
                  >
                    <span className="flex items-center gap-1 text-[10px] uppercase tracking-wide text-zinc-500">
                      {item.kind === 'text' ? <Type className="size-3" />
                        : item.kind === 'dropdown' ? <ChevronDown className="size-3" />
                          : <TextCursorInput className="size-3" />}
                      {item.kind}
                      <Move className="ml-auto size-3 text-zinc-600 opacity-0 transition-opacity group-hover:opacity-100" />
                    </span>
                    <span className="min-w-0 break-words text-xs font-medium text-zinc-200">
                      {item.label || item.id}
                    </span>
                    {item.kind === 'input' ? (
                      <span className="text-[10px] text-zinc-500">
                        {item.inputType === 'number' ? t('editor.number') : t('editor.text')}
                      </span>
                    ) : null}
                    {item.kind === 'dropdown' ? (
                      <span className="text-[10px] text-zinc-500">
                        {t('formBuilder.options')}: {(item.options ?? []).length}
                      </span>
                    ) : null}
                  </button>
                )
              })}
            </div>
          </div>

          {/* Side panel */}
          <aside className="muted-scroll min-h-0 overflow-y-auto bg-zinc-950/40 p-4">
            {selected ? (
              <div className="space-y-2.5">
                <div className="flex items-center justify-between gap-2">
                  <span className="text-[11px] font-medium text-zinc-300">{t('formBuilder.selected')}</span>
                  <Button
                    type="button"
                    size="sm"
                    variant="ghost"
                    className="size-6 p-0 text-zinc-500 hover:text-red-300"
                    aria-label={t('formBuilder.deleteItem')}
                    onClick={() => deleteItem(selected.id)}
                  >
                    <Trash2 className="size-3.5" />
                  </Button>
                </div>
                <label className="block">
                  <span className="mb-1 block text-[10px] font-medium text-zinc-500">
                    {t('formBuilder.itemLabel')}
                  </span>
                  <Input
                    value={selected.label}
                    onChange={(event) => updateItem(selected.id, { label: event.target.value })}
                    aria-label={t('formBuilder.itemLabel')}
                  />
                </label>
                <label className="block">
                  <span className="mb-1 block text-[10px] font-medium text-zinc-500">
                    {t('formBuilder.span')}
                  </span>
                  <Select
                    value={String(selected.span)}
                    onValueChange={(next) => updateItem(selected.id, { span: clampSpan(Number(next)) })}
                    options={[1, 2, 3, 4].map((n) => ({ value: String(n), label: String(n) }))}
                    ariaLabel={t('formBuilder.span')}
                  />
                </label>
                <label className="block">
                  <span className="mb-1 block text-[10px] font-medium text-zinc-500">
                    {t('formBuilder.span')} (rows)
                  </span>
                  <Input
                    type="number"
                    min={1}
                    max={10}
                    value={String(selected.rowSpan)}
                    onChange={(event) => updateItem(selected.id, { rowSpan: clampRowSpan(Number(event.target.value) || 1) })}
                    aria-label={`${t('formBuilder.span')} (rows)`}
                  />
                </label>
                {selected.kind === 'input' ? (
                  <>
                    <label className="block">
                      <span className="mb-1 block text-[10px] font-medium text-zinc-500">
                        {t('formBuilder.inputType')}
                      </span>
                      <Select
                        value={selected.inputType === 'number' ? 'number' : 'text'}
                        onValueChange={(next) => updateItem(selected.id, { inputType: next === 'number' ? 'number' : 'text' })}
                        options={[
                          { value: 'text', label: t('editor.text') },
                          { value: 'number', label: t('editor.number') },
                        ]}
                        ariaLabel={t('formBuilder.inputType')}
                      />
                    </label>
                    <label className="block">
                      <span className="mb-1 block text-[10px] font-medium text-zinc-500">
                        {t('formBuilder.placeholder')}
                      </span>
                      <Input
                        value={selected.placeholder ?? ''}
                        onChange={(event) => updateItem(selected.id, { placeholder: event.target.value })}
                        aria-label={t('formBuilder.placeholder')}
                      />
                    </label>
                  </>
                ) : null}
                {selected.kind === 'dropdown' ? (
                  <div className="space-y-1.5">
                    <span className="block text-[10px] font-medium text-zinc-500">
                      {t('formBuilder.options')}
                    </span>
                    {(selected.options ?? []).map((option, index) => (
                      <div key={`${selected.id}-${index}`} className="space-y-1 rounded-md border border-zinc-800 bg-zinc-900/60 p-1.5">
                        <Input
                          value={option.value}
                          onChange={(event) => updateOption(selected.id, index, { value: event.target.value })}
                          aria-label={t('formBuilder.optionValue')}
                          placeholder={t('formBuilder.optionValue')}
                        />
                        <Input
                          value={option.label ?? ''}
                          onChange={(event) => updateOption(selected.id, index, { label: event.target.value })}
                          aria-label={t('formBuilder.optionLabel')}
                          placeholder={t('formBuilder.optionLabel')}
                        />
                        <Button
                          type="button"
                          size="sm"
                          variant="ghost"
                          className="h-6 px-2 text-[11px] text-zinc-500 hover:text-red-300"
                          onClick={() => removeOption(selected.id, index)}
                        >
                          <Trash2 className="size-3" />
                          {t('formBuilder.deleteItem')}
                        </Button>
                      </div>
                    ))}
                    <Button
                      type="button"
                      size="sm"
                      variant="ghost"
                      onClick={() => addOption(selected.id)}
                    >
                      <Plus className="size-3.5" />
                      {t('formBuilder.addOption')}
                    </Button>
                  </div>
                ) : null}
              </div>
            ) : (
              <p className="text-[11px] text-zinc-500">{t('formBuilder.noSelection')}</p>
            )}
          </aside>
        </div>

        {/* Footer */}
        <div className="flex justify-end border-t border-zinc-800 px-5 py-3">
          <Button onClick={onClose}>
            {t('common.done', 'Done')}
          </Button>
        </div>
      </section>
    </div>,
    document.body,
  )
}
