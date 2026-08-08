package services

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
	"k8s.io/apimachinery/pkg/util/intstr"
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
	single.Get("/:namespace/:name", cc.GetAPI)
	single.Get("/:namespace/:name/yaml", cc.YAMLAPI)
	single.Put("/:namespace/:name/yaml", cc.ApplyYAMLAPI)
	single.Delete("/:namespace/:name", cc.DeleteAPI)
}

func (cc *controller) respondErr(c fiber.Ctx, err error) error {
	r := cc.app.Render()
	return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("KUBERNETES_ERROR"))
}

type svcRow struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	ClusterIP string `json:"cluster_ip"`
	External  string `json:"external_ip,omitempty"`
	Ports     string `json:"ports"`
	CreatedAt string `json:"created_at"`
}

func (cc *controller) ListAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ns := kubecli.NamespaceQuery(c)
	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()
	list, err := cli.CoreV1().Services(ns).List(ctx, metav1.ListOptions{Limit: 5000})
	if err != nil {
		return cc.respondErr(c, err)
	}
	rows := make([]svcRow, 0, len(list.Items))
	for _, s := range list.Items {
		ports := make([]string, 0, len(s.Spec.Ports))
		for _, p := range s.Spec.Ports {
			ports = append(ports, fmt.Sprintf("%d/%s", p.Port, p.Protocol))
		}
		ext := strings.Join(s.Spec.ExternalIPs, ",")
		if ext == "" && s.Spec.Type == corev1.ServiceTypeLoadBalancer {
			var lbs []string
			for _, ing := range s.Status.LoadBalancer.Ingress {
				if ing.IP != "" {
					lbs = append(lbs, ing.IP)
				} else if ing.Hostname != "" {
					lbs = append(lbs, ing.Hostname)
				}
			}
			ext = strings.Join(lbs, ",")
		}
		rows = append(rows, svcRow{
			Namespace: s.Namespace,
			Name:      s.Name,
			Type:      string(s.Spec.Type),
			ClusterIP: s.Spec.ClusterIP,
			External:  ext,
			Ports:     strings.Join(ports, ", "),
			CreatedAt: s.CreationTimestamp.UTC().Format(time.RFC3339),
		})
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": rows}))
}

func (cc *controller) GetAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
	defer cancel()
	s, err := cli.CoreV1().Services(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	ports := make([]fiber.Map, 0, len(s.Spec.Ports))
	for _, p := range s.Spec.Ports {
		ports = append(ports, fiber.Map{
			"name":        p.Name,
			"port":        p.Port,
			"target_port": p.TargetPort.String(),
			"node_port":   p.NodePort,
			"protocol":    string(p.Protocol),
		})
	}
	ext := strings.Join(s.Spec.ExternalIPs, ",")
	if ext == "" && s.Spec.Type == corev1.ServiceTypeLoadBalancer {
		var lbs []string
		for _, ing := range s.Status.LoadBalancer.Ingress {
			if ing.IP != "" {
				lbs = append(lbs, ing.IP)
			} else if ing.Hostname != "" {
				lbs = append(lbs, ing.Hostname)
			}
		}
		ext = strings.Join(lbs, ",")
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"namespace":   s.Namespace,
			"name":        s.Name,
			"type":        string(s.Spec.Type),
			"cluster_ip":  s.Spec.ClusterIP,
			"external_ip": ext,
			"ports":       ports,
			"selector":    s.Spec.Selector,
			"labels":      s.Labels,
			"annotations": s.Annotations,
			"created_at":  s.CreationTimestamp.UTC().Format(time.RFC3339),
		},
	}))
}

func (cc *controller) YAMLAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
	defer cancel()
	s, err := cli.CoreV1().Services(ns).Get(ctx, name, metav1.GetOptions{})
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

type applyYAMLBody struct {
	YAML string `json:"yaml"`
}

func (cc *controller) ApplyYAMLAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	var body applyYAMLBody
	if err := c.Bind().Body(&body); err != nil {
		return cc.respondErr(c, fmt.Errorf("invalid body"))
	}
	res, err := kubecli.ApplyYAML(cc.app, body.YAML, kubecli.ApplyExpect{
		APIVersion: "v1",
		Kind:       "Service",
		Namespace:  ns,
		Name:       name,
	})
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    res,
		"message": "Service YAML applied",
	}))
}

type createBody struct {
	Namespace string            `json:"namespace"`
	Name      string            `json:"name"`
	Type      string            `json:"type"`
	Port      int32             `json:"port"`
	TargetPort int32            `json:"target_port"`
	Selector  map[string]string `json:"selector"`
}

func (cc *controller) CreateAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	var body createBody
	if err := c.Bind().Body(&body); err != nil {
		return cc.respondErr(c, fmt.Errorf("invalid body"))
	}
	ns := strings.TrimSpace(body.Namespace)
	name := strings.TrimSpace(body.Name)
	if ns == "" {
		ns = "default"
	}
	if name == "" || body.Port <= 0 {
		return cc.respondErr(c, fmt.Errorf("name and port are required"))
	}
	target := body.TargetPort
	if target <= 0 {
		target = body.Port
	}
	svcType := corev1.ServiceTypeClusterIP
	switch strings.ToLower(strings.TrimSpace(body.Type)) {
	case "nodeport":
		svcType = corev1.ServiceTypeNodePort
	case "loadbalancer":
		svcType = corev1.ServiceTypeLoadBalancer
	case "externalname":
		svcType = corev1.ServiceTypeExternalName
	}
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()
	svc, err := cli.CoreV1().Services(ns).Create(ctx, &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: corev1.ServiceSpec{
			Type:     svcType,
			Selector: body.Selector,
			Ports: []corev1.ServicePort{{
				Port:       body.Port,
				TargetPort: intstr.FromInt32(target),
				Protocol:   corev1.ProtocolTCP,
			}},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"namespace":  svc.Namespace,
			"name":       svc.Name,
			"cluster_ip": svc.Spec.ClusterIP,
			"type":       string(svc.Spec.Type),
		},
		"message": "Service created",
	}))
}

func (cc *controller) DeleteAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()
	if err := cli.CoreV1().Services(ns).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    fiber.Map{"namespace": ns, "name": name},
		"message": "Service deleted",
	}))
}
