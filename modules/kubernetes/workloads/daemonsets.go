package workloads

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"

	"github.com/izetmolla/containerws/modules/kubernetes/kubecli"
)

type dsRow struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Desired   int32  `json:"desired"`
	Current   int32  `json:"current"`
	Ready     int32  `json:"ready"`
	UpToDate  int32  `json:"up_to_date"`
	Available int32  `json:"available"`
	CreatedAt string `json:"created_at"`
	Images    string `json:"images"`
}

func (cc *controller) ListDaemonSetsAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ns := kubecli.NamespaceQuery(c)
	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()
	list, err := cli.AppsV1().DaemonSets(ns).List(ctx, metav1.ListOptions{Limit: 5000})
	if err != nil {
		return cc.respondErr(c, err)
	}
	rows := make([]dsRow, 0, len(list.Items))
	for _, d := range list.Items {
		var images []string
		for _, ctr := range d.Spec.Template.Spec.Containers {
			images = append(images, ctr.Image)
		}
		rows = append(rows, dsRow{
			Namespace: d.Namespace,
			Name:      d.Name,
			Desired:   d.Status.DesiredNumberScheduled,
			Current:   d.Status.CurrentNumberScheduled,
			Ready:     d.Status.NumberReady,
			UpToDate:  d.Status.UpdatedNumberScheduled,
			Available: d.Status.NumberAvailable,
			CreatedAt: d.CreationTimestamp.UTC().Format(time.RFC3339),
			Images:    strings.Join(images, ", "),
		})
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": rows}))
}

type createDaemonSetBody struct {
	Namespace string            `json:"namespace"`
	Name      string            `json:"name"`
	Image     string            `json:"image"`
	Labels    map[string]string `json:"labels"`
	Command   []string          `json:"command"`
	Args      []string          `json:"args"`
}

func (cc *controller) CreateDaemonSetAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	var body createDaemonSetBody
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

	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    matchLabels,
		},
		Spec: appsv1.DaemonSetSpec{
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
	created, err := cli.AppsV1().DaemonSets(ns).Create(ctx, ds, metav1.CreateOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": dsRow{
			Namespace: created.Namespace,
			Name:      created.Name,
			Desired:   created.Status.DesiredNumberScheduled,
			Current:   created.Status.CurrentNumberScheduled,
			Ready:     created.Status.NumberReady,
			UpToDate:  created.Status.UpdatedNumberScheduled,
			Available: created.Status.NumberAvailable,
			CreatedAt: created.CreationTimestamp.UTC().Format(time.RFC3339),
			Images:    image,
		},
		"message": "DaemonSet created",
	}))
}

func (cc *controller) GetDaemonSetAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
	defer cancel()
	d, err := cli.AppsV1().DaemonSets(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	images := make([]string, 0)
	ctrs := make([]fiber.Map, 0)
	for _, ctr := range d.Spec.Template.Spec.Containers {
		images = append(images, ctr.Image)
		ctrs = append(ctrs, fiber.Map{"name": ctr.Name, "image": ctr.Image})
	}
	sel := map[string]string{}
	if d.Spec.Selector != nil {
		sel = d.Spec.Selector.MatchLabels
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"namespace":   d.Namespace,
			"name":        d.Name,
			"desired":     d.Status.DesiredNumberScheduled,
			"current":     d.Status.CurrentNumberScheduled,
			"ready":       d.Status.NumberReady,
			"up_to_date":  d.Status.UpdatedNumberScheduled,
			"available":   d.Status.NumberAvailable,
			"images":      images,
			"containers":  ctrs,
			"selector":    sel,
			"labels":      d.Labels,
			"annotations": d.Annotations,
			"created_at":  d.CreationTimestamp.UTC().Format(time.RFC3339),
		},
	}))
}

func (cc *controller) DaemonSetPodsAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()
	d, err := cli.AppsV1().DaemonSets(ns).Get(ctx, name, metav1.GetOptions{})
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

func (cc *controller) DaemonSetYAMLAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
	defer cancel()
	d, err := cli.AppsV1().DaemonSets(ns).Get(ctx, name, metav1.GetOptions{})
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

func (cc *controller) ApplyDaemonSetYAMLAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	var body applyYAMLBody
	if err := c.Bind().Body(&body); err != nil {
		return cc.respondErr(c, fmt.Errorf("invalid body"))
	}
	res, err := kubecli.ApplyYAML(cc.app, body.YAML, kubecli.ApplyExpect{
		APIVersion: "apps/v1",
		Kind:       "DaemonSet",
		Namespace:  ns,
		Name:       name,
	})
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    res,
		"message": "DaemonSet YAML applied",
	}))
}

func (cc *controller) DeleteDaemonSetAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()
	if err := cli.AppsV1().DaemonSets(ns).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    fiber.Map{"namespace": ns, "name": name},
		"message": "DaemonSet deleted",
	}))
}

func (cc *controller) RestartDaemonSetAPI(c fiber.Ctx) error {
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
	if _, err := cli.AppsV1().DaemonSets(ns).Patch(
		ctx, name, types.StrategicMergePatchType, []byte(patch), metav1.PatchOptions{},
	); err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    fiber.Map{"namespace": ns, "name": name},
		"message": "DaemonSet restarted",
	}))
}
