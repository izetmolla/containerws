package k8s

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

type PodsListInput struct {
	ClusterRef
	FieldSelector string `json:"fieldSelector,omitempty"`
	LabelSelector string `json:"labelSelector,omitempty"`
}

type PodItem struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Ready     string `json:"ready"`
	Restarts  int32  `json:"restarts"`
	Node      string `json:"node,omitempty"`
	IP        string `json:"ip,omitempty"`
	CreatedAt string `json:"created_at"`
}

type PodsListOutput struct {
	KubeconfigID string    `json:"kubeconfig_id"`
	Context      string    `json:"context"`
	Count        int       `json:"count"`
	Items        []PodItem `json:"items"`
}

func podItem(p *corev1.Pod) PodItem {
	var ready, total, restarts int32
	for _, cs := range p.Status.ContainerStatuses {
		total++
		if cs.Ready {
			ready++
		}
		restarts += cs.RestartCount
	}
	if total == 0 {
		total = int32(len(p.Spec.Containers))
	}
	return PodItem{
		Namespace: p.Namespace,
		Name:      p.Name,
		Status:    string(p.Status.Phase),
		Ready:     fmt.Sprintf("%d/%d", ready, total),
		Restarts:  restarts,
		Node:      p.Spec.NodeName,
		IP:        p.Status.PodIP,
		CreatedAt: p.CreationTimestamp.UTC().Format(time.RFC3339),
	}
}

func (c *Controller) PodsListTool(ctx context.Context, _ *mcp.CallToolRequest, input PodsListInput) (*mcp.CallToolResult, any, error) {
	resolved, err := c.resolve(input.ClusterRef)
	if err != nil {
		return nil, nil, err
	}
	runCtx, cancel := toolCtx(ctx)
	defer cancel()
	list, err := resolved.Client.CoreV1().Pods("").List(runCtx, metav1.ListOptions{
		FieldSelector: strings.TrimSpace(input.FieldSelector),
		LabelSelector: strings.TrimSpace(input.LabelSelector),
		Limit:         5000,
	})
	if err != nil {
		return nil, nil, err
	}
	items := make([]PodItem, 0, len(list.Items))
	for i := range list.Items {
		items = append(items, podItem(&list.Items[i]))
	}
	return nil, PodsListOutput{
		KubeconfigID: resolved.KubeconfigID,
		Context:      resolved.Context,
		Count:        len(items),
		Items:        items,
	}, nil
}

type PodsListInNamespaceInput struct {
	ClusterRef
	Namespace     string `json:"namespace" jsonschema:"required namespace to list pods from"`
	FieldSelector string `json:"fieldSelector,omitempty"`
	LabelSelector string `json:"labelSelector,omitempty"`
}

func (c *Controller) PodsListInNamespaceTool(ctx context.Context, _ *mcp.CallToolRequest, input PodsListInNamespaceInput) (*mcp.CallToolResult, any, error) {
	ns := strings.TrimSpace(input.Namespace)
	if ns == "" {
		return nil, nil, fmt.Errorf("namespace is required")
	}
	resolved, err := c.resolve(input.ClusterRef)
	if err != nil {
		return nil, nil, err
	}
	runCtx, cancel := toolCtx(ctx)
	defer cancel()
	list, err := resolved.Client.CoreV1().Pods(ns).List(runCtx, metav1.ListOptions{
		FieldSelector: strings.TrimSpace(input.FieldSelector),
		LabelSelector: strings.TrimSpace(input.LabelSelector),
	})
	if err != nil {
		return nil, nil, err
	}
	items := make([]PodItem, 0, len(list.Items))
	for i := range list.Items {
		items = append(items, podItem(&list.Items[i]))
	}
	return nil, PodsListOutput{
		KubeconfigID: resolved.KubeconfigID,
		Context:      resolved.Context,
		Count:        len(items),
		Items:        items,
	}, nil
}

type PodsGetInput struct {
	ClusterRef
	Name      string `json:"name" jsonschema:"required"`
	Namespace string `json:"namespace,omitempty"`
}

type PodsGetOutput struct {
	KubeconfigID string  `json:"kubeconfig_id"`
	Context      string  `json:"context"`
	Pod          PodItem `json:"pod"`
}

func (c *Controller) PodsGetTool(ctx context.Context, _ *mcp.CallToolRequest, input PodsGetInput) (*mcp.CallToolResult, any, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, nil, fmt.Errorf("name is required")
	}
	resolved, err := c.resolve(input.ClusterRef)
	if err != nil {
		return nil, nil, err
	}
	ns := defaultNS(resolved, input.Namespace)
	runCtx, cancel := toolCtx(ctx)
	defer cancel()
	pod, err := resolved.Client.CoreV1().Pods(ns).Get(runCtx, name, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}
	return nil, PodsGetOutput{
		KubeconfigID: resolved.KubeconfigID,
		Context:      resolved.Context,
		Pod:          podItem(pod),
	}, nil
}

type PodsDeleteInput struct {
	ClusterRef
	Name      string `json:"name" jsonschema:"required"`
	Namespace string `json:"namespace,omitempty"`
}

type PodsDeleteOutput struct {
	KubeconfigID string `json:"kubeconfig_id"`
	Context      string `json:"context"`
	Deleted      string `json:"deleted"`
}

func (c *Controller) PodsDeleteTool(ctx context.Context, _ *mcp.CallToolRequest, input PodsDeleteInput) (*mcp.CallToolResult, any, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, nil, fmt.Errorf("name is required")
	}
	resolved, err := c.resolve(input.ClusterRef)
	if err != nil {
		return nil, nil, err
	}
	ns := defaultNS(resolved, input.Namespace)
	runCtx, cancel := toolCtx(ctx)
	defer cancel()
	if err := resolved.Client.CoreV1().Pods(ns).Delete(runCtx, name, metav1.DeleteOptions{}); err != nil {
		return nil, nil, err
	}
	return nil, PodsDeleteOutput{
		KubeconfigID: resolved.KubeconfigID,
		Context:      resolved.Context,
		Deleted:      ns + "/" + name,
	}, nil
}

type PodsLogInput struct {
	ClusterRef
	Name      string `json:"name" jsonschema:"required"`
	Namespace string `json:"namespace,omitempty"`
	Container string `json:"container,omitempty"`
	Previous  bool   `json:"previous,omitempty"`
	Tail      *int64 `json:"tail,omitempty" jsonschema:"lines from end (default 100)"`
}

type PodsLogOutput struct {
	KubeconfigID string `json:"kubeconfig_id"`
	Context      string `json:"context"`
	Namespace    string `json:"namespace"`
	Name         string `json:"name"`
	Container    string `json:"container,omitempty"`
	Logs         string `json:"logs"`
}

func (c *Controller) PodsLogTool(ctx context.Context, _ *mcp.CallToolRequest, input PodsLogInput) (*mcp.CallToolResult, any, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, nil, fmt.Errorf("name is required")
	}
	resolved, err := c.resolve(input.ClusterRef)
	if err != nil {
		return nil, nil, err
	}
	ns := defaultNS(resolved, input.Namespace)
	tail := int64(100)
	if input.Tail != nil {
		tail = *input.Tail
	}
	opts := &corev1.PodLogOptions{
		Container: strings.TrimSpace(input.Container),
		Previous:  input.Previous,
		TailLines: &tail,
	}
	runCtx, cancel := toolCtx(ctx)
	defer cancel()
	raw, err := resolved.Client.CoreV1().Pods(ns).GetLogs(name, opts).Do(runCtx).Raw()
	if err != nil {
		return nil, nil, err
	}
	return nil, PodsLogOutput{
		KubeconfigID: resolved.KubeconfigID,
		Context:      resolved.Context,
		Namespace:    ns,
		Name:         name,
		Container:    opts.Container,
		Logs:         string(raw),
	}, nil
}

type PodsExecInput struct {
	ClusterRef
	Name      string   `json:"name" jsonschema:"required"`
	Namespace string   `json:"namespace,omitempty"`
	Container string   `json:"container,omitempty"`
	Command   []string `json:"command" jsonschema:"required command argv e.g. [\"ls\",\"-la\"]"`
}

type PodsExecOutput struct {
	KubeconfigID string `json:"kubeconfig_id"`
	Context      string `json:"context"`
	Namespace    string `json:"namespace"`
	Name         string `json:"name"`
	Stdout       string `json:"stdout"`
	Stderr       string `json:"stderr"`
}

func (c *Controller) PodsExecTool(ctx context.Context, _ *mcp.CallToolRequest, input PodsExecInput) (*mcp.CallToolResult, any, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, nil, fmt.Errorf("name is required")
	}
	if len(input.Command) == 0 {
		return nil, nil, fmt.Errorf("command is required")
	}
	resolved, err := c.resolve(input.ClusterRef)
	if err != nil {
		return nil, nil, err
	}
	ns := defaultNS(resolved, input.Namespace)
	req := resolved.Client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(name).
		Namespace(ns).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: strings.TrimSpace(input.Container),
			Command:   input.Command,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(resolved.REST, "POST", req.URL())
	if err != nil {
		return nil, nil, err
	}
	var stdout, stderr bytes.Buffer
	runCtx, cancel := toolCtx(ctx)
	defer cancel()
	streamErr := exec.StreamWithContext(runCtx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	out := PodsExecOutput{
		KubeconfigID: resolved.KubeconfigID,
		Context:      resolved.Context,
		Namespace:    ns,
		Name:         name,
		Stdout:       stdout.String(),
		Stderr:       stderr.String(),
	}
	if streamErr != nil {
		if out.Stderr != "" {
			out.Stderr += "\n"
		}
		out.Stderr += streamErr.Error()
	}
	return nil, out, nil
}

type PodsRunInput struct {
	ClusterRef
	Image     string `json:"image" jsonschema:"required container image"`
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Port      *int32 `json:"port,omitempty" jsonschema:"optional container port to expose via a Service"`
}

type PodsRunOutput struct {
	KubeconfigID string `json:"kubeconfig_id"`
	Context      string `json:"context"`
	Namespace    string `json:"namespace"`
	PodName      string `json:"pod_name"`
	ServiceName  string `json:"service_name,omitempty"`
}

func (c *Controller) PodsRunTool(ctx context.Context, _ *mcp.CallToolRequest, input PodsRunInput) (*mcp.CallToolResult, any, error) {
	image := strings.TrimSpace(input.Image)
	if image == "" {
		return nil, nil, fmt.Errorf("image is required")
	}
	resolved, err := c.resolve(input.ClusterRef)
	if err != nil {
		return nil, nil, err
	}
	ns := defaultNS(resolved, input.Namespace)
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = "run-" + strings.ReplaceAll(strings.ToLower(image), "/", "-")
		if len(name) > 50 {
			name = name[:50]
		}
		name = strings.Trim(name, "-")
	}
	labels := map[string]string{"app": name, "created-by": "containerws-mcp"}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns, Labels: labels},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{{
				Name:  "main",
				Image: image,
			}},
		},
	}
	if input.Port != nil && *input.Port > 0 {
		pod.Spec.Containers[0].Ports = []corev1.ContainerPort{{ContainerPort: *input.Port}}
	}
	runCtx, cancel := toolCtx(ctx)
	defer cancel()
	created, err := resolved.Client.CoreV1().Pods(ns).Create(runCtx, pod, metav1.CreateOptions{})
	if err != nil {
		return nil, nil, err
	}
	out := PodsRunOutput{
		KubeconfigID: resolved.KubeconfigID,
		Context:      resolved.Context,
		Namespace:    ns,
		PodName:      created.Name,
	}
	if input.Port != nil && *input.Port > 0 {
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns, Labels: labels},
			Spec: corev1.ServiceSpec{
				Selector: labels,
				Ports: []corev1.ServicePort{{
					Port:       *input.Port,
					TargetPort: intstr.FromInt32(*input.Port),
				}},
			},
		}
		if s, err := resolved.Client.CoreV1().Services(ns).Create(runCtx, svc, metav1.CreateOptions{}); err == nil {
			out.ServiceName = s.Name
		}
	}
	return nil, out, nil
}

type PodsTopInput struct {
	ClusterRef
	Name          string `json:"name,omitempty"`
	Namespace     string `json:"namespace,omitempty"`
	AllNamespaces bool   `json:"all_namespaces,omitempty"`
	LabelSelector string `json:"label_selector,omitempty"`
}

type PodsTopOutput struct {
	KubeconfigID string `json:"kubeconfig_id"`
	Context      string `json:"context"`
	MetricsJSON  string `json:"metrics_json"`
}

func (c *Controller) PodsTopTool(ctx context.Context, _ *mcp.CallToolRequest, input PodsTopInput) (*mcp.CallToolResult, any, error) {
	resolved, err := c.resolve(input.ClusterRef)
	if err != nil {
		return nil, nil, err
	}
	runCtx, cancel := toolCtx(ctx)
	defer cancel()

	ns := ""
	name := strings.TrimSpace(input.Name)
	if name != "" {
		ns = defaultNS(resolved, input.Namespace)
	} else if !input.AllNamespaces {
		ns = strings.TrimSpace(input.Namespace)
	}

	path := "/apis/metrics.k8s.io/v1beta1/pods"
	switch {
	case name != "":
		path = fmt.Sprintf("/apis/metrics.k8s.io/v1beta1/namespaces/%s/pods/%s", ns, name)
	case ns != "":
		path = fmt.Sprintf("/apis/metrics.k8s.io/v1beta1/namespaces/%s/pods", ns)
	}

	req := resolved.Client.CoreV1().RESTClient().Get().AbsPath(path)
	if sel := strings.TrimSpace(input.LabelSelector); sel != "" {
		req = req.Param("labelSelector", sel)
	}
	raw, err := req.Do(runCtx).Raw()
	if err != nil {
		return nil, nil, fmt.Errorf("metrics.k8s.io unavailable (is Metrics Server installed?): %w", err)
	}
	return nil, PodsTopOutput{
		KubeconfigID: resolved.KubeconfigID,
		Context:      resolved.Context,
		MetricsJSON:  string(raw),
	}, nil
}
