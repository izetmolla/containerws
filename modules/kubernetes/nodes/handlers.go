package nodes

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/modules/kubernetes/kubecli"
	"github.com/izetmolla/containerws/packages/machine"
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
	list := api.Group("/list")
	list.Get("/", cc.ListAPI)

	single := api.Group("/single")
	single.Get("/:name", cc.GetAPI)
	single.Get("/:name/pods", cc.PodsAPI)
	single.Post("/:name/cordon", cc.CordonAPI)
	single.Post("/:name/uncordon", cc.UncordonAPI)

	api.Get("/host-metrics", cc.HostMetricsAPI)
}

func (cc *controller) respondErr(c fiber.Ctx, err error) error {
	r := cc.app.Render()
	return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("KUBERNETES_ERROR"))
}

type conditionRow struct {
	Type               string `json:"type"`
	Status             string `json:"status"`
	Reason             string `json:"reason,omitempty"`
	Message            string `json:"message,omitempty"`
	LastHeartbeatTime  string `json:"last_heartbeat,omitempty"`
	LastTransitionTime string `json:"last_transition,omitempty"`
}

type addressRow struct {
	Type    string `json:"type"`
	Address string `json:"address"`
}

type taintRow struct {
	Key    string `json:"key"`
	Value  string `json:"value,omitempty"`
	Effect string `json:"effect"`
}

type nodeRow struct {
	Name             string         `json:"name"`
	Status           string         `json:"status"`
	Roles            string         `json:"roles"`
	Version          string         `json:"version"`
	OSImage          string         `json:"os_image"`
	Kernel           string         `json:"kernel"`
	Architecture     string         `json:"architecture,omitempty"`
	ContainerRuntime string         `json:"container_runtime"`
	CPU              string         `json:"cpu"`
	CPUAllocatable   string         `json:"cpu_allocatable,omitempty"`
	Memory           string         `json:"memory"`
	MemoryAllocatable string        `json:"memory_allocatable,omitempty"`
	PodsCapacity     string         `json:"pods_capacity,omitempty"`
	PodCount         int            `json:"pod_count"`
	Unschedulable    bool           `json:"unschedulable"`
	InternalIP       string         `json:"internal_ip,omitempty"`
	ExternalIP       string         `json:"external_ip,omitempty"`
	Hostname         string         `json:"hostname,omitempty"`
	CreatedAt        string         `json:"created_at"`
	Conditions       []conditionRow `json:"conditions,omitempty"`
	Addresses        []addressRow   `json:"addresses,omitempty"`
	Taints           []taintRow     `json:"taints,omitempty"`
	Labels           map[string]string `json:"labels,omitempty"`
}

type podOnNode struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Ready     string `json:"ready"`
	Restarts  int32  `json:"restarts"`
	IP        string `json:"ip,omitempty"`
	CreatedAt string `json:"created_at"`
}

func (cc *controller) ListAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()
	list, err := cli.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}

	podCountByNode := map[string]int{}
	if pods, err := cli.CoreV1().Pods("").List(ctx, metav1.ListOptions{Limit: 10000}); err == nil {
		for _, p := range pods.Items {
			if p.Spec.NodeName == "" {
				continue
			}
			podCountByNode[p.Spec.NodeName]++
		}
	}

	rows := make([]nodeRow, 0, len(list.Items))
	for i := range list.Items {
		rows = append(rows, buildNodeRow(&list.Items[i], podCountByNode[list.Items[i].Name], false))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": rows}))
}

func (cc *controller) GetAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	name := strings.TrimSpace(c.Params("name"))
	if name == "" {
		return cc.respondErr(c, fmt.Errorf("name is required"))
	}
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()
	n, err := cli.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	podCount := 0
	if pods, err := cli.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + name,
		Limit:         5000,
	}); err == nil {
		podCount = len(pods.Items)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": buildNodeRow(n, podCount, true),
	}))
}

func (cc *controller) PodsAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	name := strings.TrimSpace(c.Params("name"))
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()
	list, err := cli.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + name,
		Limit:         5000,
	})
	if err != nil {
		return cc.respondErr(c, err)
	}
	rows := make([]podOnNode, 0, len(list.Items))
	for _, p := range list.Items {
		var ready, total, restarts int32
		for _, cs := range p.Status.ContainerStatuses {
			total++
			if cs.Ready {
				ready++
			}
			restarts += cs.RestartCount
		}
		rows = append(rows, podOnNode{
			Namespace: p.Namespace,
			Name:      p.Name,
			Status:    string(p.Status.Phase),
			Ready:     fmt.Sprintf("%d/%d", ready, total),
			Restarts:  restarts,
			IP:        p.Status.PodIP,
			CreatedAt: p.CreationTimestamp.UTC().Format(time.RFC3339),
		})
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": rows}))
}

func (cc *controller) CordonAPI(c fiber.Ctx) error {
	return cc.setUnschedulable(c, true)
}

func (cc *controller) UncordonAPI(c fiber.Ctx) error {
	return cc.setUnschedulable(c, false)
}

func (cc *controller) setUnschedulable(c fiber.Ctx, unschedulable bool) error {
	r := cc.app.Render()
	name := strings.TrimSpace(c.Params("name"))
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()
	n, err := cli.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	n.Spec.Unschedulable = unschedulable
	updated, err := cli.CoreV1().Nodes().Update(ctx, n, metav1.UpdateOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	msg := "Node uncordoned"
	if unschedulable {
		msg = "Node cordoned"
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"name":          updated.Name,
			"unschedulable": updated.Spec.Unschedulable,
		},
		"message": msg,
	}))
}

// HostMetricsAPI returns live metrics for the machine running this app
// (typically the node host in single-node / workspace setups).
func (cc *controller) HostMetricsAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	m := machine.CollectMetrics()
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"ok":      true,
			"metrics": m,
			"note":    "Live metrics for this Container Workspace host (local node).",
		},
	}))
}

func buildNodeRow(n *corev1.Node, podCount int, detailed bool) nodeRow {
	roles := "worker"
	if _, ok := n.Labels["node-role.kubernetes.io/control-plane"]; ok {
		roles = "control-plane"
	} else if _, ok := n.Labels["node-role.kubernetes.io/master"]; ok {
		roles = "control-plane"
	}
	status := "NotReady"
	if nodeReady(n) {
		status = "Ready"
	}

	row := nodeRow{
		Name:              n.Name,
		Status:            status,
		Roles:             roles,
		Version:           n.Status.NodeInfo.KubeletVersion,
		OSImage:           n.Status.NodeInfo.OSImage,
		Kernel:            n.Status.NodeInfo.KernelVersion,
		Architecture:      n.Status.NodeInfo.Architecture,
		ContainerRuntime:  n.Status.NodeInfo.ContainerRuntimeVersion,
		CPU:               n.Status.Capacity.Cpu().String(),
		CPUAllocatable:    n.Status.Allocatable.Cpu().String(),
		Memory:            n.Status.Capacity.Memory().String(),
		MemoryAllocatable: n.Status.Allocatable.Memory().String(),
		PodsCapacity:      n.Status.Capacity.Pods().String(),
		PodCount:          podCount,
		Unschedulable:     n.Spec.Unschedulable,
		CreatedAt:         n.CreationTimestamp.UTC().Format(time.RFC3339),
	}

	for _, a := range n.Status.Addresses {
		switch a.Type {
		case corev1.NodeInternalIP:
			row.InternalIP = a.Address
		case corev1.NodeExternalIP:
			row.ExternalIP = a.Address
		case corev1.NodeHostName:
			row.Hostname = a.Address
		}
		if detailed {
			row.Addresses = append(row.Addresses, addressRow{Type: string(a.Type), Address: a.Address})
		}
	}

	if detailed {
		row.Labels = n.Labels
		for _, cond := range n.Status.Conditions {
			row.Conditions = append(row.Conditions, conditionRow{
				Type:               string(cond.Type),
				Status:             string(cond.Status),
				Reason:             cond.Reason,
				Message:            cond.Message,
				LastHeartbeatTime:  cond.LastHeartbeatTime.UTC().Format(time.RFC3339),
				LastTransitionTime: cond.LastTransitionTime.UTC().Format(time.RFC3339),
			})
		}
		for _, t := range n.Spec.Taints {
			row.Taints = append(row.Taints, taintRow{
				Key:    t.Key,
				Value:  t.Value,
				Effect: string(t.Effect),
			})
		}
	}
	return row
}

func nodeReady(n *corev1.Node) bool {
	for _, c := range n.Status.Conditions {
		if c.Type == corev1.NodeReady && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}
