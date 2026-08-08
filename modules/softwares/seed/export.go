package seed

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/izetmolla/containerws/packages/softwarepkg"
)

// ExportResult summarizes ExportRegistry.
type ExportResult struct {
	Root     string   `json:"root"`
	Packages []string `json:"packages"`
	Files    []string `json:"files"`
}

// ExportRegistry writes the seed (+ buildin) catalog into a cws-packages tree.
// Each software gets package.json, default/install.json (real seed scripts), and
// apt-family Hub-tag install.json copies when Docker Hub tags are reachable.
func ExportRegistry(ctx context.Context, root string) (*ExportResult, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, fmt.Errorf("output root is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(abs, "softwares"), 0o755); err != nil {
		return nil, err
	}

	hubTags, _ := softwarepkg.ListHubTags(ctx, &softwarepkg.ListHubTagsOptions{
		Image: softwarepkg.DefaultHubImage,
	})

	items := catalog()
	out := &ExportResult{Root: abs, Files: make([]string, 0), Packages: make([]string, 0, len(items))}
	index := loadOrNewIndex(abs)

	for _, item := range items {
		name := strings.TrimSpace(item.Software.Name)
		if name == "" {
			continue
		}
		slug := sanitizeName(name)
		ver := pickLatestVersion(item.Versions)
		if ver == nil {
			continue
		}
		applyLinuxAptTargets(ver)

		active := item.Software.IsActive
		meta := softwarepkg.PackageMeta{
			Name:           name,
			Details:        item.Software.Details,
			Category:       item.Software.Category,
			SubCategory:    item.Software.SubCategory,
			Tags:           item.Software.Tags,
			Icon:           item.Software.Icon,
			Image:          item.Software.Image,
			Color:          item.Software.Color,
			Order:          item.Software.Order,
			ServiceUnits:   item.Software.ServiceUnits,
			CanControl:     boolPtr(item.Software.CanControl),
			ControlBackend: item.Software.ControlBackend,
			StartCommand:   item.Software.StartCommand,
			RestartCommand: item.Software.RestartCommand,
			StopCommand:    item.Software.StopCommand,
			IsActive:       &active,
		}
		metaPath := filepath.Join(abs, "softwares", slug, "package.json")
		if err := writeJSON(metaPath, meta); err != nil {
			return nil, err
		}
		out.Files = append(out.Files, rel(abs, metaPath))

		latest := ver.IsLatest
		if !latest {
			latest = true
		}
		spec := softwarepkg.InstallSpec{
			Version:          ver.Version,
			IsLatest:         &latest,
			OS:               first(ver.OS, "linux"),
			DistroID:         ver.DistroID,
			DistroVersion:    ver.DistroVersion,
			Distro:           ver.Distro,
			Arch:             ver.Arch,
			Platform:         ver.Platform,
			PackageFamily:    first(ver.PackageFamily, "apt"),
			Kernel:           ver.Kernel,
			Virtualization:   ver.Virtualization,
			ContainerRuntime: ver.ContainerRuntime,
			CloudProvider:    ver.CloudProvider,
			InstallScript:    ver.InstallScript,
			UninstallScript:  ver.UninstallScript,
			UpgradeScript:    ver.UpgradeScript,
			CustomScript:     ver.CustomScript,
		}

		paths := []string{
			filepath.Join(abs, "softwares", slug, "default", "install.json"),
		}
		// Apt-oriented seeds also get family fallbacks used by Hub workspaces.
		if strings.EqualFold(spec.PackageFamily, "apt") || spec.PackageFamily == "" {
			paths = append(paths,
				filepath.Join(abs, "softwares", slug, "ubuntu", "any", "any", "install.json"),
				filepath.Join(abs, "softwares", slug, "debian", "any", "any", "install.json"),
				filepath.Join(abs, "softwares", slug, "kali", "any", "any", "install.json"),
			)
			for _, tag := range hubTags {
				if !tag.Workspace {
					continue
				}
				switch tag.DistroID {
				case "ubuntu", "debian", "kali":
					paths = append(paths, filepath.Join(
						abs, "softwares", slug, tag.DistroID, tag.DistroVersion, "any", "install.json",
					))
				}
			}
		}

		seenPath := map[string]struct{}{}
		for _, p := range paths {
			if _, ok := seenPath[p]; ok {
				continue
			}
			seenPath[p] = struct{}{}
			// Fill distro fields for concrete Hub paths.
			s := spec
			relParts := strings.Split(filepath.ToSlash(rel(abs, p)), "/")
			// softwares/{slug}/{distro}/{ver}/{arch}/install.json
			if len(relParts) >= 6 && relParts[len(relParts)-1] == "install.json" {
				s.DistroID = relParts[2]
				if relParts[2] != "default" {
					s.DistroVersion = relParts[3]
					if s.DistroVersion == "any" {
						s.DistroVersion = ""
					}
					s.Arch = relParts[4]
					if s.Arch == "any" {
						s.Arch = ""
					}
				}
			}
			if err := writeJSON(p, s); err != nil {
				return nil, err
			}
			out.Files = append(out.Files, rel(abs, p))
		}

		upsertIndex(&index, meta)
		out.Packages = append(out.Packages, name)
	}

	indexPath := filepath.Join(abs, "softwares", "index.json")
	if err := writeJSON(indexPath, index); err != nil {
		return nil, err
	}
	out.Files = append(out.Files, rel(abs, indexPath))
	return out, nil
}

func pickLatestVersion(versions []VersionMeta) *VersionMeta {
	if len(versions) == 0 {
		return nil
	}
	for i := range versions {
		if versions[i].IsLatest {
			return &versions[i]
		}
	}
	return &versions[0]
}

func sanitizeName(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, "..", "")
	return s
}

func loadOrNewIndex(root string) softwarepkg.CatalogIndex {
	path := filepath.Join(root, "softwares", "index.json")
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) == 0 {
		return softwarepkg.CatalogIndex{}
	}
	var idx softwarepkg.CatalogIndex
	if json.Unmarshal(raw, &idx) != nil {
		return softwarepkg.CatalogIndex{}
	}
	return idx
}

func upsertIndex(idx *softwarepkg.CatalogIndex, meta softwarepkg.PackageMeta) {
	for i := range idx.Softwares {
		if strings.EqualFold(strings.TrimSpace(idx.Softwares[i].Name), meta.Name) {
			idx.Softwares[i] = meta
			return
		}
	}
	idx.Softwares = append(idx.Softwares, meta)
}

func boolPtr(v bool) *bool { return &v }

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func rel(root, abs string) string {
	r, err := filepath.Rel(root, abs)
	if err != nil {
		return abs
	}
	return filepath.ToSlash(r)
}

func first(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// ExportRegistryWithTimeout is a convenience wrapper with a Hub API timeout.
func ExportRegistryWithTimeout(root string, timeout time.Duration) (*ExportResult, error) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return ExportRegistry(ctx, root)
}
