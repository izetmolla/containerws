export function getGlobalOptionsJson(): Record<string, unknown> {
  const el = document.getElementById("__GLOBAL_DATA__")
  if (!el) return {}
  try {
    return JSON.parse(el.textContent || "{}") as Record<string, unknown>
  } catch {
    return {}
  }
}

export function getGlobalOptions<T>(
  obj?: string,
  cd?: T,
  appendArrayItems: boolean = false
): T | undefined {
  let globalOptions = getGlobalOptionsJson()
  // Ensure globalOptions is defined.
  if (!globalOptions) {
    globalOptions = {}
  }

  // No key provided: return the entire globalOptions.
  if (!obj) {
    return globalOptions as T
  }

  // Key exists in globalOptions.
  if (obj in globalOptions) {
    if (
      window.location.pathname + window.location.search !==
      globalOptions.originalUrl
    ) {
      return undefined
    }
    const globalVal = globalOptions[obj]

    // If both the default value and the global value are arrays,
    // either append the items or return the global array.
    if (Array.isArray(cd) && Array.isArray(globalVal)) {
      const mergedArray = appendArrayItems ? [...globalVal, ...cd] : globalVal
      globalOptions[obj] = mergedArray
      return mergedArray as T
    }

    // If both the default value and the global value are objects (but not arrays),
    // merge them so that properties from globalVal take precedence.
    if (
      typeof cd === "object" &&
      cd !== null &&
      !Array.isArray(cd) &&
      typeof globalVal === "object" &&
      globalVal !== null &&
      !Array.isArray(globalVal)
    ) {
      const mergedObject = mergeInitialData(cd, globalVal as T)
      globalOptions[obj] = mergedObject
      return mergedObject as T
    }

    // For any other type, return the global value.
    return globalVal as T
  } else {
    // Key not present in globalOptions: add cd to globalOptions.
    if (cd !== undefined) {
      globalOptions[obj] = cd
    }
    return cd
  }
}

export function withLanguages(languages: string[] = []) {
  const el = document.getElementById("__GLOBAL_LANGUAGES_DATA__")
  if (!el) return [...languages]
  try {
    return [
      ...languages,
      ...JSON.parse(el.textContent || "[]").map(
        (l: { code?: string }) => l?.code
      ),
    ]
  } catch {
    return languages
  }
}

export function isDeepMergeRecord(
  value: unknown
): value is Record<string, unknown> {
  if (value === null || typeof value !== "object" || Array.isArray(value)) {
    return false
  }
  const proto = Object.getPrototypeOf(value as object)
  return proto === null || proto === Object.prototype
}

export function mergeDeepRecords(
  initialData: Record<string, unknown>,
  data: Record<string, unknown>
): Record<string, unknown> {
  const merged: Record<string, unknown> = { ...initialData }
  for (const key of Object.keys(data)) {
    const incoming = data[key]
    const existing = merged[key]
    merged[key] =
      isDeepMergeRecord(existing) && isDeepMergeRecord(incoming)
        ? mergeDeepRecords(existing, incoming)
        : incoming
  }
  return merged
}

export function mergeInitialData<T>(initialData: T, data: T): T {
  if (!isDeepMergeRecord(initialData) || !isDeepMergeRecord(data)) {
    return (data !== undefined && data !== null ? data : initialData) as T
  }
  return mergeDeepRecords(initialData, data) as T
}
