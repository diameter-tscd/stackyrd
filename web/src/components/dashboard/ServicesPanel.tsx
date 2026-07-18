import { useEffect, useState } from 'react'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { Boxes } from 'lucide-react'
import { getDependencies, type DepsResponse } from '@/api/client'

export function ServicesPanel() {
  const [data, setData] = useState<DepsResponse | null>(null)
  const [loading, setLoading] = useState(true)

  async function load() {
    try {
      const d = await getDependencies()
      setData(d)
    } catch {
      setData(null)
    }
    setLoading(false)
  }

  useEffect(() => { load(); const t = setInterval(load, 30000); return () => clearInterval(t) }, [])

  return (
    <Card>
      <CardHeader>
        <div className='flex items-center gap-3'>
          <div className='w-8 h-8 rounded-lg bg-surface-800 flex items-center justify-center'>
            <Boxes className='w-4 h-4 text-accent-400' />
          </div>
          <div>
            <CardTitle>Registered Services & Dependencies</CardTitle>
            <p className='text-sm text-surface-400 mt-1'>
              Auto-discovered at boot from serviceFactories sync.Map
            </p>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        {loading ? (
          <div className='grid grid-cols-1 lg:grid-cols-2 gap-6'>
            <Skeleton className='h-40 w-full' />
            <Skeleton className='h-40 w-full' />
          </div>
        ) : !data ? (
          <p className='text-surface-500 text-sm'>Could not load dependency info</p>
        ) : (
          <div className='grid grid-cols-1 lg:grid-cols-2 gap-6'>
            <div>
              <div className='flex items-center justify-between mb-3'>
                <h4 className='text-sm font-semibold text-surface-200'>Services</h4>
                <Badge variant='secondary'>{data.total_service} total</Badge>
              </div>
              <div className='space-y-1'>
                {data.list_service.length === 0 ? (
                  <p className='text-surface-500 text-xs'>No registered services (enable in config.yaml)</p>
                ) : (
                  data.list_service.map((s) => (
                    <div key={s} className='flex items-center gap-2 px-3 py-2 rounded-lg bg-surface-800/30 text-sm text-surface-300'>
                      <div className='w-1.5 h-1.5 rounded-full bg-accent-500' />
                      {s}
                    </div>
                  ))
                )}
              </div>
            </div>
            <div>
              <div className='flex items-center justify-between mb-3'>
                <h4 className='text-sm font-semibold text-surface-200'>Infrastructure</h4>
                <Badge variant='secondary'>{data.total_infrastructure} total</Badge>
              </div>
              <div className='space-y-1'>
                {data.list_infrastructure.length === 0 ? (
                  <p className='text-surface-500 text-xs'>No infrastructure dependencies</p>
                ) : (
                  data.list_infrastructure.map((s) => (
                    <div key={s} className='flex items-center gap-2 px-3 py-2 rounded-lg bg-surface-800/30 text-sm text-surface-300'>
                      <div className='w-1.5 h-1.5 rounded-full bg-accent-500' />
                      {s}
                    </div>
                  ))
                )}
              </div>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
