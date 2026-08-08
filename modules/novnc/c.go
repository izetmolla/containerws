package novnc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/proxy"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/models"
)

// Prefix mounts a full reverse-proxy of the user's active VncSession noVNC UI:
//
//	/novnc/vnc.html  →  http://{session.address}:{session.no_vnc_port}/vnc.html
//	/novnc/app/*     →  http://{session.address}:{session.no_vnc_port}/app/*
//	/novnc/websockify (WS) → same host:port
//	/novnc/mandatory.json → panel-injected password (skips VNC credential dialog)
const Prefix = "/novnc"

var upstreamHTTP = &http.Client{Timeout: 15 * time.Second}

type Controller struct {
	app *config.AppClients
}

func NewController(app *config.AppClients) *Controller {
	return &Controller{app: app}
}

// SetupRoutesView registers the /novnc reverse-proxy (HTTP assets + WebSocket).
func SetupRoutesView(app fiber.Router, appClients *config.AppClients) {
	cc := NewController(appClients)

	app.Get(Prefix, cc.GetVNCView)
	app.Get(Prefix+"/", cc.GetVNCView)
	app.All(Prefix+"/*", cc.Proxy)
}

// GetVNCView redirects to the proxied vnc.html with seed-quality query defaults.
func (cc *Controller) GetVNCView(ctx fiber.Ctx) error {
	session, err := cc.requireActiveSession(ctx)
	if err != nil {
		return cc.handleSessionError(ctx, err)
	}
	cc.rememberSessionCookie(ctx, session.ID)
	target := withAccessToken(session.ClientURL(), extractAccessToken(ctx))
	return ctx.Redirect().Status(fiber.StatusFound).To(target)
}

// Proxy reverse-proxies /novnc/* to the caller's active VncSession upstream.
func (cc *Controller) Proxy(ctx fiber.Ctx) error {
	session, err := cc.requireActiveSession(ctx)
	if err != nil {
		return cc.handleSessionError(ctx, err)
	}
	cc.rememberSessionCookie(ctx, session.ID)

	upstream := session.UpstreamBaseURL()
	upHost := session.UpstreamHostPort()
	reqPath := pathOnly(stripPrefix(string(ctx.Request().URI().PathOriginal())))

	// Panel auth already verified — inject RFB password via noVNC mandatory.json
	// so the credential dialog is never shown for reverse-proxied sessions.
	if isMandatoryJSON(reqPath) {
		return cc.serveMandatoryJSON(ctx, session)
	}

	if ctx.IsWebSocket() {
		return cc.proxyWebSocket(ctx, upHost)
	}

	// Soften the credentials UI on the main page (belt-and-suspenders with mandatory.json).
	if isVncHTML(reqPath) {
		return cc.proxyVncHTML(ctx, session, upstream, upHost)
	}

	target := upstream + stripAccessTokenQuery(stripPrefix(string(ctx.Request().RequestURI())))
	ctx.Request().SetHost(upHost)
	ctx.Request().Header.Set("X-Forwarded-Host", ctx.Hostname())
	ctx.Request().Header.Set("X-Forwarded-Proto", ctx.Protocol())
	ctx.Request().Header.Set("X-Forwarded-For", ctx.IP())
	ctx.Request().Header.Set("X-Real-IP", ctx.IP())
	return proxy.Do(ctx, target)
}

const sessionCookie = "cws_novnc_session"

// ClientURL is the in-app entry for the authenticated user's own session.
func ClientURL() string {
	return defaultClientSession("").ClientURL()
}

// ClientURLForSession includes session_id so operators can open a specific desktop.
// Uses default client options when the full session row is not loaded.
func ClientURLForSession(sessionID string) string {
	return defaultClientSession(sessionID).ClientURL()
}

func defaultClientSession(sessionID string) models.VncSession {
	s := models.VncSession{
		ID:             strings.TrimSpace(sessionID),
		Autoconnect:    true,
		Reconnect:      true,
		ReconnectDelay: models.VncDefaultReconnectDelay,
		Resize:         models.VncDefaultResize,
		Quality:        models.VncDefaultQuality,
		Compression:    models.VncDefaultCompression,
		Shared:         true,
		Bell:           "on",
		Logging:        "warn",
	}
	s.ApplyDefaults()
	return s
}

func (cc *Controller) rememberSessionCookie(ctx fiber.Ctx, sessionID string) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return
	}
	ctx.Cookie(&fiber.Cookie{
		Name:     sessionCookie,
		Value:    sessionID,
		Path:     Prefix,
		HTTPOnly: true,
		SameSite: "Lax",
		MaxAge:   60 * 60 * 12,
	})
}

func resolveSessionID(ctx fiber.Ctx) string {
	if raw := strings.TrimSpace(ctx.Query("session_id")); raw != "" {
		return raw
	}
	if raw := sessionIDFromReferer(string(ctx.Request().Header.Referer())); raw != "" {
		return raw
	}
	if raw := strings.TrimSpace(ctx.Cookies(sessionCookie)); raw != "" {
		return raw
	}
	return ""
}

func sessionIDFromReferer(referer string) string {
	referer = strings.TrimSpace(referer)
	if referer == "" {
		return ""
	}
	u, err := url.Parse(referer)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(u.Query().Get("session_id"))
}

func (cc *Controller) handleSessionError(ctx fiber.Ctx, err error) error {
	var gate *sessionGateError
	if errors.As(err, &gate) {
		if ctx.IsWebSocket() {
			status := fiber.StatusServiceUnavailable
			switch {
			case errors.Is(gate.kind, errUnauthorized):
				status = fiber.StatusUnauthorized
			case errors.Is(gate.kind, errNoSession), errors.Is(gate.kind, errNoProfile):
				status = fiber.StatusNotFound
			case errors.Is(gate.kind, errPackagesNotReady):
				status = fiber.StatusServiceUnavailable
			}
			return fiber.NewError(status, gate.Error())
		}
		return cc.renderGateError(ctx, gate)
	}

	switch {
	case errors.Is(err, errUnauthorized):
		if ctx.IsWebSocket() {
			return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
		}
		redirect := "/sign-in?redirectUrl=" + url.QueryEscape(ctx.OriginalURL())
		return ctx.Redirect().Status(fiber.StatusFound).To(redirect)
	case errors.Is(err, errNoSession),
		errors.Is(err, errNoProfile),
		errors.Is(err, errPackagesNotReady),
		errors.Is(err, errDesktopStopped):
		if ctx.IsWebSocket() {
			return fiber.NewError(fiber.StatusServiceUnavailable, err.Error())
		}
		return cc.renderGateError(ctx, &sessionGateError{
			kind:        err,
			title:       "VNC unavailable",
			message:     err.Error(),
			actionURL:   "/vnc-novnc",
			actionLabel: "Open VNC settings",
		})
	default:
		return fiber.NewError(fiber.StatusBadGateway, err.Error())
	}
}

func (cc *Controller) renderGateError(ctx fiber.Ctx, gate *sessionGateError) error {
	if gate == nil {
		gate = &sessionGateError{
			title:       "VNC unavailable",
			message:     "The remote desktop cannot be opened.",
			actionURL:   "/vnc-novnc",
			actionLabel: "Open VNC settings",
		}
	}
	actionURL := gate.actionURL
	if actionURL == "" {
		actionURL = "/vnc-novnc"
	}
	actionLabel := gate.actionLabel
	if actionLabel == "" {
		actionLabel = "Open VNC settings"
	}
	title := gate.title
	if title == "" {
		title = "VNC unavailable"
	}
	message := gate.message
	if message == "" {
		message = "The remote desktop cannot be opened."
	}

	status := fiber.StatusServiceUnavailable
	switch {
	case errors.Is(gate.kind, errNoSession), errors.Is(gate.kind, errNoProfile):
		status = fiber.StatusNotFound
	case errors.Is(gate.kind, errUnauthorized):
		status = fiber.StatusUnauthorized
	}

	body := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>%s</title>
  <style>
    body{font-family:system-ui,sans-serif;margin:0;min-height:100vh;display:grid;place-items:center;background:#0f1419;color:#e7ecf1}
    main{max-width:30rem;padding:2rem;text-align:center}
    h1{font-size:1.25rem;margin:0 0 .75rem}
    p{margin:0 0 1.25rem;line-height:1.5;color:#9aa7b5}
    .actions{display:flex;flex-wrap:wrap;gap:.75rem;justify-content:center}
    a{display:inline-block;padding:.65rem 1rem;border-radius:.5rem;background:#3b82f6;color:#fff;text-decoration:none;font-weight:600}
    a:hover{background:#2563eb}
    a.secondary{background:transparent;border:1px solid #334155;color:#e7ecf1;font-weight:500}
    a.secondary:hover{background:#1e293b}
  </style>
</head>
<body>
  <main>
    <h1>%s</h1>
    <p>%s</p>
    <div class="actions">
      <a href="%s">%s</a>
      <a class="secondary" href="/vnc-novnc">VNC settings</a>
    </div>
  </main>
</body>
</html>`,
		html.EscapeString(title),
		html.EscapeString(title),
		html.EscapeString(message),
		html.EscapeString(actionURL),
		html.EscapeString(actionLabel),
	)

	ctx.Set("Content-Type", "text/html; charset=utf-8")
	return ctx.Status(status).SendString(body)
}

// serveMandatoryJSON supplies noVNC mandatory settings for panel-authenticated
// reverse-proxy access. The RFB password is taken from the session record so
// the browser never prompts for VNC credentials.
func (cc *Controller) serveMandatoryJSON(ctx fiber.Ctx, session *models.VncSession) error {
	payload := map[string]any{
		"autoconnect": true,
		"reconnect":   true,
	}
	if pwd := rfbPassword(session.VncPassword); pwd != "" {
		payload["password"] = pwd
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "encode mandatory.json")
	}
	ctx.Set("Content-Type", "application/json; charset=utf-8")
	ctx.Set("Cache-Control", "no-store, no-cache, must-revalidate")
	ctx.Set("Pragma", "no-cache")
	return ctx.Send(body)
}

// proxyVncHTML fetches upstream vnc.html and hides the credentials dialog for
// panel-authenticated proxy sessions (password is provided via mandatory.json).
func (cc *Controller) proxyVncHTML(ctx fiber.Ctx, session *models.VncSession, upstream, upHost string) error {
	target := upstream + "/vnc.html"
	req, err := http.NewRequestWithContext(ctx.Context(), http.MethodGet, target, nil)
	if err != nil {
		return fiber.NewError(fiber.StatusBadGateway, "novnc: build request: "+err.Error())
	}
	req.Host = upHost
	req.Header.Set("Accept", "text/html")

	resp, err := upstreamHTTP.Do(req)
	if err != nil {
		return fiber.NewError(fiber.StatusBadGateway, "novnc: fetch vnc.html: "+err.Error())
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return fiber.NewError(fiber.StatusBadGateway, "novnc: read vnc.html: "+err.Error())
	}
	if resp.StatusCode >= 400 {
		ctx.Status(resp.StatusCode)
		ctx.Set("Content-Type", resp.Header.Get("Content-Type"))
		return ctx.Send(raw)
	}

	injected := injectPanelNoVNCChrome(raw, rfbPassword(session.VncPassword) != "")
	ctx.Status(fiber.StatusOK)
	ctx.Set("Content-Type", "text/html; charset=utf-8")
	ctx.Set("Cache-Control", "no-store")
	return ctx.Send(injected)
}

func injectPanelNoVNCChrome(htmlBody []byte, hideCredentials bool) []byte {
	snippet := `
<script>
(function () {
  document.documentElement.classList.add("cws-novnc-panel");
})();
</script>
`
	if hideCredentials {
		snippet = `
<style id="cws-novnc-panel-auth">
  /* Panel auth + session password available — hide VNC credential prompt. */
  #noVNC_credentials_dlg { display: none !important; }
  #noVNC_setting_password,
  label[for="noVNC_setting_password"] { display: none !important; }
</style>
<script>
(function () {
  document.documentElement.classList.add("cws-novnc-panel");
})();
</script>
`
	}
	lower := bytes.ToLower(htmlBody)
	if i := bytes.Index(lower, []byte("</head>")); i >= 0 {
		out := make([]byte, 0, len(htmlBody)+len(snippet))
		out = append(out, htmlBody[:i]...)
		out = append(out, []byte(snippet)...)
		out = append(out, htmlBody[i:]...)
		return out
	}
	return append([]byte(snippet), htmlBody...)
}

func (cc *Controller) proxyWebSocket(ctx fiber.Ctx, upHost string) error {
	if upHost == "" {
		return fiber.NewError(fiber.StatusBadGateway, "novnc upstream not configured")
	}

	req := ctx.Request()
	req.SetRequestURI(stripAccessTokenQuery(stripPrefix(string(req.RequestURI()))))
	req.SetHost(upHost)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")

	var handshake bytes.Buffer
	if _, err := req.WriteTo(&handshake); err != nil {
		return fiber.NewError(fiber.StatusBadGateway, "novnc: encode upgrade: "+err.Error())
	}
	handshakeBytes := bytes.Clone(handshake.Bytes())

	fctx := ctx.RequestCtx()
	fctx.SetConnectionClose()
	fctx.HijackSetNoResponse(true)
	fctx.Hijack(func(client net.Conn) {
		defer client.Close()

		upstream, err := net.DialTimeout("tcp", upHost, 10*time.Second)
		if err != nil {
			return
		}
		defer upstream.Close()

		if tc, ok := upstream.(*net.TCPConn); ok {
			_ = tc.SetKeepAlive(true)
			_ = tc.SetKeepAlivePeriod(30 * time.Second)
			_ = tc.SetNoDelay(true)
		}
		if tc, ok := client.(*net.TCPConn); ok {
			_ = tc.SetKeepAlive(true)
			_ = tc.SetKeepAlivePeriod(30 * time.Second)
			_ = tc.SetNoDelay(true)
		}

		if _, err := upstream.Write(handshakeBytes); err != nil {
			return
		}

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, _ = io.Copy(upstream, client)
			closeWrite(upstream)
		}()
		go func() {
			defer wg.Done()
			_, _ = io.Copy(client, upstream)
			closeWrite(client)
		}()
		wg.Wait()
	})
	return nil
}

func closeWrite(c net.Conn) {
	if tc, ok := c.(*net.TCPConn); ok {
		_ = tc.CloseWrite()
		return
	}
	_ = c.Close()
}

func stripPrefix(uri string) string {
	stripped := strings.TrimPrefix(uri, Prefix)
	switch {
	case stripped == "":
		return "/"
	case stripped[0] == '?':
		return "/" + stripped
	default:
		return stripped
	}
}

func pathOnly(uriPath string) string {
	uriPath = strings.TrimSpace(uriPath)
	if uriPath == "" {
		return "/"
	}
	if i := strings.IndexByte(uriPath, '?'); i >= 0 {
		uriPath = uriPath[:i]
	}
	return path.Clean("/" + strings.TrimPrefix(uriPath, "/"))
}

func isMandatoryJSON(p string) bool {
	return pathOnly(p) == "/mandatory.json"
}

func isVncHTML(p string) bool {
	switch pathOnly(p) {
	case "/vnc.html", "/vnc_lite.html":
		return true
	default:
		return false
	}
}

// rfbPassword returns the VNC/RFB password TigerVNC accepts (first 8 bytes).
func rfbPassword(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if len(raw) > 8 {
		return raw[:8]
	}
	return raw
}
