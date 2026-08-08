import { AlertCircle, ArrowLeft, RotateCcw } from "lucide-react"
import { Button } from "@/components/ui/button"

export interface ContentErrorShape {
  message?: string
  code?: string
  status?: number
  details?: unknown
  error?: boolean
}

interface ContentErrorProps {
  minimal?: boolean
  error: Error | ContentErrorShape
}

const getErrorMessage = (error: Error | ContentErrorShape) => {
  if (error instanceof Error) {
    return error.message || "Something went wrong while loading this page."
  }

  return error.message || "Something went wrong while loading this page."
}

const statusUiMap: Record<number, { title: string; hint: string }> = {
  400: {
    title: "Bad Request",
    hint: "Please review your input and try again.",
  },
  401: { title: "Unauthorized", hint: "Please sign in and try again." },
  403: {
    title: "Forbidden",
    hint: "You do not have permission to perform this action.",
  },
  404: {
    title: "Not Found",
    hint: "The resource you requested could not be found.",
  },
  409: {
    title: "Conflict",
    hint: "This action conflicts with the current state.",
  },
  422: {
    title: "Validation Error",
    hint: "Some fields are invalid. Please verify and retry.",
  },
  429: {
    title: "Too Many Requests",
    hint: "Too many attempts. Please wait and try again.",
  },
  500: {
    title: "Server Error",
    hint: "Something failed on our side. Please try again shortly.",
  },
  503: {
    title: "Service Unavailable",
    hint: "The service is temporarily unavailable.",
  },
}

const statusCodeFromError = (
  error: Error | ContentErrorShape
): number | null => {
  if (!(error instanceof Error)) {
    if (typeof error.status === "number") return error.status
    if (typeof error.code === "string") {
      const parsed = Number(error.code)
      if (Number.isInteger(parsed) && parsed >= 100 && parsed <= 599)
        return parsed
    }
  }
  return null
}

const ContentError = ({ error, minimal = false }: ContentErrorProps) => {
  const message = getErrorMessage(error)
  const code = "code" in error && error.code ? error.code : null
  const details =
    "details" in error && error.details ? String(error.details) : null
  const statusCode = statusCodeFromError(error)
  const mappedStatus = statusCode ? statusUiMap[statusCode] : null
  const title = mappedStatus?.title ?? "Error"
  const hint =
    mappedStatus?.hint ??
    "We could not complete your request. Please try again."

  if (minimal) {
    return <p className="text-sm text-destructive">Error: {message}</p>
  }

  return (
    <div className="flex min-h-screen w-full items-center justify-center bg-background p-4 sm:p-6">
      <div className="w-full max-w-2xl rounded-2xl border bg-card p-6 shadow-sm sm:p-8">
        <div className="flex flex-col items-center text-center">
          <div className="mb-3 rounded-full bg-destructive/10 p-3 text-destructive">
            <AlertCircle className="size-6" />
          </div>
          <h2 className="text-2xl font-bold tracking-tight text-destructive">
            {title}
          </h2>
          <p className="mt-1 text-sm font-medium text-muted-foreground">
            {hint}
          </p>
        </div>

        <div className="mt-4 rounded-xl border bg-gradient-to-br from-muted/70 to-background p-3 sm:p-4">
          <p className="text-sm leading-relaxed break-words text-foreground/90 sm:text-base">
            {message}
          </p>
        </div>

        {(statusCode || code || details) && (
          <div className="mt-3 rounded-lg border bg-muted/50 p-3 text-sm">
            {statusCode && (
              <p className="font-semibold">Status: {statusCode}</p>
            )}
            {code && <p className="font-semibold">Code: {code}</p>}
            {details && (
              <p className="mt-1 break-words text-muted-foreground">
                Details: {details}
              </p>
            )}
          </div>
        )}

        <div className="mt-5 flex flex-wrap items-center justify-center gap-2">
          <Button
            onClick={() => window.location.reload()}
            className="min-w-28 gap-2"
          >
            <RotateCcw className="size-4" />
            Reload
          </Button>
          <Button
            variant="outline"
            onClick={() => window.history.back()}
            className="min-w-28 gap-2"
          >
            <ArrowLeft className="size-4" />
            Back
          </Button>
        </div>
      </div>
    </div>
  )
}

export default ContentError
