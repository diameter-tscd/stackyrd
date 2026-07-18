export interface HealthResponse {
  status: string
  server_ready: boolean
  infrastructure: Record<string, InfraStatus>
  initialization_progress: number
}

export interface InfraStatus {
  name: string
  initialized: boolean
  error?: string
  start_time: string
  duration: number
  progress: number
}

export interface DepsResponse {
  total_infrastructure: number
  list_infrastructure: string[]
  total_service: number
  list_service: string[]
}

export interface ResourcesResponse {
  memory_usage: number
  routine_running: number
}

const BASE = '/api/v1'

async function fetchJson<T>(url: string): Promise<T> {
  const res = await fetch(`${BASE}${url}`)
  const body = await res.json()
  if (!res.ok) throw new Error(body.error?.message || res.statusText)
  return body.data as T
}

export function getHealth() {
  return fetchJson<HealthResponse>('/health')
}

export function getDependencies() {
  return fetchJson<DepsResponse>('/health/dependencies')
}

export function getResources() {
  return fetchJson<ResourcesResponse>('/health/resources')
}
