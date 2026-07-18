export interface StatItem {
  label: string
  value: string | number
  sub?: string
  icon: 'server' | 'activity' | 'wifi' | 'drive'
  status: 'ok' | 'warn' | 'err'
}