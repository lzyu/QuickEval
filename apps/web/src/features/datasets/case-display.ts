export function caseDisplayName(
  name: string | null | undefined,
  userPrompt: string,
  maxLength = 48,
) {
  const explicitName = name?.trim()
  if (explicitName) return explicitName

  const firstLine = userPrompt.trim().split(/\r?\n/, 1)[0]
  if (!firstLine) return '未命名用例'
  return firstLine.length > maxLength ? `${firstLine.slice(0, maxLength)}…` : firstLine
}
