import { useMemo, useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  Loader2,
  Pencil,
  Plus,
  Search,
  Settings2,
  Trash2,
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
  createOption,
  deleteOption,
  getOption,
  listOptions,
  SETTINGS_OPTIONS_FETCH_KEY,
  updateOption,
  type OsOption,
} from "./api"
import { asArray } from "@/lib/as-array"

type EditorState = {
  mode: "create" | "edit"
  id?: string
  name: string
  value: string
}

const emptyEditor = (): EditorState => ({
  mode: "create",
  name: "",
  value: "",
})

export default function OptionsSettingsPage() {
  const queryClient = useQueryClient()
  const [q, setQ] = useState("")
  const [groupFilter, setGroupFilter] = useState("all")
  const [editorOpen, setEditorOpen] = useState(false)
  const [editor, setEditor] = useState<EditorState>(emptyEditor)
  const [deleteTarget, setDeleteTarget] = useState<OsOption | null>(null)
  const [loadingEdit, setLoadingEdit] = useState(false)

  const listQuery = useQuery({
    queryKey: [SETTINGS_OPTIONS_FETCH_KEY, "list"],
    queryFn: () => listOptions(),
  })

  const invalidate = () =>
    void queryClient.invalidateQueries({
      queryKey: [SETTINGS_OPTIONS_FETCH_KEY],
    })

  const saveMutation = useMutation({
    mutationFn: async () => {
      if (editor.mode === "create") {
        return createOption({
          name: editor.name.trim(),
          value: editor.value,
        })
      }
      if (!editor.id) throw new Error("Missing option id")
      return updateOption(editor.id, {
        name: editor.name.trim(),
        value: editor.value,
      })
    },
    onSuccess: (res) => {
      toast.success(res.message || "Option saved")
      setEditorOpen(false)
      invalidate()
    },
    onError: (err) => {
      toast.error(getRequestErrorMessage(err, "Failed to save option"))
    },
  })

  const removeMutation = useMutation({
    mutationFn: (id: string) => deleteOption(id),
    onSuccess: (res) => {
      toast.success(res.message || "Option deleted")
      setDeleteTarget(null)
      invalidate()
    },
    onError: (err) => {
      toast.error(getRequestErrorMessage(err, "Failed to delete option"))
    },
  })

  const rows = asArray(listQuery.data?.data)
  const groups = listQuery.data?.groups ?? []

  const filtered = useMemo(() => {
    const list = rows
    const needle = q.trim().toLowerCase()
    return list.filter((row) => {
      if (groupFilter !== "all" && (row.group || "") !== groupFilter) {
        return false
      }
      if (!needle) return true
      return (
        row.name.toLowerCase().includes(needle) ||
        row.value.toLowerCase().includes(needle) ||
        (row.group || "").toLowerCase().includes(needle)
      )
    })
  }, [rows, q, groupFilter])

  const openCreate = () => {
    setEditor(emptyEditor())
    setEditorOpen(true)
  }

  const openEdit = async (row: OsOption) => {
    setLoadingEdit(true)
    try {
      const res = await getOption(row.id)
      const data = res.data
      setEditor({
        mode: "edit",
        id: data.id,
        name: data.name,
        value: data.value,
      })
      setEditorOpen(true)
    } catch (err) {
      toast.error(getRequestErrorMessage(err, "Failed to load option"))
    } finally {
      setLoadingEdit(false)
    }
  }

  return (
    <ContentLoader
      title="Options"
      description="Persistent install and feature flags (VNC, codeserver, MCP, workspace)."
      breadcrumb={[
        { label: "Settings", to: "/settings" },
        { label: "Options" },
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
              placeholder="Search name, value, group…"
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
              {groups.map((g) => (
                <option key={g} value={g}>
                  {g}
                </option>
              ))}
            </select>
            <Button type="button" size="sm" onClick={openCreate}>
              <Plus className="size-4" />
              Add option
            </Button>
          </div>
        </div>

        <Separator />

        {filtered.length === 0 ? (
          <div className="flex flex-col items-center gap-2 rounded-lg border border-dashed border-border/80 px-4 py-12 text-center">
            <Settings2 className="size-8 text-muted-foreground/70" />
            <p className="text-sm text-muted-foreground">
              No options match this filter.
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
                    {row.group ? (
                      <Badge variant="outline">{row.group}</Badge>
                    ) : null}
                  </div>
                  <p className="truncate font-mono text-xs text-muted-foreground">
                    {row.value || "(empty)"}
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
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>
              {editor.mode === "create" ? "Add option" : `Edit ${editor.name}`}
            </DialogTitle>
            <DialogDescription>
              Options store install state and feature flags (for example{" "}
              <code className="font-mono text-xs">CODESERVER_INSTALLED</code>,{" "}
              <code className="font-mono text-xs">VNC_INSTALLED</code>).
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="option-name">Name</Label>
              <Input
                id="option-name"
                value={editor.name}
                placeholder="CODESERVER_INSTALLED"
                className="font-mono"
                onChange={(e) =>
                  setEditor((s) => ({ ...s, name: e.target.value }))
                }
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="option-value">Value</Label>
              <Input
                id="option-value"
                value={editor.value}
                placeholder="true"
                className="font-mono"
                onChange={(e) =>
                  setEditor((s) => ({ ...s, value: e.target.value }))
                }
              />
            </div>
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => setEditorOpen(false)}
            >
              Cancel
            </Button>
            <Button
              type="button"
              disabled={saveMutation.isPending || !editor.name.trim()}
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
            <DialogTitle>Delete option</DialogTitle>
            <DialogDescription>
              Remove{" "}
              <code className={cn("font-mono text-foreground")}>
                {deleteTarget?.name}
              </code>
              ? Related features may fall back to defaults.
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
