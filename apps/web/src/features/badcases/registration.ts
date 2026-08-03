import type { CatalogItem, Scenario, Tag } from '@/api/types'

export type RegistrationEnvironment = 'test' | 'staging' | 'production' | 'other'

export interface RegistrationFormState {
  scenario_id: string
  title: string
  description: string
  agent_response_text: string
  agent_version: string
  environment: RegistrationEnvironment
  occurred_at: string
  business_reference: string
  session_id: string
  issue_tag_ids: string[]
}

export interface RegistrationDraft {
  version: 1
  form: RegistrationFormState
  had_screenshots: boolean
  saved_at: string
}

export interface TargetChoice extends CatalogItem {
  availableScenarioCount: number
}

export function nowForDateTimeInput(date = new Date()) {
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60_000)
  return local.toISOString().slice(0, 16)
}

export function todayForAgentVersion(date = new Date()) {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}${month}${day}`
}

export function submissionTitle(title: string, originalInput: string) {
  const explicitTitle = title.trim()
  if (explicitTitle) return explicitTitle
  return Array.from(originalInput.trim().replace(/\s+/g, ' ')).slice(0, 200).join('')
}

export function emptyRegistrationForm(): RegistrationFormState {
  return {
    scenario_id: '',
    title: '',
    description: '',
    agent_response_text: '',
    agent_version: todayForAgentVersion(),
    environment: 'production',
    occurred_at: nowForDateTimeInput(),
    business_reference: '',
    session_id: '',
    issue_tag_ids: [],
  }
}

export function targetChoices(targets: CatalogItem[], scenarios: Scenario[]): TargetChoice[] {
  return targets.map((target) => ({
    ...target,
    availableScenarioCount: scenarios.filter(
      (scenario) =>
        scenario.status === 'active' && scenario.evaluation_target_id === target.id,
    ).length,
  }))
}

export function availableTags(tags: Tag[]) {
  return tags.filter((tag) => tag.status === 'active')
}

export function validateRegistration(form: RegistrationFormState) {
  return {
    description: form.description.trim() ? '' : '请填写原始输入',
    environment: form.environment ? '' : '请选择运行环境',
    occurred_at: form.occurred_at ? '' : '请选择发生时间',
  }
}

export function isRegistrationValid(form: RegistrationFormState) {
  return Object.values(validateRegistration(form)).every((message) => !message)
}

export function resetAfterRegistration(form: RegistrationFormState): RegistrationFormState {
  return {
    ...emptyRegistrationForm(),
    scenario_id: form.scenario_id,
    environment: form.environment,
    agent_version: form.agent_version,
  }
}

export function draftKey(userId: string, targetId: string) {
  return `quickeval:badcase-draft:${userId}:${targetId}`
}

export function parseDraft(value: string | null): RegistrationDraft | null {
  if (!value) return null
  try {
    const draft = JSON.parse(value) as RegistrationDraft
    return draft.version === 1 && draft.form ? draft : null
  } catch {
    return null
  }
}
