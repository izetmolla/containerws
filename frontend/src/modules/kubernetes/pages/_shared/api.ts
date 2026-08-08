import ApiService from "@/lib/network"
import { baseApiURL } from "@/lib/network/env"
import { getCurrentTokens } from "@/store/authorization"
import useAuthorizationStore from "@/store/authorization"

export const K8S_CONFIG_KEY = "k8s-config"
export const K8S_CLUSTER_KEY = "k8s-cluster"
export const K8S_NAMESPACES_KEY = "k8s-namespaces"
export const K8S_PODS_KEY = "k8s-pods"
export const K8S_DEPLOYMENTS_KEY = "k8s-deployments"
export const K8S_STATEFULSETS_KEY = "k8s-statefulsets"
export const K8S_DAEMONSETS_KEY = "k8s-daemonsets"
export const K8S_JOBS_KEY = "k8s-jobs"
export const K8S_CRONJOBS_KEY = "k8s-cronjobs"
export const K8S_SERVICES_KEY = "k8s-services"
export const K8S_INGRESSES_KEY = "k8s-ingresses"
export const K8S_NETWORK_POLICIES_KEY = "k8s-network-policies"
export const K8S_CONFIGMAPS_KEY = "k8s-configmaps"
export const K8S_SECRETS_KEY = "k8s-secrets"
export const K8S_PVCS_KEY = "k8s-pvcs"
export const K8S_APPLICATIONS_KEY = "k8s-applications"
export const K8S_EVENTS_KEY = "k8s-events"
export const K8S_NODES_KEY = "k8s-nodes"
export const K8S_NODE_DETAIL_KEY = "k8s-node-detail"
export const K8S_HOST_METRICS_KEY = "k8s-host-metrics"

export const K8S_CONFIG_BASE = "/kubernetes/config"
export const K8S_CLUSTER_BASE = "/kubernetes/cluster"
export const K8S_NODES_BASE = "/kubernetes/nodes"
export const K8S_NODES_LIST = "/kubernetes/nodes/list"
export const K8S_NODES_SINGLE = "/kubernetes/nodes/single"
export const K8S_NAMESPACES_LIST = "/kubernetes/namespaces/list"
export const K8S_NAMESPACES_SINGLE = "/kubernetes/namespaces/single"
export const K8S_WORKLOADS_BASE = "/kubernetes/workloads"
export const K8S_SERVICES_LIST = "/kubernetes/services/list"
export const K8S_SERVICES_SINGLE = "/kubernetes/services/single"
export const K8S_INGRESSES_LIST = "/kubernetes/ingresses/list"
export const K8S_INGRESSES_SINGLE = "/kubernetes/ingresses/single"
export const K8S_INGRESSES_OPTIONS = "/kubernetes/ingresses/options"
export const K8S_NETWORK_POLICIES_LIST = "/kubernetes/network-policies/list"
export const K8S_NETWORK_POLICIES_SINGLE = "/kubernetes/network-policies/single"
export const K8S_CONFIGS_BASE = "/kubernetes/configs"
export const K8S_STORAGE_BASE = "/kubernetes/storage"
export const K8S_PVCS_LIST = "/kubernetes/storage/persistentvolumeclaims/list"
export const K8S_PVCS_SINGLE = "/kubernetes/storage/persistentvolumeclaims/single"
export const K8S_APPLICATIONS_BASE = "/kubernetes/applications"

const NS_STORAGE_KEY = "cws.k8s.namespace"

export function getStoredNamespace(): string {
  try {
    return localStorage.getItem(NS_STORAGE_KEY) || ""
  } catch {
    return ""
  }
}

export function setStoredNamespace(ns: string) {
  try {
    if (ns) localStorage.setItem(NS_STORAGE_KEY, ns)
    else localStorage.removeItem(NS_STORAGE_KEY)
  } catch {
    /* ignore */
  }
}

export function nsQueryParams(namespace?: string | null) {
  const ns = namespace ?? getStoredNamespace()
  return ns ? { namespace: ns } : undefined
}

export type KubeContext = {
  name: string
  cluster: string
  user: string
  namespace?: string
  current: boolean
}

export type KubeConfigFile = {
  id: string
  name: string
  path: string
  managed: boolean
  exists: boolean
  active: boolean
  updated_at?: string
}

export type KubeConfigSettings = {
  path: string
  context: string
  exists: boolean
  default_path: string
  contexts: KubeContext[]
  context_count?: number
  active_id?: string
  files?: KubeConfigFile[]
  secret_map?: KubeSecretMapEntry[]
}

export type KubeSecretMapEntry = {
  id: string
  name: string
  path: string
  managed: boolean
  exists: boolean
  active: boolean
  contexts: KubeContext[]
}

export type KubeConfigFileDetail = {
  file: KubeConfigFile
  content: string
  contexts: KubeContext[]
  current?: string
}

export type ClusterStatus = {
  reachable: boolean
  path?: string
  context?: string
  exists?: boolean
  error?: string
  version?: string
  platform?: string
  nodes?: number
  nodes_ready?: number
  namespaces?: number
  pods?: number
  pods_running?: number
  deployments?: number
  services?: number
}

export async function getKubeConfig() {
  return ApiService.fetchData<{ data: KubeConfigSettings }>({
    url: K8S_CONFIG_BASE,
    method: "get",
  })
}

export async function updateKubeConfig(body: {
  path?: string
  context?: string
  active_id?: string
}) {
  return ApiService.fetchData<{ data: KubeConfigSettings; message?: string }>({
    url: K8S_CONFIG_BASE,
    method: "put",
    data: body,
  })
}

export async function testKubeConfig(body?: {
  path?: string
  context?: string
  active_id?: string
}) {
  return ApiService.fetchData<{
    data: {
      ok: boolean
      error?: string
      path?: string
      context?: string
      version?: string
      platform?: string
      namespace_count?: number
    }
  }>({
    url: `${K8S_CONFIG_BASE}/test`,
    method: "post",
    data: body ?? {},
  })
}

export async function listKubeConfigFiles() {
  return ApiService.fetchData<{
    data: { files: KubeConfigFile[]; active_id?: string }
  }>({
    url: `${K8S_CONFIG_BASE}/files/list`,
    method: "get",
  })
}

export async function getKubeConfigFile(id: string) {
  return ApiService.fetchData<{ data: KubeConfigFileDetail }>({
    url: `${K8S_CONFIG_BASE}/files/single/${encodeURIComponent(id)}`,
    method: "get",
  })
}

export async function createKubeConfigFile(body: {
  name: string
  content: string
}) {
  return ApiService.fetchData<{
    data: {
      file: KubeConfigFile
      files: KubeConfigFile[]
      active_id?: string
    }
    message?: string
  }>({
    url: `${K8S_CONFIG_BASE}/files/single`,
    method: "post",
    data: body,
  })
}

export async function updateKubeConfigFile(
  id: string,
  body: { name?: string; content?: string },
) {
  return ApiService.fetchData<{
    data: KubeConfigFileDetail & { files?: KubeConfigFile[] }
    message?: string
  }>({
    url: `${K8S_CONFIG_BASE}/files/single/${encodeURIComponent(id)}`,
    method: "put",
    data: body,
  })
}

export async function deleteKubeConfigFile(id: string) {
  return ApiService.fetchData<{
    data: { files: KubeConfigFile[]; active_id?: string }
    message?: string
  }>({
    url: `${K8S_CONFIG_BASE}/files/single/${encodeURIComponent(id)}`,
    method: "delete",
  })
}

export async function activateKubeConfigFile(
  id: string,
  body?: { context?: string },
) {
  return ApiService.fetchData<{
    data: {
      files: KubeConfigFile[]
      active_id?: string
      path?: string
      context?: string
      contexts?: KubeContext[]
    }
    message?: string
  }>({
    url: `${K8S_CONFIG_BASE}/files/single/${encodeURIComponent(id)}/activate`,
    method: "post",
    data: body ?? {},
  })
}

export async function getClusterStatus() {
  return ApiService.fetchData<{ data: ClusterStatus }>({
    url: `${K8S_CLUSTER_BASE}/status`,
    method: "get",
  })
}

export type NodeCondition = {
  type: string
  status: string
  reason?: string
  message?: string
  last_heartbeat?: string
  last_transition?: string
}

export type NodeRow = {
  name: string
  status: string
  roles: string
  version: string
  os_image: string
  kernel: string
  architecture?: string
  container_runtime: string
  cpu: string
  cpu_allocatable?: string
  memory: string
  memory_allocatable?: string
  pods_capacity?: string
  pod_count: number
  unschedulable: boolean
  internal_ip?: string
  external_ip?: string
  hostname?: string
  created_at: string
  conditions?: NodeCondition[]
  addresses?: { type: string; address: string }[]
  taints?: { key: string; value?: string; effect: string }[]
  labels?: Record<string, string>
}

export type NodePodRow = {
  namespace: string
  name: string
  status: string
  ready: string
  restarts: number
  ip?: string
  created_at: string
}

export async function listNodes() {
  return ApiService.fetchData<{ data: NodeRow[] }>({
    url: K8S_NODES_LIST,
    method: "get",
  })
}

export async function getNode(name: string) {
  return ApiService.fetchData<{ data: NodeRow }>({
    url: `${K8S_NODES_SINGLE}/${encodeURIComponent(name)}`,
    method: "get",
  })
}

export async function listNodePods(name: string) {
  return ApiService.fetchData<{ data: NodePodRow[] }>({
    url: `${K8S_NODES_SINGLE}/${encodeURIComponent(name)}/pods`,
    method: "get",
  })
}

export async function cordonNode(name: string) {
  return ApiService.fetchData<{ message?: string }>({
    url: `${K8S_NODES_SINGLE}/${encodeURIComponent(name)}/cordon`,
    method: "post",
  })
}

export async function uncordonNode(name: string) {
  return ApiService.fetchData<{ message?: string }>({
    url: `${K8S_NODES_SINGLE}/${encodeURIComponent(name)}/uncordon`,
    method: "post",
  })
}

export async function getHostMetrics() {
  return ApiService.fetchData<{
    data: { ok: boolean; metrics?: unknown; note?: string; error?: string }
  }>({
    url: `${K8S_NODES_BASE}/host-metrics`,
    method: "get",
  })
}

export type EventRow = {
  namespace: string
  name: string
  type: string
  reason: string
  message: string
  object: string
  count: number
  last_seen: string
}

export async function listEvents(
  namespace?: string,
  opts?: { kind?: string; name?: string },
) {
  return ApiService.fetchData<{ data: EventRow[] }>({
    url: `${K8S_CLUSTER_BASE}/events`,
    method: "get",
    params: {
      ...nsQueryParams(namespace),
      ...(opts?.kind ? { kind: opts.kind } : {}),
      ...(opts?.name ? { name: opts.name } : {}),
    },
  })
}

export type NamespaceRow = {
  name: string
  status: string
  created_at: string
  labels?: Record<string, string>
}

export async function listNamespaces() {
  return ApiService.fetchData<{ data: NamespaceRow[] }>({
    url: K8S_NAMESPACES_LIST,
    method: "get",
  })
}

export type NamespaceDetail = NamespaceRow & {
  annotations?: Record<string, string>
  pods: number
  deployments: number
  services: number
  configmaps: number
  secrets: number
}

export async function getNamespace(name: string) {
  return ApiService.fetchData<{ data: NamespaceDetail }>({
    url: `${K8S_NAMESPACES_SINGLE}/${encodeURIComponent(name)}`,
    method: "get",
  })
}

export async function getNamespaceYaml(name: string) {
  return ApiService.fetchData<{ data: { name: string; yaml: string } }>({
    url: `${K8S_NAMESPACES_SINGLE}/${encodeURIComponent(name)}/yaml`,
    method: "get",
  })
}

export async function applyNamespaceYaml(name: string, yaml: string) {
  return ApiService.fetchData<{
    data: { yaml: string; name: string; kind: string }
    message?: string
  }>({
    url: `${K8S_NAMESPACES_SINGLE}/${encodeURIComponent(name)}/yaml`,
    method: "put",
    data: { yaml },
  })
}

export async function createNamespace(name: string, labels?: Record<string, string>) {
  return ApiService.fetchData<{ data: NamespaceRow; message?: string }>({
    url: `${K8S_NAMESPACES_SINGLE}/`,
    method: "post",
    data: { name, labels },
  })
}

export async function deleteNamespace(name: string) {
  return ApiService.fetchData<{ data: { name: string }; message?: string }>({
    url: `${K8S_NAMESPACES_SINGLE}/${encodeURIComponent(name)}`,
    method: "delete",
  })
}

export type PodRow = {
  namespace: string
  name: string
  status: string
  ready: string
  restarts: number
  node?: string
  created_at: string
  ip?: string
}

export type PodContainer = {
  name: string
  image: string
  ready: boolean
  restart_count: number
  state: string
  started_at?: string
}

export type PodDetail = PodRow & {
  host_ip?: string
  qos_class?: string
  labels?: Record<string, string>
  annotations?: Record<string, string>
  containers: PodContainer[]
  conditions?: { type: string; status: string; reason?: string; message?: string }[]
  owner?: string
}

export async function listPods(namespace?: string) {
  return ApiService.fetchData<{ data: PodRow[] }>({
    url: `${K8S_WORKLOADS_BASE}/pods`,
    method: "get",
    params: nsQueryParams(namespace),
  })
}

export async function getPod(namespace: string, name: string) {
  return ApiService.fetchData<{ data: PodDetail }>({
    url: `${K8S_WORKLOADS_BASE}/pods/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`,
    method: "get",
  })
}

export async function getPodLogs(
  namespace: string,
  name: string,
  opts?: { container?: string; tail?: number },
) {
  return ApiService.fetchData<{
    data: {
      namespace: string
      name: string
      container: string
      tail: number
      logs: string
    }
  }>({
    url: `${K8S_WORKLOADS_BASE}/pods/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/logs`,
    method: "get",
    params: {
      container: opts?.container || undefined,
      tail: opts?.tail ?? 300,
    },
  })
}

export async function getPodYaml(namespace: string, name: string) {
  return ApiService.fetchData<{
    data: { namespace: string; name: string; yaml: string }
  }>({
    url: `${K8S_WORKLOADS_BASE}/pods/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/yaml`,
    method: "get",
  })
}

export async function applyPodYaml(
  namespace: string,
  name: string,
  yaml: string,
) {
  return ApiService.fetchData<{
    data: { yaml: string; namespace?: string; name: string; kind: string }
    message?: string
  }>({
    url: `${K8S_WORKLOADS_BASE}/pods/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/yaml`,
    method: "put",
    data: { yaml },
  })
}

/** WebSocket URL for interactive `kubectl exec` into a pod container. */
export function buildPodExecWebSocketURL(opts: {
  namespace: string
  name: string
  command: string
  container?: string
}): string {
  const tokens = getCurrentTokens(useAuthorizationStore.getState())
  const apiBase = baseApiURL()
  const wsBase = apiBase.replace(/^http/i, "ws")
  const url = new URL(
    `${wsBase}${K8S_WORKLOADS_BASE}/pods/${encodeURIComponent(opts.namespace)}/${encodeURIComponent(opts.name)}/exec`,
  )
  if (tokens?.access_token) {
    url.searchParams.set("access_token", tokens.access_token)
  }
  url.searchParams.set("command", opts.command)
  if (opts.container?.trim()) {
    url.searchParams.set("container", opts.container.trim())
  }
  return url.toString()
}

export async function deletePod(namespace: string, name: string) {
  return ApiService.fetchData<{ message?: string }>({
    url: `${K8S_WORKLOADS_BASE}/pods/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`,
    method: "delete",
  })
}

export type DeploymentRow = {
  namespace: string
  name: string
  ready: string
  up_to_date: number
  available: number
  replicas: number
  created_at: string
  images: string
}

export type DeploymentDetail = {
  namespace: string
  name: string
  ready: string
  up_to_date: number
  available: number
  replicas: number
  updated_replicas: number
  unavailable: number
  created_at: string
  images: string[]
  labels?: Record<string, string>
  selector?: Record<string, string>
  strategy?: string
  conditions?: { type: string; status: string; reason?: string; message?: string }[]
  containers?: { name: string; image: string }[]
}

export async function listDeployments(namespace?: string) {
  return ApiService.fetchData<{ data: DeploymentRow[] }>({
    url: `${K8S_WORKLOADS_BASE}/deployments`,
    method: "get",
    params: nsQueryParams(namespace),
  })
}

export async function createDeployment(body: {
  namespace: string
  name: string
  image: string
  replicas?: number
  labels?: Record<string, string>
  command?: string[]
  args?: string[]
  port?: number
}) {
  return ApiService.fetchData<{ data: DeploymentRow; message?: string }>({
    url: `${K8S_WORKLOADS_BASE}/deployments`,
    method: "post",
    data: body,
  })
}

export async function getDeployment(namespace: string, name: string) {
  return ApiService.fetchData<{ data: DeploymentDetail }>({
    url: `${K8S_WORKLOADS_BASE}/deployments/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`,
    method: "get",
  })
}

export async function listDeploymentPods(namespace: string, name: string) {
  return ApiService.fetchData<{ data: PodRow[] }>({
    url: `${K8S_WORKLOADS_BASE}/deployments/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/pods`,
    method: "get",
  })
}

export async function getDeploymentYaml(namespace: string, name: string) {
  return ApiService.fetchData<{
    data: { namespace: string; name: string; yaml: string }
  }>({
    url: `${K8S_WORKLOADS_BASE}/deployments/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/yaml`,
    method: "get",
  })
}

export async function applyDeploymentYaml(
  namespace: string,
  name: string,
  yaml: string,
) {
  return ApiService.fetchData<{
    data: { yaml: string; namespace?: string; name: string; kind: string }
    message?: string
  }>({
    url: `${K8S_WORKLOADS_BASE}/deployments/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/yaml`,
    method: "put",
    data: { yaml },
  })
}

export async function deleteDeployment(namespace: string, name: string) {
  return ApiService.fetchData<{ message?: string }>({
    url: `${K8S_WORKLOADS_BASE}/deployments/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`,
    method: "delete",
  })
}

export async function scaleDeployment(
  namespace: string,
  name: string,
  replicas: number,
) {
  return ApiService.fetchData<{ message?: string }>({
    url: `${K8S_WORKLOADS_BASE}/deployments/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/scale`,
    method: "post",
    data: { replicas },
  })
}

export async function restartDeployment(namespace: string, name: string) {
  return ApiService.fetchData<{ message?: string }>({
    url: `${K8S_WORKLOADS_BASE}/deployments/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/restart`,
    method: "post",
  })
}

export async function pullRestartDeployment(namespace: string, name: string) {
  return ApiService.fetchData<{ message?: string }>({
    url: `${K8S_WORKLOADS_BASE}/deployments/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/pull-restart`,
    method: "post",
  })
}

export type ServiceRow = {
  namespace: string
  name: string
  type: string
  cluster_ip: string
  external_ip?: string
  ports: string
  created_at: string
}

export type ServiceDetail = {
  namespace: string
  name: string
  type: string
  cluster_ip: string
  external_ip?: string
  ports: {
    name?: string
    port: number
    target_port: string
    node_port?: number
    protocol: string
  }[]
  selector?: Record<string, string>
  labels?: Record<string, string>
  annotations?: Record<string, string>
  created_at: string
}

export async function listServices(namespace?: string) {
  return ApiService.fetchData<{ data: ServiceRow[] }>({
    url: K8S_SERVICES_LIST,
    method: "get",
    params: nsQueryParams(namespace),
  })
}

export async function getService(namespace: string, name: string) {
  return ApiService.fetchData<{ data: ServiceDetail }>({
    url: `${K8S_SERVICES_SINGLE}/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`,
    method: "get",
  })
}

export async function getServiceYaml(namespace: string, name: string) {
  return ApiService.fetchData<{
    data: { namespace: string; name: string; yaml: string }
  }>({
    url: `${K8S_SERVICES_SINGLE}/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/yaml`,
    method: "get",
  })
}

export async function applyServiceYaml(
  namespace: string,
  name: string,
  yaml: string,
) {
  return ApiService.fetchData<{
    data: { yaml: string; namespace?: string; name: string; kind: string }
    message?: string
  }>({
    url: `${K8S_SERVICES_SINGLE}/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/yaml`,
    method: "put",
    data: { yaml },
  })
}

export async function createService(body: {
  namespace: string
  name: string
  type?: string
  port: number
  target_port?: number
  selector?: Record<string, string>
}) {
  return ApiService.fetchData<{ message?: string }>({
    url: K8S_SERVICES_SINGLE,
    method: "post",
    data: body,
  })
}

export async function deleteService(namespace: string, name: string) {
  return ApiService.fetchData<{ message?: string }>({
    url: `${K8S_SERVICES_SINGLE}/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`,
    method: "delete",
  })
}

export type IngressPath = {
  path: string
  path_type: string
  service_name: string
  service_port: number
  service_port_name?: string
}

export type IngressRule = {
  host: string
  paths: IngressPath[]
}

export type IngressTLS = {
  hosts: string[]
  secret_name: string
}

export type IngressRow = {
  namespace: string
  name: string
  class: string
  hosts: string
  address: string
  created_at: string
}

export type IngressDetail = {
  namespace: string
  name: string
  ingress_class: string
  hosts: string
  address: string
  rules: IngressRule[]
  tls: IngressTLS[]
  labels?: Record<string, string>
  annotations?: Record<string, string>
  created_at: string
}

export type IngressWriteBody = {
  namespace?: string
  name?: string
  ingress_class?: string
  rules: IngressRule[]
  tls?: IngressTLS[]
  labels?: Record<string, string>
  annotations?: Record<string, string>
}

export type IngressServicePortOption = {
  name?: string
  port: number
  protocol?: string
  target_port?: number
}

export type IngressServiceOption = {
  name: string
  type?: string
  ports: IngressServicePortOption[]
}

export type IngressClassOption = {
  name: string
  controller?: string
  default?: boolean
}

export type IngressTLSSecretOption = {
  name: string
}

export type IngressFormOptions = {
  namespace: string
  classes: IngressClassOption[]
  tls_secrets: IngressTLSSecretOption[]
  services: IngressServiceOption[]
}

export async function getIngressFormOptions(namespace: string) {
  return ApiService.fetchData<{ data: IngressFormOptions }>({
    url: K8S_INGRESSES_OPTIONS,
    method: "get",
    params: { namespace },
  })
}

export async function listIngresses(namespace?: string) {
  return ApiService.fetchData<{ data: IngressRow[] }>({
    url: K8S_INGRESSES_LIST,
    method: "get",
    params: nsQueryParams(namespace),
  })
}

export async function getIngress(namespace: string, name: string) {
  return ApiService.fetchData<{ data: IngressDetail }>({
    url: `${K8S_INGRESSES_SINGLE}/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`,
    method: "get",
  })
}

export async function getIngressYaml(namespace: string, name: string) {
  return ApiService.fetchData<{
    data: { namespace: string; name: string; yaml: string }
  }>({
    url: `${K8S_INGRESSES_SINGLE}/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/yaml`,
    method: "get",
  })
}

export async function applyIngressYaml(
  namespace: string,
  name: string,
  yaml: string,
) {
  return ApiService.fetchData<{
    data: { yaml: string; namespace?: string; name: string; kind: string }
    message?: string
  }>({
    url: `${K8S_INGRESSES_SINGLE}/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/yaml`,
    method: "put",
    data: { yaml },
  })
}

export async function createIngress(body: IngressWriteBody) {
  return ApiService.fetchData<{ message?: string; data?: IngressDetail }>({
    url: K8S_INGRESSES_SINGLE,
    method: "post",
    data: body,
  })
}

export async function updateIngress(
  namespace: string,
  name: string,
  body: IngressWriteBody,
) {
  return ApiService.fetchData<{ message?: string; data?: IngressDetail }>({
    url: `${K8S_INGRESSES_SINGLE}/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`,
    method: "put",
    data: body,
  })
}

export async function deleteIngress(namespace: string, name: string) {
  return ApiService.fetchData<{ message?: string }>({
    url: `${K8S_INGRESSES_SINGLE}/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`,
    method: "delete",
  })
}

export type ConfigMapRow = {
  namespace: string
  name: string
  keys: number
  created_at: string
}

export type ConfigMapDetail = ConfigMapRow & {
  labels?: Record<string, string>
  annotations?: Record<string, string>
  data: Record<string, string>
  binary_keys?: string[]
}

export async function listConfigMaps(namespace?: string) {
  return ApiService.fetchData<{ data: ConfigMapRow[] }>({
    url: `${K8S_CONFIGS_BASE}/configmaps`,
    method: "get",
    params: nsQueryParams(namespace),
  })
}

export async function getConfigMap(namespace: string, name: string) {
  return ApiService.fetchData<{ data: ConfigMapDetail }>({
    url: `${K8S_CONFIGS_BASE}/configmaps/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`,
    method: "get",
  })
}

export async function updateConfigMap(
  namespace: string,
  name: string,
  data: Record<string, string>,
) {
  return ApiService.fetchData<{ data: ConfigMapDetail; message?: string }>({
    url: `${K8S_CONFIGS_BASE}/configmaps/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`,
    method: "put",
    data: { data },
  })
}

export async function getConfigMapYaml(namespace: string, name: string) {
  return ApiService.fetchData<{
    data: { namespace: string; name: string; yaml: string }
  }>({
    url: `${K8S_CONFIGS_BASE}/configmaps/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/yaml`,
    method: "get",
  })
}

export async function applyConfigMapYaml(
  namespace: string,
  name: string,
  yaml: string,
) {
  return ApiService.fetchData<{
    data: { yaml: string; namespace?: string; name: string; kind: string }
    message?: string
  }>({
    url: `${K8S_CONFIGS_BASE}/configmaps/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/yaml`,
    method: "put",
    data: { yaml },
  })
}

export async function createConfigMap(body: {
  namespace: string
  name: string
  data?: Record<string, string>
}) {
  return ApiService.fetchData<{ message?: string }>({
    url: `${K8S_CONFIGS_BASE}/configmaps`,
    method: "post",
    data: body,
  })
}

export async function deleteConfigMap(namespace: string, name: string) {
  return ApiService.fetchData<{ message?: string }>({
    url: `${K8S_CONFIGS_BASE}/configmaps/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`,
    method: "delete",
  })
}

export type SecretRow = {
  namespace: string
  name: string
  type: string
  keys: number
  created_at: string
}

export type SecretDetail = SecretRow & {
  labels?: Record<string, string>
  annotations?: Record<string, string>
  data: Record<string, string>
}

export async function listSecrets(namespace?: string) {
  return ApiService.fetchData<{ data: SecretRow[] }>({
    url: `${K8S_CONFIGS_BASE}/secrets`,
    method: "get",
    params: nsQueryParams(namespace),
  })
}

export async function getSecret(namespace: string, name: string) {
  return ApiService.fetchData<{ data: SecretDetail }>({
    url: `${K8S_CONFIGS_BASE}/secrets/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`,
    method: "get",
  })
}

export async function updateSecret(
  namespace: string,
  name: string,
  data: Record<string, string>,
) {
  return ApiService.fetchData<{ data: SecretDetail; message?: string }>({
    url: `${K8S_CONFIGS_BASE}/secrets/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`,
    method: "put",
    data: { data },
  })
}

export async function createSecret(body: {
  namespace: string
  name: string
  type?: string
  data?: Record<string, string>
}) {
  return ApiService.fetchData<{ message?: string }>({
    url: `${K8S_CONFIGS_BASE}/secrets`,
    method: "post",
    data: body,
  })
}

export async function getSecretYaml(namespace: string, name: string) {
  return ApiService.fetchData<{
    data: { namespace: string; name: string; yaml: string }
  }>({
    url: `${K8S_CONFIGS_BASE}/secrets/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/yaml`,
    method: "get",
  })
}

export async function applySecretYaml(
  namespace: string,
  name: string,
  yaml: string,
) {
  return ApiService.fetchData<{
    data: { yaml: string; namespace?: string; name: string; kind: string }
    message?: string
  }>({
    url: `${K8S_CONFIGS_BASE}/secrets/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/yaml`,
    method: "put",
    data: { yaml },
  })
}

export async function deleteSecret(namespace: string, name: string) {
  return ApiService.fetchData<{ message?: string }>({
    url: `${K8S_CONFIGS_BASE}/secrets/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`,
    method: "delete",
  })
}

/* ── StatefulSets ─────────────────────────────────────────────── */

export type StatefulSetRow = {
  namespace: string
  name: string
  ready: string
  replicas: number
  service_name?: string
  created_at: string
  images: string
}

export type StatefulSetDetail = {
  namespace: string
  name: string
  ready: string
  replicas: number
  current_replicas?: number
  updated_replicas?: number
  service_name?: string
  update_strategy?: string
  images: string[]
  containers?: { name: string; image: string }[]
  selector?: Record<string, string>
  labels?: Record<string, string>
  annotations?: Record<string, string>
  created_at: string
}

export async function listStatefulSets(namespace?: string) {
  return ApiService.fetchData<{ data: StatefulSetRow[] }>({
    url: `${K8S_WORKLOADS_BASE}/statefulsets`,
    method: "get",
    params: nsQueryParams(namespace),
  })
}

export async function getStatefulSet(namespace: string, name: string) {
  return ApiService.fetchData<{ data: StatefulSetDetail }>({
    url: `${K8S_WORKLOADS_BASE}/statefulsets/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`,
    method: "get",
  })
}

export async function createStatefulSet(body: {
  namespace: string
  name: string
  image: string
  replicas?: number
  service_name?: string
  create_service?: boolean
  port?: number
  labels?: Record<string, string>
  command?: string[]
  args?: string[]
}) {
  return ApiService.fetchData<{ data: StatefulSetRow; message?: string }>({
    url: `${K8S_WORKLOADS_BASE}/statefulsets`,
    method: "post",
    data: body,
  })
}

export async function listStatefulSetPods(namespace: string, name: string) {
  return ApiService.fetchData<{ data: PodRow[] }>({
    url: `${K8S_WORKLOADS_BASE}/statefulsets/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/pods`,
    method: "get",
  })
}

export async function getStatefulSetYaml(namespace: string, name: string) {
  return ApiService.fetchData<{
    data: { namespace: string; name: string; yaml: string }
  }>({
    url: `${K8S_WORKLOADS_BASE}/statefulsets/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/yaml`,
    method: "get",
  })
}

export async function applyStatefulSetYaml(
  namespace: string,
  name: string,
  yaml: string,
) {
  return ApiService.fetchData<{
    data: { yaml: string; namespace?: string; name: string; kind: string }
    message?: string
  }>({
    url: `${K8S_WORKLOADS_BASE}/statefulsets/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/yaml`,
    method: "put",
    data: { yaml },
  })
}

export async function deleteStatefulSet(namespace: string, name: string) {
  return ApiService.fetchData<{ message?: string }>({
    url: `${K8S_WORKLOADS_BASE}/statefulsets/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`,
    method: "delete",
  })
}

export async function scaleStatefulSet(
  namespace: string,
  name: string,
  replicas: number,
) {
  return ApiService.fetchData<{ message?: string }>({
    url: `${K8S_WORKLOADS_BASE}/statefulsets/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/scale`,
    method: "post",
    data: { replicas },
  })
}

export async function restartStatefulSet(namespace: string, name: string) {
  return ApiService.fetchData<{ message?: string }>({
    url: `${K8S_WORKLOADS_BASE}/statefulsets/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/restart`,
    method: "post",
  })
}

/* ── DaemonSets ───────────────────────────────────────────────── */

export type DaemonSetRow = {
  namespace: string
  name: string
  desired: number
  current: number
  ready: number
  up_to_date: number
  available: number
  created_at: string
  images: string
}

export type DaemonSetDetail = {
  namespace: string
  name: string
  desired: number
  current: number
  ready: number
  up_to_date: number
  available: number
  images: string[]
  containers?: { name: string; image: string }[]
  selector?: Record<string, string>
  labels?: Record<string, string>
  annotations?: Record<string, string>
  created_at: string
}

export async function listDaemonSets(namespace?: string) {
  return ApiService.fetchData<{ data: DaemonSetRow[] }>({
    url: `${K8S_WORKLOADS_BASE}/daemonsets`,
    method: "get",
    params: nsQueryParams(namespace),
  })
}

export async function getDaemonSet(namespace: string, name: string) {
  return ApiService.fetchData<{ data: DaemonSetDetail }>({
    url: `${K8S_WORKLOADS_BASE}/daemonsets/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`,
    method: "get",
  })
}

export async function createDaemonSet(body: {
  namespace: string
  name: string
  image: string
  labels?: Record<string, string>
  command?: string[]
  args?: string[]
}) {
  return ApiService.fetchData<{ data: DaemonSetRow; message?: string }>({
    url: `${K8S_WORKLOADS_BASE}/daemonsets`,
    method: "post",
    data: body,
  })
}

export async function listDaemonSetPods(namespace: string, name: string) {
  return ApiService.fetchData<{ data: PodRow[] }>({
    url: `${K8S_WORKLOADS_BASE}/daemonsets/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/pods`,
    method: "get",
  })
}

export async function getDaemonSetYaml(namespace: string, name: string) {
  return ApiService.fetchData<{
    data: { namespace: string; name: string; yaml: string }
  }>({
    url: `${K8S_WORKLOADS_BASE}/daemonsets/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/yaml`,
    method: "get",
  })
}

export async function applyDaemonSetYaml(
  namespace: string,
  name: string,
  yaml: string,
) {
  return ApiService.fetchData<{
    data: { yaml: string; namespace?: string; name: string; kind: string }
    message?: string
  }>({
    url: `${K8S_WORKLOADS_BASE}/daemonsets/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/yaml`,
    method: "put",
    data: { yaml },
  })
}

export async function deleteDaemonSet(namespace: string, name: string) {
  return ApiService.fetchData<{ message?: string }>({
    url: `${K8S_WORKLOADS_BASE}/daemonsets/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`,
    method: "delete",
  })
}

export async function restartDaemonSet(namespace: string, name: string) {
  return ApiService.fetchData<{ message?: string }>({
    url: `${K8S_WORKLOADS_BASE}/daemonsets/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/restart`,
    method: "post",
  })
}

/* ── Jobs / CronJobs ──────────────────────────────────────────── */

export type JobRow = {
  namespace: string
  name: string
  completions: string
  succeeded: number
  failed: number
  active: number
  duration?: string
  created_at: string
  images: string
}

export type JobDetail = {
  namespace: string
  name: string
  completions: string
  succeeded: number
  failed: number
  active: number
  duration?: string
  parallelism?: number
  backoff_limit?: number
  images: string[]
  containers?: { name: string; image: string }[]
  conditions?: { type: string; status: string; reason?: string; message?: string }[]
  labels?: Record<string, string>
  annotations?: Record<string, string>
  created_at: string
}

export type CronJobRow = {
  namespace: string
  name: string
  schedule: string
  suspend: boolean
  active: number
  last_schedule?: string
  created_at: string
  images: string
}

export type CronJobDetail = {
  namespace: string
  name: string
  schedule: string
  suspend: boolean
  concurrency?: string
  active: number
  last_schedule?: string
  images: string[]
  labels?: Record<string, string>
  annotations?: Record<string, string>
  created_at: string
}

export async function listJobs(namespace?: string) {
  return ApiService.fetchData<{ data: JobRow[] }>({
    url: `${K8S_WORKLOADS_BASE}/jobs`,
    method: "get",
    params: nsQueryParams(namespace),
  })
}

export async function getJob(namespace: string, name: string) {
  return ApiService.fetchData<{ data: JobDetail }>({
    url: `${K8S_WORKLOADS_BASE}/jobs/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`,
    method: "get",
  })
}

export async function createJob(body: {
  namespace: string
  name: string
  image: string
  command?: string[]
  args?: string[]
  completions?: number
  parallelism?: number
  backoff_limit?: number
  restart_policy?: "Never" | "OnFailure"
}) {
  return ApiService.fetchData<{ data: JobRow; message?: string }>({
    url: `${K8S_WORKLOADS_BASE}/jobs`,
    method: "post",
    data: body,
  })
}

export async function getJobYaml(namespace: string, name: string) {
  return ApiService.fetchData<{
    data: { namespace: string; name: string; yaml: string }
  }>({
    url: `${K8S_WORKLOADS_BASE}/jobs/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/yaml`,
    method: "get",
  })
}

export async function applyJobYaml(
  namespace: string,
  name: string,
  yaml: string,
) {
  return ApiService.fetchData<{
    data: { yaml: string; namespace?: string; name: string; kind: string }
    message?: string
  }>({
    url: `${K8S_WORKLOADS_BASE}/jobs/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/yaml`,
    method: "put",
    data: { yaml },
  })
}

export async function deleteJob(namespace: string, name: string) {
  return ApiService.fetchData<{ message?: string }>({
    url: `${K8S_WORKLOADS_BASE}/jobs/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`,
    method: "delete",
  })
}

export async function listCronJobs(namespace?: string) {
  return ApiService.fetchData<{ data: CronJobRow[] }>({
    url: `${K8S_WORKLOADS_BASE}/cronjobs`,
    method: "get",
    params: nsQueryParams(namespace),
  })
}

export async function getCronJob(namespace: string, name: string) {
  return ApiService.fetchData<{ data: CronJobDetail }>({
    url: `${K8S_WORKLOADS_BASE}/cronjobs/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`,
    method: "get",
  })
}

export async function getCronJobYaml(namespace: string, name: string) {
  return ApiService.fetchData<{
    data: { namespace: string; name: string; yaml: string }
  }>({
    url: `${K8S_WORKLOADS_BASE}/cronjobs/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/yaml`,
    method: "get",
  })
}

export async function applyCronJobYaml(
  namespace: string,
  name: string,
  yaml: string,
) {
  return ApiService.fetchData<{
    data: { yaml: string; namespace?: string; name: string; kind: string }
    message?: string
  }>({
    url: `${K8S_WORKLOADS_BASE}/cronjobs/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/yaml`,
    method: "put",
    data: { yaml },
  })
}

export async function deleteCronJob(namespace: string, name: string) {
  return ApiService.fetchData<{ message?: string }>({
    url: `${K8S_WORKLOADS_BASE}/cronjobs/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`,
    method: "delete",
  })
}

export async function createCronJob(body: {
  namespace: string
  name: string
  schedule: string
  image: string
  labels?: Record<string, string>
  command?: string[]
  args?: string[]
  suspend?: boolean
}) {
  return ApiService.fetchData<{
    data: { namespace: string; name: string; schedule: string; suspend: boolean }
    message?: string
  }>({
    url: `${K8S_WORKLOADS_BASE}/cronjobs`,
    method: "post",
    data: body,
  })
}

export async function suspendCronJob(namespace: string, name: string) {
  return ApiService.fetchData<{ message?: string }>({
    url: `${K8S_WORKLOADS_BASE}/cronjobs/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/suspend`,
    method: "post",
    data: {},
  })
}

export async function resumeCronJob(namespace: string, name: string) {
  return ApiService.fetchData<{ message?: string }>({
    url: `${K8S_WORKLOADS_BASE}/cronjobs/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/resume`,
    method: "post",
    data: {},
  })
}

export async function triggerCronJob(namespace: string, name: string) {
  return ApiService.fetchData<{
    data: { namespace: string; name: string; cronjob: string }
    message?: string
  }>({
    url: `${K8S_WORKLOADS_BASE}/cronjobs/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/trigger`,
    method: "post",
    data: {},
  })
}

/* ── NetworkPolicies ──────────────────────────────────────────── */

export type NetworkPolicyRow = {
  namespace: string
  name: string
  pod_selector: string
  policy_types: string
  ingress_rules: number
  egress_rules: number
  created_at: string
}

export type NetworkPolicyDetail = {
  namespace: string
  name: string
  pod_selector: string
  policy_types: string[]
  ingress: {
    ports?: { protocol?: string; port?: string }[]
    from?: Record<string, unknown>[]
  }[]
  egress: {
    ports?: { protocol?: string; port?: string }[]
    to?: Record<string, unknown>[]
  }[]
  labels?: Record<string, string>
  annotations?: Record<string, string>
  created_at: string
}

export async function listNetworkPolicies(namespace?: string) {
  return ApiService.fetchData<{ data: NetworkPolicyRow[] }>({
    url: `${K8S_NETWORK_POLICIES_LIST}/`,
    method: "get",
    params: nsQueryParams(namespace),
  })
}

export async function getNetworkPolicy(namespace: string, name: string) {
  return ApiService.fetchData<{ data: NetworkPolicyDetail }>({
    url: `${K8S_NETWORK_POLICIES_SINGLE}/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`,
    method: "get",
  })
}

export async function createNetworkPolicy(body: {
  namespace: string
  name: string
  pod_selector?: Record<string, string>
  policy_types?: string[]
  allow_from_same_namespace?: boolean
}) {
  return ApiService.fetchData<{ data: NetworkPolicyRow; message?: string }>({
    url: `${K8S_NETWORK_POLICIES_SINGLE}/`,
    method: "post",
    data: body,
  })
}

export async function getNetworkPolicyYaml(namespace: string, name: string) {
  return ApiService.fetchData<{
    data: { namespace: string; name: string; yaml: string }
  }>({
    url: `${K8S_NETWORK_POLICIES_SINGLE}/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/yaml`,
    method: "get",
  })
}

export async function applyNetworkPolicyYaml(
  namespace: string,
  name: string,
  yaml: string,
) {
  return ApiService.fetchData<{
    data: { yaml: string; namespace?: string; name: string; kind: string }
    message?: string
  }>({
    url: `${K8S_NETWORK_POLICIES_SINGLE}/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/yaml`,
    method: "put",
    data: { yaml },
  })
}

export async function deleteNetworkPolicy(namespace: string, name: string) {
  return ApiService.fetchData<{ message?: string }>({
    url: `${K8S_NETWORK_POLICIES_SINGLE}/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`,
    method: "delete",
  })
}

/* ── PersistentVolumeClaims ───────────────────────────────────── */

export type PvcRow = {
  namespace: string
  name: string
  status: string
  volume?: string
  capacity?: string
  access_modes: string
  storage_class?: string
  created_at: string
}

export type PvcDetail = {
  namespace: string
  name: string
  status: string
  volume?: string
  capacity?: string
  request?: string
  access_modes?: string[]
  storage_class?: string
  volume_mode?: string
  labels?: Record<string, string>
  annotations?: Record<string, string>
  created_at: string
}

export async function listPvcs(namespace?: string) {
  return ApiService.fetchData<{ data: PvcRow[] }>({
    url: `${K8S_PVCS_LIST}/`,
    method: "get",
    params: nsQueryParams(namespace),
  })
}

export async function getPvc(namespace: string, name: string) {
  return ApiService.fetchData<{ data: PvcDetail }>({
    url: `${K8S_PVCS_SINGLE}/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`,
    method: "get",
  })
}

export type StorageClassRow = {
  name: string
  provisioner: string
  reclaim_policy?: string
  volume_binding_mode?: string
  allow_volume_expansion: boolean
  default: boolean
}

export const K8S_STORAGE_CLASSES_KEY = "k8s-storage-classes"
export const K8S_STORAGE_CLASSES_LIST =
  "/kubernetes/storage/storageclasses/list"

export async function listStorageClasses() {
  return ApiService.fetchData<{ data: StorageClassRow[] }>({
    url: `${K8S_STORAGE_CLASSES_LIST}/`,
    method: "get",
  })
}

export async function createPvc(body: {
  namespace: string
  name: string
  storage: string
  access_modes?: string[]
  storage_class?: string
  volume_mode?: "Filesystem" | "Block"
}) {
  return ApiService.fetchData<{ data: PvcRow; message?: string }>({
    url: `${K8S_PVCS_SINGLE}/`,
    method: "post",
    data: body,
  })
}

export async function getPvcYaml(namespace: string, name: string) {
  return ApiService.fetchData<{
    data: { namespace: string; name: string; yaml: string }
  }>({
    url: `${K8S_PVCS_SINGLE}/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/yaml`,
    method: "get",
  })
}

export async function applyPvcYaml(
  namespace: string,
  name: string,
  yaml: string,
) {
  return ApiService.fetchData<{
    data: { yaml: string; namespace?: string; name: string; kind: string }
    message?: string
  }>({
    url: `${K8S_PVCS_SINGLE}/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/yaml`,
    method: "put",
    data: { yaml },
  })
}

export async function deletePvc(namespace: string, name: string) {
  return ApiService.fetchData<{ message?: string }>({
    url: `${K8S_PVCS_SINGLE}/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`,
    method: "delete",
  })
}

/* ── Applications (saved YAML + live cluster status) ──────────── */

export type ManifestResult = {
  index: number
  apiVersion: string
  kind: string
  namespace?: string
  name: string
  action: string
  error?: string
}

export type ManifestApplySummary = {
  dry_run: boolean
  total: number
  applied: number
  failed: number
  results: ManifestResult[]
}

export type ManifestRef = {
  apiVersion: string
  kind: string
  name: string
  namespace?: string
  cluster_scoped?: boolean
}

export type K8sApplicationRow = {
  id: string
  name: string
  namespace: string
  version?: number
  resource_count: number
  status?: "healthy" | "partial" | "missing" | "empty" | "unknown"
  ready_count?: number
  missing_count?: number
  updated_at: string
  created_at: string
}

export type K8sApplication = {
  id: string
  name: string
  namespace: string
  yaml: string
  version?: number
  resources?: ManifestRef[] | unknown[]
  created_at?: string
  updated_at?: string
}

export type K8sApplicationRevisionRow = {
  id: string
  version: number
  name: string
  namespace: string
  source: string
  note?: string
  created_at: string
  current: boolean
}

export type K8sApplicationRevision = {
  id: string
  application_id: string
  version: number
  name: string
  namespace: string
  yaml: string
  resources?: ManifestRef[] | unknown[]
  source: string
  note?: string
  created_at: string
}

export type LiveAppResource = {
  apiVersion: string
  kind: string
  namespace?: string
  name: string
  exists: boolean
  status?: string
  ready?: string
  error?: string
}

export const K8S_APPLICATIONS_LIST = `${K8S_APPLICATIONS_BASE}/list`
export const K8S_APPLICATIONS_SINGLE = `${K8S_APPLICATIONS_BASE}/single`

export async function listApplications() {
  return ApiService.fetchData<{ data: K8sApplicationRow[] }>({
    url: `${K8S_APPLICATIONS_LIST}/`,
    method: "get",
  })
}

export async function getApplication(id: string) {
  return ApiService.fetchData<{ data: K8sApplication }>({
    url: `${K8S_APPLICATIONS_SINGLE}/${encodeURIComponent(id)}`,
    method: "get",
  })
}

export async function createApplication(body: {
  name: string
  namespace?: string
  yaml: string
}) {
  return ApiService.fetchData<{ data: K8sApplication; message?: string }>({
    url: `${K8S_APPLICATIONS_SINGLE}/`,
    method: "post",
    data: body,
  })
}

export async function updateApplication(
  id: string,
  body: { name: string; namespace?: string; yaml: string },
) {
  return ApiService.fetchData<{ data: K8sApplication; message?: string }>({
    url: `${K8S_APPLICATIONS_SINGLE}/${encodeURIComponent(id)}`,
    method: "put",
    data: body,
  })
}

export async function deleteApplication(id: string) {
  return ApiService.fetchData<{ message?: string }>({
    url: `${K8S_APPLICATIONS_SINGLE}/${encodeURIComponent(id)}`,
    method: "delete",
  })
}

export async function getApplicationResources(id: string) {
  return ApiService.fetchData<{
    data: {
      application: K8sApplicationRow
      namespace: string
      resources: LiveAppResource[]
    }
  }>({
    url: `${K8S_APPLICATIONS_SINGLE}/${encodeURIComponent(id)}/resources`,
    method: "get",
  })
}

export async function validateApplicationManifests(body: {
  yaml: string
  namespace?: string
}) {
  return ApiService.fetchData<{
    data: {
      namespace: string
      yaml: string
      resources: ManifestRef[]
      valid: boolean
    }
  }>({
    url: `${K8S_APPLICATIONS_BASE}/validate`,
    method: "post",
    data: body,
  })
}

export async function applyApplicationManifests(body: {
  yaml: string
  dry_run?: boolean
  namespace?: string
  default_namespace?: string
  name?: string
  id?: string
}) {
  return ApiService.fetchData<{
    data: {
      summary: ManifestApplySummary
      namespace?: string
      yaml?: string
      resources?: ManifestRef[]
      application?: K8sApplication
    }
    message?: string
  }>({
    url: `${K8S_APPLICATIONS_BASE}/apply`,
    method: "post",
    data: body,
  })
}

export async function applySavedApplication(
  id: string,
  body?: { yaml?: string; namespace?: string; dry_run?: boolean },
) {
  return ApiService.fetchData<{
    data: {
      summary: ManifestApplySummary
      namespace?: string
      yaml?: string
      resources?: ManifestRef[]
      application?: K8sApplication
    }
    message?: string
  }>({
    url: `${K8S_APPLICATIONS_SINGLE}/${encodeURIComponent(id)}/apply`,
    method: "post",
    data: body ?? {},
  })
}

export async function removeApplicationFromCluster(
  id: string,
  body?: { also_delete_store?: boolean },
) {
  return ApiService.fetchData<{
    data: {
      summary: ManifestApplySummary
      application?: K8sApplicationRow
      deleted?: boolean
    }
    message?: string
  }>({
    url: `${K8S_APPLICATIONS_SINGLE}/${encodeURIComponent(id)}/remove`,
    method: "post",
    data: body ?? {},
  })
}

export async function duplicateApplication(
  id: string,
  body?: { name?: string },
) {
  return ApiService.fetchData<{ data: K8sApplication; message?: string }>({
    url: `${K8S_APPLICATIONS_SINGLE}/${encodeURIComponent(id)}/duplicate`,
    method: "post",
    data: body ?? {},
  })
}

export async function listApplicationRevisions(id: string) {
  return ApiService.fetchData<{
    data: {
      application: K8sApplicationRow
      revisions: K8sApplicationRevisionRow[]
    }
  }>({
    url: `${K8S_APPLICATIONS_SINGLE}/${encodeURIComponent(id)}/revisions`,
    method: "get",
  })
}

export async function getApplicationRevision(id: string, version: number) {
  return ApiService.fetchData<{ data: K8sApplicationRevision }>({
    url: `${K8S_APPLICATIONS_SINGLE}/${encodeURIComponent(id)}/revisions/${version}`,
    method: "get",
  })
}

export async function restoreApplicationRevision(id: string, version: number) {
  return ApiService.fetchData<{ data: K8sApplication; message?: string }>({
    url: `${K8S_APPLICATIONS_SINGLE}/${encodeURIComponent(id)}/revisions/${version}/restore`,
    method: "post",
    data: {},
  })
}

export function formatAge(iso: string) {
  if (!iso) return "—"
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  const sec = Math.max(0, Math.floor((Date.now() - d.getTime()) / 1000))
  if (sec < 60) return `${sec}s`
  if (sec < 3600) return `${Math.floor(sec / 60)}m`
  if (sec < 86400) return `${Math.floor(sec / 3600)}h`
  return `${Math.floor(sec / 86400)}d`
}
