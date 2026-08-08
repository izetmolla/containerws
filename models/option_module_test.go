package models

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&Option{}, &User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestModuleEnabledDefaultsOffWhenMissing(t *testing.T) {
	db := testDB(t)
	if ModuleEnabled(db, OptionDockerModuleEnabled) {
		t.Fatal("expected missing module option to be off")
	}
}

func TestEnsureModuleSidebarDefaultsNewInstallOff(t *testing.T) {
	db := testDB(t)
	if err := EnsureModuleSidebarDefaults(db); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{
		OptionDockerModuleEnabled,
		OptionKubernetesModuleEnabled,
		OptionProxymanagerModuleEnabled,
	} {
		if ModuleEnabled(db, key) {
			t.Fatalf("%s should be off on new install", key)
		}
	}
	if err := EnsureModuleSidebarDefaults(db); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureModuleSidebarDefaultsLegacyKeepsOn(t *testing.T) {
	db := testDB(t)
	if err := db.Create(&User{Username: "admin"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := EnsureModuleSidebarDefaults(db); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{
		OptionDockerModuleEnabled,
		OptionKubernetesModuleEnabled,
		OptionProxymanagerModuleEnabled,
	} {
		if !ModuleEnabled(db, key) {
			t.Fatalf("%s should stay on for existing installs", key)
		}
	}
}
