package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/modules/kubernetes/kubecli"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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
	list := api.Group("/persistentvolumeclaims/list")
	list.Get("/", cc.ListPVCAPI)

	single := api.Group("/persistentvolumeclaims/single")
	single.Post("/", cc.CreatePVCAPI)
	single.Get("/:namespace/:name", cc.GetPVCAPI)
	single.Get("/:namespace/:name/yaml", cc.PVCYAMLAPI)
	single.Put("/:namespace/:name/yaml", cc.ApplyPVCYAMLAPI)
	single.Delete("/:namespace/:name", cc.DeletePVCAPI)

	sc := api.Group("/storageclasses/list")
	sc.Get("/", cc.ListStorageClassesAPI)
}

func (cc *controller) respondErr(c fiber.Ctx, err error) error {
	r := cc.app.Render()
	return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("KUBERNETES_ERROR"))
}

type pvcRow struct {
	Namespace    string `json:"namespace"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	Volume       string `json:"volume,omitempty"`
	Capacity     string `json:"capacity,omitempty"`
	AccessModes  string `json:"access_modes"`
	StorageClass string `json:"storage_class,omitempty"`
	CreatedAt    string `json:"created_at"`
}

func accessModesString(modes []corev1.PersistentVolumeAccessMode) string {
	parts := make([]string, 0, len(modes))
	for _, m := range modes {
		parts = append(parts, string(m))
	}
	return strings.Join(parts, ", ")
}

func (cc *controller) ListPVCAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ns := kubecli.NamespaceQuery(c)
	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()
	list, err := cli.CoreV1().PersistentVolumeClaims(ns).List(ctx, metav1.ListOptions{Limit: 5000})
	if err != nil {
		return cc.respondErr(c, err)
	}
	rows := make([]pvcRow, 0, len(list.Items))
	for _, p := range list.Items {
		capStr := ""
		if q, ok := p.Status.Capacity[corev1.ResourceStorage]; ok {
			capStr = q.String()
		} else if q, ok := p.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
			capStr = q.String()
		}
		sc := ""
		if p.Spec.StorageClassName != nil {
			sc = *p.Spec.StorageClassName
		}
		rows = append(rows, pvcRow{
			Namespace:    p.Namespace,
			Name:         p.Name,
			Status:       string(p.Status.Phase),
			Volume:       p.Spec.VolumeName,
			Capacity:     capStr,
			AccessModes:  accessModesString(p.Spec.AccessModes),
			StorageClass: sc,
			CreatedAt:    p.CreationTimestamp.UTC().Format(time.RFC3339),
		})
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": rows}))
}

type createPVCBody struct {
	Namespace    string   `json:"namespace"`
	Name         string   `json:"name"`
	Storage      string   `json:"storage"`
	AccessModes  []string `json:"access_modes"`
	StorageClass string   `json:"storage_class"`
	VolumeMode   string   `json:"volume_mode"`
}

func (cc *controller) CreatePVCAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	var body createPVCBody
	if err := c.Bind().Body(&body); err != nil {
		return cc.respondErr(c, fmt.Errorf("invalid body"))
	}
	ns := strings.TrimSpace(body.Namespace)
	name := strings.TrimSpace(body.Name)
	storage := strings.TrimSpace(body.Storage)
	if ns == "" {
		ns = "default"
	}
	if name == "" {
		return cc.respondErr(c, fmt.Errorf("name is required"))
	}
	if storage == "" {
		storage = "1Gi"
	}
	qty, err := resource.ParseQuantity(storage)
	if err != nil {
		return cc.respondErr(c, fmt.Errorf("invalid storage size: %w", err))
	}

	modes := make([]corev1.PersistentVolumeAccessMode, 0, len(body.AccessModes))
	for _, m := range body.AccessModes {
		switch strings.TrimSpace(m) {
		case string(corev1.ReadWriteOnce), "RWO":
			modes = append(modes, corev1.ReadWriteOnce)
		case string(corev1.ReadOnlyMany), "ROX":
			modes = append(modes, corev1.ReadOnlyMany)
		case string(corev1.ReadWriteMany), "RWX":
			modes = append(modes, corev1.ReadWriteMany)
		case string(corev1.ReadWriteOncePod), "RWOP":
			modes = append(modes, corev1.ReadWriteOncePod)
		}
	}
	if len(modes) == 0 {
		modes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: modes,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: qty,
				},
			},
		},
	}
	if sc := strings.TrimSpace(body.StorageClass); sc != "" {
		pvc.Spec.StorageClassName = &sc
	}
	switch strings.TrimSpace(body.VolumeMode) {
	case "Block":
		mode := corev1.PersistentVolumeBlock
		pvc.Spec.VolumeMode = &mode
	default:
		mode := corev1.PersistentVolumeFilesystem
		pvc.Spec.VolumeMode = &mode
	}

	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()
	created, err := cli.CoreV1().PersistentVolumeClaims(ns).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	capStr := ""
	if q, ok := created.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
		capStr = q.String()
	}
	scName := ""
	if created.Spec.StorageClassName != nil {
		scName = *created.Spec.StorageClassName
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": pvcRow{
			Namespace:    created.Namespace,
			Name:         created.Name,
			Status:       string(created.Status.Phase),
			Volume:       created.Spec.VolumeName,
			Capacity:     capStr,
			AccessModes:  accessModesString(created.Spec.AccessModes),
			StorageClass: scName,
			CreatedAt:    created.CreationTimestamp.UTC().Format(time.RFC3339),
		},
		"message": "PersistentVolumeClaim created",
	}))
}

type storageClassRow struct {
	Name                 string `json:"name"`
	Provisioner          string `json:"provisioner"`
	ReclaimPolicy        string `json:"reclaim_policy,omitempty"`
	VolumeBindingMode    string `json:"volume_binding_mode,omitempty"`
	AllowVolumeExpansion bool   `json:"allow_volume_expansion"`
	Default              bool   `json:"default"`
}

func (cc *controller) ListStorageClassesAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
	defer cancel()
	list, err := cli.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{Limit: 500})
	if err != nil {
		return cc.respondErr(c, err)
	}
	rows := make([]storageClassRow, 0, len(list.Items))
	for _, sc := range list.Items {
		row := storageClassRow{
			Name:        sc.Name,
			Provisioner: sc.Provisioner,
			Default:     sc.Annotations["storageclass.kubernetes.io/is-default-class"] == "true",
		}
		if sc.ReclaimPolicy != nil {
			row.ReclaimPolicy = string(*sc.ReclaimPolicy)
		}
		if sc.VolumeBindingMode != nil {
			row.VolumeBindingMode = string(*sc.VolumeBindingMode)
		}
		if sc.AllowVolumeExpansion != nil {
			row.AllowVolumeExpansion = *sc.AllowVolumeExpansion
		}
		rows = append(rows, row)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": rows}))
}

func (cc *controller) GetPVCAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
	defer cancel()
	p, err := cli.CoreV1().PersistentVolumeClaims(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return cc.respondErr(c, err)
	}
	req := ""
	if q, ok := p.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
		req = q.String()
	}
	capStr := ""
	if q, ok := p.Status.Capacity[corev1.ResourceStorage]; ok {
		capStr = q.String()
	}
	sc := ""
	if p.Spec.StorageClassName != nil {
		sc = *p.Spec.StorageClassName
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"namespace":     p.Namespace,
			"name":          p.Name,
			"status":        string(p.Status.Phase),
			"volume":        p.Spec.VolumeName,
			"capacity":      capStr,
			"request":       req,
			"access_modes":  p.Spec.AccessModes,
			"storage_class": sc,
			"volume_mode":   p.Spec.VolumeMode,
			"labels":        p.Labels,
			"annotations":   p.Annotations,
			"created_at":    p.CreationTimestamp.UTC().Format(time.RFC3339),
		},
	}))
}

func (cc *controller) PVCYAMLAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 15*time.Second)
	defer cancel()
	p, err := cli.CoreV1().PersistentVolumeClaims(ns).Get(ctx, name, metav1.GetOptions{})
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

func (cc *controller) ApplyPVCYAMLAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	var body applyYAMLBody
	if err := c.Bind().Body(&body); err != nil {
		return cc.respondErr(c, fmt.Errorf("invalid body"))
	}
	res, err := kubecli.ApplyYAML(cc.app, body.YAML, kubecli.ApplyExpect{
		APIVersion: "v1",
		Kind:       "PersistentVolumeClaim",
		Namespace:  ns,
		Name:       name,
	})
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    res,
		"message": "PersistentVolumeClaim YAML applied",
	}))
}

func (cc *controller) DeletePVCAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	ns := strings.TrimSpace(c.Params("namespace"))
	name := strings.TrimSpace(c.Params("name"))
	cli, _, err := kubecli.Client(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()
	if err := cli.CoreV1().PersistentVolumeClaims(ns).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    fiber.Map{"namespace": ns, "name": name},
		"message": "PersistentVolumeClaim deleted",
	}))
}
