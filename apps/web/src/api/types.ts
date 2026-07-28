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

export interface Dataset {
  id: string
  scenario_id: string
  scenario_name: string
  evaluation_target_id: string
  evaluation_target_name: string
  name: string
  description: string | null
  status: 'active' | 'archived'
  lock_version: number
  created_at: string
  updated_at: string
  latest_version_no: number | null
  published_version_count: number
  draft_version_id: string | null
  draft_case_count: number
}

export interface DatasetVersion {
  id: string
  dataset_id: string
  base_version_id: string | null
  version_no: number | null
  status: 'draft' | 'published' | 'archived'
  release_note: string | null
  lock_version: number
  published_at: string | null
  archived_at: string | null
  created_at: string
  updated_at: string
  case_count: number
  enabled_count: number
}

export interface DatasetDetail {
  dataset: Dataset
  versions: DatasetVersion[]
}

export interface VersionCase {
  id: string
  dataset_version_id: string
  case_key: string
  name: string | null
  user_prompt: string
  precondition: string | null
  expected_result: string | null
  judging_guide: string | null
  sort_order: number
  is_enabled: boolean
  lock_version: number
  tags: Array<{ id: string; name: string }>
  created_at: string
  updated_at: string
}

export interface ImportPreviewRow {
  row_number: number
  name: string
  user_prompt: string
  precondition: string
  expected_result: string
  judging_guide: string
  tag_names: string[]
  is_enabled: boolean
  errors: Array<{ field: string; message: string }>
}

export interface ImportPreview {
  import_token: string
  version_lock_version: number
  rows: ImportPreviewRow[]
  has_errors: boolean
  valid_row_count: number
  error_row_count: number
}

export type RunStatus = 'in_progress' | 'completed' | 'voided'
export type ResultStatus = 'pending' | 'evaluated' | 'skipped'
export type EvaluationEnvironment = 'test' | 'staging' | 'production' | 'other'

export interface EvaluationProgress {
  total_count: number
  pending_count: number
  evaluated_count: number
  skipped_count: number
  scored_count: number
  badcase_count: number
  average_score: number | null
  completion_rate: number
}

export interface EvaluationRun {
  id: string
  dataset_version_id: string
  dataset_id: string
  dataset_name: string
  version_no: number
  scenario_id: string
  scenario_name: string
  evaluation_target_id: string
  evaluation_target_name: string
  evaluator_id: string
  evaluator_name: string
  agent_version: string
  environment: EvaluationEnvironment
  purpose_note: string | null
  config_note: string | null
  status: RunStatus
  lock_version: number
  first_completed_at: string | null
  completed_at: string | null
  voided_at: string | null
  void_reason: string | null
  created_at: string
  updated_at: string
  progress: EvaluationProgress
}

export interface CaseResult {
  id: string
  evaluation_run_id: string
  version_case_id: string
  case_key: string
  name: string | null
  user_prompt: string
  precondition: string | null
  expected_result: string | null
  judging_guide: string | null
  sort_order: number
  tags: Array<{ id: string; name: string }>
  status: ResultStatus
  answer_text: string | null
  score: number | null
  comment: string | null
  skip_reason: string | null
  has_badcase: boolean
  attachments: Attachment[]
  badcase: BadcaseSummary | null
  lock_version: number
  created_at: string
  updated_at: string
}

export interface EvaluationWorkbench {
  run: EvaluationRun
  results: PageData<CaseResult>
}

export interface Attachment {
  id: string
  original_name: string
  media_type: 'image/png' | 'image/jpeg' | 'image/webp'
  file_size: number
  width: number
  height: number
  sort_order: number
  created_by: string
  created_at: string
  content_url: string
}

export interface BadcaseSummary {
  id: string
  title: string
  description: string | null
  status: string
  issue_tags: Array<{ id: string; name: string }>
}

export interface Badcase {
  id: string
  source_type: 'evaluation' | 'business'
  scenario_id: string
  scenario_name: string
  evaluation_target_id: string
  evaluation_target_name: string
  title: string
  description: string | null
  agent_response_text: string | null
  agent_version: string | null
  environment: EvaluationEnvironment
  occurred_at: string
  business_reference: string | null
  session_id: string | null
  status: 'pending' | 'processing' | 'resolved' | 'deferred'
  assignee_id: string | null
  assignee_name: string | null
  resolved_at: string | null
  invalidated_at: string | null
  invalidated_by: string | null
  invalidator_name: string | null
  invalid_reason: string | null
  lock_version: number
  created_by: string
  creator_name: string
  created_at: string
  updated_at: string
  issue_tags: Array<{ id: string; name: string }>
  evaluation: {
    case_result_id: string
    evaluation_run_id: string
    evaluator_id: string
    evaluator_name: string
    dataset_id: string
    dataset_name: string
    dataset_version_id: string
    version_no: number
    version_case_id: string
    case_name: string | null
    user_prompt: string | null
    score: number | null
    comment: string | null
  } | null
  original_attachments: Attachment[]
  attachments: Attachment[]
  activities: Array<{
    id: string
    activity_type: string
    note: string | null
    actor_id: string
    actor_name: string
    from_status: string | null
    to_status: string | null
    from_assignee_id: string | null
    from_assignee_name: string | null
    to_assignee_id: string | null
    to_assignee_name: string | null
    created_at: string
  }>
}

export interface BadcasePage extends Badcase {
  candidate_assignees: Array<{ id: string; display_name: string }>
  candidate_issue_tags: Array<{ id: string; name: string }>
  allowed_actions: string[]
}
