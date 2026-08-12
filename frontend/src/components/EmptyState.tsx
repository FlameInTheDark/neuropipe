import type { LucideIcon } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

export function EmptyState({ icon: Icon, title, description, action, variant = 'surface' }: { icon: LucideIcon; title: string; description: string; action?: { label: string; onClick: () => void }; variant?: 'surface' | 'plain' }) {
  return <div className={cn('flex flex-col items-center justify-center text-center', variant === 'surface' ? 'surface min-h-72 rounded-xl p-8' : 'px-6 py-16')}><div className={cn('mb-4 flex size-11 items-center justify-center rounded-lg text-zinc-300', variant === 'surface' ? 'bg-zinc-800' : 'border border-zinc-800 bg-zinc-900/70')}><Icon className="size-5" /></div><h2 className="text-sm font-semibold">{title}</h2><p className="mt-2 max-w-sm text-sm leading-6 text-zinc-500">{description}</p>{action && <Button className="mt-5" onClick={action.onClick}>{action.label}</Button>}</div>
}
