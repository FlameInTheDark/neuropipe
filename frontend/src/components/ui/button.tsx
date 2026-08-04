import { Slot } from '@radix-ui/react-slot'
import { cva, type VariantProps } from 'class-variance-authority'
import { forwardRef, type ButtonHTMLAttributes } from 'react'
import { cn } from '@/lib/utils'

const buttonVariants = cva('inline-flex h-8 items-center justify-center gap-2 rounded-md px-3 text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/30 disabled:pointer-events-none disabled:opacity-50', {
  variants: {
    variant: {
      default: 'bg-white text-zinc-950 hover:bg-zinc-200',
      secondary: 'bg-zinc-800 text-zinc-100 hover:bg-zinc-700',
      ghost: 'text-zinc-400 hover:bg-zinc-800 hover:text-zinc-100',
      outline: 'border border-zinc-700 bg-transparent text-zinc-200 hover:bg-zinc-800',
      danger: 'bg-red-500 text-white hover:bg-red-400',
    },
    size: { default: 'h-8', sm: 'h-7 px-2 text-xs', lg: 'h-10 px-4' },
  },
  defaultVariants: { variant: 'default', size: 'default' },
})

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement>, VariantProps<typeof buttonVariants> { asChild?: boolean }
export const Button = forwardRef<HTMLButtonElement, ButtonProps>(({ className, variant, size, asChild, ...props }, ref) => {
  const Component = asChild ? Slot : 'button'
  return <Component ref={ref} className={cn(buttonVariants({ variant, size }), className)} {...props} />
})
Button.displayName = 'Button'
