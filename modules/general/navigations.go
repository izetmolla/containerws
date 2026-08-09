package general

import (
	"context"
	"strings"

	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/packages/navigations"
	"gorm.io/gorm"
)

// GetNavigationData returns built-in static menu items, then any DB rows.
// If the container is missing, static items still return; DB rows without a
// container (or matching freestanding ones) are included when possible.
func (cc *Controller) GetNavigationData(ctx context.Context, containerID string, containerFound bool, userRoles []string) ([]models.Navigation, error) {
	out := make([]models.Navigation, 0, 8)
	db := cc.app.DB()
	dockerOn := models.ModuleEnabled(db, models.OptionDockerModuleEnabled)
	k8sOn := models.ModuleEnabled(db, models.OptionKubernetesModuleEnabled)
	proxyOn := models.ModuleEnabled(db, models.OptionProxymanagerModuleEnabled)
	brewOn := models.BrewModuleEnabled(db)

	for _, group := range navigations.BuiltIn() {
		if group.ID == "builtin-docker" && !dockerOn {
			continue
		}
		if group.ID == "builtin-kubernetes" && !k8sOn {
			continue
		}
		if group.ID == "builtin-proxymanager" && !proxyOn {
			continue
		}
		if group.ID == "builtin-brew" && !brewOn {
			continue
		}
		// Nested under Workspace children.
		if len(group.Children) > 0 {
			kids := make([]models.Navigation, 0, len(group.Children))
			for _, child := range group.Children {
				if child.ID == "builtin-docker" && !dockerOn {
					continue
				}
				if child.ID == "builtin-kubernetes" && !k8sOn {
					continue
				}
				if child.ID == "builtin-proxymanager" && !proxyOn {
					continue
				}
				if child.ID == "builtin-brew" && !brewOn {
					continue
				}
				kids = append(kids, child)
			}
			group.Children = kids
		}
		filtered := cc.filterNavigationTree(group, userRoles)
		if filtered == nil {
			continue
		}
		out = append(out, *filtered)
	}

	dbNavs, err := cc.loadDBNavigations(ctx, containerID, containerFound, userRoles)
	if err != nil {
		// Soft-fail DB nav load — static menu must remain available.
		return out, nil
	}
	out = append(out, dbNavs...)
	return out, nil
}

func (cc *Controller) loadDBNavigations(ctx context.Context, containerID string, containerFound bool, userRoles []string) ([]models.Navigation, error) {
	db := cc.app.DB()
	if db == nil {
		return nil, nil
	}

	q := gorm.G[models.Navigation](db).
		Where("is_active = ?", true).
		Where("(parent_id IS NULL OR parent_id = '')").
		Order("order_nr ASC")

	if containerFound && containerID != "" {
		q = q.Where("container_id = ?", containerID)
	} else {
		// No container: still show freestanding DB records.
		q = q.Where("(container_id IS NULL OR container_id = '')")
	}

	roots, err := q.Find(ctx)
	if err != nil {
		return nil, err
	}

	filtered := make([]models.Navigation, 0, len(roots))
	for i := range roots {
		if !cc.userCanAccessNavigation(roots[i].Roles, userRoles) {
			continue
		}
		children, err := cc.getNavigationChildren(ctx, roots[i].ContainerID, roots[i].ID, userRoles)
		if err != nil {
			return nil, err
		}
		roots[i].Children = children
		filtered = append(filtered, roots[i])
	}
	return filtered, nil
}

func (cc *Controller) getNavigationChildren(ctx context.Context, containerID, parentID string, userRoles []string) ([]models.Navigation, error) {
	db := cc.app.DB()
	if db == nil {
		return nil, nil
	}

	q := gorm.G[models.Navigation](db).
		Where("parent_id = ?", parentID).
		Where("is_active = ?", true).
		Order("order_nr ASC")
	if containerID != "" {
		q = q.Where("container_id = ?", containerID)
	}

	children, err := q.Find(ctx)
	if err != nil {
		return nil, err
	}

	filtered := make([]models.Navigation, 0, len(children))
	for i := range children {
		if !cc.userCanAccessNavigation(children[i].Roles, userRoles) {
			continue
		}
		nested, err := cc.getNavigationChildren(ctx, containerID, children[i].ID, userRoles)
		if err != nil {
			return nil, err
		}
		children[i].Children = nested
		filtered = append(filtered, children[i])
	}
	return filtered, nil
}

func (cc *Controller) filterNavigationTree(nav models.Navigation, userRoles []string) *models.Navigation {
	if !cc.userCanAccessNavigation(nav.Roles, userRoles) {
		return nil
	}
	if len(nav.Children) == 0 {
		cp := nav
		return &cp
	}
	kids := make([]models.Navigation, 0, len(nav.Children))
	for _, child := range nav.Children {
		if filtered := cc.filterNavigationTree(child, userRoles); filtered != nil {
			kids = append(kids, *filtered)
		}
	}
	nav.Children = kids
	return &nav
}

func (cc *Controller) userCanAccessNavigation(navRoles models.JSONBArray, userRoles []string) bool {
	if cc.userHasAdminRead(userRoles) {
		return true
	}
	return cc.userCanAccessByRoles(requiredRoleNames(navRoles), userRoles)
}

func (cc *Controller) userHasAdminRead(userRoles []string) bool {
	if len(userRoles) == 0 {
		return false
	}
	auth := cc.app.Authorization()
	if auth == nil {
		return false
	}
	hasRole, canRead, _ := auth.GetRole([]string{"admin"}, userRoles)
	return hasRole && canRead
}

// Empty required = public for any authenticated caller.
func (cc *Controller) userCanAccessByRoles(required []string, userRoles []string) bool {
	if len(required) == 0 {
		return true
	}
	if len(userRoles) == 0 {
		return false
	}
	auth := cc.app.Authorization()
	if auth == nil {
		return false
	}
	hasRole, _, _ := auth.GetRole(required, userRoles)
	return hasRole
}

func requiredRoleNames(required models.JSONBArray) []string {
	if len(required) == 0 {
		return nil
	}
	out := make([]string, 0, len(required))
	for _, r := range required {
		s, ok := r.(string)
		if !ok {
			continue
		}
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
