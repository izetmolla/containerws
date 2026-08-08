package ingresses

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/modules/kubernetes/kubecli"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type servicePortOption struct {
	Name     string `json:"name,omitempty"`
	Port     int32  `json:"port"`
	Protocol string `json:"protocol,omitempty"`
	Target   int32  `json:"target_port,omitempty"`
}

type serviceOption struct {
	Name  string              `json:"name"`
	Type  string              `json:"type,omitempty"`
	Ports []servicePortOption `json:"ports"`
}

type classOption struct {
	Name       string `json:"name"`
	Controller string `json:"controller,omitempty"`
	Default    bool   `json:"default,omitempty"`
}

type tlsSecretOption struct {
	Name string `json:"name"`
}

// OptionsAPI returns ingress form helpers for a namespace: classes, TLS secrets, services+ports.
func (cc *controller) OptionsAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Query("namespace"))
	if ns == "" {
		ns = "default"
	}
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()

	classes := make([]classOption, 0)
	if list, err := cli.NetworkingV1().IngressClasses().List(ctx, metav1.ListOptions{Limit: 500}); err == nil {
		for i := range list.Items {
			ic := &list.Items[i]
			isDefault := false
			if ic.Annotations != nil {
				v := strings.ToLower(strings.TrimSpace(ic.Annotations["ingressclass.kubernetes.io/is-default-class"]))
				isDefault = v == "true"
			}
			classes = append(classes, classOption{
				Name:       ic.Name,
				Controller: ic.Spec.Controller,
				Default:    isDefault,
			})
		}
		sort.Slice(classes, func(i, j int) bool {
			if classes[i].Default != classes[j].Default {
				return classes[i].Default
			}
			return classes[i].Name < classes[j].Name
		})
	}

	tlsSecrets := make([]tlsSecretOption, 0)
	if list, err := cli.CoreV1().Secrets(ns).List(ctx, metav1.ListOptions{Limit: 5000}); err == nil {
		for i := range list.Items {
			sec := &list.Items[i]
			if sec.Type != corev1.SecretTypeTLS {
				continue
			}
			tlsSecrets = append(tlsSecrets, tlsSecretOption{Name: sec.Name})
		}
		sort.Slice(tlsSecrets, func(i, j int) bool {
			return tlsSecrets[i].Name < tlsSecrets[j].Name
		})
	}

	services := make([]serviceOption, 0)
	if list, err := cli.CoreV1().Services(ns).List(ctx, metav1.ListOptions{Limit: 5000}); err == nil {
		for i := range list.Items {
			svc := &list.Items[i]
			ports := make([]servicePortOption, 0, len(svc.Spec.Ports))
			for _, p := range svc.Spec.Ports {
				ports = append(ports, servicePortOption{
					Name:     p.Name,
					Port:     p.Port,
					Protocol: string(p.Protocol),
					Target:   p.TargetPort.IntVal,
				})
			}
			services = append(services, serviceOption{
				Name:  svc.Name,
				Type:  string(svc.Spec.Type),
				Ports: ports,
			})
		}
		sort.Slice(services, func(i, j int) bool {
			return services[i].Name < services[j].Name
		})
	}

	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"namespace":   ns,
			"classes":     classes,
			"tls_secrets": tlsSecrets,
			"services":    services,
		},
	}))
}
