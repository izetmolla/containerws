package proxymanager

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/izetmolla/containerws/models"
	fiberproxy "github.com/izetmolla/containerws/packages/proxymanager/fiber"
	"github.com/izetmolla/containerws/packages/proxymanager/nginx"
	"github.com/izetmolla/containerws/packages/proxymanager/traefik"
	"gorm.io/gorm"
)

// ApplyOptions controls Apply behavior.
type ApplyOptions struct {
	AppBaseURL string // used to resolve app_path upstreams
	PreviewOnly bool  // generate configs only, do not reload runtimes
}

// ApplyResult is returned to API handlers.
type ApplyResult struct {
	Run     *models.ProxyApplyRun `json:"run"`
	Files   []string              `json:"files"`
	Engine  string                `json:"engine"`
	Preview bool                  `json:"preview"`
	Log     string                `json:"log"`
}

// Apply generates configs for the active engine and reloads that runtime.
func Apply(ctx context.Context, db *gorm.DB, opts ApplyOptions) (*ApplyResult, error) {
	snap, err := LoadSnapshot(ctx, db)
	if err != nil {
		return nil, err
	}
	if err := ValidateSnapshot(snap); err != nil {
		return nil, fmt.Errorf("validation: %w", err)
	}
	if err := EnsureConfigRoot(snap.Settings); err != nil {
		return nil, err
	}

	engine := snap.Settings.ActiveEngine
	run := &models.ProxyApplyRun{
		Engine:    engine,
		Status:    models.ProxyApplyRunning,
		StartedAt: time.Now().UTC(),
		FilesJSON: models.JSONBArray{},
	}
	if err := db.Create(run).Error; err != nil {
		return nil, err
	}

	var log strings.Builder
	AppendLog(&log, "apply start engine=%s preview=%v", engine, opts.PreviewOnly)

	var files []string
	var applyErr error

	switch engine {
	case models.ProxyEngineFiber:
		var table *fiberproxy.Table
		files, table, applyErr = GenerateFiber(snap, opts.AppBaseURL)
		if applyErr == nil && !opts.PreviewOnly && table != nil {
			fiberproxy.Set(table)
			AppendLog(&log, "fiber route table active=%v routes=%d redirects=%d", table.Active, len(table.Routes), len(table.Redirects))
		}
	case models.ProxyEngineNginx:
		var dir string
		dir, applyErr = ConfigDirFor(snap.Settings, models.ProxyEngineNginx)
		if applyErr == nil {
			files, applyErr = nginx.Generate(snap.Settings, snap.Hosts, snap.Redirects, snap.Certificates, dir, opts.AppBaseURL)
		}
		if applyErr == nil {
			AppendLog(&log, "nginx config generated files=%d", len(files))
		}
		if applyErr == nil && !opts.PreviewOnly {
			switch snap.Settings.NginxRuntime {
			case models.ProxyRuntimeHost:
				applyErr = nginx.ApplyHost(ctx, snap.Settings, dir)
			default:
				applyErr = nginx.ApplyDocker(ctx, db, snap.Settings, dir)
			}
			if applyErr == nil {
				fiberproxy.Clear()
				AppendLog(&log, "nginx runtime applied (%s)", snap.Settings.NginxRuntime)
			}
		}
	case models.ProxyEngineTraefik:
		var dir string
		dir, applyErr = ConfigDirFor(snap.Settings, models.ProxyEngineTraefik)
		if applyErr == nil {
			files, applyErr = traefik.Generate(snap.Settings, snap.Hosts, snap.Redirects, snap.Certificates, dir, opts.AppBaseURL)
		}
		if applyErr == nil {
			AppendLog(&log, "traefik config generated files=%d", len(files))
		}
		if applyErr == nil && !opts.PreviewOnly {
			switch snap.Settings.TraefikRuntime {
			case models.ProxyRuntimeHost:
				applyErr = traefik.ApplyHost(ctx, snap.Settings, dir)
			default:
				applyErr = traefik.ApplyDocker(ctx, db, snap.Settings, dir)
			}
			if applyErr == nil {
				fiberproxy.Clear()
				AppendLog(&log, "traefik runtime applied (%s)", snap.Settings.TraefikRuntime)
			}
		}
	default:
		applyErr = fmt.Errorf("unsupported engine %q", engine)
	}

	now := time.Now().UTC()
	run.FinishedAt = &now
	run.LogText = log.String()
	fileList := make(models.JSONBArray, 0, len(files))
	for _, f := range files {
		fileList = append(fileList, f)
	}
	run.FilesJSON = fileList

	if applyErr != nil {
		run.Status = models.ProxyApplyFailed
		run.ErrorText = applyErr.Error()
		AppendLog(&log, "ERROR: %v", applyErr)
		run.LogText = log.String()
		_ = db.Save(run).Error
		if !opts.PreviewOnly {
			_ = ClearDirty(db, engine, applyErr)
		}
		return &ApplyResult{Run: run, Files: files, Engine: engine, Preview: opts.PreviewOnly, Log: run.LogText}, applyErr
	}

	run.Status = models.ProxyApplySuccess
	_ = db.Save(run).Error
	if !opts.PreviewOnly {
		_ = ClearDirty(db, engine, nil)
	}
	AppendLog(&log, "apply success")
	run.LogText = log.String()
	_ = db.Save(run).Error

	return &ApplyResult{Run: run, Files: files, Engine: engine, Preview: opts.PreviewOnly, Log: run.LogText}, nil
}

// PreviewConfigs generates without reloading and returns file paths + contents map.
func PreviewConfigs(ctx context.Context, db *gorm.DB, appBaseURL string) (*ApplyResult, map[string]string, error) {
	res, err := Apply(ctx, db, ApplyOptions{AppBaseURL: appBaseURL, PreviewOnly: true})
	contents := map[string]string{}
	if res != nil {
		for _, f := range res.Files {
			if b, rerr := readFileLimited(f, 256*1024); rerr == nil {
				contents[f] = string(b)
			}
		}
	}
	return res, contents, err
}

func readFileLimited(path string, max int) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(b) > max {
		return b[:max], nil
	}
	return b, nil
}
