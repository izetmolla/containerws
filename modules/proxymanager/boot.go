package proxymanager

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/models"
	pkg "github.com/izetmolla/containerws/packages/proxymanager"
	"github.com/izetmolla/containerws/packages/softwaresync"
)

var bootOnce sync.Once

// StartAsync checks that Proxy Manager is enabled, verifies engine components
// (starting Docker when needed), then applies the active engine config once at boot.
func StartAsync(app *config.AppClients) {
	if app == nil || app.DB() == nil {
		return
	}
	bootOnce.Do(func() {
		go runBoot(app)
	})
}

func runBoot(app *config.AppClients) {
	// Let migrations / softwaresync / other boot work settle.
	time.Sleep(3 * time.Second)

	db := app.DB()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	if !models.ModuleEnabled(db, models.OptionProxymanagerModuleEnabled) {
		log.Printf("proxysync: module disabled (PROXYMANAGER_MODULE_ENABLED=false) — skip")
		return
	}

	// Host nginx/traefik may be installed by softwaresync on first boot.
	waitCtx, waitCancel := context.WithTimeout(ctx, 90*time.Second)
	_ = softwaresync.WaitReady(waitCtx)
	waitCancel()

	settings, err := pkg.EnsureSettings(db)
	if err != nil {
		log.Printf("proxysync: ensure settings: %v", err)
		return
	}
	log.Printf(
		"proxysync: enabled engine=%s nginx_runtime=%s traefik_runtime=%s docker_net=%s",
		settings.ActiveEngine, settings.NginxRuntime, settings.TraefikRuntime, settings.DockerNetworkMode,
	)

	needsDocker := (settings.ActiveEngine == models.ProxyEngineNginx && settings.NginxRuntime == models.ProxyRuntimeDocker) ||
		(settings.ActiveEngine == models.ProxyEngineTraefik && settings.TraefikRuntime == models.ProxyRuntimeDocker)

	if needsDocker {
		log.Printf("proxysync: ensuring docker engine is running…")
		if err := pkg.EnsureDockerRunning(ctx); err != nil {
			log.Printf("proxysync: docker not ready: %v — skip apply", err)
			_ = pkg.ClearDirty(db, settings.ActiveEngine, fmt.Errorf("boot: docker not ready: %w", err))
			return
		}
		log.Printf("proxysync: docker ready")
	}

	runtime, err := pkg.DetectRuntime(ctx, db)
	if err != nil {
		log.Printf("proxysync: detect runtime: %v", err)
		return
	}
	check := pkg.CheckComponents(settings, runtime)
	if !check.Ready {
		log.Printf(
			"proxysync: components missing for engine=%s runtime=%s: %v — skip apply",
			check.Engine, check.Runtime, check.Missing,
		)
		_ = pkg.ClearDirty(db, settings.ActiveEngine, fmt.Errorf("boot: missing components: %s", strings.Join(check.Missing, ", ")))
		return
	}
	for _, d := range check.Details {
		log.Printf("proxysync: %s", d)
	}

	base := appBaseURL(app)
	log.Printf("proxysync: applying configuration (base=%s)…", base)
	res, err := pkg.Apply(ctx, db, pkg.ApplyOptions{AppBaseURL: base})
	if err != nil {
		log.Printf("proxysync: apply failed: %v", err)
		if res != nil && res.Log != "" {
			log.Printf("proxysync: apply log:\n%s", res.Log)
		}
		return
	}
	files := 0
	if res != nil {
		files = len(res.Files)
	}
	log.Printf("proxysync: apply success engine=%s files=%d", settings.ActiveEngine, files)
}

func appBaseURL(app *config.AppClients) string {
	cfg := app.ServerConfig()
	scheme := "http"
	port := "9000"
	if cfg != nil {
		if cfg.ENABLE_HTTPS {
			scheme = "https"
		}
		if cfg.PORT != "" {
			port = cfg.PORT
		}
	}
	return fmt.Sprintf("%s://127.0.0.1:%s", scheme, port)
}
