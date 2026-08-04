import type { LucideIcon } from 'lucide-react'
import { Button } from '@/components/ui/button'

export function EmptyState({ icon: Icon, title, description, action }: { icon: LucideIcon; title: string; description: string; action?: { label: string; onClick: () => void } }) {
  return <div className="surface flex min-h-72 flex-col items-center justify-center rounded-xl p-8 text-center"><div className="mb-4 flex size-11 items-center justify-center rounded-lg bg-zinc-800 text-zinc-300"><Icon className="size-5" /></div><h2 className="text-sm font-semibold">{title}</h2><p className="mt-2 max-w-sm text-sm leading-6 text-zinc-500">{description}</p>{action && <Button className="mt-5" onClick={action.onClick}>{action.label}</Button>}</div>
}
