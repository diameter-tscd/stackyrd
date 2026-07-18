import { Server, Activity, Wifi, HardDrive } from 'lucide-react'
import type { StatItem } from '@/types'

const icons = {
  server: Server,
  activity: Activity,
  wifi: Wifi,
  drive: HardDrive,
}

export function StatCards({ stats }: { stats: StatItem[] }) {
  return (
    <div className='grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4'>
      {stats.map((s) => {
        const Icon = icons[s.icon]
        const isWarn = s.status === 'warn'
        return (
          <div
            key={s.label}
            className={`rounded-xl p-5 ${isWarn ? 'stat-card-warn' : 'stat-card'} glass-hover`}
          >
            <div className='flex items-start justify-between mb-3'>
              <div className='w-9 h-9 rounded-lg bg-surface-800 flex items-center justify-center'>
                <Icon className={`w-5 h-5 ${isWarn ? 'text-amber-400' : 'text-accent-400'}`} />
              </div>
              <span className={`api-badge ${isWarn ? 'text-amber-400 bg-amber-600/10' : 'text-accent-400 bg-accent-600/10'}`}>
                <span className={`live-dot ${isWarn ? 'bg-amber-400' : 'bg-accent-400'}`} />
                {s.status === 'ok' ? 'Live' : s.status === 'warn' ? 'Warning' : 'Error'}
              </span>
            </div>
            <p className='text-2xl font-bold text-surface-100 tracking-tight'>{s.value}</p>
            <p className='text-sm text-surface-400 mt-0.5'>{s.label}</p>
            {s.sub && (
              <p className='text-xs text-surface-500 mt-1.5'>{s.sub}</p>
            )}
          </div>
        )
      })}
    </div>
  )
}
