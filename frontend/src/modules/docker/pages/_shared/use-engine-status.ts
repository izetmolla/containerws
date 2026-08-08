import { useQuery } from "@tanstack/react-query"

import { DOCKER_ENGINE_KEY, getEngineStatus } from "./engine-api"

export function useEngineStatus() {
  return useQuery({
    queryKey: [DOCKER_ENGINE_KEY],
    queryFn: getEngineStatus,
    refetchInterval: 15_000,
  })
}

/** True when engine status has loaded and is unreachable. */
export function useEngineDown() {
  const q = useEngineStatus()
  return Boolean(q.data?.data && !q.data.data.reachable)
}

/** True when Docker needs attention (unreachable or env disabled). */
export function useDockerNeedsAttention() {
  const q = useEngineStatus()
  const data = q.data?.data
  if (!data || q.isLoading) return false
  return !data.reachable || Boolean(data.env_disabled)
}
