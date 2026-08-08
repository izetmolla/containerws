package general

import (
	"context"
	"errors"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/version"
	"gorm.io/gorm"
)

func (cc *Controller) GetGeneralDataAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	reqCtx := c.Context()

	// Frontend still sends ?module=…; treat it as the container name.
	containerName := c.Query("module", c.Query("container", ""))

	data, err := cc.GetGeneralData(c, reqCtx, containerName)
	if err != nil {
		return r.Api(c, r.WithError(err))
	}
	return r.Api(c, r.WithContext(reqCtx), r.WithData(data))
}

func (cc *Controller) GetGeneralData(c fiber.Ctx, reqCtx context.Context, containerName string) (map[string]any, error) {
	data := map[string]any{
		"current_user_id": "",
		"module":          map[string]any{},
		"modules":         []map[string]any{},
		"containers":      []map[string]any{},
		"container":       map[string]any{},
		"navigations":     []models.Navigation{},
		"app_version":     version.Version,
		"commit_sha":      version.CommitSHA,
	}

	auth := cc.app.Authorization()
	if auth == nil {
		return nil, errors.New("authorization not configured")
	}

	user, err := auth.User(c, reqCtx, true)
	if err != nil {
		return nil, err
	}
	data["current_user_id"] = user.UserID
	userRoles := cc.app.FreshUserRoles(reqCtx, user.UserID, user.Roles)

	// Containers list (exposed as modules for existing UI switcher).
	containers, err := cc.GetContainerList(reqCtx, userRoles)
	if err != nil {
		return nil, err
	}
	data["modules"] = containers
	data["containers"] = containers

	// Resolve current container — missing container is OK.
	container, found, err := cc.ResolveContainer(reqCtx, containerName)
	if err != nil {
		return nil, err
	}
	modulePayload := containerToModuleMap(container, found, containerName)
	data["module"] = modulePayload
	data["container"] = modulePayload

	navigations, err := cc.GetNavigationData(reqCtx, container.ID, found, userRoles)
	if err != nil {
		return nil, err
	}
	data["navigations"] = navigations

	return data, nil
}

func containerToModuleMap(c models.Container, found bool, fallbackName string) map[string]any {
	if !found {
		name := fallbackName
		if name == "" {
			name = "workspace"
		}
		return map[string]any{
			"id":          "",
			"name":        name,
			"title":       "Workspace",
			"icon":        "Container",
			"description": "",
			"roles":       []string{},
		}
	}
	title := c.Title
	if title == "" {
		title = c.Name
	}
	if title == "" {
		title = "Workspace"
	}
	return map[string]any{
		"id":                 c.ID,
		"name":               c.Name,
		"title":              title,
		"icon":               firstNonEmpty(c.Icon, "Container"),
		"description":        c.Description,
		"roles":              []string{},
		"is_master":          c.IsMaster,
		"machine_id":         c.MachineID,
		"hostname":           c.Hostname,
		"os":                 c.OS,
		"os_version":         c.OSVersion,
		"kernel":             c.Kernel,
		"platform":           c.Platform,
		"distro":             c.Distro,
		"distro_id":          c.DistroID,
		"distro_version":     c.DistroVersion,
		"arch":               c.Arch,
		"processor":          c.Processor,
		"cpu_model":          c.CPUModel,
		"cpu_cores":          c.CPUCores,
		"memory_total":       c.MemoryTotal,
		"memory_human":       c.MemoryHuman,
		"ips":                []string(c.IPs),
		"mac_addresses":      []string(c.MACAddresses),
		"primary_ip":         c.PrimaryIP,
		"type":               c.Type,
		"virtualization":     c.Virtualization,
		"hypervisor":         c.Hypervisor,
		"container_runtime":  c.ContainerRuntime,
		"cloud_provider":     c.CloudProvider,
		"is_containerized":   c.IsContainerized,
		"is_virtual_machine": c.IsVirtualMachine,
		"product_name":       c.ProductName,
		"sys_vendor":         c.SysVendor,
		"app_version":        c.AppVersion,
		"commit_sha":         c.CommitSHA,
		"booted_at":          c.BootedAt,
		"last_seen_at":       c.LastSeenAt,
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// ResolveContainer looks up by name, then master, then first active.
// found=false means no row — callers must still return static navigations.
func (cc *Controller) ResolveContainer(ctx context.Context, name string) (models.Container, bool, error) {
	db := cc.app.DB()
	if db == nil {
		return models.Container{}, false, nil
	}

	if name != "" {
		row, err := gorm.G[models.Container](db).
			Where("name = ? AND is_active = ?", name, true).
			First(ctx)
		if err == nil {
			return row, true, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Container{}, false, err
		}
	}

	row, err := gorm.G[models.Container](db).
		Where("is_master = ? AND is_active = ?", true, true).
		First(ctx)
	if err == nil {
		return row, true, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.Container{}, false, err
	}

	row, err = gorm.G[models.Container](db).
		Where("is_active = ?", true).
		Order("name ASC").
		First(ctx)
	if err == nil {
		return row, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.Container{}, false, nil
	}
	return models.Container{}, false, err
}
