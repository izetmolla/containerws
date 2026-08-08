package softwarepkg

import (
	"fmt"
	"log"
	"strings"

	"github.com/izetmolla/containerws/models"
	"gorm.io/gorm"
)

// DefaultRegistryURL is the public GitHub software package registry used when none is configured.
// https://github.com/izetmolla/containerwspkg
const DefaultRegistryURL = "https://github.com/izetmolla/containerwspkg"

// EnsureDefaultRegistry inserts the default GitHub registry if no matching row exists.
// Safe to call repeatedly (idempotent). Returns the default row when present/created.
func EnsureDefaultRegistry(db *gorm.DB) (*models.SoftwarePackage, error) {
	if db == nil {
		return nil, fmt.Errorf("database unavailable")
	}

	var rows []models.SoftwarePackage
	if err := db.Find(&rows).Error; err != nil {
		return nil, err
	}
	for i := range rows {
		if SameGitHubRepo(rows[i].PackageURL, DefaultRegistryURL) {
			return &rows[i], nil
		}
	}

	row := models.SoftwarePackage{
		PackageURL: DefaultRegistryURL,
	}
	if err := db.Create(&row).Error; err != nil {
		return nil, fmt.Errorf("create default registry: %w", err)
	}
	log.Printf("softwarepkg: seeded default registry %s (id=%s)", DefaultRegistryURL, row.ID)
	return &row, nil
}

// SameGitHubRepo reports whether two package URLs point at the same GitHub owner/repo.
func SameGitHubRepo(a, b string) bool {
	ao, ar, aerr := ParseGitHubRepo(a)
	bo, br, berr := ParseGitHubRepo(b)
	if aerr != nil || berr != nil {
		return normalizePackageURL(a) == normalizePackageURL(b)
	}
	return strings.EqualFold(ao, bo) && strings.EqualFold(ar, br)
}

func normalizePackageURL(u string) string {
	u = strings.TrimSpace(strings.ToLower(u))
	u = strings.TrimSuffix(u, "/")
	u = strings.TrimSuffix(u, ".git")
	return u
}
