import { useEffect, useState } from 'react'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { Button } from '@/components/ui/button'
import { RefreshCw } from 'lucide-react'
import { getHealth, type HealthResponse } from '@/api/client'

export function InfrastructurePanel() {
  const [data, setData] = useState<HealthResponse | null>(null)
  const [loading, setLoading] = useState(true)

  async function load() {
    setLoading(true)
    try {
      const h = await getHealth()
      setData(h)
    } catch {
      setData(null)
    }
    setLoading(false)
  }

  useEffect(() => { load(); const t = setInterval(load, 15000); return () => clearInterval(t) }, [])

  const components = data?.infrastructure ? Object.entries(data.infrastructure) : []

  return (
    <Card>
      <CardHeader>
        <div className='flex items-center justify-between'>
          <div>
            <CardTitle>Infrastructure Components</CardTitle>
            <p className='text-sm text-surface-400 mt-1'>
              Health-checked on startup with 10s timeout per component
            </p>
          </div>
          <Button variant='ghost' size='icon' onClick={load} disabled={loading}>
            <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        {loading && !data ? (
          <div className='space-y-3'>
            {[1,2,3,4].map(i => <Skeleton key={i} className='h-10 w-full' />)}
          </div>
        ) : components.length === 0 ? (
          <div className='text-center py-8'>
            <p className='text-surface-500'>No infrastructure components initialized</p>
            <p className='text-xs text-surface-600 mt-1'>Enable components (redis / postgres / mongo / kafka) in config.yaml</p>
          </div>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Component</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Progress</TableHead>
                <TableHead>Duration</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {components.map(([name, comp]) => (
                <TableRow key={name}>
                  <TableCell className='font-medium text-surface-100 capitalize'>{name}</TableCell>
                  <TableCell>
                    <Badge variant={comp.initialized ? 'success' : 'warning'}>
                      {comp.initialized ? 'Connected' : comp.error ? comp.error : 'Pending'}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    <div className='flex items-center gap-2'>
                      <div className='w-24 h-1.5 rounded-full bg-surface-700 overflow-hidden'>
                        <div
                          className='h-full rounded-full bg-accent-500 transition-all duration-500'
                          style={{ width: `${comp.progress * 100}%` }}
                        />
                      </div>
                      <span className='text-xs text-surface-400'>
                        {Math.round(comp.progress * 100)}%
                      </span>
                    </div>
                  </TableCell>
                  <TableCell className='text-surface-400 font-mono text-xs'>
                    {comp.duration ? `${(comp.duration / 1e9).toFixed(2)}s` : '-'}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  )
}
