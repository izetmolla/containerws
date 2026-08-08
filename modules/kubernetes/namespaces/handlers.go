package namespaces

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/modules/kubernetes/kubecli"
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
	single.Post("/", cc.CreateAPI)
	single.Get("/:name", cc.GetAPI)
	single.Get("/:name/yaml", cc.YAMLAPI)
	single.Put("/:name/yaml", cc.ApplyYAMLAPI)
	single.Delete("/:name", cc.DeleteAPI)
}

func (cc *controller) respondErr(c fiber.Ctx, err error) error {
	r := cc.app.Render()
	return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("KUBERNETES_ERROR"))
}

type nsRow struct {
	Name      string            `json:"name"`
	Status    string            `json:"status"`
	CreatedAt string            `json:"created_at"`
	Labels    map[string]string `json:"labels,omitempty"`
}

type nsDetail struct {
	Name         string            `json:"name"`
	Status       string            `json:"status"`
	CreatedAt    string            `json:"created_at"`
	Labels       map[string]string `json:"labels,omitempty"`
	Annotations  map[string]string `json:"annotations,omitempty"`
	Pods         int               `json:"pods"`
	Deployments  int               `json:"deployments"`
	Services     int               `json:"services"`
	ConfigMaps   int               `json:"configmaps"`
	Secrets      int               `json:"secrets"`
}

func (cc *controller) ListAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
	defer cancel()
	list, err := cli.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	rows := make([]nsRow, 0, len(list.Items))
	for _, n := range list.Items {
		rows = append(rows, nsRow{
			Name:      n.Name,
			Status:    string(n.Status.Phase),
			CreatedAt: n.CreationTimestamp.UTC().Format(time.RFC3339),
			Labels:    n.Labels,
		})
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": rows}))
}

func (cc *controller) GetAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	name := strings.TrimSpace(c.Params("name"))
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 25*time.Second)
	defer cancel()
	ns, err := cli.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	detail := nsDetail{
		Name:        ns.Name,
		Status:      string(ns.Status.Phase),
		CreatedAt:   ns.CreationTimestamp.UTC().Format(time.RFC3339),
		Labels:      ns.Labels,
		Annotations: ns.Annotations,
	}
	if list, err := cli.CoreV1().Pods(name).List(ctx, metav1.ListOptions{Limit: 5000}); err == nil {
		detail.Pods = len(list.Items)
	}
	if list, err := cli.AppsV1().Deployments(name).List(ctx, metav1.ListOptions{Limit: 5000}); err == nil {
		detail.Deployments = len(list.Items)
	}
	if list, err := cli.CoreV1().Services(name).List(ctx, metav1.ListOptions{Limit: 5000}); err == nil {
		detail.Services = len(list.Items)
	}
	if list, err := cli.CoreV1().ConfigMaps(name).List(ctx, metav1.ListOptions{Limit: 5000}); err == nil {
		detail.ConfigMaps = len(list.Items)
	}
	if list, err := cli.CoreV1().Secrets(name).List(ctx, metav1.ListOptions{Limit: 5000}); err == nil {
		detail.Secrets = len(list.Items)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": detail}))
}

func (cc *controller) YAMLAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	name := strings.TrimSpace(c.Params("name"))
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
	defer cancel()
	ns, err := cli.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	y, err := kubecli.ToYAML(ns)
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{"name": name, "yaml": y},
	}))
}

type applyYAMLBody struct {
	YAML string `json:"yaml"`
}

func (cc *controller) ApplyYAMLAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	name := strings.TrimSpace(c.Params("name"))
	var body applyYAMLBody
	if err := c.Bind().Body(&body); err != nil {
		return cc.respondErr(c, fmt.Errorf("invalid body"))
	}
	res, err := kubecli.ApplyYAML(cc.app, body.YAML, kubecli.ApplyExpect{
		APIVersion: "v1",
		Kind:       "Namespace",
		Name:       name,
	})
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    res,
		"message": "Namespace YAML applied",
	}))
}

type createBody struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels,omitempty"`
}

func (cc *controller) CreateAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	var body createBody
	if err := c.Bind().Body(&body); err != nil {
		return cc.respondErr(c, fmt.Errorf("invalid body"))
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		return cc.respondErr(c, fmt.Errorf("name is required"))
	}
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
	defer cancel()
	ns, err := cli.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: body.Labels},
	}, metav1.CreateOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    nsRow{Name: ns.Name, Status: string(ns.Status.Phase), CreatedAt: ns.CreationTimestamp.UTC().Format(time.RFC3339)},
		"message": "Namespace created",
	}))
}

func (cc *controller) DeleteAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	name := strings.TrimSpace(c.Params("name"))
	if name == "" {
		return cc.respondErr(c, fmt.Errorf("name is required"))
	}
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()
	if err := cli.CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    fiber.Map{"name": name},
		"message": "Namespace deleted",
	}))
}
