import { describe, expect, it } from 'vitest'

import { badcaseDisplayTitle } from './display'

describe('badcaseDisplayTitle', () => {
  it('prefers and normalizes the original input', () => {
    expect(badcaseDisplayTitle({ title: '问题描述', description: '  用户\n实际输入  ' })).toBe('用户 实际输入')
  })

  it('falls back to the legacy title and truncates long input', () => {
    expect(badcaseDisplayTitle({ title: '历史 Badcase' })).toBe('历史 Badcase')
    expect(badcaseDisplayTitle({ title: '', description: 'abcdef' }, 5)).toBe('abcde…')
  })

  it('uses the evaluated case input when available', () => {
    expect(
      badcaseDisplayTitle({
        title: '评测问题',
        description: '具体问题',
        source_type: 'evaluation',
        evaluation: { user_prompt: '评测用例原始输入' },
      }),
    ).toBe('评测用例原始输入')
  })
})
