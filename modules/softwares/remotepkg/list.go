package remotepkg

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/packages/softwarepkg"
)

func (cc *controller) GetRegistriesListAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()

	if _, err := softwarepkg.EnsureDefaultRegistry(db); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	var rows []models.SoftwarePackage
	if err := db.WithContext(ctx).Order("created_at DESC").Find(&rows).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	q := strings.TrimSpace(c.Query("q"))
	out := make([]fiber.Map, 0, len(rows))
	for _, row := range rows {
		item := registryPublic(row)
		if q != "" {
			hay := strings.ToLower(row.PackageURL + " " + row.Username + " " + row.ID)
			if !strings.Contains(hay, strings.ToLower(q)) {
				continue
			}
		}
		// Best-effort remote catalog count (cached).
		if items, err := softwarepkg.ListRemoteFromPackage(ctx, row, "main", nil); err == nil {
			item["remote_count"] = len(items)
		} else {
			item["remote_count"] = 0
			item["catalog_error"] = err.Error()
		}
		item["is_default"] = softwarepkg.SameGitHubRepo(row.PackageURL, softwarepkg.DefaultRegistryURL)
		out = append(out, item)
	}

	return r.Api(c, r.WithContext(ctx), r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": out,
		"pagination": fiber.Map{
			"page":        1,
			"limit":       len(out),
			"total":       len(out),
			"total_pages": 1,
			"pageCount":   1,
		},
	}))
}

func (cc *controller) GetRemotePackagesAPI(c fiber.Ctx) error {
	ctx := c.Context()
	r := cc.app.Render()
	db := cc.app.DB()

	if _, err := softwarepkg.EnsureDefaultRegistry(db); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	q := strings.TrimSpace(c.Query("q"))
	packageID := strings.TrimSpace(c.Query("package_id"))

	var regs []models.SoftwarePackage
	query := db.WithContext(ctx).Order("created_at ASC")
	if packageID != "" {
		query = query.Where("id = ?", packageID)
	}
	if err := query.Find(&regs).Error; err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}

	type remoteItem struct {
		Name         string   `json:"name"`
		Details      string   `json:"details"`
		Category     string   `json:"category"`
		SubCategory  string   `json:"sub_category"`
		Tags         []string `json:"tags"`
		Icon         string   `json:"icon"`
		Image        string   `json:"image,omitempty"`
		Color        string   `json:"color"`
		Order        int      `json:"order"`
		ServiceUnits []string `json:"service_units"`
		CanControl   *bool    `json:"can_control,omitempty"`
		ControlBackend string `json:"control_backend,omitempty"`
		StartCommand   string `json:"start_command,omitempty"`
		RestartCommand string `json:"restart_command,omitempty"`
		StopCommand    string `json:"stop_command,omitempty"`
		PackageID    string   `json:"package_id"`
		PackageURL   string   `json:"package_url"`
	}

	seen := map[string]struct{}{}
	out := make([]remoteItem, 0)
	for _, reg := range regs {
		if strings.TrimSpace(reg.PackageURL) == "" {
			continue
		}
		items, err := softwarepkg.ListRemoteFromPackage(ctx, reg, "main", nil)
		if err != nil {
			continue
		}
		for _, meta := range items {
			if q != "" && !softwarepkg.MatchQuery(meta, q) {
				continue
			}
			key := strings.ToLower(strings.TrimSpace(meta.Name))
			if key == "" {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, remoteItem{
				Name:           meta.Name,
				Details:        meta.Details,
				Category:       meta.Category,
				SubCategory:    meta.SubCategory,
				Tags:           meta.Tags,
				Icon:           meta.Icon,
				Image:          meta.Image,
				Color:          meta.Color,
				Order:          meta.Order,
				ServiceUnits:   meta.ServiceUnits,
				CanControl:     meta.CanControl,
				ControlBackend: meta.ControlBackend,
				StartCommand:   meta.StartCommand,
				RestartCommand: meta.RestartCommand,
				StopCommand:    meta.StopCommand,
				PackageID:      reg.ID,
				PackageURL:     reg.PackageURL,
			})
		}
	}

	return r.Api(c, r.WithContext(ctx), r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": out,
		"pagination": fiber.Map{
			"page":        1,
			"limit":       len(out),
			"total":       len(out),
			"total_pages": 1,
			"pageCount":   1,
		},
	}))
}

func registryPublic(row models.SoftwarePackage) fiber.Map {
	return fiber.Map{
		"id":           row.ID,
		"package_url":  row.PackageURL,
		"username":     row.Username,
		"has_token":    strings.TrimSpace(row.Token) != "",
		"has_password": row.Password != "",
		"created_at":   row.CreatedAt,
		"updated_at":   row.UpdatedAt,
	}
}
