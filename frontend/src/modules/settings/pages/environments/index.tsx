import { useMemo, useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  Eye,
  EyeOff,
  Loader2,
  Pencil,
  Plus,
  Search,
  Trash2,
  Variable,
} from "lucide-react"
import { toast } from "sonner"

import ContentLoader from "@/components/content-loader"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Separator } from "@/components/ui/separator"
import { getRequestErrorMessage, withError } from "@/lib/network"
import { cn } from "@/lib/utils"

import {
  createEnvironment,
  deleteEnvironment,
  getEnvironment,
  listEnvironments,
  SETTINGS_ENV_FETCH_KEY,
  updateEnvironment,
  type OsEnvironment,
} from "./api"
import { asArray } from "@/lib/as-array"

type EditorState = {
  mode: "create" | "edit"
  id?: string
  name: string
  value: string
  group: string
  is_secret: boolean
  is_disabled: boolean
  is_core: boolean
  is_textarea: boolean
}

const emptyEditor = (): EditorState => ({
  mode: "create",
  name: "",
  value: "",
  group: "",
  is_secret: false,
  is_disabled: false,
  is_core: false,
  is_textarea: false,
})

const valueFieldClass = cn(
  "w-full min-w-0 rounded-lg border border-input bg-transparent px-3 py-2 font-mono text-sm outline-none",
  "placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50",
  "dark:bg-input/30"
)

export default function EnvironmentsSettingsPage() {
  const queryClient = useQueryClient()
  const [q, setQ] = useState("")
  const [groupFilter, setGroupFilter] = useState("all")
  const [editorOpen, setEditorOpen] = useState(false)
  const [editor, setEditor] = useState<EditorState>(emptyEditor)
  const [showValue, setShowValue] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<OsEnvironment | null>(null)
  const [loadingEdit, setLoadingEdit] = useState(false)

  const listQuery = useQuery({
    queryKey: [SETTINGS_ENV_FETCH_KEY, "list"],
    queryFn: () => listEnvironments(),
  })

  const invalidate = () =>
    void queryClient.invalidateQueries({ queryKey: [SETTINGS_ENV_FETCH_KEY] })

  const saveMutation = useMutation({
    mutationFn: async () => {
      if (editor.mode === "create") {
        return createEnvironment({
          name: editor.name.trim(),
          value: editor.value,
          group: editor.group.trim(),
          is_secret: editor.is_secret,
          is_disabled: editor.is_disabled,
          is_textarea: editor.is_textarea,
        })
      }
      if (!editor.id) throw new Error("Missing environment id")
      const payload: {
        name?: string
        value?: string
        group?: string
        is_secret?: boolean
        is_disabled?: boolean
        is_textarea?: boolean
      } = {
        value: editor.value,
        group: editor.group.trim(),
        is_secret: editor.is_secret,
        is_disabled: editor.is_disabled,
        is_textarea: editor.is_textarea,
      }
      if (!editor.is_core) {
        payload.name = editor.name.trim()
      }
      return updateEnvironment(editor.id, payload)
    },
    onSuccess: (res) => {
      toast.success(res.message || "Environment saved")
      setEditorOpen(false)
      invalidate()
    },
    onError: (err) => {
      toast.error(getRequestErrorMessage(err, "Failed to save environment"))
    },
  })

  const removeMutation = useMutation({
    mutationFn: (id: string) => deleteEnvironment(id),
    onSuccess: (res) => {
      toast.success(res.message || "Environment deleted")
      setDeleteTarget(null)
      invalidate()
    },
    onError: (err) => {
      toast.error(getRequestErrorMessage(err, "Failed to delete environment"))
    },
  })

  const rows = asArray(listQuery.data?.data)
  const groups = listQuery.data?.groups ?? []

  const filtered = useMemo(() => {
    const list = rows
    const needle = q.trim().toLowerCase()
    return list.filter((row) => {
      if (groupFilter === "ungrouped" && row.group.trim() !== "") return false
      if (
        groupFilter !== "all" &&
        groupFilter !== "ungrouped" &&
        row.group !== groupFilter
      ) {
        return false
      }
      if (!needle) return true
      return (
        row.name.toLowerCase().includes(needle) ||
        row.group.toLowerCase().includes(needle) ||
        (!row.is_secret && row.value.toLowerCase().includes(needle))
      )
    })
  }, [rows, q, groupFilter])

  const openCreate = () => {
    setEditor(emptyEditor())
    setShowValue(true)
    setEditorOpen(true)
  }

  const openEdit = async (row: OsEnvironment) => {
    setLoadingEdit(true)
    setShowValue(!row.is_secret)
    try {
      const res = await getEnvironment(row.id)
      const data = res.data
      setEditor({
        mode: "edit",
        id: data.id,
        name: data.name,
        value: data.value,
        group: data.group || "",
        is_secret: data.is_secret,
        is_disabled: data.is_disabled,
        is_core: data.is_core,
        is_textarea: Boolean(data.is_textarea),
      })
      setEditorOpen(true)
    } catch (err) {
      toast.error(getRequestErrorMessage(err, "Failed to load environment"))
    } finally {
      setLoadingEdit(false)
    }
  }

  const useTextarea =
    editor.is_textarea || editor.value.includes("\n") || editor.value.length > 120

  return (
    <ContentLoader
      title="Environments"
      description="Manage process environment variables and core server settings."
      breadcrumb={[
        { label: "Settings", to: "/settings" },
        { label: "Environments" },
      ]}
      isLoading={listQuery.isLoading}
      error={withError(listQuery.error, listQuery.data)}
      showHeaderSeparator
    >
      <div className="w-full space-y-6">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div className="relative min-w-[220px] flex-1">
            <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              value={q}
              onChange={(e) => setQ(e.target.value)}
              placeholder="Search name, group, value…"
              className="pl-8"
            />
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <select
              className="h-8 rounded-lg border border-input bg-transparent px-2.5 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 dark:bg-input/30"
              value={groupFilter}
              onChange={(e) => setGroupFilter(e.target.value)}
            >
              <option value="all">All groups</option>
              <option value="ungrouped">Ungrouped</option>
              {groups.map((g) => (
                <option key={g} value={g}>
                  {g}
                </option>
              ))}
            </select>
            <Button type="button" size="sm" onClick={openCreate}>
              <Plus className="size-4" />
              Add variable
            </Button>
          </div>
        </div>

        <Separator />

        {filtered.length === 0 ? (
          <div className="flex flex-col items-center gap-2 rounded-lg border border-dashed border-border/80 px-4 py-12 text-center">
            <Variable className="size-8 text-muted-foreground/70" />
            <p className="text-sm text-muted-foreground">
              No environment variables match this filter.
            </p>
          </div>
        ) : (
          <ul className="divide-y divide-border/70 rounded-lg border border-border/70">
            {filtered.map((row) => (
              <li
                key={row.id}
                className="flex flex-col gap-3 px-3 py-3 sm:flex-row sm:items-center sm:justify-between"
              >
                <div className="min-w-0 space-y-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <code className="truncate font-mono text-sm font-medium">
                      {row.name}
                    </code>
                    {row.is_core ? (
                      <Badge variant="secondary">core</Badge>
                    ) : null}
                    {row.is_secret ? (
                      <Badge variant="outline">secret</Badge>
                    ) : null}
                    {row.is_textarea ? (
                      <Badge variant="outline">multiline</Badge>
                    ) : null}
                    {row.is_disabled ? (
                      <Badge variant="destructive">disabled</Badge>
                    ) : (
                      <Badge variant="default">active</Badge>
                    )}
                    {row.group ? (
                      <Badge variant="outline">{row.group}</Badge>
                    ) : null}
                  </div>
                  <p className="truncate font-mono text-xs text-muted-foreground">
                    {row.is_secret || row.secret_masked
                      ? "••••••••"
                      : row.value || "(empty)"}
                  </p>
                </div>
                <div className="flex shrink-0 flex-wrap gap-1.5">
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    disabled={loadingEdit}
                    onClick={() => void openEdit(row)}
                  >
                    <Pencil className="size-3.5" />
                    Edit
                  </Button>
                  {!row.is_core ? (
                    <Button
                      type="button"
                      size="sm"
                      variant="ghost"
                      className="text-destructive hover:text-destructive"
                      onClick={() => setDeleteTarget(row)}
                    >
                      <Trash2 className="size-3.5" />
                      Delete
                    </Button>
                  ) : null}
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>

      <Dialog
        open={editorOpen}
        onOpenChange={(open) => {
          setEditorOpen(open)
          if (!open) setEditor(emptyEditor())
        }}
      >
        <DialogContent className="gap-0 overflow-hidden p-0 sm:max-w-2xl">
          <DialogHeader className="space-y-1 border-b border-border/60 px-6 py-5">
            <DialogTitle className="text-base">
              {editor.mode === "create"
                ? "Add environment variable"
                : `Edit ${editor.name}`}
            </DialogTitle>
            <DialogDescription>
              {editor.is_core
                ? "Core server settings sync into the running process after save."
                : "Names must be UPPER_SNAKE_CASE. Changes reload into the process environment."}
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-5 px-6 py-5">
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="space-y-2 sm:col-span-2">
                <Label htmlFor="env-name">Name</Label>
                <Input
                  id="env-name"
                  value={editor.name}
                  disabled={editor.is_core}
                  placeholder="MY_SETTING"
                  className="font-mono"
                  onChange={(e) =>
                    setEditor((s) => ({ ...s, name: e.target.value }))
                  }
                />
              </div>
              <div className="space-y-2 sm:col-span-2">
                <div className="flex items-center justify-between gap-2">
                  <Label htmlFor="env-value">Value</Label>
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    className="h-7 px-2 text-xs text-muted-foreground"
                    onClick={() => setShowValue((v) => !v)}
                  >
                    {showValue ? (
                      <EyeOff className="size-3.5" />
                    ) : (
                      <Eye className="size-3.5" />
                    )}
                    {showValue ? "Hide" : "Reveal"}
                  </Button>
                </div>
                {useTextarea ? (
                  <textarea
                    id="env-value"
                    value={
                      showValue
                        ? editor.value
                        : editor.value
                          ? "•".repeat(Math.min(editor.value.length, 48))
                          : ""
                    }
                    readOnly={!showValue}
                    rows={8}
                    spellCheck={false}
                    placeholder="Value (supports multiple lines)"
                    className={cn(
                      valueFieldClass,
                      "min-h-[10rem] resize-y leading-relaxed"
                    )}
                    onChange={(e) => {
                      if (!showValue) return
                      setEditor((s) => ({ ...s, value: e.target.value }))
                    }}
                  />
                ) : (
                  <Input
                    id="env-value"
                    type={showValue ? "text" : "password"}
                    value={editor.value}
                    className="h-10 font-mono"
                    onChange={(e) =>
                      setEditor((s) => ({ ...s, value: e.target.value }))
                    }
                  />
                )}
                <p className="text-xs text-muted-foreground">
                  {editor.value.length} character
                  {editor.value.length === 1 ? "" : "s"}
                  {useTextarea
                    ? ` · ${editor.value.split("\n").length} line${editor.value.split("\n").length === 1 ? "" : "s"}`
                    : ""}
                </p>
              </div>
              <div className="space-y-2 sm:col-span-2">
                <Label htmlFor="env-group">Group</Label>
                <Input
                  id="env-group"
                  value={editor.group}
                  placeholder="optional · e.g. server, secrets"
                  onChange={(e) =>
                    setEditor((s) => ({ ...s, group: e.target.value }))
                  }
                />
              </div>
            </div>

            <div className="rounded-lg border border-border/70 bg-muted/15 p-3">
              <p className="mb-3 text-xs font-medium tracking-wide text-muted-foreground uppercase">
                Advanced
              </p>
              <div className="grid gap-2 sm:grid-cols-3">
                {(
                  [
                    {
                      key: "is_textarea" as const,
                      label: "Multiline",
                      hint: "Use a textarea editor",
                    },
                    {
                      key: "is_secret" as const,
                      label: "Secret",
                      hint: "Mask value in lists",
                    },
                    {
                      key: "is_disabled" as const,
                      label: "Disabled",
                      hint: "Skip process sync",
                    },
                  ] as const
                ).map((opt) => (
                  <label
                    key={opt.key}
                    className={cn(
                      "flex cursor-pointer items-start gap-2.5 rounded-md border border-transparent px-2 py-2",
                      "hover:border-border/80 hover:bg-background/60"
                    )}
                  >
                    <input
                      type="checkbox"
                      className="mt-0.5 size-4 accent-primary"
                      checked={editor[opt.key]}
                      onChange={(e) =>
                        setEditor((s) => ({
                          ...s,
                          [opt.key]: e.target.checked,
                        }))
                      }
                    />
                    <span>
                      <span className="block text-sm font-medium">
                        {opt.label}
                      </span>
                      <span className="mt-0.5 block text-xs text-muted-foreground">
                        {opt.hint}
                      </span>
                    </span>
                  </label>
                ))}
              </div>
            </div>
          </div>

          <DialogFooter className="border-t border-border/60 bg-muted/10 px-6 py-4">
            <Button
              type="button"
              variant="outline"
              onClick={() => setEditorOpen(false)}
            >
              Cancel
            </Button>
            <Button
              type="button"
              disabled={
                saveMutation.isPending ||
                !editor.name.trim() ||
                (editor.mode === "create" && !/^[A-Za-z]/.test(editor.name))
              }
              onClick={() => saveMutation.mutate()}
            >
              {saveMutation.isPending ? (
                <Loader2 className="size-4 animate-spin" />
              ) : null}
              Save
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={Boolean(deleteTarget)}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null)
        }}
      >
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>Delete environment variable</DialogTitle>
            <DialogDescription>
              Remove{" "}
              <code className={cn("font-mono text-foreground")}>
                {deleteTarget?.name}
              </code>
              ? Core settings cannot be deleted.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => setDeleteTarget(null)}
            >
              Cancel
            </Button>
            <Button
              type="button"
              variant="destructive"
              disabled={removeMutation.isPending || !deleteTarget}
              onClick={() => {
                if (deleteTarget) removeMutation.mutate(deleteTarget.id)
              }}
            >
              {removeMutation.isPending ? (
                <Loader2 className="size-4 animate-spin" />
              ) : (
                <Trash2 className="size-4" />
              )}
              Delete
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </ContentLoader>
  )
}
