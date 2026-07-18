import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'
import { BookOpen } from 'lucide-react'

const endpoints = [
  { path: 'GET /health', desc: 'Aggregate health (infra + readiness)', section: 'observability' },
  { path: 'GET /health/infrastructure', desc: 'Infra component statuses', section: 'observability' },
  { path: 'GET /health/dependencies', desc: 'Registered services + infra deps', section: 'observability' },
  { path: 'GET /health/resources', desc: 'Memory usage + goroutines', section: 'observability' },
  { path: 'GET /metrics', desc: 'Prometheus metrics', section: 'observability' },
  { path: 'GET /swagger/index.html', desc: 'Swagger UI docs', section: 'docs' },
  { path: 'GET /api/v1/users', desc: 'List users', section: 'services' },
  { path: 'GET /api/v1/users/:id', desc: 'Get user by ID', section: 'services' },
  { path: 'POST /api/v1/users', desc: 'Create user', section: 'services' },
  { path: 'PUT /api/v1/users/:id', desc: 'Update user', section: 'services' },
  { path: 'GET /api/v1/tasks', desc: 'List tasks', section: 'services' },
  { path: 'POST /api/v1/tasks', desc: 'Create task', section: 'services' },
  { path: 'PUT /api/v1/tasks/:id', desc: 'Update task', section: 'services' },
  { path: 'DELETE /api/v1/tasks/:id', desc: 'Delete task', section: 'services' },
  { path: 'GET /api/v1/products', desc: 'List products', section: 'services' },
  { path: 'GET /api/v1/products/:id', desc: 'Get product', section: 'services' },
  { path: 'GET /api/v1/orders', desc: 'List orders (multi-tenant)', section: 'services' },
  { path: 'GET /api/v1/cache/:key', desc: 'Get cached value', section: 'services' },
  { path: 'POST /api/v1/cache/:key', desc: 'Set cached value', section: 'services' },
  { path: 'POST /api/v1/encrypt', desc: 'Encrypt data', section: 'services' },
  { path: 'POST /api/v1/decrypt', desc: 'Decrypt data', section: 'services' },
  { path: 'POST /api/v1/key-rotate', desc: 'Rotate encryption key', section: 'services' },
  { path: 'POST /api/v1/webhook', desc: 'Webhook receiver', section: 'services' },
].sort((a, b) => a.path.localeCompare(b.path))

const sectionColors: Record<string, 'success' | 'default' | 'secondary'> = {
  observability: 'success',
  services: 'default',
  docs: 'secondary',
}

const sectionLabels: Record<string, string> = {
  observability: 'Observability',
  services: 'Business',
  docs: 'Documentation',
}

export function QuickReference() {
  const grouped = endpoints.reduce<Record<string, typeof endpoints>>((acc, ep) => {
    if (!acc[ep.section]) acc[ep.section] = []
    acc[ep.section].push(ep)
    return acc
  }, {})

  return (
    <Card>
      <CardHeader>
        <CardTitle>API Quick Reference</CardTitle>
        <p className='text-sm text-surface-400 mt-1'>
          <BookOpen className='w-3 h-3 inline mr-1' />
          {endpoints.length} endpoints available via Echo router
        </p>
      </CardHeader>
      <CardContent>
        {Object.entries(grouped).map(([section, eps], idx) => (
          <div key={section}>
            {idx > 0 && <Separator className='my-4' />}
            <div className='flex items-center gap-2 mb-3'>
              <Badge variant={sectionColors[section]}>{sectionLabels[section]}</Badge>
              <span className='text-xs text-surface-500'>{eps.length} endpoint{eps.length > 1 ? 's' : ''}</span>
            </div>
            <div className='space-y-1.5'>
              {eps.map((ep) => (
                <div
                  key={ep.path}
                  className='flex items-center justify-between px-3 py-2 rounded-lg bg-surface-800/20 hover:bg-surface-800/40 transition-colors'
                >
                  <code className='text-xs font-mono text-surface-300'>{ep.path}</code>
                  <span className='text-xs text-surface-500 ml-4'>{ep.desc}</span>
                </div>
              ))}
            </div>
          </div>
        ))}
      </CardContent>
    </Card>
  )
}
