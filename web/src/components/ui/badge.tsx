import * as React from 'react'
import { cn } from '@/lib/utils'

export interface BadgeProps extends React.HTMLAttributes<HTMLDivElement> {
  variant?: 'default' | 'secondary' | 'destructive' | 'outline' | 'success' | 'warning'
}

const badgeVariants = {
  default: 'border-transparent bg-accent-600/20 text-accent-400',
  secondary: 'border-transparent bg-surface-700 text-surface-200',
  destructive: 'border-transparent bg-red-600/20 text-red-400',
  outline: 'text-surface-300 border border-surface-600',
  success: 'border-transparent bg-accent-600/20 text-accent-400',
  warning: 'border-transparent bg-amber-600/20 text-amber-400',
}

function Badge({ className, variant = 'default', ...props }: BadgeProps) {
  return (
    <div
      className={cn(
        'inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-semibold transition-colors focus:outline-none focus:ring-2 focus:ring-surface-950 focus:ring-offset-2',
        badgeVariants[variant],
        className
      )}
      {...props}
    />
  )
}

export { Badge }