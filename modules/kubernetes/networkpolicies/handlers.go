package networkpolicies

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

type policyRow struct {
	Namespace    string `json:"namespace"`
	Name         string `json:"name"`
	PodSelector  string `json:"pod_selector"`
	PolicyTypes  string `json:"policy_types"`
	IngressRules int    `json:"ingress_rules"`
	EgressRules  int    `json:"egress_rules"`
	CreatedAt    string `json:"created_at"`
}

func formatSelector(sel *metav1.LabelSelector) string {
	if sel == nil || (len(sel.MatchLabels) == 0 && len(sel.MatchExpressions) == 0) {
		return "*"
	}
	parts := make([]string, 0, len(sel.MatchLabels)+len(sel.MatchExpressions))
	for k, v := range sel.MatchLabels {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	for _, e := range sel.MatchExpressions {
		parts = append(parts, fmt.Sprintf("%s %s %s", e.Key, e.Operator, strings.Join(e.Values, ",")))
	}
	return strings.Join(parts, ", ")
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
	list, err := cli.NetworkingV1().NetworkPolicies(ns).List(ctx, metav1.ListOptions{Limit: 5000})
	if err != nil {
		return cc.respondErr(c, err)
	}
	rows := make([]policyRow, 0, len(list.Items))
	for _, p := range list.Items {
		types := make([]string, 0, len(p.Spec.PolicyTypes))
		for _, t := range p.Spec.PolicyTypes {
			types = append(types, string(t))
		}
		rows = append(rows, policyRow{
			Namespace:    p.Namespace,
			Name:         p.Name,
			PodSelector:  formatSelector(&p.Spec.PodSelector),
			PolicyTypes:  strings.Join(types, ", "),
			IngressRules: len(p.Spec.Ingress),
			EgressRules:  len(p.Spec.Egress),
			CreatedAt:    p.CreationTimestamp.UTC().Format(time.RFC3339),
		})
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": rows}))
}

func peerSummary(peers []networkingv1.NetworkPolicyPeer) []fiber.Map {
	out := make([]fiber.Map, 0, len(peers))
	for _, peer := range peers {
		m := fiber.Map{}
		if peer.PodSelector != nil {
			m["pod_selector"] = formatSelector(peer.PodSelector)
		}
		if peer.NamespaceSelector != nil {
			m["namespace_selector"] = formatSelector(peer.NamespaceSelector)
		}
		if peer.IPBlock != nil {
			m["cidr"] = peer.IPBlock.CIDR
			if len(peer.IPBlock.Except) > 0 {
				m["except"] = peer.IPBlock.Except
			}
		}
		out = append(out, m)
	}
	return out
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
	p, err := cli.NetworkingV1().NetworkPolicies(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	types := make([]string, 0, len(p.Spec.PolicyTypes))
	for _, t := range p.Spec.PolicyTypes {
		types = append(types, string(t))
	}
	ingress := make([]fiber.Map, 0, len(p.Spec.Ingress))
	for _, rule := range p.Spec.Ingress {
		ports := make([]fiber.Map, 0, len(rule.Ports))
		for _, port := range rule.Ports {
			pm := fiber.Map{}
			if port.Protocol != nil {
				pm["protocol"] = string(*port.Protocol)
			}
			if port.Port != nil {
				pm["port"] = port.Port.String()
			}
			ports = append(ports, pm)
		}
		ingress = append(ingress, fiber.Map{
			"ports": ports,
			"from":  peerSummary(rule.From),
		})
	}
	egress := make([]fiber.Map, 0, len(p.Spec.Egress))
	for _, rule := range p.Spec.Egress {
		ports := make([]fiber.Map, 0, len(rule.Ports))
		for _, port := range rule.Ports {
			pm := fiber.Map{}
			if port.Protocol != nil {
				pm["protocol"] = string(*port.Protocol)
			}
			if port.Port != nil {
				pm["port"] = port.Port.String()
			}
			ports = append(ports, pm)
		}
		egress = append(egress, fiber.Map{
			"ports": ports,
			"to":    peerSummary(rule.To),
		})
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"namespace":    p.Namespace,
			"name":         p.Name,
			"pod_selector": formatSelector(&p.Spec.PodSelector),
			"policy_types": types,
			"ingress":      ingress,
			"egress":       egress,
			"labels":       p.Labels,
			"annotations":  p.Annotations,
			"created_at":   p.CreationTimestamp.UTC().Format(time.RFC3339),
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
	p, err := cli.NetworkingV1().NetworkPolicies(ns).Get(ctx, name, metav1.GetOptions{})
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
		Kind:       "NetworkPolicy",
		Namespace:  ns,
		Name:       name,
	})
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    res,
		"message": "NetworkPolicy YAML applied",
	}))
}

type createBody struct {
	Namespace              string            `json:"namespace"`
	Name                   string            `json:"name"`
	PodSelector            map[string]string `json:"pod_selector"`
	PolicyTypes            []string          `json:"policy_types"`
	AllowFromSameNamespace bool              `json:"allow_from_same_namespace"`
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
	if name == "" {
		return cc.respondErr(c, fmt.Errorf("name is required"))
	}

	types := make([]networkingv1.PolicyType, 0, 2)
	wantIngress, wantEgress := false, false
	for _, t := range body.PolicyTypes {
		switch strings.TrimSpace(t) {
		case "Ingress":
			wantIngress = true
		case "Egress":
			wantEgress = true
		}
	}
	if !wantIngress && !wantEgress {
		wantIngress = true
	}
	if wantIngress {
		types = append(types, networkingv1.PolicyTypeIngress)
	}
	if wantEgress {
		types = append(types, networkingv1.PolicyTypeEgress)
	}

	sel := map[string]string{}
	for k, v := range body.PodSelector {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		sel[k] = strings.TrimSpace(v)
	}

	spec := networkingv1.NetworkPolicySpec{
		PodSelector: metav1.LabelSelector{MatchLabels: sel},
		PolicyTypes: types,
	}
	if wantIngress {
		if body.AllowFromSameNamespace {
			spec.Ingress = []networkingv1.NetworkPolicyIngressRule{{
				From: []networkingv1.NetworkPolicyPeer{{
					PodSelector: &metav1.LabelSelector{},
				}},
			}}
		} else {
			// Empty ingress list + Ingress policy type = deny all ingress.
			spec.Ingress = []networkingv1.NetworkPolicyIngressRule{}
		}
	}
	if wantEgress {
		// Empty egress list + Egress policy type = deny all egress.
		spec.Egress = []networkingv1.NetworkPolicyEgressRule{}
	}

	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()
	created, err := cli.NetworkingV1().NetworkPolicies(ns).Create(ctx, &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec:       spec,
	}, metav1.CreateOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}

	typeStrs := make([]string, 0, len(created.Spec.PolicyTypes))
	for _, t := range created.Spec.PolicyTypes {
		typeStrs = append(typeStrs, string(t))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": policyRow{
			Namespace:    created.Namespace,
			Name:         created.Name,
			PodSelector:  formatSelector(&created.Spec.PodSelector),
			PolicyTypes:  strings.Join(typeStrs, ", "),
			IngressRules: len(created.Spec.Ingress),
			EgressRules:  len(created.Spec.Egress),
			CreatedAt:    created.CreationTimestamp.UTC().Format(time.RFC3339),
		},
		"message": "NetworkPolicy created",
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
	if err := cli.NetworkingV1().NetworkPolicies(ns).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    fiber.Map{"namespace": ns, "name": name},
		"message": "NetworkPolicy deleted",
	}))
}
