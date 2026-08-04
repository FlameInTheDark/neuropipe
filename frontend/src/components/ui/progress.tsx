import type { HTMLAttributes } from 'react'
import { cn } from '@/lib/utils'

interface ProgressProps extends HTMLAttributes<HTMLDivElement> {
  value: number
  indicatorClassName?: string
}

export function Progress({ value, className, indicatorClassName, ...props }: ProgressProps) {
  const percentage = Math.max(0, Math.min(100, Math.round(value)))

  return <div className={cn('h-1.5 w-full overflow-hidden rounded-full bg-zinc-800', className)} role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={percentage} {...props}>
    <div className={cn('h-full rounded-full bg-white transition-[width] duration-150 ease-out', indicatorClassName)} style={{ width: `${percentage}%` }} />
  </div>
}
