export function foldSearchText(value: string): string {
  return value.normalize("NFD").replace(/\p{M}/gu, "").toLocaleLowerCase()
}

export function searchTextIncludes(haystack: string, needle: string): boolean {
  const foldedNeedle = foldSearchText(needle).trim()
  if (!foldedNeedle) {
    return true
  }

  return foldSearchText(haystack).includes(foldedNeedle)
}
