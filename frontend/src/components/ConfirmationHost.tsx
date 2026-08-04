import { ConfirmDialog } from '@/components/ui/confirm-dialog'
import { useConfirmationStore } from '@/stores/confirmation'

/** Renders the one shared confirmation modal above all application views. */
export function ConfirmationHost() {
  const request = useConfirmationStore((state) => state.request)
  const respond = useConfirmationStore((state) => state.respond)
  return <ConfirmDialog
    open={Boolean(request)}
    title={request?.title ?? ''}
    description={request?.description ?? ''}
    confirmLabel={request?.confirmLabel ?? 'Confirm'}
    onConfirm={() => respond(true)}
    onOpenChange={(open) => { if (!open) respond(false) }}
  />
}
