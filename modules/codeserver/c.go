package codeserver

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/proxy"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/packages/machine"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

// Prefix is the reverse-proxy mount for VS Code Server (serve-web) sessions.
//
// Upstream listens at / (no --server-base-path). The proxy strips
// /codeserver/:uuid and sets X-Forwarded-Prefix so the workbench builds
// asset and WebSocket URLs under this public path (same pattern as the
// upstream-supported nginx config).
const Prefix = "/codeserver"

type Controller struct {
	app *config.AppClients
}

func NewController(app *config.AppClients) *Controller {
	return &Controller{app: app}
}

// SetupRoutesView registers /codeserver/:uuid reverse-proxy (HTTP + WebSocket).
func SetupRoutesView(app fiber.Router, appClients *config.AppClients) {
	cc := NewController(appClients)

	g := app.Group(Prefix)
	// All methods so WebSocket upgrades (GET + Upgrade) never miss the handler.
	g.All("/:uuid", cc.GetIndex)
	g.All("/:uuid/", cc.GetIndex)
	g.All("/:uuid/*", cc.Proxy)
}

// ClientURL returns the in-app entry path for a codeserver session.
func ClientURL(sessionID string) string {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return Prefix + "/"
	}
	return Prefix + "/" + sessionID + "/"
}

// ClientURLForFolder is ClientURL with ?folder= so the workbench opens that path.
// Slashes are left readable (e.g. ?folder=/workspace/app) for copy-paste URLs.
func ClientURLForFolder(sessionID, folder string) string {
	base := ClientURL(sessionID)
	folder = strings.TrimSpace(folder)
	if folder == "" {
		return base
	}
	return base + "?folder=" + FolderQueryValue(folder)
}

// FolderQueryValue formats a filesystem path for a ?folder= query value,
// keeping "/" readable while escaping query delimiters.
func FolderQueryValue(folder string) string {
	folder = strings.TrimSpace(folder)
	if folder == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(folder))
	for i := 0; i < len(folder); i++ {
		c := folder[i]
		switch c {
		case ' ', '"', '\'', '%', '?', '&', '#', '+':
			b.WriteString(url.QueryEscape(string(c)))
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// PublicClientURL returns the absolute workspace URL for a session:
//
//	{AUTH_URL}/codeserver/{uuid}/
//
// When AUTH_URL is unset, falls back to the host primary IP and the app
// listen port (PORT, default 9000):
//
//	http://{ip}:{port}/codeserver/{uuid}/
func PublicClientURL(sessionID string) string {
	return publicURL(ClientURL(sessionID))
}

// PublicClientURLForFolder is PublicClientURL with ?folder= for the workspace path.
func PublicClientURLForFolder(sessionID, folder string) string {
	return publicURL(ClientURLForFolder(sessionID, folder))
}

func publicURL(path string) string {
	base := strings.TrimRight(strings.TrimSpace(appPublicBase()), "/")
	if base == "" {
		return path
	}
	return base + path
}

func appPublicBase() string {
	if base := strings.TrimSpace(os.Getenv("AUTH_URL")); base != "" {
		return base
	}
	if base := strings.TrimSpace(viper.GetString("AUTH_URL")); base != "" {
		return base
	}
	return localAppBaseURL()
}

// localAppBaseURL builds http(s)://{primary-ip}:{PORT} for CLI/status output
// when AUTH_URL is not configured.
func localAppBaseURL() string {
	ip := strings.TrimSpace(machine.Detect().PrimaryIP)
	if ip == "" {
		ip = "127.0.0.1"
	}

	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = strings.TrimSpace(viper.GetString("PORT"))
	}
	if port == "" {
		port = config.DefaultHTTPPort
	}

	scheme := "http"
	https := strings.TrimSpace(os.Getenv("ENABLE_HTTPS"))
	if https == "" {
		https = strings.TrimSpace(viper.GetString("ENABLE_HTTPS"))
	}
	if config.ParseBoolEnv(https) {
		scheme = "https"
	}

	return scheme + "://" + net.JoinHostPort(ip, port)
}

// ServerBasePath is the public URL prefix for a session (X-Forwarded-Prefix).
func ServerBasePath(sessionID string) string {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return Prefix
	}
	return Prefix + "/" + sessionID
}

// GetIndex redirects bare /codeserver/:uuid → /codeserver/:uuid/ (HTTP only).
func (cc *Controller) GetIndex(ctx fiber.Ctx) error {
	session, err := cc.requireSession(ctx)
	if err != nil {
		return cc.handleError(ctx, err)
	}
	path := string(ctx.Request().URI().PathOriginal())
	// Never 302 a WebSocket upgrade — browsers treat that as close 1006.
	if !strings.HasSuffix(path, "/") && !isWebSocketUpgrade(ctx) {
		q := string(ctx.Request().URI().QueryString())
		target := ClientURLForFolder(session.ID, session.Path)
		// Preserve non-folder query params from the original request.
		if q != "" && !strings.Contains(q, "folder=") {
			if strings.Contains(target, "?") {
				target += "&" + q
			} else {
				target += "?" + q
			}
		}
		return ctx.Redirect().Status(fiber.StatusFound).To(target)
	}
	// Bare /codeserver/:uuid/ with no folder query → send them to the session path.
	if !isWebSocketUpgrade(ctx) && string(ctx.Request().URI().QueryString()) == "" && strings.TrimSpace(session.Path) != "" {
		return ctx.Redirect().Status(fiber.StatusFound).To(ClientURLForFolder(session.ID, session.Path))
	}
	return cc.Proxy(ctx)
}

// Proxy reverse-proxies /codeserver/:uuid/* to the session upstream.
func (cc *Controller) Proxy(ctx fiber.Ctx) error {
	session, err := cc.requireSession(ctx)
	if err != nil {
		return cc.handleError(ctx, err)
	}

	upstream := session.UpstreamBaseURL()
	upHost := session.UpstreamHostPort()
	reqURI := string(ctx.Request().RequestURI())
	stripped := stripSessionURI(reqURI, session.ID)

	applyForwardHeaders(ctx, session.ID)

	if isWebSocketUpgrade(ctx) {
		return cc.proxyWebSocket(ctx, upHost, stripped)
	}

	target := upstream + stripped
	return proxy.Do(ctx, target)
}

func isWebSocketUpgrade(ctx fiber.Ctx) bool {
	if ctx.IsWebSocket() {
		return true
	}
	upgrade := strings.ToLower(strings.TrimSpace(ctx.Get(fiber.HeaderUpgrade)))
	if strings.Contains(upgrade, "websocket") {
		return true
	}
	// Cloudflare / some proxies only set Connection: Upgrade.
	conn := strings.ToLower(ctx.Get(fiber.HeaderConnection))
	return strings.Contains(conn, "upgrade") && ctx.Get("Sec-WebSocket-Key") != ""
}

// stripSessionURI removes /codeserver/{uuid} so upstream (listening at /) sees
// the path the workbench expects. Query string is preserved.
func stripSessionURI(reqURI, sessionID string) string {
	base := ServerBasePath(sessionID)
	pathPart := reqURI
	query := ""
	if i := strings.IndexByte(reqURI, '?'); i >= 0 {
		pathPart = reqURI[:i]
		query = reqURI[i:]
	}
	pathPart = strings.TrimPrefix(pathPart, base)
	switch {
	case pathPart == "":
		pathPart = "/"
	case pathPart[0] != '/':
		pathPart = "/" + pathPart
	}
	return pathPart + query
}

// applyForwardHeaders sets the headers serve-web needs behind a path prefix /
// Cloudflare Tunnel (browser HTTPS → origin HTTP).
func applyForwardHeaders(ctx fiber.Ctx, sessionID string) {
	req := ctx.Request()
	proto := clientProto(ctx)
	publicHost := clientHost(ctx, proto)

	req.SetHost(publicHost)
	req.Header.Set(fiber.HeaderXForwardedHost, publicHost)
	req.Header.Set(fiber.HeaderXForwardedProto, proto)
	req.Header.Set(fiber.HeaderXForwardedFor, ctx.IP())
	req.Header.Set("X-Forwarded-Prefix", ServerBasePath(sessionID))
	req.Header.Set("X-Forwarded-Port", clientPort(ctx, proto, publicHost))
	req.Header.Set("X-Real-IP", ctx.IP())
	if proto == "https" {
		req.Header.Set("X-Forwarded-Ssl", "on")
	}
}

// clientProto returns the browser-facing scheme. Cloudflare Tunnel terminates
// TLS at the edge and talks plain HTTP to the origin, so ctx.Protocol() is
// often "http" unless TrustProxy is on — still prefer explicit forwarded headers
// and AUTH_URL so serve-web emits wss:// URLs.
func clientProto(ctx fiber.Ctx) string {
	if p := firstCSV(ctx.Get(fiber.HeaderXForwardedProto)); p != "" {
		return normalizeProto(p)
	}
	if p := firstCSV(ctx.Get("X-Forwarded-Protocol")); p != "" {
		return normalizeProto(p)
	}
	if ssl := strings.ToLower(strings.TrimSpace(ctx.Get("X-Forwarded-Ssl"))); ssl == "on" || ssl == "1" {
		return "https"
	}
	if v := strings.TrimSpace(ctx.Get("CF-Visitor")); v != "" {
		var vis struct {
			Scheme string `json:"scheme"`
		}
		if json.Unmarshal([]byte(v), &vis) == nil && vis.Scheme != "" {
			return normalizeProto(vis.Scheme)
		}
		if strings.Contains(strings.ToLower(v), "https") {
			return "https"
		}
	}
	if base := appPublicBase(); strings.HasPrefix(strings.ToLower(base), "https://") {
		return "https"
	}
	if ctx != nil {
		if p := normalizeProto(ctx.Protocol()); p != "" {
			return p
		}
		if p := normalizeProto(ctx.Scheme()); p != "" {
			return p
		}
	}
	return "http"
}

func normalizeProto(p string) string {
	p = strings.ToLower(strings.TrimSpace(p))
	switch p {
	case "https", "http":
		return p
	default:
		return ""
	}
}

func firstCSV(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if before, _, ok := strings.Cut(v, ","); ok {
		return strings.TrimSpace(before)
	}
	return v
}

// clientHost is the public Host the browser used (no :443 / :80 suffix).
func clientHost(ctx fiber.Ctx, proto string) string {
	candidates := []string{
		firstCSV(ctx.Get(fiber.HeaderXForwardedHost)),
		strings.TrimSpace(ctx.Get("CF-Connecting-Host")),
		strings.TrimSpace(ctx.Hostname()),
		string(ctx.Request().Host()),
	}
	if base := appPublicBase(); base != "" {
		if u, err := url.Parse(base); err == nil && u.Host != "" {
			candidates = append(candidates, u.Host)
		}
	}
	for _, h := range candidates {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		// Drop default ports so Host matches browser Origin (https://x vs https://x:443).
		if host, port, err := net.SplitHostPort(h); err == nil {
			if (proto == "https" && port == "443") || (proto == "http" && port == "80") {
				return host
			}
			return h
		}
		return h
	}
	return "localhost"
}

func clientPort(ctx fiber.Ctx, proto, publicHost string) string {
	if p := firstCSV(ctx.Get("X-Forwarded-Port")); p != "" {
		return p
	}
	if _, port, err := net.SplitHostPort(publicHost); err == nil && port != "" {
		return port
	}
	if proto == "https" {
		return "443"
	}
	return "80"
}

func (cc *Controller) requireSession(ctx fiber.Ctx) (*models.CodeserverSession, error) {
	id := strings.TrimSpace(ctx.Params("uuid"))
	if id == "" {
		return nil, errBadUUID
	}

	auth := cc.app.Authorization()
	if auth == nil {
		return nil, errUnauthorized
	}
	user, err := auth.User(ctx, ctx.Context())
	if err != nil || user == nil || user.UserID == "" {
		return nil, errUnauthorized
	}

	db := cc.app.DB()
	if db == nil {
		return nil, errors.New("database unavailable")
	}

	var session models.CodeserverSession
	if err := db.Where("id = ?", id).First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errNotFound
		}
		return nil, err
	}
	if !strings.EqualFold(strings.TrimSpace(session.Status), models.CodeserverSessionStatusActive) {
		return nil, errInactive
	}
	if session.Port <= 0 || strings.TrimSpace(session.Address) == "" {
		return nil, errNoUpstream
	}

	if !cc.canAccessSession(ctx, user.UserID, user.Roles, &session) {
		return nil, errForbidden
	}
	return &session, nil
}

// canAccessSession allows the session owner, or any user with the admin role.
func (cc *Controller) canAccessSession(ctx fiber.Ctx, userID string, fallbackRoles []string, session *models.CodeserverSession) bool {
	if session == nil {
		return false
	}
	if strings.TrimSpace(session.UserID) != "" && session.UserID == userID {
		return true
	}
	roles := cc.app.FreshUserRoles(ctx.Context(), userID, fallbackRoles)
	return userHasAdminRole(cc.app, roles)
}

func userHasAdminRole(app *config.AppClients, userRoles []string) bool {
	if app == nil || len(userRoles) == 0 {
		return false
	}
	auth := app.Authorization()
	if auth == nil {
		return false
	}
	normalized := make([]string, 0, len(userRoles))
	for _, r := range userRoles {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		normalized = append(normalized, r)
		if i := strings.IndexByte(r, ':'); i > 0 {
			normalized = append(normalized, r[:i])
		}
	}
	hasRole, canRead, _ := auth.GetRole([]string{"admin"}, normalized)
	if hasRole && canRead {
		return true
	}
	for _, r := range normalized {
		if strings.EqualFold(r, "admin") {
			return true
		}
	}
	return false
}

var (
	errBadUUID      = errors.New("missing codeserver session id")
	errNotFound     = errors.New("codeserver session not found")
	errInactive     = errors.New("codeserver session is not active")
	errNoUpstream   = errors.New("codeserver session has no address/port")
	errUnauthorized = errors.New("unauthorized")
	errForbidden    = errors.New("forbidden: only the session owner or an admin can access this codeserver")
)

func (cc *Controller) handleError(ctx fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, errUnauthorized):
		redirect := "/sign-in?redirectUrl=" + url.QueryEscape(ctx.OriginalURL())
		if isWebSocketUpgrade(ctx) {
			return fiber.NewError(fiber.StatusUnauthorized, err.Error())
		}
		return ctx.Redirect().Status(fiber.StatusFound).To(redirect)
	case errors.Is(err, errForbidden):
		return fiber.NewError(fiber.StatusForbidden, err.Error())
	case errors.Is(err, errBadUUID), errors.Is(err, errNotFound):
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	case errors.Is(err, errInactive), errors.Is(err, errNoUpstream):
		return fiber.NewError(fiber.StatusBadGateway, err.Error())
	default:
		return fiber.NewError(fiber.StatusBadGateway, err.Error())
	}
}

func (cc *Controller) proxyWebSocket(ctx fiber.Ctx, upHost, strippedURI string) error {
	if upHost == "" {
		return fiber.NewError(fiber.StatusBadGateway, "codeserver upstream not configured")
	}

	req := ctx.Request()
	req.SetRequestURI(strippedURI)
	// Force a clean upgrade; Cloudflare may send Connection: keep-alive, Upgrade.
	req.Header.Set(fiber.HeaderConnection, "Upgrade")
	req.Header.Set(fiber.HeaderUpgrade, "websocket")
	// Avoid compressed upgrade responses confusing the tunnel.
	req.Header.Del(fiber.HeaderAcceptEncoding)

	var handshake bytes.Buffer
	if _, err := req.WriteTo(&handshake); err != nil {
		return fiber.NewError(fiber.StatusBadGateway, "codeserver: encode upgrade: "+err.Error())
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

		tuneConn(upstream)
		tuneConn(client)

		_ = upstream.SetDeadline(time.Now().Add(20 * time.Second))
		if _, err := upstream.Write(handshakeBytes); err != nil {
			return
		}

		br := bufio.NewReaderSize(upstream, 32<<10)
		head, ok, err := readHTTPResponseHead(br)
		if err != nil || len(head) == 0 {
			return
		}
		if _, err := client.Write(head); err != nil {
			return
		}
		if !ok {
			return
		}

		_ = upstream.SetDeadline(time.Time{})
		_ = client.SetDeadline(time.Time{})

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, _ = io.Copy(upstream, client)
			closeWrite(upstream)
		}()
		go func() {
			defer wg.Done()
			// br may still hold bytes that arrived with the 101 response.
			_, _ = io.Copy(client, br)
			closeWrite(client)
		}()
		wg.Wait()
	})
	return nil
}

func tuneConn(c net.Conn) {
	tc, ok := c.(*net.TCPConn)
	if !ok {
		return
	}
	_ = tc.SetKeepAlive(true)
	_ = tc.SetKeepAlivePeriod(30 * time.Second)
	_ = tc.SetNoDelay(true)
}

// readHTTPResponseHead reads until the end of HTTP headers.
func readHTTPResponseHead(r *bufio.Reader) (head []byte, is101 bool, err error) {
	var buf bytes.Buffer
	for {
		line, readErr := r.ReadBytes('\n')
		buf.Write(line)
		if readErr != nil {
			return buf.Bytes(), isHTTP101(buf.Bytes()), readErr
		}
		if bytes.Equal(line, []byte("\r\n")) || bytes.Equal(line, []byte("\n")) {
			break
		}
		if buf.Len() > 64<<10 {
			return buf.Bytes(), false, errors.New("response headers too large")
		}
	}
	raw := buf.Bytes()
	return raw, isHTTP101(raw), nil
}

func isHTTP101(resp []byte) bool {
	if len(resp) < 12 {
		return false
	}
	line := resp
	if before, _, ok := bytes.Cut(resp, []byte{'\n'}); ok {
		line = before
	}
	line = bytes.TrimSpace(line)
	parts := bytes.Fields(line)
	return len(parts) >= 2 && bytes.Equal(parts[1], []byte(strconv.Itoa(fiber.StatusSwitchingProtocols)))
}

func closeWrite(c net.Conn) {
	if tc, ok := c.(*net.TCPConn); ok {
		_ = tc.CloseWrite()
		return
	}
	_ = c.Close()
}
