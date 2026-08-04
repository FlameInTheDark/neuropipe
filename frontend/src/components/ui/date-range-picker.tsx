import { useState } from 'react'
import { CalendarDays, X } from 'lucide-react'
import { type DateRange } from 'react-day-picker'
import { Button } from '@/components/ui/button'
import { Calendar } from '@/components/ui/calendar'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Select } from '@/components/ui/select'
import { cn } from '@/lib/utils'

interface DateRangePickerProps {
  from: string
  to: string
  onChange: (range: { from: string; to: string }) => void
  className?: string
}

function dateFromInput(value: string): Date | undefined {
	const match = /^(\d{4}-\d{2}-\d{2})(?:T\d{2}:\d{2})?$/.exec(value)
	if (!match) return undefined
	const date = new Date(`${match[1]}T12:00:00`)
	return Number.isNaN(date.getTime()) ? undefined : date
}

function timeFromInput(value: string, fallback: string): string {
	const match = /^\d{4}-\d{2}-\d{2}T(\d{2}:\d{2})$/.exec(value)
	return match ? match[1] : fallback
}

function dateToInput(value: Date | undefined, time: string): string {
	if (!value) return ''
	const year = value.getFullYear()
	const month = String(value.getMonth() + 1).padStart(2, '0')
	const day = String(value.getDate()).padStart(2, '0')
	return `${year}-${month}-${day}T${time}`
}

function formatDate(value?: Date): string {
	return value ? new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric', year: 'numeric' }).format(value) : ''
}

const timeOptions = [...Array.from({ length: 96 }, (_, index) => {
	const hours = String(Math.floor(index / 4)).padStart(2, '0')
	const minutes = String((index % 4) * 15).padStart(2, '0')
	const value = `${hours}:${minutes}`
	return { value, label: value }
}), { value: '23:59', label: '23:59' }]

/** A shadcn-style popover/calendar range picker with no browser-native date UI. */
export function DateRangePicker({ from, to, onChange, className }: DateRangePickerProps) {
  const [open, setOpen] = useState(false)
  const fromDate = dateFromInput(from)
  const toDate = dateFromInput(to)
	const selected: DateRange | undefined = fromDate ? { from: fromDate, to: toDate } : undefined
	const [draft, setDraft] = useState<DateRange | undefined>()
	const [fromTime, setFromTime] = useState('00:00')
	const [toTime, setToTime] = useState('23:59')
	const label = fromDate && toDate ? `${formatDate(fromDate)} ${timeFromInput(from, '00:00')} – ${formatDate(toDate)} ${timeFromInput(to, '23:59')}` : fromDate ? `${formatDate(fromDate)} ${timeFromInput(from, '00:00')} – Select end` : 'Created date range'
  const selectionHint = !draft?.from ? 'Select a start date' : !draft.to ? 'Now select an end date' : `${formatDate(draft.from)} – ${formatDate(draft.to)}`

	const apply = () => {
		onChange({ from: dateToInput(draft?.from, fromTime), to: dateToInput(draft?.to, toTime) })
		setOpen(false)
	}

	const changeOpen = (next: boolean) => {
		if (next) {
			setDraft(selected)
			setFromTime(timeFromInput(from, '00:00'))
			setToTime(timeFromInput(to, '23:59'))
		}
		setOpen(next)
  }

  return <Popover open={open} onOpenChange={changeOpen}>
    <PopoverTrigger asChild>
      <Button variant="outline" className={cn('h-8 justify-start px-2.5 text-xs font-normal', !fromDate && 'text-zinc-500', className)} aria-label="Filter reports by created date range">
        <CalendarDays className="size-3.5 shrink-0 text-zinc-500" />
        <span className="truncate">{label}</span>
      </Button>
    </PopoverTrigger>
    <PopoverContent align="start" className="w-auto p-0">
      <div className="flex items-center justify-between border-b border-zinc-800 px-3 py-2">
        <span className="text-xs font-medium text-zinc-300">{selectionHint}</span>
		<button type="button" onClick={() => setDraft(undefined)} disabled={!draft?.from} className="inline-flex items-center gap-1 rounded px-1.5 py-1 text-[11px] text-zinc-500 hover:bg-zinc-800 hover:text-zinc-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-zinc-500 disabled:cursor-not-allowed disabled:opacity-40"><X className="size-3" />Reset</button>
      </div>
      <Calendar mode="range" selected={draft} defaultMonth={draft?.from ?? fromDate ?? toDate} onSelect={(range) => setDraft(range as DateRange | undefined)} />
	  <div className="grid grid-cols-2 gap-3 border-t border-zinc-800 px-3 py-3">
		<div className="space-y-1.5"><span className="text-[10px] font-medium uppercase tracking-[0.12em] text-zinc-600">Start time</span><Select value={fromTime} onValueChange={setFromTime} options={timeOptions} disabled={!draft?.from} ariaLabel="Report range start time" /></div>
		<div className="space-y-1.5"><span className="text-[10px] font-medium uppercase tracking-[0.12em] text-zinc-600">End time</span><Select value={toTime} onValueChange={setToTime} options={timeOptions} disabled={!draft?.to} ariaLabel="Report range end time" /></div>
	  </div>
      <div className="flex items-center justify-between border-t border-zinc-800 px-3 py-2">
        <button type="button" onClick={() => { onChange({ from: '', to: '' }); setDraft(undefined); setOpen(false) }} className="inline-flex items-center gap-1 rounded px-1.5 py-1 text-[11px] text-zinc-500 hover:bg-zinc-800 hover:text-zinc-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-zinc-500"><X className="size-3" />Clear filter</button>
        <div className="flex items-center gap-2">
          <Button size="sm" variant="ghost" onClick={() => setOpen(false)}>Cancel</Button>
          <Button size="sm" onClick={apply} disabled={!draft?.from}>Apply</Button>
        </div>
      </div>
    </PopoverContent>
  </Popover>
}
