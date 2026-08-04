import { cn } from '@/lib/utils'

interface SwitchProps { checked: boolean; disabled?: boolean; onCheckedChange: (checked: boolean) => void; label: string }
export function Switch({ checked, disabled, onCheckedChange, label }: SwitchProps) {
  return <button type="button" aria-label={label} role="switch" aria-checked={checked} disabled={disabled} onClick={() => onCheckedChange(!checked)} className={cn('relative h-5 w-9 rounded-full border p-0.5 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/50 disabled:cursor-not-allowed disabled:opacity-50', checked ? 'border-sky-400/80 bg-sky-500' : 'border-zinc-700 bg-zinc-800')}><span className={cn('block size-4 rounded-full bg-white shadow-sm transition-transform', checked ? 'translate-x-4' : 'translate-x-0')} /></button>
}
