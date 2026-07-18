import {
  LayoutDashboard,
  Boxes,
  Network,
  Cpu,
  Cog,
  ChevronDown,
} from 'lucide-react'
import { cn } from '@/lib/utils'

const nav = [
  { label: 'Overview', icon: LayoutDashboard, href: '#overview', active: true },
  { label: 'Services', icon: Boxes, href: '#services' },
  { label: 'Infrastructure', icon: Network, href: '#infrastructure' },
  { label: 'Resources', icon: Cpu, href: '#resources' },
  { label: 'Plugins', icon: Cog, href: '#plugins' },
]

export function Sidebar() {
  return (
    <aside className='w-64 min-h-screen glass border-r border-surface-800 p-4 flex flex-col'>
      <div className='flex items-center gap-3 px-2 py-4 mb-6'>
        <div className='w-8 h-8 rounded-lg bg-gradient-to-br from-accent-400 to-accent-600 flex items-center justify-center text-xs font-bold text-white'>
          S
        </div>
        <div className='flex flex-col'>
          <span className='text-sm font-semibold text-surface-100'>stackyrd</span>
          <span className='text-xs text-surface-500'>v1.0</span>
        </div>
      </div>

      <nav className='flex flex-col gap-1 flex-1'>
        {nav.map((item) => (
          <a
            key={item.label}
            href={item.href}
            className={cn(
              'sidebar-item',
              item.active && 'active'
            )}
            onClick={(e) => {
              e.preventDefault()
              document.getElementById(item.href.slice(1))?.scrollIntoView({ behavior: 'smooth' })
            }}
          >
            <item.icon className='w-4 h-4' />
            {item.label}
          </a>
        ))}
      </nav>

      <div className='border-t border-surface-800 pt-4 mt-4'>
        <div className='flex items-center gap-3 px-2'>
          <div className='w-7 h-7 rounded-full bg-surface-700 flex items-center justify-center text-xs font-medium text-surface-300'>
            S
          </div>
          <div className='flex-1 min-w-0'>
            <p className='text-sm text-surface-300 truncate'>Gab.</p>
            <p className='text-xs text-surface-500'>maintainer</p>
          </div>
          <ChevronDown className='w-4 h-4 text-surface-500' />
        </div>
      </div>
    </aside>
  )
}
