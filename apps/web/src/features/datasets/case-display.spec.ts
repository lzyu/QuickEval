import { describe, expect, it } from 'vitest'

import { caseDisplayName } from './case-display'

describe('caseDisplayName', () => {
  it('prefers an explicit case name', () => {
    expect(caseDisplayName('  预算边界  ', '预算是多少？')).toBe('预算边界')
  })

  it('uses the first line of the user input when the name is absent', () => {
    expect(caseDisplayName(null, '请推荐一个采购方案\n预算为十万元')).toBe('请推荐一个采购方案')
  })

  it('truncates a long user input without inventing a stored name', () => {
    expect(caseDisplayName(null, '一二三四五六', 4)).toBe('一二三四…')
  })

  it('keeps an honest fallback when both values are blank', () => {
    expect(caseDisplayName('', '   ')).toBe('未命名用例')
  })
})
