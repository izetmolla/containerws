package configapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/packages/kubeclient"
	"gorm.io/gorm"
)

// fileMeta is a registered kubeconfig (path always absolute; secret lives in SQLite).
type fileMeta struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Path    string `json:"path"`
	Secret  string `json:"-"`
	Managed bool   `json:"managed"`
}

type fileRow struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	Managed   bool   `json:"managed"`
	Exists    bool   `json:"exists"`
	Active    bool   `json:"active"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

func keyToMeta(k models.K8sKey) fileMeta {
	return fileMeta{
		ID:      k.ID,
		Name:    k.Name,
		Path:    k.Path,
		Secret:  k.Secret,
		Managed: kubeclient.IsManagedPath(k.Path),
	}
}

func loadRegistry(db *gorm.DB) ([]fileMeta, error) {
	var keys []models.K8sKey
	if err := db.Order("created_at asc").Find(&keys).Error; err != nil {
		return nil, err
	}
	out := make([]fileMeta, 0, len(keys))
	for _, k := range keys {
		if strings.TrimSpace(k.ID) == "" || strings.TrimSpace(k.Path) == "" {
			continue
		}
		if strings.TrimSpace(k.Name) == "" {
			k.Name = k.ID
		}
		out = append(out, keyToMeta(k))
	}
	return out, nil
}

func activeFileID(db *gorm.DB) (string, error) {
	id, _, err := models.GetOption(db, models.OptionKubeconfigActiveID)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(id), nil
}

func findFile(list []fileMeta, id string) (fileMeta, bool) {
	id = strings.TrimSpace(id)
	for _, f := range list {
		if f.ID == id {
			return f, true
		}
	}
	return fileMeta{}, false
}

func toRows(list []fileMeta, activeID string) []fileRow {
	out := make([]fileRow, 0, len(list))
	for _, f := range list {
		row := fileRow{
			ID:      f.ID,
			Name:    f.Name,
			Path:    f.Path,
			Managed: f.Managed || kubeclient.IsManagedPath(f.Path),
			Exists:  kubeclient.FileExists(f.Path) || strings.TrimSpace(f.Secret) != "",
			Active:  f.ID == activeID,
		}
		if st, err := os.Stat(f.Path); err == nil {
			row.UpdatedAt = st.ModTime().UTC().Format(time.RFC3339)
		} else if f.Secret != "" {
			row.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		}
		out = append(out, row)
	}
	return out
}

// syncSecretToPath writes the DB secret to path so client-go can load it.
func syncSecretToPath(path, secret string) error {
	path = strings.TrimSpace(path)
	secret = strings.TrimSpace(secret)
	if path == "" {
		return fmt.Errorf("path is required")
	}
	if secret == "" {
		return fmt.Errorf("secret is empty")
	}
	return kubeclient.WriteFile(path, []byte(secret))
}

// materializeFile ensures the kubeconfig file exists on disk from the DB secret.
func materializeFile(entry fileMeta) error {
	if kubeclient.FileExists(entry.Path) && strings.TrimSpace(entry.Secret) == "" {
		return nil
	}
	if strings.TrimSpace(entry.Secret) == "" {
		if kubeclient.FileExists(entry.Path) {
			return nil
		}
		return fmt.Errorf("kubeconfig secret missing for %s", entry.Name)
	}
	if kubeclient.IsManagedPath(entry.Path) {
		if err := kubeclient.EnsureManagedStore(); err != nil {
			return err
		}
	}
	// Host profile path: only rewrite when missing so we don't clobber live edits.
	if !kubeclient.IsManagedPath(entry.Path) && kubeclient.FileExists(entry.Path) {
		return nil
	}
	return syncSecretToPath(entry.Path, entry.Secret)
}

// migrateLegacyOptionsRegistry copies OptionKubeconfigFiles into k8s_keys once.
func migrateLegacyOptionsRegistry(db *gorm.DB) error {
	var count int64
	if err := db.Model(&models.K8sKey{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	raw, ok, err := models.GetOption(db, models.OptionKubeconfigFiles)
	if err != nil || !ok || strings.TrimSpace(raw) == "" {
		return err
	}
	type legacy struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Path    string `json:"path"`
		Managed bool   `json:"managed"`
	}
	var list []legacy
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		return nil // ignore corrupt legacy blob
	}
	for _, e := range list {
		e.ID = strings.TrimSpace(e.ID)
		e.Path = strings.TrimSpace(e.Path)
		e.Name = strings.TrimSpace(e.Name)
		if e.ID == "" || e.Path == "" {
			continue
		}
		if e.Name == "" {
			e.Name = e.ID
		}
		secret := ""
		if data, err := kubeclient.ReadFile(e.Path); err == nil {
			secret = string(data)
		}
		row := models.K8sKey{ID: e.ID, Name: e.Name, Path: e.Path, Secret: secret}
		if err := db.Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

// SeedMissingFromUserProfile inserts the host kubeconfig (/root/.kube/config or
// $KUBECONFIG) into k8s_keys when that secret is not yet stored.
func SeedMissingFromUserProfile(db *gorm.DB) error {
	if db == nil {
		return errors.New("database unavailable")
	}
	if err := migrateLegacyOptionsRegistry(db); err != nil {
		return err
	}

	path := kubeclient.DefaultPath()
	if !kubeclient.FileExists(path) {
		// Fall back to persisted path option when default is missing.
		if opt, ok, _ := models.GetOption(db, models.OptionKubeconfigPath); ok {
			path = kubeclient.ResolvePath(opt)
		}
	}
	if !kubeclient.FileExists(path) {
		return nil
	}

	var existing models.K8sKey
	err := db.Where("path = ?", path).First(&existing).Error
	if err == nil {
		// Backfill secret column if empty.
		if strings.TrimSpace(existing.Secret) == "" {
			data, readErr := kubeclient.ReadFile(path)
			if readErr == nil && len(data) > 0 {
				_ = db.Model(&existing).Update("secret", string(data)).Error
			}
		}
		return ensureActivePointsAtKey(db, existing)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	data, err := kubeclient.ReadFile(path)
	if err != nil {
		return nil
	}
	if err := kubeclient.ValidateContent(data); err != nil {
		return nil
	}

	row := models.K8sKey{
		ID:     uuid.New().String(),
		Name:   "Default",
		Path:   path,
		Secret: string(data),
	}
	if err := db.Create(&row).Error; err != nil {
		return err
	}
	return ensureActivePointsAtKey(db, row)
}

func ensureActivePointsAtKey(db *gorm.DB, key models.K8sKey) error {
	activeID, err := activeFileID(db)
	if err != nil {
		return err
	}
	if activeID != "" {
		var n int64
		if err := db.Model(&models.K8sKey{}).Where("id = ?", activeID).Count(&n).Error; err != nil {
			return err
		}
		if n > 0 {
			return nil
		}
	}
	_ = models.SetOption(db, models.OptionKubeconfigActiveID, key.ID)
	_ = models.SetOption(db, models.OptionKubeconfigPath, key.Path)
	return nil
}

// ensureSeededRegistry loads k8s_keys, seeds from the user kubeconfig when empty,
// and returns the active file id.
func ensureSeededRegistry(app *config.AppClients) ([]fileMeta, string, error) {
	db := app.DB()
	if err := SeedMissingFromUserProfile(db); err != nil {
		return nil, "", err
	}

	list, err := loadRegistry(db)
	if err != nil {
		return nil, "", err
	}
	activeID, err := activeFileID(db)
	if err != nil {
		return nil, "", err
	}

	if len(list) == 0 {
		return list, activeID, nil
	}

	if activeID == "" {
		path, _, _ := models.GetOption(db, models.OptionKubeconfigPath)
		path = kubeclient.ResolvePath(path)
		for _, f := range list {
			if f.Path == path {
				activeID = f.ID
				_ = models.SetOption(db, models.OptionKubeconfigActiveID, activeID)
				break
			}
		}
		if activeID == "" {
			activeID = list[0].ID
			_ = models.SetOption(db, models.OptionKubeconfigActiveID, activeID)
			_ = models.SetOption(db, models.OptionKubeconfigPath, list[0].Path)
		}
	}
	return list, activeID, nil
}

func activateFile(db *gorm.DB, entry fileMeta, contextName string) error {
	if err := materializeFile(entry); err != nil {
		return err
	}
	if err := models.SetOption(db, models.OptionKubeconfigActiveID, entry.ID); err != nil {
		return err
	}
	if err := models.SetOption(db, models.OptionKubeconfigPath, entry.Path); err != nil {
		return err
	}
	if err := models.SetOption(db, models.OptionKubeconfigContext, strings.TrimSpace(contextName)); err != nil {
		return err
	}
	kubeclient.Reset()
	return nil
}

func createManagedFile(db *gorm.DB, name, content string) (fileMeta, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return fileMeta{}, fmt.Errorf("name is required")
	}
	if err := kubeclient.ValidateContent([]byte(content)); err != nil {
		return fileMeta{}, err
	}
	if err := kubeclient.EnsureManagedStore(); err != nil {
		return fileMeta{}, err
	}
	id := uuid.New().String()
	path := kubeclient.ManagedFilePath(id)
	if err := kubeclient.WriteFile(path, []byte(content)); err != nil {
		return fileMeta{}, err
	}
	row := models.K8sKey{ID: id, Name: name, Path: path, Secret: content}
	if err := db.Create(&row).Error; err != nil {
		_ = os.Remove(path)
		return fileMeta{}, err
	}
	entry := keyToMeta(row)
	var count int64
	_ = db.Model(&models.K8sKey{}).Count(&count).Error
	if count == 1 {
		_ = activateFile(db, entry, "")
	}
	return entry, nil
}

func updateManagedContent(db *gorm.DB, id, name, content string) (fileMeta, error) {
	var row models.K8sKey
	if err := db.Where("id = ?", strings.TrimSpace(id)).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fileMeta{}, fmt.Errorf("kubeconfig %q not found", id)
		}
		return fileMeta{}, err
	}
	if strings.TrimSpace(name) != "" {
		row.Name = strings.TrimSpace(name)
	}
	if content != "" {
		if err := kubeclient.ValidateContent([]byte(content)); err != nil {
			return fileMeta{}, err
		}
		row.Secret = content
		if err := syncSecretToPath(row.Path, content); err != nil {
			return fileMeta{}, err
		}
	}
	if err := db.Save(&row).Error; err != nil {
		return fileMeta{}, err
	}
	entry := keyToMeta(row)
	activeID, _ := activeFileID(db)
	if activeID == entry.ID {
		kubeclient.Reset()
	}
	return entry, nil
}

func deleteFile(db *gorm.DB, id string) error {
	var row models.K8sKey
	if err := db.Where("id = ?", strings.TrimSpace(id)).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("kubeconfig %q not found", id)
		}
		return err
	}
	activeID, err := activeFileID(db)
	if err != nil {
		return err
	}
	if err := db.Delete(&row).Error; err != nil {
		return err
	}
	if kubeclient.IsManagedPath(row.Path) {
		_ = os.Remove(row.Path)
	}
	if activeID == row.ID {
		list, err := loadRegistry(db)
		if err != nil {
			return err
		}
		if len(list) > 0 {
			_ = activateFile(db, list[0], "")
		} else {
			_ = models.SetOption(db, models.OptionKubeconfigActiveID, "")
			_ = models.SetOption(db, models.OptionKubeconfigPath, kubeclient.DefaultPath())
			kubeclient.Reset()
		}
	}
	return nil
}

func readFileContent(entry fileMeta) (string, error) {
	if strings.TrimSpace(entry.Secret) != "" {
		return entry.Secret, nil
	}
	data, err := kubeclient.ReadFile(entry.Path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
