import { useEffect, useMemo, useState } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"

import { ReactSelect } from "@/components/ui/reactselect"

import {
  getStoredNamespace,
  K8S_CONFIGMAPS_KEY,
  K8S_DEPLOYMENTS_KEY,
  K8S_EVENTS_KEY,
  K8S_INGRESSES_KEY,
  K8S_NAMESPACES_KEY,
  K8S_PODS_KEY,
  K8S_SECRETS_KEY,
  K8S_SERVICES_KEY,
  listNamespaces,
  setStoredNamespace,
} from "./api"
import {
  isSystemNamespace,
  useShowSystemResources,
} from "./system-resources"

const ALL = "__all__"
const NS_EVENT = "cws-k8s-namespace"

type NamespaceOption = { label: string; value: string }

export function useNamespaceFilter() {
  const [ns, setNs] = useState(getStoredNamespace)
  useEffect(() => {
    const onChange = () => setNs(getStoredNamespace())
    window.addEventListener(NS_EVENT, onChange)
    return () => window.removeEventListener(NS_EVENT, onChange)
  }, [])
  return ns
}

function invalidateNamespacedLists(
  queryClient: ReturnType<typeof useQueryClient>,
) {
  for (const key of [
    K8S_PODS_KEY,
    K8S_DEPLOYMENTS_KEY,
    K8S_SERVICES_KEY,
    K8S_CONFIGMAPS_KEY,
    K8S_SECRETS_KEY,
    K8S_EVENTS_KEY,
    K8S_INGRESSES_KEY,
  ]) {
    void queryClient.invalidateQueries({ queryKey: [key] })
  }
}

export function NamespaceSelector() {
  const queryClient = useQueryClient()
  const [showSystem] = useShowSystemResources()
  const ns = useNamespaceFilter()
  const nsQuery = useQuery({
    queryKey: [K8S_NAMESPACES_KEY],
    queryFn: listNamespaces,
    staleTime: 30_000,
  })
  const current = ns || ALL
  const rows = nsQuery.data?.data ?? []

  const options = useMemo<NamespaceOption[]>(() => {
    const visible = showSystem
      ? rows
      : rows.filter((n) => !isSystemNamespace(n.name))
    return [
      { label: "All namespaces", value: ALL },
      ...visible.map((n) => ({ label: n.name, value: n.name })),
    ]
  }, [rows, showSystem])

  return (
    <div className="w-[220px]">
      <ReactSelect<NamespaceOption, false>
        size="sm"
        options={options}
        value={current}
        isSearchable
        isClearable={false}
        isLoading={nsQuery.isLoading}
        placeholder="Namespace"
        noOptionsMessage={() => "No namespaces"}
        onValueChange={(v) => {
          if (!v) return
          const next = v === ALL ? "" : v
          setStoredNamespace(next)
          window.dispatchEvent(new Event(NS_EVENT))
          invalidateNamespacedLists(queryClient)
        }}
      />
    </div>
  )
}
