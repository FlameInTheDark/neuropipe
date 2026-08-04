import * as PopoverPrimitive from '@radix-ui/react-popover'
import { forwardRef, type ComponentPropsWithoutRef, type ElementRef } from 'react'
import { cn } from '@/lib/utils'

export const Popover = PopoverPrimitive.Root
export const PopoverTrigger = PopoverPrimitive.Trigger

export const PopoverContent = forwardRef<
  ElementRef<typeof PopoverPrimitive.Content>,
  ComponentPropsWithoutRef<typeof PopoverPrimitive.Content>
>(({ className, sideOffset = 8, ...props }, ref) => <PopoverPrimitive.Portal><PopoverPrimitive.Content ref={ref} sideOffset={sideOffset} className={cn('z-50 rounded-lg border border-zinc-700 bg-zinc-950 text-zinc-200 shadow-2xl shadow-black/60 outline-none data-[state=open]:animate-in data-[state=closed]:animate-out', className)} {...props} /></PopoverPrimitive.Portal>)
PopoverContent.displayName = PopoverPrimitive.Content.displayName
