import { MonacoCodeEditor } from "@/components/monaco-editor"
import { cn } from "@/lib/utils"

export function KubeconfigYamlEditor({
  value,
  onChange,
  height = "min(60vh, 28rem)",
  readOnly,
  className,
}: {
  value: string
  onChange: (next: string) => void
  height?: string
  readOnly?: boolean
  className?: string
}) {
  return (
    <div
      className={cn(
        "overflow-hidden rounded-xl border border-input",
        "focus-within:border-ring focus-within:ring-[3px] focus-within:ring-ring/50",
        className,
      )}
      style={{ height }}
    >
      <MonacoCodeEditor
        value={value}
        onChange={onChange}
        language="yaml"
        height="100%"
        readOnly={readOnly}
      />
    </div>
  )
}
