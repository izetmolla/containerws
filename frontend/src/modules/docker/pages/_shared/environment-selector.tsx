import { useEffect, useMemo } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { Link } from "react-router"
import { Settings2 } from "lucide-react"

import { Button } from "@/components/ui/button"
import { ReactSelect } from "@/components/ui/reactselect"

import {
  activateDockerEnvironment,
  DOCKER_ENV_KEY,
  getStoredEnvironmentId,
  listDockerEnvironments,
  setStoredEnvironmentId,
} from "../environments/api"
import { DOCKER_ENGINE_KEY } from "../_shared/engine-api"
import { asArray } from "@/lib/as-array"

type Option = { value: string; label: string }

/** Environment picker shown on Docker list pages. */
export function EnvironmentSelector({ className }: { className?: string }) {
  const queryClient = useQueryClient()
  const listQuery = useQuery({
    queryKey: [DOCKER_ENV_KEY],
    queryFn: listDockerEnvironments,
    staleTime: 10_000,
  })

  const envs = asArray(listQuery.data?.data)
  const envList = envs
  const defaultId = envList.find((e) => e.is_default)?.id
  const stored = getStoredEnvironmentId()
  const selected =
    (stored && envList.some((e) => e.id === stored) ? stored : null) ||
    defaultId ||
    envList[0]?.id ||
    ""

  useEffect(() => {
    if (!selected) return
    if (stored !== selected) setStoredEnvironmentId(selected)
  }, [selected, stored])

  const options: Option[] = useMemo(
    () =>
      envs
        .filter((e) => !e.is_disabled)
        .map((e) => ({
          value: e.id,
          label: `${e.name} (${e.conn_type})${e.is_default ? " · default" : ""}`,
        })),
    [envs]
  )

  const onChange = async (id: string | null) => {
    if (!id) return
    setStoredEnvironmentId(id)
    try {
      await activateDockerEnvironment(id)
    } catch {
      // still use local selection even if activate fails
    }
    void queryClient.invalidateQueries({ queryKey: [DOCKER_ENV_KEY] })
    void queryClient.invalidateQueries({ queryKey: [DOCKER_ENGINE_KEY] })
    void queryClient.invalidateQueries({ queryKey: ["docker-containers"] })
    void queryClient.invalidateQueries({ queryKey: ["docker-images"] })
    void queryClient.invalidateQueries({ queryKey: ["docker-networks"] })
    void queryClient.invalidateQueries({ queryKey: ["docker-volumes"] })
    void queryClient.invalidateQueries({ queryKey: ["docker-stacks"] })
    void queryClient.invalidateQueries({ queryKey: ["docker-templates"] })
  }

  if (!options.length) {
    return (
      <Button size="sm" variant="outline" asChild className={className}>
        <Link to="/docker/environments">
          <Settings2 data-icon="inline-start" />
          Environments
        </Link>
      </Button>
    )
  }

  return (
    <div className={`flex flex-wrap items-center gap-2 ${className || ""}`}>
      <div className="min-w-[14rem]">
        <ReactSelect<Option, false>
          size="sm"
          options={options}
          value={selected}
          onValueChange={(v) => void onChange(v)}
          placeholder="Docker environment"
        />
      </div>
      <Button size="sm" variant="ghost" asChild>
        <Link to="/docker/environments">
          <Settings2 className="size-3.5" />
        </Link>
      </Button>
    </div>
  )
}
