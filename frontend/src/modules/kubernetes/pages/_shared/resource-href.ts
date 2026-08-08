/** Deep-link into UI pages for common Kubernetes kinds. */
export function k8sResourceHref(input: {
  kind?: string
  name?: string
  namespace?: string
}): string | null {
  const kind = (input.kind || "").trim()
  const name = (input.name || "").trim()
  if (!kind || !name) return null

  const ns = (input.namespace || "").trim()
  const enc = encodeURIComponent

  switch (kind) {
    case "Namespace":
      return `/kubernetes/namespaces/${enc(name)}`
    case "Node":
      return `/kubernetes/nodes/${enc(name)}`
    case "Pod":
      return ns ? `/kubernetes/pods/${enc(ns)}/${enc(name)}` : null
    case "Deployment":
      return ns ? `/kubernetes/deployments/${enc(ns)}/${enc(name)}` : null
    case "StatefulSet":
      return ns ? `/kubernetes/statefulsets/${enc(ns)}/${enc(name)}` : null
    case "DaemonSet":
      return ns ? `/kubernetes/daemonsets/${enc(ns)}/${enc(name)}` : null
    case "Job":
      return ns ? `/kubernetes/jobs/${enc(ns)}/${enc(name)}` : null
    case "CronJob":
      return ns ? `/kubernetes/jobs/cron/${enc(ns)}/${enc(name)}` : null
    case "Service":
      return ns ? `/kubernetes/services/${enc(ns)}/${enc(name)}` : null
    case "Ingress":
      return ns ? `/kubernetes/ingresses/${enc(ns)}/${enc(name)}` : null
    case "NetworkPolicy":
      return ns
        ? `/kubernetes/network-policies/${enc(ns)}/${enc(name)}`
        : null
    case "ConfigMap":
      return ns ? `/kubernetes/configmaps/${enc(ns)}/${enc(name)}` : null
    case "Secret":
      return ns ? `/kubernetes/secrets/${enc(ns)}/${enc(name)}` : null
    case "PersistentVolumeClaim":
      return ns
        ? `/kubernetes/persistentvolumeclaims/${enc(ns)}/${enc(name)}`
        : null
    case "StorageClass":
      return `/kubernetes/storageclasses`
    default:
      return null
  }
}
