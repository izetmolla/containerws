package fiberproxy

import (
	"context"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/proxy"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/packages/proxymanager"
	fiberpm "github.com/izetmolla/containerws/packages/proxymanager/fiber"
)

type controller struct {
	app *config.AppClients
}

func NewController(app *config.AppClients) *controller {
	return &controller{app: app}
}

// SetupRoutesView installs a catch-all Host-based reverse proxy when Fiber engine is active.
// It only handles requests whose Host matches a configured proxy host; otherwise Next().
func SetupRoutesView(app fiber.Router, appClients *config.AppClients) {
	cc := NewController(appClients)
	// Warm table from DB on boot (best-effort).
	go cc.warmTable()
	app.Use(cc.Middleware)
}

func (cc *controller) warmTable() {
	db := cc.app.DB()
	if db == nil {
		return
	}
	settings, err := proxymanager.EnsureSettings(db)
	if err != nil || settings.ActiveEngine != models.ProxyEngineFiber {
		return
	}
	snap, err := proxymanager.LoadSnapshot(context.Background(), db)
	if err != nil {
		return
	}
	cfg := cc.app.ServerConfig()
	base := "http://127.0.0.1"
	if cfg != nil && cfg.PORT != "" {
		base = "http://127.0.0.1:" + cfg.PORT
	}
	table := fiberpm.BuildTable(fiberpm.BuildInput{
		ActiveEngine: settings.ActiveEngine,
		Hosts:        snap.Hosts,
		Redirects:    snap.Redirects,
		AppBaseURL:   base,
	})
	fiberpm.Set(table)
}

// Middleware proxies matching Host/path requests via Fiber proxy.Do.
func (cc *controller) Middleware(ctx fiber.Ctx) error {
	table := fiberpm.Get()
	if !table.Active {
		return ctx.Next()
	}

	host := string(ctx.Request().Host())
	reqPath := string(ctx.Path())

	// Never hijack API / static / auth paths of this app when Host is the app itself
	// and no proxy host matches — Match returns nil and we Next().
	if redir := table.MatchRedirect(host, reqPath); redir != nil {
		to := redir.ToURL
		if redir.PreservePath {
			u, err := url.Parse(to)
			if err == nil {
				u.Path = strings.TrimRight(u.Path, "/") + reqPath
				u.RawQuery = string(ctx.Request().URI().QueryString())
				to = u.String()
			}
		}
		code := redir.StatusCode
		if code == 0 {
			code = fiber.StatusMovedPermanently
		}
		return ctx.Redirect().Status(code).To(to)
	}

	route := table.Match(host, reqPath)
	if route == nil {
		return ctx.Next()
	}

	target, err := route.TargetURL(reqPath, string(ctx.Request().URI().QueryString()))
	if err != nil || target == "" {
		return ctx.Status(fiber.StatusBadGateway).SendString("invalid upstream")
	}

	ctx.Request().Header.Set("X-Forwarded-Host", host)
	ctx.Request().Header.Set("X-Forwarded-Proto", ctx.Protocol())
	for k, v := range route.CustomHeaders {
		ctx.Request().Header.Set(k, v)
	}
	return proxy.Do(ctx, target)
}
