import { useMemo } from "react"
import { useQuery } from "@tanstack/react-query"
import type { Column, Row } from "@tanstack/react-table"

import { DataTableColumnHeader } from "@/components/datatable"

import { K8S_NAMESPACES_KEY, listNamespaces } from "./api"
import {
  isSystemNamespace,
  useShowSystemResources,
} from "./system-resources"

export type K8sFilterOption = { label: string; value: string }

export function k8sHeader(title: string) {
  return function Header<TData, TValue>({
    column,
  }: {
    column: Column<TData, TValue>
  }) {
    return <DataTableColumnHeader column={column} title={title} />
  }
}

/** Sortable column with no filter (Age, Ready, …). */
export function k8sSortable(label: string) {
  return {
    enableSorting: true as const,
    enableColumnFilter: false as const,
    header: k8sHeader(label),
    meta: { label },
  }
}

/** Sortable text filter column. */
export function k8sTextFilter(label: string, placeholder?: string) {
  return {
    enableSorting: true as const,
    enableColumnFilter: true as const,
    header: k8sHeader(label),
    meta: {
      label,
      variant: "text" as const,
      placeholder: placeholder ?? `Filter ${label.toLowerCase()}…`,
    },
  }
}

/** Primary Name column: text filter that expands to fill leftover table width. */
export function k8sNameFilter(placeholder = "Filter name…") {
  return {
    ...k8sTextFilter("Name", placeholder),
    meta: {
      ...k8sTextFilter("Name", placeholder).meta,
      className: "w-full min-w-[12rem]",
    },
  }
}

function multiSelectFilterFn<TData>(
  row: Row<TData>,
  id: string,
  value: unknown,
) {
  const selected = Array.isArray(value)
    ? value.map(String)
    : value == null || value === ""
      ? []
      : [String(value)]
  if (!selected.length) return true
  return selected.includes(String(row.getValue(id) ?? ""))
}

/** Sortable multi-select filter (e.g. Status, Type). */
export function k8sMultiSelectFilter(
  label: string,
  options: K8sFilterOption[],
) {
  return {
    enableSorting: true as const,
    enableColumnFilter: true as const,
    header: k8sHeader(label),
    filterFn: multiSelectFilterFn,
    meta: {
      label,
      variant: "multiSelect" as const,
      options,
    },
  }
}

/** Namespace multi-select filter used on namespaced resource tables. */
export function k8sNamespaceFilter(options: K8sFilterOption[]) {
  return k8sMultiSelectFilter("Namespace", options)
}

export function optionsFromValues(values: Iterable<string>): K8sFilterOption[] {
  return Array.from(new Set(Array.from(values).map((v) => v.trim()).filter(Boolean)))
    .sort((a, b) => a.localeCompare(b))
    .map((value) => ({ label: value, value }))
}

/** Namespace options from cluster list + optional values present in the current table rows. */
export function useK8sNamespaceOptions(rowNamespaces?: string[]) {
  const [showSystem] = useShowSystemResources()
  const nsQuery = useQuery({
    queryKey: [K8S_NAMESPACES_KEY],
    queryFn: listNamespaces,
    staleTime: 30_000,
  })
  const rowKey = (rowNamespaces ?? []).join("\0")

  return useMemo(() => {
    const values = [
      ...(nsQuery.data?.data?.map((n) => n.name) ?? []),
      ...rowKey.split("\0").filter(Boolean),
    ]
    return optionsFromValues(
      showSystem ? values : values.filter((ns) => !isSystemNamespace(ns)),
    )
  }, [nsQuery.data?.data, rowKey, showSystem])
}

export const k8sClientTableProps = {
  enableToolbar: true,
  enablePagination: true,
  disableParamPersistence: true,
  showTotalRows: true,
} as const

export function k8sInitialState(pageSize = 20, pinActions = true) {
  return {
    pagination: { pageIndex: 0, pageSize },
    ...(pinActions
      ? { columnPinning: { left: ["select"] as string[], right: ["actions"] as string[] } }
      : { columnPinning: { left: ["select"] as string[] } }),
  }
}
