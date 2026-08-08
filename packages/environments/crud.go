package environments

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

func mapDBError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}

func isRecordNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

// CreateEnvironment inserts a row and notifies other pods.
func (m *Environments) CreateEnvironment(ctx context.Context, in CreateEnvironmentInput) (OsEnvironment, error) {
	db, err := m.db(ctx)
	if err != nil {
		return OsEnvironment{}, err
	}

	name, err := formatName(in.Name)
	if err != nil {
		return OsEnvironment{}, err
	}
	if err := validateCreateName(name); err != nil {
		return OsEnvironment{}, err
	}

	var count int64
	if err := db.Model(&OsEnvironment{}).Where("name = ?", name).Count(&count).Error; err != nil {
		return OsEnvironment{}, err
	}
	if count > 0 {
		return OsEnvironment{}, ErrNameConflict
	}

	row := OsEnvironment{
		Name:       name,
		Value:      in.Value,
		Group:      formatGroup(in.Group),
		ModuleID:   m.config.ModuleID(),
		Source:     OsEnvironmentSourceEnv,
		IsSecret:   in.IsSecret,
		IsDisabled: in.IsDisabled,
		IsTextarea: in.IsTextarea,
	}
	if err := validateEnvironmentRow(&row); err != nil {
		return OsEnvironment{}, err
	}

	if err := db.Create(&row).Error; err != nil {
		return OsEnvironment{}, fmt.Errorf("create environment: %w", err)
	}

	m.debug("crud", "created environment id=%s name=%s", row.ID, row.Name)
	return row, nil
}

// UpdateEnvironment updates an existing row and notifies other pods.
func (m *Environments) UpdateEnvironment(ctx context.Context, id string, in UpdateEnvironmentInput) (OsEnvironment, error) {
	db, err := m.db(ctx)
	if err != nil {
		return OsEnvironment{}, err
	}
	if id == "" {
		return OsEnvironment{}, ErrInvalidName
	}
	if in.isEmpty() {
		return m.GetEnvironment(ctx, id)
	}

	var row OsEnvironment
	if err := db.Where("id = ?", id).First(&row).Error; err != nil {
		return OsEnvironment{}, mapDBError(err)
	}

	if err := applyUpdate(&row, in); err != nil {
		return OsEnvironment{}, err
	}

	if in.Name != nil {
		exists, err := m.NameExists(ctx, row.Name, row.ID)
		if err != nil {
			return OsEnvironment{}, err
		}
		if exists {
			return OsEnvironment{}, ErrNameConflict
		}
	}

	if err := db.Save(&row).Error; err != nil {
		return OsEnvironment{}, fmt.Errorf("update environment: %w", err)
	}

	m.debug("crud", "updated environment id=%s name=%s", row.ID, row.Name)
	return row, nil
}

// DeleteEnvironment soft-deletes a row and notifies other pods.
func (m *Environments) DeleteEnvironment(ctx context.Context, id string) error {
	db, err := m.db(ctx)
	if err != nil {
		return err
	}
	if id == "" {
		return ErrInvalidName
	}

	var row OsEnvironment
	if err := db.Where("id = ?", id).First(&row).Error; err != nil {
		return mapDBError(err)
	}
	if row.IsCore || IsCoreName(row.Name) {
		return ErrCoreNotDeletable
	}

	res := db.Delete(&OsEnvironment{}, "id = ?", id)
	if res.Error != nil {
		return fmt.Errorf("delete environment: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}

	m.debug("crud", "deleted environment id=%s", id)
	return nil
}

// GetEnvironment returns a row by id.
func (m *Environments) GetEnvironment(ctx context.Context, id string) (OsEnvironment, error) {
	db, err := m.db(ctx)
	if err != nil {
		return OsEnvironment{}, err
	}
	if id == "" {
		return OsEnvironment{}, ErrInvalidName
	}

	var row OsEnvironment
	if err := db.Where("id = ?", id).First(&row).Error; err != nil {
		return OsEnvironment{}, mapDBError(err)
	}
	return row, nil
}

// ListEnvironments returns all rows, optionally filtered by group.
func (m *Environments) ListEnvironments(ctx context.Context, groupFilter string) ([]OsEnvironment, []string, error) {
	db, err := m.db(ctx)
	if err != nil {
		return nil, nil, err
	}

	var rows []OsEnvironment
	if err := db.Order("name ASC").Find(&rows).Error; err != nil {
		return nil, nil, fmt.Errorf("list environments: %w", err)
	}

	groups := collectGroups(rows)
	if groupFilter == "" {
		return rows, groups, nil
	}

	filtered := make([]OsEnvironment, 0, len(rows))
	for _, row := range rows {
		if matchesGroupFilter(row.Group, groupFilter) {
			filtered = append(filtered, row)
		}
	}
	return filtered, groups, nil
}

const ungroupedGroupFilter = "__none__"

func matchesGroupFilter(group, filter string) bool {
	if filter == ungroupedGroupFilter {
		return formatGroup(group) == ""
	}
	return formatGroup(group) == filter
}

func collectGroups(rows []OsEnvironment) []string {
	seen := make(map[string]struct{})
	groups := make([]string, 0)
	for _, row := range rows {
		group := formatGroup(row.Group)
		if group == "" {
			continue
		}
		if _, ok := seen[group]; ok {
			continue
		}
		seen[group] = struct{}{}
		groups = append(groups, group)
	}
	return groups
}

func (in UpdateEnvironmentInput) isEmpty() bool {
	return in.Name == nil &&
		in.Value == nil &&
		in.Group == nil &&
		in.IsSecret == nil &&
		in.IsDisabled == nil &&
		in.IsTextarea == nil
}

func applyUpdate(row *OsEnvironment, in UpdateEnvironmentInput) error {
	newName := row.Name
	if in.Name != nil {
		name, err := formatName(*in.Name)
		if err != nil {
			return err
		}
		if err := validateUpdateName(*row, name); err != nil {
			return err
		}
		newName = name
	}
	if in.Name != nil {
		row.Name = newName
	}
	if in.Value != nil {
		row.Value = *in.Value
	}
	if in.Group != nil {
		row.Group = formatGroup(*in.Group)
	}
	if in.IsSecret != nil {
		row.IsSecret = *in.IsSecret
	}
	if in.IsDisabled != nil {
		row.IsDisabled = *in.IsDisabled
	}
	if in.IsTextarea != nil {
		row.IsTextarea = *in.IsTextarea
	}
	return validateEnvironmentRow(row)
}

// UngroupedGroupFilter is the query token for rows without a group.
func UngroupedGroupFilter() string {
	return ungroupedGroupFilter
}

func (m *Environments) NameExists(ctx context.Context, name, excludeID string) (bool, error) {
	db, err := m.db(ctx)
	if err != nil {
		return false, err
	}
	name = normalizeName(name)
	query := db.Model(&OsEnvironment{}).Where("name = ?", name)
	if excludeID != "" {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
