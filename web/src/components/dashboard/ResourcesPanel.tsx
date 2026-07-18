import { useEffect, useState } from 'react'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Cpu, HardDrive, Activity } from 'lucide-react'
import { getResources, type ResourcesResponse } from '@/api/client'

function fmtBytes(b: number): string {
  if (b < 1024) return `${b.toFixed(1)} B`
  if (b < 1024 ** 2) return `${(b / 1024).toFixed(1)} KB`
  if (b < 1024 ** 3) return `${(b / 1024 ** 2).toFixed(1)} MB`
  return `${(b / 1024 ** 3).toFixed(2)} GB`
}

function gaugePercent(value: number, max: number): number {
  if (max === 0) return 0
  return Math.min((value / max) * 100, 100)
}

export function ResourcesPanel() {
  const [data, setData] = useState<ResourcesResponse | null>(null)
  const [loading, setLoading] = useState(true)

  async function load() {
    try {
      const r = await getResources()
      setData(r)
    } catch { /* noop */ }
    setLoading(false)
  }

  useEffect(() => { load(); const t = setInterval(load, 10000); return () => clearInterval(t) }, [])

  if (loading && !data) {
    return (
      <Card>
        <CardHeader><CardTitle>Runtime Resources</CardTitle></CardHeader>
        <CardContent>
          <div className='grid grid-cols-1 sm:grid-cols-2 gap-6'>
            <Skeleton className='h-24 w-full' />
            <Skeleton className='h-24 w-full' />
          </div>
        </CardContent>
      </Card>
    )
  }

  const memory = data?.memory_usage ?? 0
  // Assume 256MB = typical dev box or container cap for the gauge gaugePercent
  const memPercent = gaugePercent(memory, 256 * 1024 * 1024)
  const goroutines = data?.routine_running ?? 0

  return (
    <Card>
      <CardHeader>
        <div className='flex items-center gap-3'>
          <div className='w-8 h-8 rounded-lg bg-surface-800 flex items-center justify-center'>
            <Activity className='w-4 h-4 text-accent-400' />
          </div>
          <div>
            <CardTitle>Runtime Resources</CardTitle>
            <p className='text-sm text-surface-400 mt-1'>Live process metrics</p>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <div className='grid grid-cols-1 sm:grid-cols-2 gap-6'>
          <div className='space-y-3 p-4 rounded-xl bg-surface-800/30 border border-surface-700/50'>
            <div className='flex items-center gap-2'>
              <Cpu className='w-4 h-4 text-accent-400' />
              <span className='text-sm font-medium text-surface-200'>Memory</span>
            </div>
            <p className='text-xl font-bold text-surface-100'>{fmtBytes(memory)}</p>
            <div className='w-full h-1.5 rounded-full bg-surface-700 overflow-hidden'>
              <div
                className={`h-full rounded-full transition-all duration-500 ${
                  memPercent > 80 ? 'bg-red-500' : memPercent > 60 ? 'bg-amber-500' : 'bg-accent-500'
                }`}
                style={{ width: `${memPercent}%` }}
              />
            </div>
            <p className='text-xs text-surface-500'>{memPercent.toFixed(0)}% of 256 MB</p>
          </div>
          <div className='space-y-3 p-4 rounded-xl bg-surface-800/30 border border-surface-700/50'>
            <div className='flex items-center gap-2'>
              <HardDrive className='w-4 h-4 text-accent-400' />
              <span className='text-sm font-medium text-surface-200'>Goroutines</span>
            </div>
            <p className='text-xl font-bold text-surface-100'>{goroutines}</p>
            <div className='w-full h-1.5 rounded-full bg-surface-700 overflow-hidden'>
              <div
                className={`h-full rounded-full transition-all duration-500 ${
                  goroutines > 1000 ? 'bg-red-500' : goroutines > 500 ? 'bg-amber-500' : 'bg-accent-500'
                }`}
                style={{ width: `${Math.min((goroutines / 2000) * 100, 100)}%` }}
              />
            </div>
            <p className='text-xs text-surface-500'>Goroutine count</p>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
