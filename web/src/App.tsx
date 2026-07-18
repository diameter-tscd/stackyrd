import { useEffect, useState } from 'react'
import { Sidebar } from '@/components/layout/Sidebar'
import { StatCards } from '@/components/dashboard/StatCards'
import type { StatItem } from '@/types'
import { InfrastructurePanel } from '@/components/dashboard/InfrastructurePanel'
import { ServicesPanel } from '@/components/dashboard/ServicesPanel'
import { ResourcesPanel } from '@/components/dashboard/ResourcesPanel'
import { QuickReference } from '@/components/dashboard/QuickReference'
import { getHealth, getDependencies, getResources } from '@/api/client'
import { Activity, RefreshCw } from 'lucide-react'

export default function App() {
  const [stats, setStats] = useState<StatItem[]>([])
  const [loading, setLoading] = useState(true)
  const [lastSync, setLastSync] = useState<string>('')

  async function computeStats() {
    setLoading(true)
    try {
      const [h, d, r] = await Promise.all([
        getHealth().catch(() => null),
        getDependencies().catch(() => null),
        getResources().catch(() => null),
      ])

      const infraCount = h?.infrastructure ? Object.keys(h.infrastructure).length : 0
      const allHealthy = h?.infrastructure
        ? Object.values(h.infrastructure).every((c) => c.initialized)
        : false

      const memMb = r ? (r.memory_usage / 1024 / 1024).toFixed(1) : '0'
      const goroutines = r?.routine_running ?? 0

      const newStats: StatItem[] = [
        {
          label: 'Services Registered',
          value: d?.total_service ?? 0,
          sub: d?.list_service.length ? `${d.list_service.length} in session` : 'Enable in config.yaml',
          icon: 'server',
          status: 'ok',
        },
        {
          label: 'Infrastructure',
          value: infraCount,
          sub: h?.server_ready ? 'Server ready' : 'Initializing',
          icon: 'wifi',
          status: h?.server_ready ? (allHealthy ? 'ok' : 'warn') : 'warn',
        },
        {
          label: 'Memory Usage',
          value: `${memMb} MB`,
          sub: r ? `${goroutines.toLocaleString()} goroutines` : 'Runtime',
          icon: 'drive',
          status: 'ok',
        },
        {
          label: 'System Status',
          value: h?.status === 'ok' ? 'OK' : h?.status || 'Offline',
          sub: h?.server_ready ? 'Responding' : 'Not ready',
          icon: 'activity',
          status: h?.server_ready ? 'ok' : 'warn',
        },
      ]
      setStats(newStats)
      setLastSync(new Date().toLocaleTimeString())
    } catch {
      setStats([])
    }
    setLoading(false)
  }

  useEffect(() => {
    computeStats()
    const t = setInterval(computeStats, 15000)
    return () => clearInterval(t)
  }, [])

  return (
    <div className='flex min-h-screen bg-surface-950'>
      <Sidebar />
      <main className='flex-1 min-w-0'>
        <header className='sticky top-0 z-10 glass border-b border-surface-800 px-6 py-3 flex items-center justify-between'>
          <div className='flex items-center gap-3'>
            <Activity className='w-5 h-5 text-accent-400' />
            <h1 className='text-lg font-semibold text-surface-100'>Dashboard</h1>
            <span className='api-badge text-accent-400 bg-accent-600/10 ml-2'>
              <span className='live-dot bg-accent-400' />
              Live
            </span>
          </div>
          <div className='flex items-center gap-4 text-sm'>
            {lastSync && (
              <span className='text-surface-500'>
                Last sync: <span className='text-surface-300'>{lastSync}</span>
              </span>
            )}
            <button
              onClick={computeStats}
              disabled={loading}
              className='inline-flex items-center gap-1.5 text-surface-400 hover:text-surface-100 transition-colors disabled:opacity-50'
            >
              <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
              Refresh
            </button>
          </div>
        </header>

        <div className='p-6 space-y-6'>
          <section id='overview' className='scroll-mt-20'>
            <StatCards stats={stats} />
          </section>

          <section id='infrastructure' className='scroll-mt-20'>
            <InfrastructurePanel />
          </section>

          <section id='services' className='scroll-mt-20'>
            <ServicesPanel />
          </section>

          <section id='resources' className='scroll-mt-20'>
            <ResourcesPanel />
          </section>

          <section id='plugins' className='scroll-mt-20'>
            <QuickReference />
          </section>
        </div>
      </main>
    </div>
  )
}
