package general

import (
	"context"

	"github.com/izetmolla/containerws/models"
	"gorm.io/gorm"
)

func (cc *Controller) GetContainerList(ctx context.Context, _ []string) ([]map[string]any, error) {
	db := cc.app.DB()
	if db == nil {
		return []map[string]any{}, nil
	}

	rows, err := gorm.G[models.Container](db).
		Where("is_active = ?", true).
		Order("is_master DESC, name ASC").
		Find(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, containerToModuleMap(row, true, row.Name))
	}
	return out, nil
}
