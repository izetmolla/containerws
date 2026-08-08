import type { z } from "zod"

type ZodTypeAny = z.ZodType

function getDefType(schema: ZodTypeAny): string | undefined {
  const def = (schema as { def?: { type?: string } }).def
  return def?.type
}

function getDef(schema: ZodTypeAny): Record<string, unknown> | undefined {
  return (schema as unknown as { def?: Record<string, unknown> }).def
}

/** Unwrap wrappers that don't change whether the user must supply a value. */
function unwrapZodType(schema: ZodTypeAny): ZodTypeAny {
  let current = schema
  for (let i = 0; i < 16; i++) {
    const type = getDefType(current)
    const def = getDef(current)
    if (!def) break

    if (type === "pipe" && def.in && typeof def.in === "object") {
      current = def.in as ZodTypeAny
      continue
    }
    if (
      (type === "readonly" ||
        type === "catch" ||
        type === "success" ||
        type === "nonoptional") &&
      def.innerType &&
      typeof def.innerType === "object"
    ) {
      current = def.innerType as ZodTypeAny
      continue
    }
    break
  }
  return current
}

function getObjectShape(
  schema: ZodTypeAny
): Record<string, ZodTypeAny> | undefined {
  const unwrapped = unwrapZodType(schema)
  if (getDefType(unwrapped) !== "object") return undefined
  const shape = (unwrapped as { shape?: unknown }).shape
  if (typeof shape === "function") {
    return shape() as Record<string, ZodTypeAny>
  }
  if (shape && typeof shape === "object") {
    return shape as Record<string, ZodTypeAny>
  }
  return undefined
}

function resolveZodField(
  schema: ZodTypeAny,
  path: string
): ZodTypeAny | undefined {
  const parts = path.split(".").filter(Boolean)
  if (parts.length === 0) return undefined

  let current: ZodTypeAny | undefined = schema
  for (const part of parts) {
    if (!current) return undefined
    const shape = getObjectShape(current)
    if (shape) {
      current = shape[part]
      continue
    }

    const unwrapped = unwrapZodType(current)
    const type = getDefType(unwrapped)
    const def = getDef(unwrapped)
    if (type === "array" && /^\d+$/.test(part) && def?.element) {
      current = def.element as ZodTypeAny
      continue
    }
    if (type === "record" && def?.valueType) {
      current = def.valueType as ZodTypeAny
      continue
    }
    return undefined
  }
  return current
}

/**
 * Returns whether a field path is required in a Zod object schema.
 * Optional, defaulted, and `undefined`-accepting fields are not required.
 */
export function isZodFieldRequired(
  schema: ZodTypeAny | null | undefined,
  path: string
): boolean {
  if (!schema || !path) return false
  const field = resolveZodField(schema, path)
  if (!field) return false
  if (typeof field.isOptional === "function" && field.isOptional()) {
    return false
  }
  return true
}
