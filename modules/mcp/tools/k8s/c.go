package k8s

import (
	"github.com/izetmolla/containerws/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Controller struct {
	app *config.AppClients
}

func NewController(app *config.AppClients) *Controller {
	return &Controller{app: app}
}

// LoadTools registers Kubernetes MCP tools inspired by
// https://github.com/containers/kubernetes-mcp-server (config + core + helm).
// Multi-cluster: pass kubeconfig_id (secret) and/or context (cluster) on tools.
func LoadTools(server *mcp.Server, app *config.AppClients) {
	c := NewController(app)

	// --- config toolset ---
	mcp.AddTool(server, &mcp.Tool{
		Name: "configuration_contexts_list",
		Description: "List kubeconfig secrets and every context (cluster) they contain. " +
			"Use this first when multiple clusters exist to learn which secret maps to which cluster. " +
			"Returns kubeconfig_id, secret name, context name, cluster, user, and whether each is active.",
	}, c.ConfigurationContextsListTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "targets_list",
		Description: "List all available Kubernetes targets (kubeconfig secret + context/cluster pairs). " +
			"Same data as configuration_contexts_list, shaped for picking a target. " +
			"Ask the user which secret/cluster to use when more than one target exists.",
	}, c.TargetsListTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "configuration_view",
		Description: "Get the current (or selected) kubeconfig content as YAML. " +
			"By default returns a minified kubeconfig for the active context only. " +
			"Sensitive: contains credentials — do not echo secrets unless the user asked.",
	}, c.ConfigurationViewTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "configuration_set_active",
		Description: "Set which kubeconfig secret and context (cluster) is the workspace default. " +
			"Ask the user which secret and cluster to activate when multiple exist. " +
			"Subsequent tools that omit kubeconfig_id/context use this active pair.",
	}, c.ConfigurationSetActiveTool)

	// --- core: namespaces / events / projects ---
	mcp.AddTool(server, &mcp.Tool{
		Name:        "namespaces_list",
		Description: "List all Kubernetes namespaces in the selected cluster.",
	}, c.NamespacesListTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "projects_list",
		Description: "List OpenShift projects (falls back to namespaces when Project API is unavailable).",
	}, c.ProjectsListTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "events_list",
		Description: "List Kubernetes events (warnings, errors, state changes) for debugging.",
	}, c.EventsListTool)

	// --- core: pods ---
	mcp.AddTool(server, &mcp.Tool{
		Name:        "pods_list",
		Description: "List pods in all namespaces (optional field/label selectors).",
	}, c.PodsListTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "pods_list_in_namespace",
		Description: "List pods in a specific namespace.",
	}, c.PodsListInNamespaceTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "pods_get",
		Description: "Get a Kubernetes Pod by name (and optional namespace).",
	}, c.PodsGetTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "pods_delete",
		Description: "Delete a Kubernetes Pod by name.",
	}, c.PodsDeleteTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "pods_log",
		Description: "Get logs from a Kubernetes Pod (optional container, previous, tail).",
	}, c.PodsLogTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "pods_exec",
		Description: "Execute a command in a Kubernetes Pod container (non-interactive).",
	}, c.PodsExecTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "pods_run",
		Description: "Run a container image as a Pod (optionally expose a port).",
	}, c.PodsRunTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "pods_top",
		Description: "Pod CPU/memory usage from the Metrics Server (requires metrics.k8s.io).",
	}, c.PodsTopTool)

	// --- core: nodes ---
	mcp.AddTool(server, &mcp.Tool{
		Name:        "nodes_log",
		Description: "Get node logs via the kubelet proxy (e.g. query=kubelet).",
	}, c.NodesLogTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "nodes_stats_summary",
		Description: "Node resource stats from the kubelet Summary API.",
	}, c.NodesStatsSummaryTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "nodes_top",
		Description: "Node CPU/memory usage from the Metrics Server.",
	}, c.NodesTopTool)

	// --- core: generic resources ---
	mcp.AddTool(server, &mcp.Tool{
		Name: "resources_list",
		Description: "List any Kubernetes/OpenShift resources by apiVersion and kind " +
			"(e.g. v1/Pod, apps/v1/Deployment, networking.k8s.io/v1/Ingress).",
	}, c.ResourcesListTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "resources_get",
		Description: "Get a Kubernetes resource by apiVersion, kind, name, and optional namespace.",
	}, c.ResourcesGetTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "resources_create_or_update",
		Description: "Create or update a resource via Server-Side Apply. " +
			"Pass the complete YAML/JSON desired state (not a partial patch).",
	}, c.ResourcesCreateOrUpdateTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "resources_delete",
		Description: "Delete a Kubernetes resource by apiVersion, kind, name, and optional namespace.",
	}, c.ResourcesDeleteTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "resources_scale",
		Description: "Get or update the scale of a Deployment/StatefulSet/etc.",
	}, c.ResourcesScaleTool)

	// --- helm toolset (requires helm CLI on host) ---
	mcp.AddTool(server, &mcp.Tool{
		Name:        "helm_install",
		Description: "Install a Helm chart (requires helm CLI). Pass chart ref and optional values.",
	}, c.HelmInstallTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "helm_list",
		Description: "List Helm releases (requires helm CLI).",
	}, c.HelmListTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "helm_uninstall",
		Description: "Uninstall a Helm release (requires helm CLI).",
	}, c.HelmUninstallTool)
}
