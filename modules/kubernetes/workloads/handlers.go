package workloads

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/contrib/v3/websocket"
	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/modules/kubernetes/kubecli"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
)

type controller struct {
	app *config.AppClients
}

func NewController(app *config.AppClients) *controller {
	return &controller{app: app}
}

func SetupRoutesAPI(api fiber.Router, appClients *config.AppClients) {
	cc := NewController(appClients)
	api.Get("/pods", cc.ListPodsAPI)
	api.Get("/pods/:namespace/:name", cc.GetPodAPI)
	api.Get("/pods/:namespace/:name/logs", cc.PodLogsAPI)
	api.Get("/pods/:namespace/:name/yaml", cc.PodYAMLAPI)
	api.Put("/pods/:namespace/:name/yaml", cc.ApplyPodYAMLAPI)
	api.Delete("/pods/:namespace/:name", cc.DeletePodAPI)

	api.Use("/pods/:namespace/:name/exec", func(c fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	api.Get("/pods/:namespace/:name/exec", websocket.New(cc.HandlePodExecWS, websocket.Config{
		RecoverHandler: func(conn *websocket.Conn) {
			if closer, ok := conn.Locals("exec_ws_close").(func()); ok && closer != nil {
				closer()
				return
			}
			_ = conn.Close()
		},
	}))

	api.Get("/deployments", cc.ListDeploymentsAPI)
	api.Post("/deployments", cc.CreateDeploymentAPI)
	api.Get("/deployments/:namespace/:name", cc.GetDeploymentAPI)
	api.Get("/deployments/:namespace/:name/pods", cc.DeploymentPodsAPI)
	api.Get("/deployments/:namespace/:name/yaml", cc.DeploymentYAMLAPI)
	api.Put("/deployments/:namespace/:name/yaml", cc.ApplyDeploymentYAMLAPI)
	api.Delete("/deployments/:namespace/:name", cc.DeleteDeploymentAPI)
	api.Post("/deployments/:namespace/:name/scale", cc.ScaleDeploymentAPI)
	api.Post("/deployments/:namespace/:name/restart", cc.RestartDeploymentAPI)
	api.Post("/deployments/:namespace/:name/pull-restart", cc.PullRestartDeploymentAPI)

	api.Get("/statefulsets", cc.ListStatefulSetsAPI)
	api.Post("/statefulsets", cc.CreateStatefulSetAPI)
	api.Get("/statefulsets/:namespace/:name", cc.GetStatefulSetAPI)
	api.Get("/statefulsets/:namespace/:name/pods", cc.StatefulSetPodsAPI)
	api.Get("/statefulsets/:namespace/:name/yaml", cc.StatefulSetYAMLAPI)
	api.Put("/statefulsets/:namespace/:name/yaml", cc.ApplyStatefulSetYAMLAPI)
	api.Delete("/statefulsets/:namespace/:name", cc.DeleteStatefulSetAPI)
	api.Post("/statefulsets/:namespace/:name/scale", cc.ScaleStatefulSetAPI)
	api.Post("/statefulsets/:namespace/:name/restart", cc.RestartStatefulSetAPI)

	api.Get("/daemonsets", cc.ListDaemonSetsAPI)
	api.Post("/daemonsets", cc.CreateDaemonSetAPI)
	api.Get("/daemonsets/:namespace/:name", cc.GetDaemonSetAPI)
	api.Get("/daemonsets/:namespace/:name/pods", cc.DaemonSetPodsAPI)
	api.Get("/daemonsets/:namespace/:name/yaml", cc.DaemonSetYAMLAPI)
	api.Put("/daemonsets/:namespace/:name/yaml", cc.ApplyDaemonSetYAMLAPI)
	api.Delete("/daemonsets/:namespace/:name", cc.DeleteDaemonSetAPI)
	api.Post("/daemonsets/:namespace/:name/restart", cc.RestartDaemonSetAPI)

	api.Get("/jobs", cc.ListJobsAPI)
	api.Post("/jobs", cc.CreateJobAPI)
	api.Get("/jobs/:namespace/:name", cc.GetJobAPI)
	api.Get("/jobs/:namespace/:name/yaml", cc.JobYAMLAPI)
	api.Put("/jobs/:namespace/:name/yaml", cc.ApplyJobYAMLAPI)
	api.Delete("/jobs/:namespace/:name", cc.DeleteJobAPI)

	api.Get("/cronjobs", cc.ListCronJobsAPI)
	api.Post("/cronjobs", cc.CreateCronJobAPI)
	api.Get("/cronjobs/:namespace/:name", cc.GetCronJobAPI)
	api.Get("/cronjobs/:namespace/:name/yaml", cc.CronJobYAMLAPI)
	api.Put("/cronjobs/:namespace/:name/yaml", cc.ApplyCronJobYAMLAPI)
	api.Post("/cronjobs/:namespace/:name/suspend", cc.SuspendCronJobAPI)
	api.Post("/cronjobs/:namespace/:name/resume", cc.ResumeCronJobAPI)
	api.Post("/cronjobs/:namespace/:name/trigger", cc.TriggerCronJobAPI)
	api.Delete("/cronjobs/:namespace/:name", cc.DeleteCronJobAPI)
}

func (cc *controller) respondErr(c fiber.Ctx, err error) error {
	r := cc.app.Render()
	return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("KUBERNETES_ERROR"))
}

type containerStatusRow struct {
	Name         string `json:"name"`
	Image        string `json:"image"`
	Ready        bool   `json:"ready"`
	RestartCount int32  `json:"restart_count"`
	State        string `json:"state"`
	StartedAt    string `json:"started_at,omitempty"`
}

type podDetail struct {
	Namespace   string               `json:"namespace"`
	Name        string               `json:"name"`
	Status      string               `json:"status"`
	Ready       string               `json:"ready"`
	Restarts    int32                `json:"restarts"`
	Node        string               `json:"node,omitempty"`
	IP          string               `json:"ip,omitempty"`
	HostIP      string               `json:"host_ip,omitempty"`
	QosClass    string               `json:"qos_class,omitempty"`
	CreatedAt   string               `json:"created_at"`
	Labels      map[string]string    `json:"labels,omitempty"`
	Annotations map[string]string    `json:"annotations,omitempty"`
	Containers  []containerStatusRow `json:"containers"`
	Conditions  []fiber.Map          `json:"conditions,omitempty"`
	Owner       string               `json:"owner,omitempty"`
}

type podRow struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Ready     string `json:"ready"`
	Restarts  int32  `json:"restarts"`
	Node      string `json:"node,omitempty"`
	Age       string `json:"created_at"`
	IP        string `json:"ip,omitempty"`
}

func containerState(cs corev1.ContainerStatus) (state, started string) {
	switch {
	case cs.State.Running != nil:
		state = "Running"
		if !cs.State.Running.StartedAt.IsZero() {
			started = cs.State.Running.StartedAt.UTC().Format(time.RFC3339)
		}
	case cs.State.Waiting != nil:
		state = "Waiting"
		if cs.State.Waiting.Reason != "" {
			state = "Waiting:" + cs.State.Waiting.Reason
		}
	case cs.State.Terminated != nil:
		state = "Terminated"
		if cs.State.Terminated.Reason != "" {
			state = "Terminated:" + cs.State.Terminated.Reason
		}
	default:
		state = "Unknown"
	}
	return state, started
}

func podToDetail(p *corev1.Pod) podDetail {
	var ready, total int32
	var restarts int32
	containers := make([]containerStatusRow, 0, len(p.Status.ContainerStatuses))
	for _, cs := range p.Status.ContainerStatuses {
		total++
		if cs.Ready {
			ready++
		}
		restarts += cs.RestartCount
		st, started := containerState(cs)
		containers = append(containers, containerStatusRow{
			Name:         cs.Name,
			Image:        cs.Image,
			Ready:        cs.Ready,
			RestartCount: cs.RestartCount,
			State:        st,
			StartedAt:    started,
		})
	}
	// Spec containers missing status (pending)
	if len(containers) == 0 {
		for _, ctr := range p.Spec.Containers {
			total++
			containers = append(containers, containerStatusRow{
				Name:  ctr.Name,
				Image: ctr.Image,
				State: "Pending",
			})
		}
	}
	conds := make([]fiber.Map, 0, len(p.Status.Conditions))
	for _, c := range p.Status.Conditions {
		conds = append(conds, fiber.Map{
			"type":    string(c.Type),
			"status":  string(c.Status),
			"reason":  c.Reason,
			"message": c.Message,
		})
	}
	owner := ""
	if len(p.OwnerReferences) > 0 {
		o := p.OwnerReferences[0]
		owner = o.Kind + "/" + o.Name
	}
	return podDetail{
		Namespace:   p.Namespace,
		Name:        p.Name,
		Status:      string(p.Status.Phase),
		Ready:       fmt.Sprintf("%d/%d", ready, total),
		Restarts:    restarts,
		Node:        p.Spec.NodeName,
		IP:          p.Status.PodIP,
		HostIP:      p.Status.HostIP,
		QosClass:    string(p.Status.QOSClass),
		CreatedAt:   p.CreationTimestamp.UTC().Format(time.RFC3339),
		Labels:      p.Labels,
		Annotations: p.Annotations,
		Containers:  containers,
		Conditions:  conds,
		Owner:       owner,
	}
}

func (cc *controller) ListPodsAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ns := kubecli.NamespaceQuery(c)
	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()
	list, err := cli.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{Limit: 5000})
	if err != nil {
		return cc.respondErr(c, err)
	}
	rows := make([]podRow, 0, len(list.Items))
	for _, p := range list.Items {
		d := podToDetail(&p)
		rows = append(rows, podRow{
			Namespace: d.Namespace,
			Name:      d.Name,
			Status:    d.Status,
			Ready:     d.Ready,
			Restarts:  d.Restarts,
			Node:      d.Node,
			Age:       d.CreatedAt,
			IP:        d.IP,
		})
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": rows}))
}

func (cc *controller) GetPodAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
	defer cancel()
	p, err := cli.CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": podToDetail(p)}))
}

func (cc *controller) PodLogsAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	container := strings.TrimSpace(c.Query("container"))
	tail := int64(300)
	if v := strings.TrimSpace(c.Query("tail")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			tail = n
		}
	}
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()

	opts := &corev1.PodLogOptions{TailLines: &tail}
	if container != "" {
		opts.Container = container
	} else {
		p, err := cli.CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return cc.respondErr(c, err)
		}
		if len(p.Spec.Containers) > 0 {
			opts.Container = p.Spec.Containers[0].Name
			container = opts.Container
		}
	}
	req := cli.CoreV1().Pods(ns).GetLogs(name, opts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return cc.respondErr(c, err)
	}
	defer stream.Close()
	b, err := io.ReadAll(stream)
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"namespace": ns,
			"name":      name,
			"container": container,
			"tail":      tail,
			"logs":      string(b),
		},
	}))
}

func (cc *controller) PodYAMLAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
	defer cancel()
	p, err := cli.CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	y, err := kubecli.ToYAML(p)
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{"namespace": ns, "name": name, "yaml": y},
	}))
}

type applyYAMLBody struct {
	YAML string `json:"yaml"`
}

func (cc *controller) ApplyPodYAMLAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	var body applyYAMLBody
	if err := c.Bind().Body(&body); err != nil {
		return cc.respondErr(c, fmt.Errorf("invalid body"))
	}
	res, err := kubecli.ApplyYAML(cc.app, body.YAML, kubecli.ApplyExpect{
		APIVersion: "v1",
		Kind:       "Pod",
		Namespace:  ns,
		Name:       name,
	})
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    res,
		"message": "Pod YAML applied",
	}))
}

func (cc *controller) DeletePodAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	if ns == "" || name == "" {
		return cc.respondErr(c, fmt.Errorf("namespace and name are required"))
	}
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()
	if err := cli.CoreV1().Pods(ns).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    fiber.Map{"namespace": ns, "name": name},
		"message": "Pod deleted",
	}))
}

type deployDetail struct {
	Namespace      string            `json:"namespace"`
	Name           string            `json:"name"`
	Ready          string            `json:"ready"`
	UpToDate       int32             `json:"up_to_date"`
	Available      int32             `json:"available"`
	Replicas       int32             `json:"replicas"`
	Updated        int32             `json:"updated_replicas"`
	Unavailable    int32             `json:"unavailable"`
	CreatedAt      string            `json:"created_at"`
	Images         []string          `json:"images"`
	Labels         map[string]string `json:"labels,omitempty"`
	Selector       map[string]string `json:"selector,omitempty"`
	Strategy       string            `json:"strategy,omitempty"`
	Conditions     []fiber.Map       `json:"conditions,omitempty"`
	Containers     []fiber.Map       `json:"containers,omitempty"`
}

type deployRow struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Ready     string `json:"ready"`
	UpToDate  int32  `json:"up_to_date"`
	Available int32  `json:"available"`
	Replicas  int32  `json:"replicas"`
	CreatedAt string `json:"created_at"`
	Images    string `json:"images"`
}

func (cc *controller) ListDeploymentsAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ns := kubecli.NamespaceQuery(c)
	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()
	list, err := cli.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{Limit: 5000})
	if err != nil {
		return cc.respondErr(c, err)
	}
	rows := make([]deployRow, 0, len(list.Items))
	for _, d := range list.Items {
		var images []string
		for _, ctr := range d.Spec.Template.Spec.Containers {
			images = append(images, ctr.Image)
		}
		desired := int32(0)
		if d.Spec.Replicas != nil {
			desired = *d.Spec.Replicas
		}
		rows = append(rows, deployRow{
			Namespace: d.Namespace,
			Name:      d.Name,
			Ready:     fmt.Sprintf("%d/%d", d.Status.ReadyReplicas, desired),
			UpToDate:  d.Status.UpdatedReplicas,
			Available: d.Status.AvailableReplicas,
			Replicas:  desired,
			CreatedAt: d.CreationTimestamp.UTC().Format(time.RFC3339),
			Images:    strings.Join(images, ", "),
		})
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": rows}))
}

type createDeploymentBody struct {
	Namespace string            `json:"namespace"`
	Name      string            `json:"name"`
	Image     string            `json:"image"`
	Replicas  *int32            `json:"replicas"`
	Labels    map[string]string `json:"labels"`
	Command   []string          `json:"command"`
	Args      []string          `json:"args"`
	Port      *int32            `json:"port"`
}

func (cc *controller) CreateDeploymentAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	var body createDeploymentBody
	if err := c.Bind().Body(&body); err != nil {
		return cc.respondErr(c, fmt.Errorf("invalid body"))
	}
	ns := strings.TrimSpace(body.Namespace)
	name := strings.TrimSpace(body.Name)
	image := strings.TrimSpace(body.Image)
	if ns == "" {
		ns = "default"
	}
	if name == "" {
		return cc.respondErr(c, fmt.Errorf("name is required"))
	}
	if image == "" {
		return cc.respondErr(c, fmt.Errorf("image is required"))
	}
	replicas := int32(1)
	if body.Replicas != nil && *body.Replicas >= 0 {
		replicas = *body.Replicas
	}

	matchLabels := map[string]string{}
	for k, v := range body.Labels {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		matchLabels[k] = strings.TrimSpace(v)
	}
	if len(matchLabels) == 0 {
		matchLabels["app"] = name
	}

	cmd := make([]string, 0, len(body.Command))
	for _, part := range body.Command {
		part = strings.TrimSpace(part)
		if part != "" {
			cmd = append(cmd, part)
		}
	}
	args := make([]string, 0, len(body.Args))
	for _, part := range body.Args {
		part = strings.TrimSpace(part)
		if part != "" {
			args = append(args, part)
		}
	}

	ctr := corev1.Container{
		Name:  "main",
		Image: image,
	}
	if len(cmd) > 0 {
		ctr.Command = cmd
	}
	if len(args) > 0 {
		ctr.Args = args
	}
	if body.Port != nil && *body.Port > 0 {
		ctr.Ports = []corev1.ContainerPort{{
			Name:          "http",
			ContainerPort: *body.Port,
			Protocol:      corev1.ProtocolTCP,
		}}
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    matchLabels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: matchLabels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: matchLabels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{ctr},
				},
			},
		},
	}

	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()
	created, err := cli.AppsV1().Deployments(ns).Create(ctx, dep, metav1.CreateOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	desired := int32(0)
	if created.Spec.Replicas != nil {
		desired = *created.Spec.Replicas
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": deployRow{
			Namespace: created.Namespace,
			Name:      created.Name,
			Ready:     fmt.Sprintf("%d/%d", created.Status.ReadyReplicas, desired),
			UpToDate:  created.Status.UpdatedReplicas,
			Available: created.Status.AvailableReplicas,
			Replicas:  desired,
			CreatedAt: created.CreationTimestamp.UTC().Format(time.RFC3339),
			Images:    image,
		},
		"message": "Deployment created",
	}))
}

func (cc *controller) GetDeploymentAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
	defer cancel()
	d, err := cli.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	desired := int32(0)
	if d.Spec.Replicas != nil {
		desired = *d.Spec.Replicas
	}
	images := make([]string, 0)
	ctrs := make([]fiber.Map, 0)
	for _, ctr := range d.Spec.Template.Spec.Containers {
		images = append(images, ctr.Image)
		ctrs = append(ctrs, fiber.Map{
			"name":  ctr.Name,
			"image": ctr.Image,
			"ports": ctr.Ports,
		})
	}
	conds := make([]fiber.Map, 0, len(d.Status.Conditions))
	for _, cnd := range d.Status.Conditions {
		conds = append(conds, fiber.Map{
			"type":    string(cnd.Type),
			"status":  string(cnd.Status),
			"reason":  cnd.Reason,
			"message": cnd.Message,
		})
	}
	sel := map[string]string{}
	if d.Spec.Selector != nil {
		sel = d.Spec.Selector.MatchLabels
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": deployDetail{
			Namespace:   d.Namespace,
			Name:        d.Name,
			Ready:       fmt.Sprintf("%d/%d", d.Status.ReadyReplicas, desired),
			UpToDate:    d.Status.UpdatedReplicas,
			Available:   d.Status.AvailableReplicas,
			Replicas:    desired,
			Updated:     d.Status.UpdatedReplicas,
			Unavailable: d.Status.UnavailableReplicas,
			CreatedAt:   d.CreationTimestamp.UTC().Format(time.RFC3339),
			Images:      images,
			Labels:      d.Labels,
			Selector:    sel,
			Strategy:    string(d.Spec.Strategy.Type),
			Conditions:  conds,
			Containers:  ctrs,
		},
	}))
}

func (cc *controller) DeploymentPodsAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()
	d, err := cli.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	sel := labels.Everything().String()
	if d.Spec.Selector != nil {
		sel = labels.Set(d.Spec.Selector.MatchLabels).String()
	}
	list, err := cli.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{LabelSelector: sel, Limit: 5000})
	if err != nil {
		return cc.respondErr(c, err)
	}
	rows := make([]podRow, 0, len(list.Items))
	for i := range list.Items {
		pd := podToDetail(&list.Items[i])
		rows = append(rows, podRow{
			Namespace: pd.Namespace,
			Name:      pd.Name,
			Status:    pd.Status,
			Ready:     pd.Ready,
			Restarts:  pd.Restarts,
			Node:      pd.Node,
			Age:       pd.CreatedAt,
			IP:        pd.IP,
		})
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": rows}))
}

func (cc *controller) DeploymentYAMLAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
	defer cancel()
	d, err := cli.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	y, err := kubecli.ToYAML(d)
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{"namespace": ns, "name": name, "yaml": y},
	}))
}

func (cc *controller) ApplyDeploymentYAMLAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	var body applyYAMLBody
	if err := c.Bind().Body(&body); err != nil {
		return cc.respondErr(c, fmt.Errorf("invalid body"))
	}
	res, err := kubecli.ApplyYAML(cc.app, body.YAML, kubecli.ApplyExpect{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Namespace:  ns,
		Name:       name,
	})
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    res,
		"message": "Deployment YAML applied",
	}))
}

func (cc *controller) DeleteDeploymentAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()
	if err := cli.AppsV1().Deployments(ns).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    fiber.Map{"namespace": ns, "name": name},
		"message": "Deployment deleted",
	}))
}

type scaleBody struct {
	Replicas int32 `json:"replicas"`
}

func (cc *controller) ScaleDeploymentAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	var body scaleBody
	if err := c.Bind().Body(&body); err != nil {
		return cc.respondErr(c, fmt.Errorf("invalid body"))
	}
	if body.Replicas < 0 {
		return cc.respondErr(c, fmt.Errorf("replicas must be >= 0"))
	}
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()
	scale, err := cli.AppsV1().Deployments(ns).GetScale(ctx, name, metav1.GetOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	scale.Spec.Replicas = body.Replicas
	updated, err := cli.AppsV1().Deployments(ns).UpdateScale(ctx, name, scale, metav1.UpdateOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    fiber.Map{"namespace": ns, "name": name, "replicas": updated.Spec.Replicas},
		"message": "Deployment scaled",
	}))
}

func (cc *controller) RestartDeploymentAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()
	patch := fmt.Sprintf(
		`{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":%q}}}}}`,
		time.Now().UTC().Format(time.RFC3339),
	)
	_, err = cli.AppsV1().Deployments(ns).Patch(
		ctx, name, types.StrategicMergePatchType, []byte(patch), metav1.PatchOptions{},
	)
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    fiber.Map{"namespace": ns, "name": name},
		"message": "Deployment restarted",
	}))
}

// PullRestartDeploymentAPI forces image re-pull (Always) and rolls out a restart.
func (cc *controller) PullRestartDeploymentAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()

	dep, err := cli.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if dep.Spec.Template.Annotations == nil {
		dep.Spec.Template.Annotations = map[string]string{}
	}
	dep.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = now

	for i := range dep.Spec.Template.Spec.Containers {
		dep.Spec.Template.Spec.Containers[i].ImagePullPolicy = corev1.PullAlways
	}
	for i := range dep.Spec.Template.Spec.InitContainers {
		dep.Spec.Template.Spec.InitContainers[i].ImagePullPolicy = corev1.PullAlways
	}

	if _, err := cli.AppsV1().Deployments(ns).Update(ctx, dep, metav1.UpdateOptions{}); err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    fiber.Map{"namespace": ns, "name": name},
		"message": "Pulling latest image and restarting deployment",
	}))
}
