package applications

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/modules/kubernetes/kubecli"
	"gorm.io/gorm"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
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
	single.Get("/:id", cc.GetAPI)
	single.Put("/:id", cc.UpdateAPI)
	single.Delete("/:id", cc.DeleteAPI)
	single.Get("/:id/resources", cc.ResourcesAPI)
	single.Post("/:id/apply", cc.ApplySavedAPI)
	single.Post("/:id/remove", cc.RemoveFromClusterAPI)
	single.Post("/:id/duplicate", cc.DuplicateAPI)
	single.Get("/:id/revisions", cc.ListRevisionsAPI)
	single.Get("/:id/revisions/:version", cc.GetRevisionAPI)
	single.Post("/:id/revisions/:version/restore", cc.RestoreRevisionAPI)

	api.Post("/validate", cc.ValidateAPI)
	api.Post("/apply", cc.ApplyAPI)
}

func (cc *controller) respondErr(c fiber.Ctx, err error) error {
	r := cc.app.Render()
	return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest), r.WithErrorCode("KUBERNETES_ERROR"))
}

func resourcesToJSONB(refs []kubecli.ManifestRef) models.JSONBArray {
	out := make(models.JSONBArray, 0, len(refs))
	for _, ref := range refs {
		item := map[string]any{
			"apiVersion": ref.APIVersion,
			"kind":       ref.Kind,
			"name":       ref.Name,
		}
		if ref.Namespace != "" {
			item["namespace"] = ref.Namespace
		}
		if ref.ClusterScoped {
			item["cluster_scoped"] = true
		}
		out = append(out, item)
	}
	return out
}

func refsFromJSONB(arr models.JSONBArray, fallbackNS string) []kubecli.ManifestRef {
	out := make([]kubecli.ManifestRef, 0, len(arr))
	for _, raw := range arr {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		ref := kubecli.ManifestRef{
			APIVersion: fmt.Sprint(m["apiVersion"]),
			Kind:       fmt.Sprint(m["kind"]),
			Name:       fmt.Sprint(m["name"]),
		}
		if ns, ok := m["namespace"].(string); ok {
			ref.Namespace = ns
		}
		if cs, ok := m["cluster_scoped"].(bool); ok {
			ref.ClusterScoped = cs
		}
		if !ref.ClusterScoped && ref.Namespace == "" {
			ref.Namespace = fallbackNS
		}
		if ref.APIVersion == "" || ref.Kind == "" || ref.Name == "" {
			continue
		}
		out = append(out, ref)
	}
	return out
}

type appRow struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Namespace     string `json:"namespace"`
	Version       int    `json:"version"`
	ResourceCount int    `json:"resource_count"`
	Status        string `json:"status"` // healthy | partial | missing | empty | unknown
	ReadyCount    int    `json:"ready_count"`
	MissingCount  int    `json:"missing_count"`
	UpdatedAt     string `json:"updated_at"`
	CreatedAt     string `json:"created_at"`
}

func toRow(a models.K8sApplication) appRow {
	ver := a.Version
	if ver <= 0 {
		ver = 1
	}
	return appRow{
		ID:            a.ID,
		Name:          a.Name,
		Namespace:     a.Namespace,
		Version:       ver,
		ResourceCount: len(a.Resources),
		Status:        "unknown",
		UpdatedAt:     a.UpdatedAt.UTC().Format(time.RFC3339),
		CreatedAt:     a.CreatedAt.UTC().Format(time.RFC3339),
	}
}

const maxAppRevisions = 50

func recordRevision(db *gorm.DB, app models.K8sApplication, source, note string) error {
	if db == nil || strings.TrimSpace(app.ID) == "" {
		return nil
	}
	ver := app.Version
	if ver <= 0 {
		ver = 1
	}
	rev := models.K8sApplicationRevision{
		ApplicationID: app.ID,
		Version:       ver,
		Name:          app.Name,
		Namespace:     app.Namespace,
		YAML:          app.YAML,
		Resources:     app.Resources,
		Source:        source,
		Note:          strings.TrimSpace(note),
	}
	if err := db.Create(&rev).Error; err != nil {
		return err
	}
	// Keep newest N revisions.
	var old []models.K8sApplicationRevision
	if err := db.Where("application_id = ?", app.ID).
		Order("version DESC").
		Offset(maxAppRevisions).
		Find(&old).Error; err == nil && len(old) > 0 {
		ids := make([]string, 0, len(old))
		for _, r := range old {
			ids = append(ids, r.ID)
		}
		_ = db.Where("id IN ?", ids).Delete(&models.K8sApplicationRevision{}).Error
	}
	return nil
}

// bumpRevisionIfChanged updates the app and records a YAML revision when content changes.
func bumpRevisionIfChanged(db *gorm.DB, existing *models.K8sApplication, next models.K8sApplication, source, note string) error {
	changed := existing.YAML != next.YAML ||
		existing.Namespace != next.Namespace ||
		existing.Name != next.Name
	existing.Name = next.Name
	existing.Namespace = next.Namespace
	existing.YAML = next.YAML
	existing.Resources = next.Resources

	if !changed && existing.Version > 0 {
		return db.Save(existing).Error
	}

	ver := existing.Version
	if ver <= 0 {
		ver = 1
	} else if changed {
		ver++
	}
	existing.Version = ver
	if err := db.Save(existing).Error; err != nil {
		return err
	}
	return recordRevision(db, *existing, source, note)
}

func (cc *controller) ListAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	db := cc.app.DB()
	if db == nil {
		return cc.respondErr(c, fmt.Errorf("database unavailable"))
	}
	var rows []models.K8sApplication
	if err := db.Order("updated_at DESC").Find(&rows).Error; err != nil {
		return cc.respondErr(c, err)
	}
	out := make([]appRow, 0, len(rows))
	withStatus := strings.EqualFold(c.Query("status", "1"), "1") ||
		strings.EqualFold(c.Query("status"), "true")
	var probe *liveProbe
	if withStatus {
		probe, _ = newLiveProbe(cc.app)
	}
	for _, a := range rows {
		row := toRow(a)
		if probe != nil {
			cc.fillStatus(&row, a, probe)
		} else if row.ResourceCount == 0 {
			row.Status = "empty"
		}
		out = append(out, row)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": out}))
}

func (cc *controller) GetAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	id := strings.TrimSpace(c.Params("id"))
	db := cc.app.DB()
	if db == nil {
		return cc.respondErr(c, fmt.Errorf("database unavailable"))
	}
	var app models.K8sApplication
	if err := db.Where("id = ?", id).First(&app).Error; err != nil {
		return cc.respondErr(c, fmt.Errorf("application not found"))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": app}))
}

type saveBody struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	YAML      string `json:"yaml"`
}

func (cc *controller) prepareSave(body saveBody) (models.K8sApplication, error) {
	name := strings.TrimSpace(body.Name)
	if name == "" {
		return models.K8sApplication{}, fmt.Errorf("name is required")
	}
	rewritten, ns, analysis, err := kubecli.EnsureUniformNamespace(body.YAML, body.Namespace)
	if err != nil {
		return models.K8sApplication{}, err
	}
	if ns == "" {
		return models.K8sApplication{}, fmt.Errorf("namespace is required")
	}
	app := models.K8sApplication{
		Name:      name,
		Namespace: ns,
		YAML:      rewritten,
		Resources: resourcesToJSONB(analysis.Resources),
	}
	return app, nil
}

func (cc *controller) CreateAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	db := cc.app.DB()
	if db == nil {
		return cc.respondErr(c, fmt.Errorf("database unavailable"))
	}
	var body saveBody
	if err := c.Bind().Body(&body); err != nil {
		return cc.respondErr(c, fmt.Errorf("invalid body"))
	}
	app, err := cc.prepareSave(body)
	if err != nil {
		return cc.respondErr(c, err)
	}
	app.Version = 1
	if err := db.Create(&app).Error; err != nil {
		return cc.respondErr(c, err)
	}
	_ = recordRevision(db, app, "create", "")
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    app,
		"message": "Application saved",
	}))
}

func (cc *controller) UpdateAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	id := strings.TrimSpace(c.Params("id"))
	db := cc.app.DB()
	if db == nil {
		return cc.respondErr(c, fmt.Errorf("database unavailable"))
	}
	var existing models.K8sApplication
	if err := db.Where("id = ?", id).First(&existing).Error; err != nil {
		return cc.respondErr(c, fmt.Errorf("application not found"))
	}
	var body saveBody
	if err := c.Bind().Body(&body); err != nil {
		return cc.respondErr(c, fmt.Errorf("invalid body"))
	}
	if strings.TrimSpace(body.Name) == "" {
		body.Name = existing.Name
	}
	prepared, err := cc.prepareSave(body)
	if err != nil {
		return cc.respondErr(c, err)
	}
	if err := bumpRevisionIfChanged(db, &existing, prepared, "save", ""); err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    existing,
		"message": "Application updated",
	}))
}

func (cc *controller) DeleteAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	id := strings.TrimSpace(c.Params("id"))
	db := cc.app.DB()
	if db == nil {
		return cc.respondErr(c, fmt.Errorf("database unavailable"))
	}
	res := db.Where("id = ?", id).Delete(&models.K8sApplication{})
	if res.Error != nil {
		return cc.respondErr(c, res.Error)
	}
	if res.RowsAffected == 0 {
		return cc.respondErr(c, fmt.Errorf("application not found"))
	}
	_ = db.Where("application_id = ?", id).Delete(&models.K8sApplicationRevision{}).Error
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    fiber.Map{"id": id},
		"message": "Application deleted",
	}))
}

type validateBody struct {
	YAML      string `json:"yaml"`
	Namespace string `json:"namespace"`
}

func (cc *controller) ValidateAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	var body validateBody
	if err := c.Bind().Body(&body); err != nil {
		return cc.respondErr(c, fmt.Errorf("invalid body"))
	}
	rewritten, ns, analysis, err := kubecli.EnsureUniformNamespace(body.YAML, body.Namespace)
	if err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"namespace": ns,
			"yaml":      rewritten,
			"resources": analysis.Resources,
			"valid":     true,
		},
	}))
}

type applyBody struct {
	YAML             string `json:"yaml"`
	DryRun           bool   `json:"dry_run"`
	DefaultNamespace string `json:"default_namespace"`
	Namespace        string `json:"namespace"` // preferred alias
	Name             string `json:"name"`      // optional — persist as application when set and not dry-run
	ID               string `json:"id"`        // optional — update existing
}

func (cc *controller) ApplyAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	var body applyBody
	if err := c.Bind().Body(&body); err != nil {
		return cc.respondErr(c, fmt.Errorf("invalid body"))
	}
	targetNS := strings.TrimSpace(body.Namespace)
	if targetNS == "" {
		targetNS = strings.TrimSpace(body.DefaultNamespace)
	}
	rewritten, ns, analysis, err := kubecli.EnsureUniformNamespace(body.YAML, targetNS)
	if err != nil {
		return cc.respondErr(c, err)
	}

	summary, err := kubecli.ApplyManifests(cc.app, rewritten, kubecli.ManifestApplyOptions{
		DefaultNamespace: ns,
		DryRun:           body.DryRun,
	})
	if err != nil {
		return cc.respondErr(c, err)
	}

	var saved *models.K8sApplication
	if !body.DryRun && summary.Failed == 0 {
		if name := strings.TrimSpace(body.Name); name != "" {
			db := cc.app.DB()
			if db != nil {
				next := models.K8sApplication{
					Name:      name,
					Namespace: ns,
					YAML:      rewritten,
					Resources: resourcesToJSONB(analysis.Resources),
				}
				if id := strings.TrimSpace(body.ID); id != "" {
					var existing models.K8sApplication
					if err := db.Where("id = ?", id).First(&existing).Error; err == nil {
						_ = bumpRevisionIfChanged(db, &existing, next, "apply", "")
						saved = &existing
					}
				}
				if saved == nil {
					next.Version = 1
					if err := db.Create(&next).Error; err == nil {
						_ = recordRevision(db, next, "apply", "")
						saved = &next
					}
				}
			}
		}
	}

	msg := fmt.Sprintf("Applied %d of %d resource(s)", summary.Applied, summary.Total)
	if body.DryRun {
		msg = fmt.Sprintf("Dry-run validated %d of %d resource(s)", summary.Applied, summary.Total)
	}
	if summary.Failed > 0 {
		msg = fmt.Sprintf("%s (%d failed)", msg, summary.Failed)
	}

	status := fiber.StatusOK
	if summary.Applied == 0 && summary.Failed > 0 {
		status = fiber.StatusBadRequest
	}

	data := fiber.Map{
		"summary":   summary,
		"namespace": ns,
		"yaml":      rewritten,
		"resources": analysis.Resources,
	}
	if saved != nil {
		data["application"] = saved
	}

	return r.Api(c, r.WithStatus(status), r.WithData(fiber.Map{
		"data":    data,
		"message": msg,
	}))
}

func (cc *controller) ApplySavedAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	id := strings.TrimSpace(c.Params("id"))
	db := cc.app.DB()
	if db == nil {
		return cc.respondErr(c, fmt.Errorf("database unavailable"))
	}
	var app models.K8sApplication
	if err := db.Where("id = ?", id).First(&app).Error; err != nil {
		return cc.respondErr(c, fmt.Errorf("application not found"))
	}

	var body struct {
		DryRun    bool   `json:"dry_run"`
		Namespace string `json:"namespace"`
		YAML      string `json:"yaml"`
	}
	_ = c.Bind().Body(&body)

	content := strings.TrimSpace(body.YAML)
	if content == "" {
		content = app.YAML
	}
	targetNS := strings.TrimSpace(body.Namespace)
	if targetNS == "" {
		targetNS = app.Namespace
	}

	rewritten, ns, analysis, err := kubecli.EnsureUniformNamespace(content, targetNS)
	if err != nil {
		return cc.respondErr(c, err)
	}

	summary, err := kubecli.ApplyManifests(cc.app, rewritten, kubecli.ManifestApplyOptions{
		DefaultNamespace: ns,
		DryRun:           body.DryRun,
	})
	if err != nil {
		return cc.respondErr(c, err)
	}

	if !body.DryRun && summary.Failed == 0 {
		next := models.K8sApplication{
			Name:      app.Name,
			Namespace: ns,
			YAML:      rewritten,
			Resources: resourcesToJSONB(analysis.Resources),
		}
		_ = bumpRevisionIfChanged(db, &app, next, "apply", "")
	}

	msg := fmt.Sprintf("Applied %d of %d resource(s)", summary.Applied, summary.Total)
	if body.DryRun {
		msg = fmt.Sprintf("Dry-run validated %d of %d resource(s)", summary.Applied, summary.Total)
	}
	if summary.Failed > 0 {
		msg = fmt.Sprintf("%s (%d failed)", msg, summary.Failed)
	}
	status := fiber.StatusOK
	if summary.Applied == 0 && summary.Failed > 0 {
		status = fiber.StatusBadRequest
	}
	return r.Api(c, r.WithStatus(status), r.WithData(fiber.Map{
		"data": fiber.Map{
			"summary":     summary,
			"namespace":   ns,
			"yaml":        rewritten,
			"resources":   analysis.Resources,
			"application": app,
		},
		"message": msg,
	}))
}

func (cc *controller) RemoveFromClusterAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	id := strings.TrimSpace(c.Params("id"))
	db := cc.app.DB()
	if db == nil {
		return cc.respondErr(c, fmt.Errorf("database unavailable"))
	}
	var app models.K8sApplication
	if err := db.Where("id = ?", id).First(&app).Error; err != nil {
		return cc.respondErr(c, fmt.Errorf("application not found"))
	}

	var body struct {
		AlsoDeleteStore bool `json:"also_delete_store"`
	}
	_ = c.Bind().Body(&body)

	summary, err := kubecli.DeleteManifests(cc.app, app.YAML, app.Namespace)
	if err != nil {
		// Fall back to stored refs when YAML is empty/unparseable.
		refs := refsFromJSONB(app.Resources, app.Namespace)
		if len(refs) == 0 {
			return cc.respondErr(c, err)
		}
		summary, err = kubecli.DeleteRefs(cc.app, refs, app.Namespace)
		if err != nil {
			return cc.respondErr(c, err)
		}
	}

	if body.AlsoDeleteStore && summary.Failed == 0 {
		_ = db.Where("application_id = ?", id).Delete(&models.K8sApplicationRevision{}).Error
		_ = db.Where("id = ?", id).Delete(&models.K8sApplication{}).Error
	}

	msg := fmt.Sprintf("Removed %d of %d resource(s) from cluster", summary.Applied, summary.Total)
	if summary.Failed > 0 {
		msg = fmt.Sprintf("%s (%d failed)", msg, summary.Failed)
	}
	status := fiber.StatusOK
	if summary.Applied == 0 && summary.Failed > 0 {
		status = fiber.StatusBadRequest
	}
	return r.Api(c, r.WithStatus(status), r.WithData(fiber.Map{
		"data": fiber.Map{
			"summary":     summary,
			"application": toRow(app),
			"deleted":     body.AlsoDeleteStore && summary.Failed == 0,
		},
		"message": msg,
	}))
}

func (cc *controller) DuplicateAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	id := strings.TrimSpace(c.Params("id"))
	db := cc.app.DB()
	if db == nil {
		return cc.respondErr(c, fmt.Errorf("database unavailable"))
	}
	var app models.K8sApplication
	if err := db.Where("id = ?", id).First(&app).Error; err != nil {
		return cc.respondErr(c, fmt.Errorf("application not found"))
	}

	var body struct {
		Name string `json:"name"`
	}
	_ = c.Bind().Body(&body)

	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = strings.TrimSpace(app.Name) + " (copy)"
	}
	dup := models.K8sApplication{
		Name:      name,
		Namespace: app.Namespace,
		YAML:      app.YAML,
		Resources: app.Resources,
		Version:   1,
	}
	if err := db.Create(&dup).Error; err != nil {
		return cc.respondErr(c, err)
	}
	_ = recordRevision(db, dup, "create", "duplicated from "+app.ID)
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    dup,
		"message": "Application duplicated",
	}))
}

type revisionRow struct {
	ID        string `json:"id"`
	Version   int    `json:"version"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Source    string `json:"source"`
	Note      string `json:"note,omitempty"`
	CreatedAt string `json:"created_at"`
	Current   bool   `json:"current"`
}

func (cc *controller) ListRevisionsAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	id := strings.TrimSpace(c.Params("id"))
	db := cc.app.DB()
	if db == nil {
		return cc.respondErr(c, fmt.Errorf("database unavailable"))
	}
	var app models.K8sApplication
	if err := db.Where("id = ?", id).First(&app).Error; err != nil {
		return cc.respondErr(c, fmt.Errorf("application not found"))
	}
	var revs []models.K8sApplicationRevision
	if err := db.Where("application_id = ?", id).Order("version DESC").Find(&revs).Error; err != nil {
		return cc.respondErr(c, err)
	}
	cur := app.Version
	if cur <= 0 {
		cur = 1
	}
	out := make([]revisionRow, 0, len(revs))
	for _, rev := range revs {
		out = append(out, revisionRow{
			ID:        rev.ID,
			Version:   rev.Version,
			Name:      rev.Name,
			Namespace: rev.Namespace,
			Source:    rev.Source,
			Note:      rev.Note,
			CreatedAt: rev.CreatedAt.UTC().Format(time.RFC3339),
			Current:   rev.Version == cur,
		})
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"application": toRow(app),
			"revisions":   out,
		},
	}))
}

func (cc *controller) GetRevisionAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	id := strings.TrimSpace(c.Params("id"))
	verStr := strings.TrimSpace(c.Params("version"))
	db := cc.app.DB()
	if db == nil {
		return cc.respondErr(c, fmt.Errorf("database unavailable"))
	}
	var ver int
	if _, err := fmt.Sscanf(verStr, "%d", &ver); err != nil || ver <= 0 {
		return cc.respondErr(c, fmt.Errorf("invalid version"))
	}
	var rev models.K8sApplicationRevision
	if err := db.Where("application_id = ? AND version = ?", id, ver).First(&rev).Error; err != nil {
		return cc.respondErr(c, fmt.Errorf("revision not found"))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{"data": rev}))
}

func (cc *controller) RestoreRevisionAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	id := strings.TrimSpace(c.Params("id"))
	verStr := strings.TrimSpace(c.Params("version"))
	db := cc.app.DB()
	if db == nil {
		return cc.respondErr(c, fmt.Errorf("database unavailable"))
	}
	var ver int
	if _, err := fmt.Sscanf(verStr, "%d", &ver); err != nil || ver <= 0 {
		return cc.respondErr(c, fmt.Errorf("invalid version"))
	}
	var app models.K8sApplication
	if err := db.Where("id = ?", id).First(&app).Error; err != nil {
		return cc.respondErr(c, fmt.Errorf("application not found"))
	}
	var rev models.K8sApplicationRevision
	if err := db.Where("application_id = ? AND version = ?", id, ver).First(&rev).Error; err != nil {
		return cc.respondErr(c, fmt.Errorf("revision not found"))
	}
	next := models.K8sApplication{
		Name:      rev.Name,
		Namespace: rev.Namespace,
		YAML:      rev.YAML,
		Resources: rev.Resources,
	}
	if err := bumpRevisionIfChanged(db, &app, next, "restore", fmt.Sprintf("restored from v%d", ver)); err != nil {
		return cc.respondErr(c, err)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data":    app,
		"message": fmt.Sprintf("Restored YAML from v%d as v%d", ver, app.Version),
	}))
}

type liveResource struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Namespace  string `json:"namespace,omitempty"`
	Name       string `json:"name"`
	Exists     bool   `json:"exists"`
	Status     string `json:"status,omitempty"`
	Ready      string `json:"ready,omitempty"`
	Error      string `json:"error,omitempty"`
}

func (cc *controller) ResourcesAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	id := strings.TrimSpace(c.Params("id"))
	db := cc.app.DB()
	if db == nil {
		return cc.respondErr(c, fmt.Errorf("database unavailable"))
	}
	var app models.K8sApplication
	if err := db.Where("id = ?", id).First(&app).Error; err != nil {
		return cc.respondErr(c, fmt.Errorf("application not found"))
	}

	refs := refsFromJSONB(app.Resources, app.Namespace)
	if len(refs) == 0 {
		analysis, err := kubecli.AnalyzeManifests(app.YAML)
		if err == nil {
			refs = analysis.Resources
			for i := range refs {
				if !refs[i].ClusterScoped && refs[i].Namespace == "" {
					refs[i].Namespace = app.Namespace
				}
			}
		}
	}

	live := make([]liveResource, 0, len(refs))
	restCfg, _, err := kubecli.RestConfig(cc.app)
	if err != nil {
		return cc.respondErr(c, err)
	}
	dyn, err := dynamic.NewForConfig(restCfg)
	if err != nil {
		return cc.respondErr(c, err)
	}
	disco, err := discovery.NewDiscoveryClientForConfig(restCfg)
	if err != nil {
		return cc.respondErr(c, err)
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(disco))
	ctx, cancel := context.WithTimeout(c.Context(), 25*time.Second)
	defer cancel()

	for _, ref := range refs {
		row := liveResource{
			APIVersion: ref.APIVersion,
			Kind:       ref.Kind,
			Name:       ref.Name,
			Namespace:  ref.Namespace,
		}
		gv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			row.Error = err.Error()
			live = append(live, row)
			continue
		}
		mapping, err := mapper.RESTMapping(gv.WithKind(ref.Kind).GroupKind(), gv.Version)
		if err != nil {
			row.Error = err.Error()
			live = append(live, row)
			continue
		}
		var ri dynamic.ResourceInterface
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			ns := ref.Namespace
			if ns == "" {
				ns = app.Namespace
			}
			row.Namespace = ns
			ri = dyn.Resource(mapping.Resource).Namespace(ns)
		} else {
			ri = dyn.Resource(mapping.Resource)
		}
		obj, err := ri.Get(ctx, ref.Name, metav1.GetOptions{})
		if err != nil {
			row.Exists = false
			row.Status = "Missing"
			row.Error = err.Error()
			live = append(live, row)
			continue
		}
		row.Exists = true
		row.Status = "Present"
		if phase, found, _ := unstructuredNestedString(obj.Object, "status", "phase"); found && phase != "" {
			row.Status = phase
		}
		if ready, found := readySummary(obj.Object, ref.Kind); found {
			row.Ready = ready
		}
		live = append(live, row)
	}

	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"application": toRow(app),
			"namespace":   app.Namespace,
			"resources":   live,
		},
	}))
}

type liveProbe struct {
	dyn    dynamic.Interface
	mapper *restmapper.DeferredDiscoveryRESTMapper
}

func newLiveProbe(app *config.AppClients) (*liveProbe, error) {
	restCfg, _, err := kubecli.RestConfig(app)
	if err != nil {
		return nil, err
	}
	dyn, err := dynamic.NewForConfig(restCfg)
	if err != nil {
		return nil, err
	}
	disco, err := discovery.NewDiscoveryClientForConfig(restCfg)
	if err != nil {
		return nil, err
	}
	return &liveProbe{
		dyn:    dyn,
		mapper: restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(disco)),
	}, nil
}

func (cc *controller) fillStatus(row *appRow, app models.K8sApplication, probe *liveProbe) {
	refs := refsFromJSONB(app.Resources, app.Namespace)
	if len(refs) == 0 {
		analysis, err := kubecli.AnalyzeManifests(app.YAML)
		if err == nil {
			refs = analysis.Resources
			for i := range refs {
				if !refs[i].ClusterScoped && refs[i].Namespace == "" {
					refs[i].Namespace = app.Namespace
				}
			}
		}
	}
	if len(refs) == 0 {
		row.Status = "empty"
		row.ResourceCount = 0
		return
	}
	row.ResourceCount = len(refs)
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	ready, missing := 0, 0
	for _, ref := range refs {
		gv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			missing++
			continue
		}
		mapping, err := probe.mapper.RESTMapping(gv.WithKind(ref.Kind).GroupKind(), gv.Version)
		if err != nil {
			missing++
			continue
		}
		var ri dynamic.ResourceInterface
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			ns := ref.Namespace
			if ns == "" {
				ns = app.Namespace
			}
			if ns == "" {
				missing++
				continue
			}
			ri = probe.dyn.Resource(mapping.Resource).Namespace(ns)
		} else {
			ri = probe.dyn.Resource(mapping.Resource)
		}
		_, err = ri.Get(ctx, ref.Name, metav1.GetOptions{})
		if err != nil {
			missing++
			continue
		}
		ready++
	}
	row.ReadyCount = ready
	row.MissingCount = missing
	switch {
	case ready == len(refs):
		row.Status = "healthy"
	case ready == 0:
		row.Status = "missing"
	default:
		row.Status = "partial"
	}
}

func unstructuredNestedString(obj map[string]any, fields ...string) (string, bool, error) {
	cur := any(obj)
	for _, f := range fields {
		m, ok := cur.(map[string]any)
		if !ok {
			return "", false, nil
		}
		cur, ok = m[f]
		if !ok {
			return "", false, nil
		}
	}
	s, ok := cur.(string)
	return s, ok, nil
}

func readySummary(obj map[string]any, kind string) (string, bool) {
	status, ok := obj["status"].(map[string]any)
	if !ok {
		return "", false
	}
	switch kind {
	case "Deployment", "StatefulSet", "ReplicaSet":
		ready := asInt32(status["readyReplicas"])
		desired := asInt32(status["replicas"])
		if spec, ok := obj["spec"].(map[string]any); ok {
			if v := asInt32(spec["replicas"]); v > 0 {
				desired = v
			}
		}
		return fmt.Sprintf("%d/%d", ready, desired), true
	case "DaemonSet":
		ready := asInt32(status["numberReady"])
		desired := asInt32(status["desiredNumberScheduled"])
		return fmt.Sprintf("%d/%d", ready, desired), true
	case "Pod":
		if phase, ok := status["phase"].(string); ok {
			return phase, true
		}
	}
	return "", false
}

func asInt32(v any) int32 {
	switch n := v.(type) {
	case int32:
		return n
	case int64:
		return int32(n)
	case float64:
		return int32(n)
	case int:
		return int32(n)
	default:
		return 0
	}
}
