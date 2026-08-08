package proxymanager_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/packages/proxymanager"
	"gorm.io/gorm"
)

func TestApplyFiberGeneratesRoutes(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.sqlite")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&models.ProxySettings{},
		&models.ProxyHost{},
		&models.ProxyLocation{},
		&models.ProxyCertificate{},
		&models.ProxyRedirect{},
		&models.ProxyApplyRun{},
	); err != nil {
		t.Fatal(err)
	}

	s := models.NewDefaultProxySettings()
	s.ConfigDir = filepath.Join(dir, "proxymanager")
	s.ActiveEngine = models.ProxyEngineFiber
	if err := db.Create(s).Error; err != nil {
		t.Fatal(err)
	}

	host := models.ProxyHost{
		Name:           "smoke",
		Domains:        "smoke.local",
		Enabled:        true,
		ListenScheme:   models.ProxySchemeHTTP,
		UpstreamType:   models.ProxyUpstreamURL,
		UpstreamTarget: "http://127.0.0.1:9",
	}
	host.Normalize()
	if err := db.Create(&host).Error; err != nil {
		t.Fatal(err)
	}

	res, err := proxymanager.Apply(context.Background(), db, proxymanager.ApplyOptions{
		AppBaseURL: "http://127.0.0.1:3000",
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Engine != models.ProxyEngineFiber {
		t.Fatalf("engine=%s", res.Engine)
	}
	if len(res.Files) == 0 {
		t.Fatal("expected generated files")
	}
	for _, f := range res.Files {
		if _, err := os.Stat(f); err != nil {
			t.Fatalf("missing file %s: %v", f, err)
		}
	}

	// nginx preview generation
	_ = db.Model(s).Update("active_engine", models.ProxyEngineNginx)
	_ = db.Model(s).Update("nginx_runtime", models.ProxyRuntimeDocker)
	res2, err := proxymanager.Apply(context.Background(), db, proxymanager.ApplyOptions{
		AppBaseURL:  "http://127.0.0.1:3000",
		PreviewOnly: true,
	})
	if err != nil {
		t.Fatalf("nginx preview: %v", err)
	}
	if len(res2.Files) == 0 {
		t.Fatal("expected nginx files")
	}
}
