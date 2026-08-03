export interface BadcaseDisplaySource {
  title: string
  description?: string | null
  source_type?: 'evaluation' | 'business'
  evaluation?: { user_prompt: string | null } | null
}

/** Badcase is identified externally by the input that triggered it. */
export function badcaseDisplayTitle(item: BadcaseDisplaySource, limit = 80) {
  const input =
    item.source_type === 'evaluation'
      ? item.evaluation?.user_prompt || item.description || item.title
      : item.description || item.title
  const source = input.trim().replace(/\s+/g, ' ')
  const characters = Array.from(source)

  if (characters.length > limit) return `${characters.slice(0, limit).join('')}…`
  return source || '-'
}
