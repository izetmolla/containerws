package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/modules/softwares/seed"
	"github.com/izetmolla/containerws/packages/softwaresync"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

var softwareInstallCmd = &cobra.Command{
	Use:   "install <id|name>",
	Short: "Install a software from the catalog",
	Long: `Run the install script for a software version.

Streams script output to the terminal. Marks the version installed on success.

Examples:
  cws software install Go
  cws software install "Docker Engine"
  cws software install xfce --version xfce-novnc-1
  cws software install node --dry-run
`,
	Args: cobra.ExactArgs(1),
	RunE: withStore(func(cmd *cobra.Command, args []string, appClients *config.AppClients) error {
		seed.SeedIfEmpty(appClients)

		versionFlag, _ := cmd.Flags().GetString("version")
		dryRun, _ := cmd.Flags().GetBool("dry-run")

		sw, err := findSoftware(appClients.DB(), args[0])
		if err != nil {
			return err
		}

		ver, err := pickSoftwareVersion(appClients.DB(), sw.ID, versionFlag)
		if err != nil {
			return err
		}
		if strings.TrimSpace(ver.InstallScript) == "" {
			return fmt.Errorf("software %q version %q has no install script", sw.Name, ver.Version)
		}

		fmt.Printf("Installing %s %s\n", sw.Name, ver.Version)
		if dryRun {
			fmt.Println("---- install script (dry-run) ----")
			fmt.Print(ver.InstallScript)
			if !strings.HasSuffix(ver.InstallScript, "\n") {
				fmt.Println()
			}
			fmt.Println("---- end ----")
			return nil
		}

		run := exec.Command("bash", "-lc", ver.InstallScript)
		run.Env = softwareInstallEnv()
		run.Dir = "/root"
		run.Stdout = os.Stdout
		run.Stderr = os.Stderr
		run.Stdin = os.Stdin

		if err := run.Run(); err != nil {
			return fmt.Errorf("install %s %s failed: %w", sw.Name, ver.Version, err)
		}

		if err := markSoftwareVersionInstalled(appClients.DB(), sw.ID, ver.ID); err != nil {
			return fmt.Errorf("install succeeded but failed to update DB: %w", err)
		}

		fmt.Printf("Installed %s %s\n", sw.Name, ver.Version)
		return nil
	}),
}

func findSoftware(db *gorm.DB, key string) (*models.Software, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, errors.New("empty software identifier")
	}

	var sw models.Software
	err := db.Where("id = ?", key).First(&sw).Error
	if err == nil {
		return &sw, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// Exact name (case-insensitive).
	err = db.Where("LOWER(name) = ?", strings.ToLower(key)).First(&sw).Error
	if err == nil {
		return &sw, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// Partial name match when unique.
	var matches []models.Software
	if err := db.Where("LOWER(name) LIKE ?", "%"+strings.ToLower(key)+"%").
		Find(&matches).Error; err != nil {
		return nil, err
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Order != matches[j].Order {
			return matches[i].Order < matches[j].Order
		}
		return matches[i].Name < matches[j].Name
	})
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("software not found: %s", key)
	case 1:
		return &matches[0], nil
	default:
		names := make([]string, 0, len(matches))
		for _, m := range matches {
			names = append(names, m.Name)
		}
		return nil, fmt.Errorf("ambiguous software %q; matches: %s", key, strings.Join(names, ", "))
	}
}

func pickSoftwareVersion(db *gorm.DB, softwareID, version string) (*models.SoftwareVersion, error) {
	version = strings.TrimSpace(version)
	var ver models.SoftwareVersion
	if version != "" {
		err := db.Where("software_id = ? AND version = ?", softwareID, version).First(&ver).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("version not found: %s", version)
			}
			return nil, err
		}
		return &ver, nil
	}

	err := db.Where("software_id = ?", softwareID).
		Order("is_latest DESC, created_at DESC").
		First(&ver).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("no version available to install")
		}
		return nil, err
	}
	return &ver, nil
}

func markSoftwareVersionInstalled(db *gorm.DB, softwareID, versionID string) error {
	if err := models.MarkSoftwareInstalled(db, softwareID, versionID); err != nil {
		return err
	}
	softwaresync.ClearOsMissing(softwareID, versionID)
	return nil
}

func softwareInstallEnv() []string {
	home := os.Getenv("HOME")
	if home == "" {
		home = "/root"
	}
	user := os.Getenv("USER")
	if user == "" {
		user = "root"
	}

	env := os.Environ()
	overrides := map[string]string{
		"HOME":       home,
		"USER":       user,
		"GOCACHE":    home + "/.cache/go-build",
		"GOMODCACHE": home + "/go/pkg/mod",
		"GOPATH":     home + "/go",
		"GOROOT":     "/usr/local/go",
	}
	seen := make(map[string]bool, len(overrides))
	out := make([]string, 0, len(env)+len(overrides))
	for _, kv := range env {
		key, _, ok := strings.Cut(kv, "=")
		if !ok {
			out = append(out, kv)
			continue
		}
		if val, replace := overrides[key]; replace {
			out = append(out, key+"="+val)
			seen[key] = true
			continue
		}
		out = append(out, kv)
	}
	for key, val := range overrides {
		if !seen[key] {
			out = append(out, key+"="+val)
		}
	}
	return out
}
