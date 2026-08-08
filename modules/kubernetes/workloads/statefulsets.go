package workloads

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/izetmolla/containerws/modules/kubernetes/kubecli"
)

type stsRow struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Ready     string `json:"ready"`
	Replicas  int32  `json:"replicas"`
	Service   string `json:"service_name,omitempty"`
	CreatedAt string `json:"created_at"`
	Images    string `json:"images"`
}

func (cc *controller) ListStatefulSetsAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ns := kubecli.NamespaceQuery(c)
	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()
	list, err := cli.AppsV1().StatefulSets(ns).List(ctx, metav1.ListOptions{Limit: 5000})
	if err != nil {
		return cc.respondErr(c, err)
	}
	rows := make([]stsRow, 0, len(list.Items))
	for _, s := range list.Items {
		var images []string
		for _, ctr := range s.Spec.Template.Spec.Containers {
			images = append(images, ctr.Image)
		}
		desired := int32(0)
		if s.Spec.Replicas != nil {
			desired = *s.Spec.Replicas
		}
		rows = append(rows, stsRow{
			Namespace: s.Namespace,
			Name:      s.Name,
			Ready:     fmt.Sprintf("%d/%d", s.Status.ReadyReplicas, desired),
			Replicas:  desired,
			Service:   s.Spec.ServiceName,
			CreatedAt: s.CreationTimestamp.UTC().Format(time.RFC3339),
			Images:    strings.Join(images, ", "),
		})
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": rows}))
}

type createStatefulSetBody struct {
	Namespace     string            `json:"namespace"`
	Name          string            `json:"name"`
	Image         string            `json:"image"`
	Replicas      *int32            `json:"replicas"`
	ServiceName   string            `json:"service_name"`
	CreateService *bool             `json:"create_service"`
	Port          *int32            `json:"port"`
	Labels        map[string]string `json:"labels"`
	Command       []string          `json:"command"`
	Args          []string          `json:"args"`
}

func (cc *controller) CreateStatefulSetAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	var body createStatefulSetBody
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
	serviceName := strings.TrimSpace(body.ServiceName)
	if serviceName == "" {
		serviceName = name
	}
	replicas := int32(1)
	if body.Replicas != nil && *body.Replicas >= 0 {
		replicas = *body.Replicas
	}
	createSvc := true
	if body.CreateService != nil {
		createSvc = *body.CreateService
	}
	port := int32(80)
	if body.Port != nil && *body.Port > 0 {
		port = *body.Port
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
		Ports: []corev1.ContainerPort{{ContainerPort: port, Name: "http"}},
	}
	if len(cmd) > 0 {
		ctr.Command = cmd
	}
	if len(args) > 0 {
		ctr.Args = args
	}

	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()

	if createSvc {
		_, getErr := cli.CoreV1().Services(ns).Get(ctx, serviceName, metav1.GetOptions{})
		if apierrors.IsNotFound(getErr) {
			_, err = cli.CoreV1().Services(ns).Create(ctx, &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      serviceName,
					Namespace: ns,
					Labels:    matchLabels,
				},
				Spec: corev1.ServiceSpec{
					ClusterIP: corev1.ClusterIPNone,
					Selector:  matchLabels,
					Ports: []corev1.ServicePort{{
						Name:       "http",
						Port:       port,
						TargetPort: intstr.FromInt32(port),
					}},
				},
			}, metav1.CreateOptions{})
			if err != nil {
				return cc.respondErr(c, fmt.Errorf("create headless service: %w", err))
			}
		} else if getErr != nil {
			return cc.respondErr(c, getErr)
		}
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    matchLabels,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: serviceName,
			Replicas:    &replicas,
			Selector:    &metav1.LabelSelector{MatchLabels: matchLabels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: matchLabels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{ctr},
				},
			},
		},
	}

	created, err := cli.AppsV1().StatefulSets(ns).Create(ctx, sts, metav1.CreateOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	desired := int32(0)
	if created.Spec.Replicas != nil {
		desired = *created.Spec.Replicas
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": stsRow{
			Namespace: created.Namespace,
			Name:      created.Name,
			Ready:     fmt.Sprintf("%d/%d", created.Status.ReadyReplicas, desired),
			Replicas:  desired,
			Service:   created.Spec.ServiceName,
			CreatedAt: created.CreationTimestamp.UTC().Format(time.RFC3339),
			Images:    image,
		},
		"message": "StatefulSet created",
	}))
}

func (cc *controller) GetStatefulSetAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
	defer cancel()
	s, err := cli.AppsV1().StatefulSets(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	desired := int32(0)
	if s.Spec.Replicas != nil {
		desired = *s.Spec.Replicas
	}
	images := make([]string, 0)
	ctrs := make([]fiber.Map, 0)
	for _, ctr := range s.Spec.Template.Spec.Containers {
		images = append(images, ctr.Image)
		ctrs = append(ctrs, fiber.Map{"name": ctr.Name, "image": ctr.Image})
	}
	sel := map[string]string{}
	if s.Spec.Selector != nil {
		sel = s.Spec.Selector.MatchLabels
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"namespace":        s.Namespace,
			"name":             s.Name,
			"ready":            fmt.Sprintf("%d/%d", s.Status.ReadyReplicas, desired),
			"replicas":         desired,
			"current_replicas": s.Status.CurrentReplicas,
			"updated_replicas": s.Status.UpdatedReplicas,
			"service_name":     s.Spec.ServiceName,
			"update_strategy":  string(s.Spec.UpdateStrategy.Type),
			"images":           images,
			"containers":       ctrs,
			"selector":         sel,
			"labels":           s.Labels,
			"annotations":      s.Annotations,
			"created_at":       s.CreationTimestamp.UTC().Format(time.RFC3339),
		},
	}))
}

func (cc *controller) StatefulSetPodsAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()
	s, err := cli.AppsV1().StatefulSets(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	sel := labels.Everything().String()
	if s.Spec.Selector != nil {
		sel = labels.Set(s.Spec.Selector.MatchLabels).String()
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

func (cc *controller) StatefulSetYAMLAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
	defer cancel()
	s, err := cli.AppsV1().StatefulSets(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	y, err := kubecli.ToYAML(s)
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{"namespace": ns, "name": name, "yaml": y},
	}))
}

func (cc *controller) ApplyStatefulSetYAMLAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	var body applyYAMLBody
	if err := c.Bind().Body(&body); err != nil {
		return cc.respondErr(c, fmt.Errorf("invalid body"))
	}
	res, err := kubecli.ApplyYAML(cc.app, body.YAML, kubecli.ApplyExpect{
		APIVersion: "apps/v1",
		Kind:       "StatefulSet",
		Namespace:  ns,
		Name:       name,
	})
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    res,
		"message": "StatefulSet YAML applied",
	}))
}

func (cc *controller) DeleteStatefulSetAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()
	if err := cli.AppsV1().StatefulSets(ns).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    fiber.Map{"namespace": ns, "name": name},
		"message": "StatefulSet deleted",
	}))
}

func (cc *controller) ScaleStatefulSetAPI(c fiber.Ctx) error {
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
	s, err := cli.AppsV1().StatefulSets(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	s.Spec.Replicas = &body.Replicas
	if _, err := cli.AppsV1().StatefulSets(ns).Update(ctx, s, metav1.UpdateOptions{}); err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    fiber.Map{"namespace": ns, "name": name, "replicas": body.Replicas},
		"message": "StatefulSet scaled",
	}))
}

func (cc *controller) RestartStatefulSetAPI(c fiber.Ctx) error {
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
	if _, err := cli.AppsV1().StatefulSets(ns).Patch(
		ctx, name, types.StrategicMergePatchType, []byte(patch), metav1.PatchOptions{},
	); err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    fiber.Map{"namespace": ns, "name": name},
		"message": "StatefulSet restarted",
	}))
}
