export interface ResponseEnvelope<T> {
  data: T
  meta: { request_id: string }
}

export interface PageData<T> {
  items: T[]
  page: number
  page_size: number
  total: number
}

export interface User {
  id: string
  username: string
  display_name: string
  email: string | null
  role: 'admin' | 'member'
  status: 'active' | 'disabled'
  lock_version: number
  created_at?: string
  updated_at?: string
}

export interface SessionPayload {
  user: User
  permissions: Record<string, boolean>
  features: { oa_login_enabled: boolean }
  upload_policy: {
    allowed_media_types: string[]
    max_file_size: number
    max_files_per_owner: number
  }
  csrf_token: string
}

export interface CatalogItem {
  id: string
  name: string
  description: string | null
  status: 'active' | 'disabled'
  lock_version: number
  created_at?: string
  updated_at?: string
}

export interface Scenario extends CatalogItem {
  evaluation_target_id: string
  evaluation_target_name?: string
}

export interface Tag extends CatalogItem {
  scenario_id?: string
  scenario_name?: string
  color?: string | null
  sort_order: number
}

export interface AuditLog {
  id: string
  actor_user_id: string | null
  action: string
  entity_type: string
  entity_id: string
  before_data: unknown
  after_data: unknown
  request_id: string
  ip_address: string
  user_agent: string
  created_at: string
}
