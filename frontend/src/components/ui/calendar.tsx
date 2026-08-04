import { DayFlag, DayPicker, SelectionState, UI, type DayPickerProps } from 'react-day-picker'
import { cn } from '@/lib/utils'

export type CalendarProps = DayPickerProps

/** A themed DayPicker foundation for app-owned calendar popovers. */
export function Calendar({ className, classNames, showOutsideDays = true, ...props }: CalendarProps) {
  return <DayPicker
    showOutsideDays={showOutsideDays}
    className={cn('p-3', className)}
    classNames={{
      [UI.Root]: 'w-[18rem]',
      [UI.Months]: 'flex flex-col',
      [UI.Month]: 'relative space-y-3',
      [UI.MonthCaption]: 'flex h-8 items-center justify-center px-9',
      [UI.CaptionLabel]: 'text-sm font-medium text-zinc-100',
      [UI.Nav]: 'hidden',
      [UI.PreviousMonthButton]: 'absolute left-0 top-0 inline-flex size-8 items-center justify-center rounded-md text-zinc-500 transition-colors hover:bg-zinc-800 hover:text-zinc-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-zinc-500',
      [UI.NextMonthButton]: 'absolute right-0 top-0 inline-flex size-8 items-center justify-center rounded-md text-zinc-500 transition-colors hover:bg-zinc-800 hover:text-zinc-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-zinc-500',
      [UI.Chevron]: 'size-4 fill-none stroke-current',
      [UI.MonthGrid]: 'w-full border-collapse',
      [UI.Weekdays]: 'flex',
      [UI.Weekday]: 'w-9 text-center text-[10px] font-medium text-zinc-600',
      [UI.Weeks]: 'block',
      [UI.Week]: 'mt-1 flex w-full',
      [UI.Day]: 'relative size-9 p-0 text-center',
      [UI.DayButton]: 'inline-flex size-9 items-center justify-center rounded-md text-xs text-zinc-300 transition-colors hover:bg-zinc-800 hover:text-zinc-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-zinc-500',
      [DayFlag.outside]: 'text-zinc-700 opacity-60',
      [DayFlag.today]: 'font-semibold text-zinc-100',
      // DayPicker applies range state classes to the table cell, while hover
      // styling belongs to the nested button. Keeping the background on one
      // element prevents a second colour from peeking out around endpoints.
      [SelectionState.selected]: '[&>button]:bg-zinc-100 [&>button]:text-zinc-950 [&>button:hover]:bg-white [&>button:hover]:text-zinc-950',
      [SelectionState.range_start]: '[&>button]:rounded-l-md [&>button]:bg-zinc-100 [&>button]:text-zinc-950 [&>button:hover]:bg-white [&>button:hover]:text-zinc-950',
      [SelectionState.range_middle]: '[&>button]:rounded-none [&>button]:bg-zinc-800 [&>button]:text-zinc-100 [&>button:hover]:bg-zinc-700 [&>button:hover]:text-zinc-100',
      [SelectionState.range_end]: '[&>button]:rounded-r-md [&>button]:bg-zinc-100 [&>button]:text-zinc-950 [&>button:hover]:bg-white [&>button:hover]:text-zinc-950',
      ...classNames,
    }}
    navLayout="around"
    {...props}
  />
}
