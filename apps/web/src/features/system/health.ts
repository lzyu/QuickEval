import { apiClient } from '@/api/client'

export interface HealthStatus {
  status: 'ok'
  dependencies: Record<string, 'ok'>
}

interface HealthEnvelope {
  data: HealthStatus
  meta: {
    request_id: string
  }
}

export async function fetchReadiness(): Promise<HealthEnvelope> {
  const response = await apiClient.get<HealthEnvelope>('/health/ready')
  return response.data
}
