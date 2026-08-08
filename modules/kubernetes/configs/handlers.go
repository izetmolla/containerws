package configs

import (
	"context"
	"encoding/base64"
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
	api.Get("/configmaps", cc.ListConfigMapsAPI)
	api.Post("/configmaps", cc.CreateConfigMapAPI)
	api.Get("/configmaps/:namespace/:name", cc.GetConfigMapAPI)
	api.Put("/configmaps/:namespace/:name", cc.UpdateConfigMapAPI)
	api.Get("/configmaps/:namespace/:name/yaml", cc.ConfigMapYAMLAPI)
	api.Put("/configmaps/:namespace/:name/yaml", cc.ApplyConfigMapYAMLAPI)
	api.Delete("/configmaps/:namespace/:name", cc.DeleteConfigMapAPI)

	api.Get("/secrets", cc.ListSecretsAPI)
	api.Post("/secrets", cc.CreateSecretAPI)
	api.Get("/secrets/:namespace/:name", cc.GetSecretAPI)
	api.Put("/secrets/:namespace/:name", cc.UpdateSecretAPI)
	api.Get("/secrets/:namespace/:name/yaml", cc.SecretYAMLAPI)
	api.Put("/secrets/:namespace/:name/yaml", cc.ApplySecretYAMLAPI)
	api.Delete("/secrets/:namespace/:name", cc.DeleteSecretAPI)
}

func (cc *controller) respondErr(c fiber.Ctx, err error) error {
	r := cc.app.Render()
	return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("KUBERNETES_ERROR"))
}

type cmRow struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Keys      int    `json:"keys"`
	CreatedAt string `json:"created_at"`
}

type cmDetail struct {
	Namespace   string            `json:"namespace"`
	Name        string            `json:"name"`
	Keys        int               `json:"keys"`
	CreatedAt   string            `json:"created_at"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Data        map[string]string `json:"data"`
	BinaryKeys  []string          `json:"binary_keys,omitempty"`
}

func (cc *controller) ListConfigMapsAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ns := kubecli.NamespaceQuery(c)
	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()
	list, err := cli.CoreV1().ConfigMaps(ns).List(ctx, metav1.ListOptions{Limit: 5000})
	if err != nil {
		return cc.respondErr(c, err)
	}
	rows := make([]cmRow, 0, len(list.Items))
	for _, cm := range list.Items {
		rows = append(rows, cmRow{
			Namespace: cm.Namespace,
			Name:      cm.Name,
			Keys:      len(cm.Data) + len(cm.BinaryData),
			CreatedAt: cm.CreationTimestamp.UTC().Format(time.RFC3339),
		})
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": rows}))
}

func (cc *controller) GetConfigMapAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
	defer cancel()
	cm, err := cli.CoreV1().ConfigMaps(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	data := cm.Data
	if data == nil {
		data = map[string]string{}
	}
	binKeys := make([]string, 0, len(cm.BinaryData))
	for k := range cm.BinaryData {
		binKeys = append(binKeys, k)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": cmDetail{
			Namespace:   cm.Namespace,
			Name:        cm.Name,
			Keys:        len(cm.Data) + len(cm.BinaryData),
			CreatedAt:   cm.CreationTimestamp.UTC().Format(time.RFC3339),
			Labels:      cm.Labels,
			Annotations: cm.Annotations,
			Data:        data,
			BinaryKeys:  binKeys,
		},
	}))
}

type createCMBody struct {
	Namespace string            `json:"namespace"`
	Name      string            `json:"name"`
	Data      map[string]string `json:"data"`
}

func (cc *controller) CreateConfigMapAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	var body createCMBody
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
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()
	cm, err := cli.CoreV1().ConfigMaps(ns).Create(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Data:       body.Data,
	}, metav1.CreateOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    cmRow{Namespace: cm.Namespace, Name: cm.Name, Keys: len(cm.Data), CreatedAt: cm.CreationTimestamp.UTC().Format(time.RFC3339)},
		"message": "ConfigMap created",
	}))
}

type updateCMBody struct {
	Data map[string]string `json:"data"`
}

func (cc *controller) UpdateConfigMapAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	var body updateCMBody
	if err := c.Bind().Body(&body); err != nil {
		return cc.respondErr(c, fmt.Errorf("invalid body"))
	}
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()
	cm, err := cli.CoreV1().ConfigMaps(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	cm.Data = body.Data
	if cm.Data == nil {
		cm.Data = map[string]string{}
	}
	updated, err := cli.CoreV1().ConfigMaps(ns).Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": cmDetail{
			Namespace: updated.Namespace,
			Name:      updated.Name,
			Keys:      len(updated.Data) + len(updated.BinaryData),
			CreatedAt: updated.CreationTimestamp.UTC().Format(time.RFC3339),
			Data:      updated.Data,
		},
		"message": "ConfigMap updated",
	}))
}

func (cc *controller) ConfigMapYAMLAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
	defer cancel()
	cm, err := cli.CoreV1().ConfigMaps(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	y, err := kubecli.ToYAML(cm)
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

func (cc *controller) ApplyConfigMapYAMLAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	var body applyYAMLBody
	if err := c.Bind().Body(&body); err != nil {
		return cc.respondErr(c, fmt.Errorf("invalid body"))
	}
	res, err := kubecli.ApplyYAML(cc.app, body.YAML, kubecli.ApplyExpect{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		Namespace:  ns,
		Name:       name,
	})
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    res,
		"message": "ConfigMap YAML applied",
	}))
}

func (cc *controller) DeleteConfigMapAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()
	if err := cli.CoreV1().ConfigMaps(ns).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    fiber.Map{"namespace": ns, "name": name},
		"message": "ConfigMap deleted",
	}))
}

type secretRow struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Keys      int    `json:"keys"`
	CreatedAt string `json:"created_at"`
}

type secretDetail struct {
	Namespace   string            `json:"namespace"`
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Keys        int               `json:"keys"`
	CreatedAt   string            `json:"created_at"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	// Data is decoded UTF-8 text when possible; binary values are base64.
	Data map[string]string `json:"data"`
}

func (cc *controller) ListSecretsAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ns := kubecli.NamespaceQuery(c)
	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()
	list, err := cli.CoreV1().Secrets(ns).List(ctx, metav1.ListOptions{Limit: 5000})
	if err != nil {
		return cc.respondErr(c, err)
	}
	rows := make([]secretRow, 0, len(list.Items))
	for _, s := range list.Items {
		rows = append(rows, secretRow{
			Namespace: s.Namespace,
			Name:      s.Name,
			Type:      string(s.Type),
			Keys:      len(s.Data),
			CreatedAt: s.CreationTimestamp.UTC().Format(time.RFC3339),
		})
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": rows}))
}

func decodeSecretData(data map[string][]byte) map[string]string {
	out := map[string]string{}
	for k, v := range data {
		if isPrintableUTF8(v) {
			out[k] = string(v)
		} else {
			out[k] = base64.StdEncoding.EncodeToString(v)
		}
	}
	return out
}

// encodeSecretData converts UI values (decoded plaintext, or Base64 for binary) into Secret.Data bytes.
// Kubernetes persists Secret.Data Base64-encoded; callers should pass decoded text for normal secrets.
func encodeSecretData(data map[string]string) map[string][]byte {
	out := map[string][]byte{}
	for k, v := range data {
		trimmed := strings.TrimSpace(v)
		if decoded, err := base64.StdEncoding.DecodeString(trimmed); err == nil && len(decoded) > 0 && !isPrintableUTF8(decoded) {
			out[k] = decoded
			continue
		}
		out[k] = []byte(v)
	}
	return out
}

func isPrintableUTF8(b []byte) bool {
	s := string(b)
	for _, r := range s {
		if r == '\n' || r == '\r' || r == '\t' {
			continue
		}
		if r < 32 || r == 0xFFFD {
			return false
		}
	}
	return true
}

func (cc *controller) GetSecretAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
	defer cancel()
	s, err := cli.CoreV1().Secrets(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": secretDetail{
			Namespace:   s.Namespace,
			Name:        s.Name,
			Type:        string(s.Type),
			Keys:        len(s.Data),
			CreatedAt:   s.CreationTimestamp.UTC().Format(time.RFC3339),
			Labels:      s.Labels,
			Annotations: s.Annotations,
			Data:        decodeSecretData(s.Data),
		},
	}))
}

type createSecretBody struct {
	Namespace string            `json:"namespace"`
	Name      string            `json:"name"`
	Type      string            `json:"type"`
	Data      map[string]string `json:"data"`
}

func (cc *controller) CreateSecretAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	var body createSecretBody
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
	secType := corev1.SecretTypeOpaque
	if t := strings.TrimSpace(body.Type); t != "" {
		secType = corev1.SecretType(t)
	}
	raw := encodeSecretData(body.Data)
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()
	s, err := cli.CoreV1().Secrets(ns).Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Type:       secType,
		Data:       raw,
	}, metav1.CreateOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": secretRow{
			Namespace: s.Namespace,
			Name:      s.Name,
			Type:      string(s.Type),
			Keys:      len(s.Data),
			CreatedAt: s.CreationTimestamp.UTC().Format(time.RFC3339),
		},
		"message": "Secret created",
	}))
}

type updateSecretBody struct {
	Data map[string]string `json:"data"`
}

func (cc *controller) UpdateSecretAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	var body updateSecretBody
	if err := c.Bind().Body(&body); err != nil {
		return cc.respondErr(c, fmt.Errorf("invalid body"))
	}
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()
	s, err := cli.CoreV1().Secrets(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	s.Data = encodeSecretData(body.Data)
	updated, err := cli.CoreV1().Secrets(ns).Update(ctx, s, metav1.UpdateOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": secretDetail{
			Namespace: updated.Namespace,
			Name:      updated.Name,
			Type:      string(updated.Type),
			Keys:      len(updated.Data),
			CreatedAt: updated.CreationTimestamp.UTC().Format(time.RFC3339),
			Data:      decodeSecretData(updated.Data),
		},
		"message": "Secret updated",
	}))
}

func (cc *controller) SecretYAMLAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
	defer cancel()
	s, err := cli.CoreV1().Secrets(ns).Get(ctx, name, metav1.GetOptions{})
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

func (cc *controller) ApplySecretYAMLAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	var body applyYAMLBody
	if err := c.Bind().Body(&body); err != nil {
		return cc.respondErr(c, fmt.Errorf("invalid body"))
	}
	res, err := kubecli.ApplyYAML(cc.app, body.YAML, kubecli.ApplyExpect{
		APIVersion: "v1",
		Kind:       "Secret",
		Namespace:  ns,
		Name:       name,
	})
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    res,
		"message": "Secret YAML applied",
	}))
}

func (cc *controller) DeleteSecretAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()
	if err := cli.CoreV1().Secrets(ns).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    fiber.Map{"namespace": ns, "name": name},
		"message": "Secret deleted",
	}))
}
