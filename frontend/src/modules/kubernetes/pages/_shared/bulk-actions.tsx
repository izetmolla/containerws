import type { ReactNode } from "react"
import type { Table } from "@tanstack/react-table"
import { useMutation } from "@tanstack/react-query"
import { toast } from "sonner"

import {
  DataTableActionBar,
  DataTableActionBarAction,
  DataTableActionBarSelection,
} from "@/components/datatable"
import { toastRequestError } from "@/lib/network"

import { k8sClientTableProps } from "./client-table"

export const k8sSelectableTableProps = {
  ...k8sClientTableProps,
  enableRowSelection: true,
} as const

type BulkActionDef<T> = {
  key: string
  label: string
  icon?: ReactNode
  tooltip?: string
  variant?: "default" | "destructive"
  /** If set, shown via window.confirm; use `{n}` for selection count. */
  confirm?: string
  run: (rows: T[]) => Promise<unknown>
}

export function K8sBulkActionBar<T>({
  table,
  actions,
  onDone,
}: {
  table: Table<T>
  actions: BulkActionDef<T>[]
  onDone?: () => void
}) {
  const mutation = useMutation({
    mutationFn: async (action: BulkActionDef<T>) => {
      const rows = table
        .getFilteredSelectedRowModel()
        .rows.map((r) => r.original)
      if (!rows.length) throw new Error("No rows selected")
      if (action.confirm) {
        const message = action.confirm.replaceAll("{n}", String(rows.length))
        if (!window.confirm(message)) return { cancelled: true as const }
      }
      await action.run(rows)
      return { cancelled: false as const, count: rows.length }
    },
    onSuccess: (res, action) => {
      if (res.cancelled) return
      toast.success(`${action.label} completed for ${res.count} item(s)`)
      table.toggleAllRowsSelected(false)
      onDone?.()
    },
    onError: (err) => toastRequestError(err, "Bulk action failed"),
  })

  return (
    <DataTableActionBar table={table}>
      <DataTableActionBarSelection table={table} />
      {actions.map((action) => (
        <DataTableActionBarAction
          key={action.key}
          tooltip={action.tooltip ?? action.label}
          disabled={mutation.isPending}
          isPending={mutation.isPending && mutation.variables?.key === action.key}
          className={
            action.variant === "destructive"
              ? "border-destructive/30 bg-destructive/10 text-destructive hover:bg-destructive/20"
              : undefined
          }
          onClick={() => mutation.mutate(action)}
        >
          {action.icon}
          {action.label}
        </DataTableActionBarAction>
      ))}
    </DataTableActionBar>
  )
}

/** Run async work for each selected row; fails if any item fails. */
export async function runForEachSelected<T>(
  rows: T[],
  worker: (row: T) => Promise<unknown>,
) {
  const results = await Promise.allSettled(rows.map((row) => worker(row)))
  const failed = results.filter((r) => r.status === "rejected")
  if (failed.length) {
    const first = failed[0] as PromiseRejectedResult
    const msg =
      first.reason instanceof Error
        ? first.reason.message
        : String(first.reason ?? "Unknown error")
    throw new Error(
      failed.length === rows.length
        ? msg
        : `${failed.length} of ${rows.length} failed: ${msg}`,
    )
  }
}
