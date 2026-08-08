package ingresses

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/modules/kubernetes/kubecli"
	networkingv1 "k8s.io/api/networking/v1"
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

	api.Get("/options", cc.OptionsAPI)

	single := api.Group("/single")
	single.Post("/", cc.CreateAPI)
	single.Get("/:namespace/:name", cc.GetAPI)
	single.Put("/:namespace/:name", cc.UpdateAPI)
	single.Get("/:namespace/:name/yaml", cc.YAMLAPI)
	single.Put("/:namespace/:name/yaml", cc.ApplyYAMLAPI)
	single.Delete("/:namespace/:name", cc.DeleteAPI)
}

func (cc *controller) respondErr(c fiber.Ctx, err error) error {
	r := cc.app.Render()
	return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("KUBERNETES_ERROR"))
}

type ingressRow struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Class     string `json:"class"`
	Hosts     string `json:"hosts"`
	Address   string `json:"address"`
	CreatedAt string `json:"created_at"`
}

type pathBody struct {
	Path            string `json:"path"`
	PathType        string `json:"path_type"`
	ServiceName     string `json:"service_name"`
	ServicePort     int32  `json:"service_port"`
	ServicePortName string `json:"service_port_name"`
}

type ruleBody struct {
	Host  string     `json:"host"`
	Paths []pathBody `json:"paths"`
}

type tlsBody struct {
	Hosts      []string `json:"hosts"`
	SecretName string   `json:"secret_name"`
}

type ingressBody struct {
	Namespace    string            `json:"namespace"`
	Name         string            `json:"name"`
	IngressClass string            `json:"ingress_class"`
	Rules        []ruleBody        `json:"rules"`
	TLS          []tlsBody         `json:"tls"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
}

func ingressClass(ing *networkingv1.Ingress) string {
	if ing.Spec.IngressClassName != nil {
		return *ing.Spec.IngressClassName
	}
	if ing.Annotations != nil {
		if v := strings.TrimSpace(ing.Annotations["kubernetes.io/ingress.class"]); v != "" {
			return v
		}
	}
	return ""
}

func ingressAddresses(ing *networkingv1.Ingress) string {
	var parts []string
	for _, a := range ing.Status.LoadBalancer.Ingress {
		if a.IP != "" {
			parts = append(parts, a.IP)
		} else if a.Hostname != "" {
			parts = append(parts, a.Hostname)
		}
	}
	return strings.Join(parts, ", ")
}

func ingressHosts(ing *networkingv1.Ingress) string {
	seen := map[string]struct{}{}
	var hosts []string
	for _, rule := range ing.Spec.Rules {
		h := strings.TrimSpace(rule.Host)
		if h == "" {
			h = "*"
		}
		if _, ok := seen[h]; ok {
			continue
		}
		seen[h] = struct{}{}
		hosts = append(hosts, h)
	}
	for _, tls := range ing.Spec.TLS {
		for _, h := range tls.Hosts {
			h = strings.TrimSpace(h)
			if h == "" {
				continue
			}
			if _, ok := seen[h]; ok {
				continue
			}
			seen[h] = struct{}{}
			hosts = append(hosts, h)
		}
	}
	return strings.Join(hosts, ", ")
}

func pathTypeOf(s string) networkingv1.PathType {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "exact":
		return networkingv1.PathTypeExact
	case "implementationSpecific", "implementationspecific":
		return networkingv1.PathTypeImplementationSpecific
	default:
		return networkingv1.PathTypePrefix
	}
}

func buildSpec(body ingressBody) (networkingv1.IngressSpec, error) {
	if len(body.Rules) == 0 {
		return networkingv1.IngressSpec{}, fmt.Errorf("at least one rule is required")
	}
	rules := make([]networkingv1.IngressRule, 0, len(body.Rules))
	for _, rule := range body.Rules {
		if len(rule.Paths) == 0 {
			return networkingv1.IngressSpec{}, fmt.Errorf("each rule needs at least one path")
		}
		paths := make([]networkingv1.HTTPIngressPath, 0, len(rule.Paths))
		for _, p := range rule.Paths {
			svc := strings.TrimSpace(p.ServiceName)
			portName := strings.TrimSpace(p.ServicePortName)
			if svc == "" {
				return networkingv1.IngressSpec{}, fmt.Errorf("path service_name is required")
			}
			if p.ServicePort <= 0 && portName == "" {
				return networkingv1.IngressSpec{}, fmt.Errorf("path service_port or service_port_name is required")
			}
			path := strings.TrimSpace(p.Path)
			if path == "" {
				path = "/"
			}
			pt := pathTypeOf(p.PathType)
			backendPort := networkingv1.ServiceBackendPort{}
			if p.ServicePort > 0 {
				backendPort.Number = p.ServicePort
			} else {
				backendPort.Name = portName
			}
			paths = append(paths, networkingv1.HTTPIngressPath{
				Path:     path,
				PathType: &pt,
				Backend: networkingv1.IngressBackend{
					Service: &networkingv1.IngressServiceBackend{
						Name: svc,
						Port: backendPort,
					},
				},
			})
		}
		rules = append(rules, networkingv1.IngressRule{
			Host: strings.TrimSpace(rule.Host),
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{Paths: paths},
			},
		})
	}

	var tls []networkingv1.IngressTLS
	for _, t := range body.TLS {
		secret := strings.TrimSpace(t.SecretName)
		if secret == "" && len(t.Hosts) == 0 {
			continue
		}
		hosts := make([]string, 0, len(t.Hosts))
		for _, h := range t.Hosts {
			h = strings.TrimSpace(h)
			if h != "" {
				hosts = append(hosts, h)
			}
		}
		tls = append(tls, networkingv1.IngressTLS{Hosts: hosts, SecretName: secret})
	}

	spec := networkingv1.IngressSpec{Rules: rules, TLS: tls}
	if class := strings.TrimSpace(body.IngressClass); class != "" {
		spec.IngressClassName = &class
	}
	return spec, nil
}

func detailFrom(ing *networkingv1.Ingress) fiber.Map {
	rules := make([]fiber.Map, 0, len(ing.Spec.Rules))
	for _, rule := range ing.Spec.Rules {
		paths := make([]fiber.Map, 0)
		if rule.HTTP != nil {
			for _, p := range rule.HTTP.Paths {
				pt := ""
				if p.PathType != nil {
					pt = string(*p.PathType)
				}
				svcName := ""
				var svcPort int32
				portName := ""
				if p.Backend.Service != nil {
					svcName = p.Backend.Service.Name
					svcPort = p.Backend.Service.Port.Number
					portName = p.Backend.Service.Port.Name
				}
				row := fiber.Map{
					"path":         p.Path,
					"path_type":    pt,
					"service_name": svcName,
					"service_port": svcPort,
				}
				if portName != "" {
					row["service_port_name"] = portName
				}
				paths = append(paths, row)
			}
		}
		rules = append(rules, fiber.Map{
			"host":  rule.Host,
			"paths": paths,
		})
	}
	tls := make([]fiber.Map, 0, len(ing.Spec.TLS))
	for _, t := range ing.Spec.TLS {
		tls = append(tls, fiber.Map{
			"hosts":       t.Hosts,
			"secret_name": t.SecretName,
		})
	}
	return fiber.Map{
		"namespace":     ing.Namespace,
		"name":          ing.Name,
		"ingress_class": ingressClass(ing),
		"hosts":         ingressHosts(ing),
		"address":       ingressAddresses(ing),
		"rules":         rules,
		"tls":           tls,
		"labels":        ing.Labels,
		"annotations":   ing.Annotations,
		"created_at":    ing.CreationTimestamp.UTC().Format(time.RFC3339),
	}
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
	list, err := cli.NetworkingV1().Ingresses(ns).List(ctx, metav1.ListOptions{Limit: 5000})
	if err != nil {
		return cc.respondErr(c, err)
	}
	rows := make([]ingressRow, 0, len(list.Items))
	for i := range list.Items {
		ing := &list.Items[i]
		rows = append(rows, ingressRow{
			Namespace: ing.Namespace,
			Name:      ing.Name,
			Class:     ingressClass(ing),
			Hosts:     ingressHosts(ing),
			Address:   ingressAddresses(ing),
			CreatedAt: ing.CreationTimestamp.UTC().Format(time.RFC3339),
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
	ing, err := cli.NetworkingV1().Ingresses(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": detailFrom(ing)}))
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
	ing, err := cli.NetworkingV1().Ingresses(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	y, err := kubecli.ToYAML(ing)
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
		APIVersion: "networking.k8s.io/v1",
		Kind:       "Ingress",
		Namespace:  ns,
		Name:       name,
	})
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    res,
		"message": "Ingress YAML applied",
	}))
}

func (cc *controller) CreateAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	var body ingressBody
	if err := c.Bind().Body(&body); err != nil {
		return cc.respondErr(c, fmt.Errorf("invalid body"))
	}
	ns := strings.TrimSpace(body.Namespace)
	name := strings.TrimSpace(body.Name)
	if ns == "" {
		ns = "default"
	}
	if name == "" {
		return cc.respondErr(c, fmt.Errorf("name is required"))
	}
	// Convenience: if no rules, build one from legacy flat fields is not needed —
	// frontend always sends rules. Allow single empty host rule via validation.
	spec, err := buildSpec(body)
	if err != nil {
		return cc.respondErr(c, err)
	}
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()
	ing, err := cli.NetworkingV1().Ingresses(ns).Create(ctx, &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      body.Labels,
			Annotations: body.Annotations,
		},
		Spec: spec,
	}, metav1.CreateOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    detailFrom(ing),
		"message": "Ingress created",
	}))
}

func (cc *controller) UpdateAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	var body ingressBody
	if err := c.Bind().Body(&body); err != nil {
		return cc.respondErr(c, fmt.Errorf("invalid body"))
	}
	spec, err := buildSpec(body)
	if err != nil {
		return cc.respondErr(c, err)
	}
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()
	ing, err := cli.NetworkingV1().Ingresses(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	ing.Spec = spec
	if body.Labels != nil {
		ing.Labels = body.Labels
	}
	if body.Annotations != nil {
		ing.Annotations = body.Annotations
	}
	updated, err := cli.NetworkingV1().Ingresses(ns).Update(ctx, ing, metav1.UpdateOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    detailFrom(updated),
		"message": "Ingress updated",
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
	if err := cli.NetworkingV1().Ingresses(ns).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    fiber.Map{"namespace": ns, "name": name},
		"message": "Ingress deleted",
	}))
}
