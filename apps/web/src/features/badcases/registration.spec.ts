import { describe, expect, it } from 'vitest'

import type { CatalogItem, Scenario, Tag } from '@/api/types'
import {
  availableTags,
  emptyRegistrationForm,
  isRegistrationValid,
  parseDraft,
  resetAfterRegistration,
  submissionTitle,
  targetChoices,
  todayForAgentVersion,
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

  it('requires original input but not a title, scenario, tag, answer, screenshot, or agent version', () => {
    const form = emptyRegistrationForm()
    Object.assign(form, { description: '预算为 10 万元时仍推荐超预算商品' })
    expect(isRegistrationValid(form)).toBe(true)
  })

  it('defaults the agent version to today and derives a title from original input when needed', () => {
    expect(todayForAgentVersion(new Date(2026, 7, 3))).toBe('20260803')
    expect(emptyRegistrationForm().agent_version).toMatch(/^\d{8}$/)
    expect(submissionTitle('', '  预算为 10 万元\n仍推荐超预算商品  ')).toBe(
      '预算为 10 万元 仍推荐超预算商品',
    )
    expect(submissionTitle('人工标题', '原始输入')).toBe('人工标题')
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

  it('keeps active issue tags independent from scenario and ignores invalid stored drafts', () => {
    const tags = [
      { ...target('global'), sort_order: 1, scope: 'global' },
      { ...target('matching'), sort_order: 2, scope: 'target', evaluation_target_id: 't1' },
      { ...target('other'), sort_order: 3, scope: 'target', evaluation_target_id: 't2' },
    ] as Tag[]
    expect(availableTags(tags).map((item) => item.id)).toEqual(['global', 'matching', 'other'])
    expect(parseDraft('{broken')).toBeNull()
    expect(parseDraft(JSON.stringify({ version: 2, form: {} }))).toBeNull()
  })
})
