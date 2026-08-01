import { describe, expect, it } from 'vitest'

import type { CatalogItem, Scenario, Tag } from '@/api/types'
import {
  availableTags,
  emptyRegistrationForm,
  isRegistrationValid,
  parseDraft,
  resetAfterRegistration,
  targetChoices,
} from './registration'

const target = (id: string, status: CatalogItem['status'] = 'active'): CatalogItem => ({
  id,
  name: `对象 ${id}`,
  description: null,
  status,
  lock_version: 1,
})

const scenario = (id: string, targetId: string, status: Scenario['status'] = 'active'): Scenario => ({
  ...target(id, status),
  id,
  name: `场景 ${id}`,
  evaluation_target_id: targetId,
})

describe('badcase registration state', () => {
  it('counts only active scenarios for each evaluation target', () => {
    expect(
      targetChoices(
        [target('a'), target('b')],
        [scenario('s1', 'a'), scenario('s2', 'a', 'disabled'), scenario('s3', 'b')],
      ).map((item) => item.availableScenarioCount),
    ).toEqual([1, 1])
  })

  it('does not require a scenario, answer, screenshot, or agent version', () => {
    const form = emptyRegistrationForm()
    Object.assign(form, { title: '推荐结果错误', issue_tag_ids: ['tag-1'] })
    expect(isRegistrationValid(form)).toBe(true)
  })

  it('retains context and clears problem fields after a registration', () => {
    const form = emptyRegistrationForm()
    Object.assign(form, {
      scenario_id: 's1',
      title: '问题',
      description: '描述',
      agent_response_text: '回答',
      agent_version: 'v2',
      environment: 'staging',
      business_reference: 'order-1',
      session_id: 'session-1',
      issue_tag_ids: ['tag-1'],
    })
    const next = resetAfterRegistration(form)
    expect(next).toMatchObject({ scenario_id: 's1', environment: 'staging', agent_version: 'v2' })
    expect(next).toMatchObject({ title: '', description: '', agent_response_text: '', issue_tag_ids: [] })
    expect(next.business_reference).toBe('')
    expect(next.session_id).toBe('')
  })

  it('filters scenario tags and ignores invalid stored drafts', () => {
    const tags = [
      { ...target('global'), sort_order: 1, scope: 'global' },
      { ...target('matching'), sort_order: 2, scope: 'scenario', scenario_id: 's1' },
      { ...target('other'), sort_order: 3, scope: 'scenario', scenario_id: 's2' },
    ] as Tag[]
    expect(availableTags(tags, 's1').map((item) => item.id)).toEqual(['global', 'matching'])
    expect(parseDraft('{broken')).toBeNull()
    expect(parseDraft(JSON.stringify({ version: 2, form: {} }))).toBeNull()
  })
})
