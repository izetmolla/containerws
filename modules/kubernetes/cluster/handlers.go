package cluster

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/modules/kubernetes/kubecli"
	"github.com/izetmolla/containerws/packages/kubeclient"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type controller struct {
	app *config.AppClients
}

func NewController(app *config.AppClients) *controller {
	return &controller{app: app}
}

func SetupRoutesAPI(api fiber.Router, appClients *config.AppClients) {
	cc := NewController(appClients)
	api.Get("/status", cc.StatusAPI)
	api.Get("/nodes", cc.NodesAPI)
	api.Get("/events", cc.EventsAPI)
}

func (cc *controller) respondErr(c fiber.Ctx, err error) error {
	r := cc.app.Render()
	return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("KUBERNETES_ERROR"))
}

// StatusAPI cluster overview — analogous to Docker engine/status.
func (cc *controller) StatusAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	s, err := kubecli.LoadSettings(cc.app)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	out := fiber.Map{
		"reachable": false,
		"path":      s.Path,
		"context":   s.Context,
		"exists":    s.Exists,
	}
	if !s.Exists {
		out["error"] = "kubeconfig file not found"
		return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": out}))
	}
	cli, err := kubeclient.Client(kubeclient.Config{Path: s.Path, Context: s.Context})
	if err != nil {
		out["error"] = err.Error()
		return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": out}))
	}
	ctx, cancel := context.WithTimeout(c.Context(), 8*time.Second)
	defer cancel()

	ver, err := cli.Discovery().ServerVersion()
	if err != nil {
		kubeclient.Reset()
		out["error"] = err.Error()
		return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": out}))
	}
	out["reachable"] = true
	out["version"] = ver.GitVersion
	out["platform"] = ver.Platform

	if nodes, err := cli.CoreV1().Nodes().List(ctx, metav1.ListOptions{}); err == nil {
		ready := 0
		for _, n := range nodes.Items {
			if nodeReady(&n) {
				ready++
			}
		}
		out["nodes"] = len(nodes.Items)
		out["nodes_ready"] = ready
	}
	if ns, err := cli.CoreV1().Namespaces().List(ctx, metav1.ListOptions{}); err == nil {
		out["namespaces"] = len(ns.Items)
	}
	if pods, err := cli.CoreV1().Pods("").List(ctx, metav1.ListOptions{Limit: 5000}); err == nil {
		running := 0
		for _, p := range pods.Items {
			if p.Status.Phase == corev1.PodRunning {
				running++
			}
		}
		out["pods"] = len(pods.Items)
		out["pods_running"] = running
	}
	if deps, err := cli.AppsV1().Deployments("").List(ctx, metav1.ListOptions{Limit: 5000}); err == nil {
		out["deployments"] = len(deps.Items)
	}
	if svcs, err := cli.CoreV1().Services("").List(ctx, metav1.ListOptions{Limit: 5000}); err == nil {
		out["services"] = len(svcs.Items)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": out}))
}

type nodeRow struct {
	Name             string `json:"name"`
	Status           string `json:"status"`
	Roles            string `json:"roles"`
	Version          string `json:"version"`
	OSImage          string `json:"os_image"`
	Kernel           string `json:"kernel"`
	ContainerRuntime string `json:"container_runtime"`
	CPU              string `json:"cpu"`
	Memory           string `json:"memory"`
	CreatedAt        string `json:"created_at"`
}

func (cc *controller) NodesAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
	defer cancel()
	list, err := cli.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	rows := make([]nodeRow, 0, len(list.Items))
	for _, n := range list.Items {
		roles := "worker"
		if _, ok := n.Labels["node-role.kubernetes.io/control-plane"]; ok {
			roles = "control-plane"
		} else if _, ok := n.Labels["node-role.kubernetes.io/master"]; ok {
			roles = "control-plane"
		}
		status := "NotReady"
		if nodeReady(&n) {
			status = "Ready"
		}
		rows = append(rows, nodeRow{
			Name:             n.Name,
			Status:           status,
			Roles:            roles,
			Version:          n.Status.NodeInfo.KubeletVersion,
			OSImage:          n.Status.NodeInfo.OSImage,
			Kernel:           n.Status.NodeInfo.KernelVersion,
			ContainerRuntime: n.Status.NodeInfo.ContainerRuntimeVersion,
			CPU:              n.Status.Capacity.Cpu().String(),
			Memory:           n.Status.Capacity.Memory().String(),
			CreatedAt:        n.CreationTimestamp.UTC().Format(time.RFC3339),
		})
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": rows}))
}

type eventRow struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Reason    string `json:"reason"`
	Message   string `json:"message"`
	Object    string `json:"object"`
	Count     int32  `json:"count"`
	LastSeen  string `json:"last_seen"`
}

func (cc *controller) EventsAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ns := kubecli.NamespaceQuery(c)
	kind := strings.TrimSpace(c.Query("kind"))
	objName := strings.TrimSpace(c.Query("name"))
	fieldSelector := ""
	if kind != "" && objName != "" {
		fieldSelector = fmt.Sprintf("involvedObject.kind=%s,involvedObject.name=%s", kind, objName)
	} else if kind != "" {
		fieldSelector = fmt.Sprintf("involvedObject.kind=%s", kind)
	} else if objName != "" {
		fieldSelector = fmt.Sprintf("involvedObject.name=%s", objName)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
	defer cancel()
	list, err := cli.CoreV1().Events(ns).List(ctx, metav1.ListOptions{
		Limit:         500,
		FieldSelector: fieldSelector,
	})
	if err != nil {
		return cc.respondErr(c, err)
	}
	rows := make([]eventRow, 0, len(list.Items))
	for _, e := range list.Items {
		last := e.LastTimestamp.Time
		if last.IsZero() && !e.EventTime.IsZero() {
			last = e.EventTime.Time
		}
		if last.IsZero() {
			last = e.CreationTimestamp.Time
		}
		rows = append(rows, eventRow{
			Namespace: e.Namespace,
			Name:      e.Name,
			Type:      e.Type,
			Reason:    e.Reason,
			Message:   e.Message,
			Object:    e.InvolvedObject.Kind + "/" + e.InvolvedObject.Name,
			Count:     e.Count,
			LastSeen:  last.UTC().Format(time.RFC3339),
		})
	}
	// Newest first
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].LastSeen > rows[j].LastSeen
	})
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": rows}))
}

func nodeReady(n *corev1.Node) bool {
	for _, c := range n.Status.Conditions {
		if c.Type == corev1.NodeReady && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}
