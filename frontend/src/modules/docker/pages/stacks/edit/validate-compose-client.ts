import { parseDocument } from "yaml"

export type ClientComposeIssue = {
  message: string
  line?: number
  column?: number
}

/** Fast local checks: YAML syntax + basic Compose shape (services/include). */
export function validateComposeYamlClient(
  text: string
): ClientComposeIssue | null {
  const trimmed = text.trim()
  if (!trimmed) {
    return { message: "Compose YAML is required" }
  }

  try {
    const doc = parseDocument(text, { uniqueKeys: true, strict: false })
    if (doc.errors.length > 0) {
      const err = doc.errors[0]
      const line = err.linePos?.[0]?.line
      const column = err.linePos?.[0]?.col
      return {
        message: err.message.replace(/^YAMLParseError:\s*/i, ""),
        line,
        column,
      }
    }

    const data = doc.toJS() as unknown
    if (data == null || typeof data !== "object" || Array.isArray(data)) {
      return { message: "Compose file must be a YAML mapping (object)" }
    }

    const root = data as Record<string, unknown>
    const hasServices = Object.prototype.hasOwnProperty.call(root, "services")
    const hasInclude = Object.prototype.hasOwnProperty.call(root, "include")
    if (!hasServices && !hasInclude) {
      return {
        message:
          'Compose file must define a "services" mapping (or use "include")',
      }
    }

    if (hasServices) {
      const services = root.services
      if (
        services == null ||
        typeof services !== "object" ||
        Array.isArray(services)
      ) {
        return {
          message: '"services" must be a mapping of service definitions',
        }
      }
      if (Object.keys(services as object).length === 0) {
        return { message: '"services" must define at least one service' }
      }
      for (const [name, svc] of Object.entries(services as object)) {
        if (svc == null || typeof svc !== "object" || Array.isArray(svc)) {
          return {
            message: `Service "${name}" must be a mapping`,
          }
        }
        const s = svc as Record<string, unknown>
        const hasImage = typeof s.image === "string" && s.image.trim() !== ""
        const hasBuild =
          s.build != null &&
          (typeof s.build === "string" || typeof s.build === "object")
        const hasExtends = s.extends != null
        if (!hasImage && !hasBuild && !hasExtends) {
          return {
            message: `Service "${name}" needs an "image", "build", or "extends"`,
          }
        }
      }
    }

    return null
  } catch (err) {
    return {
      message: err instanceof Error ? err.message : "Invalid YAML",
    }
  }
}
