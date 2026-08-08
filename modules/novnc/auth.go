package novnc

import (
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/izetmolla/goauth"
)

// panelAuthCookie carries the panel JWT for subsequent /novnc asset + WS
// requests. Browsers do not forward ?access_token= from vnc.html onto relative
// /novnc/app/* fetches, so the first authenticated response sets this cookie.
const panelAuthCookie = "cws_novnc_auth"

// resolvePanelUser returns the authenticated panel user for /novnc requests.
//
// Accepts either:
//   - Bearer JWT / ?access_token= / Referer ?access_token= / Path=/novnc cookie
//   - WEB session cookie (cnf.id) from a full browser sign-in
func (cc *Controller) resolvePanelUser(ctx fiber.Ctx) (*goauth.AuthData, error) {
	auth := cc.app.Authorization()
	if auth == nil {
		return nil, errUnauthorized
	}

	if raw := extractAccessToken(ctx); raw != "" {
		if data, err := cc.authDataFromJWT(raw); err == nil && data != nil && data.UserID != "" {
			cc.rememberPanelAuth(ctx, raw)
			return data, nil
		}
	}

	if data, err := auth.GetAuthDataWEB(ctx, ctx.Context()); err == nil && data.UserID != "" {
		return &data, nil
	}

	return nil, errUnauthorized
}

func extractAccessToken(ctx fiber.Ctx) string {
	if auth := strings.TrimSpace(ctx.Get(fiber.HeaderAuthorization)); auth != "" {
		const prefix = "Bearer "
		if len(auth) > len(prefix) && strings.EqualFold(auth[:len(prefix)], prefix) {
			return strings.TrimSpace(auth[len(prefix):])
		}
	}
	if t := strings.TrimSpace(ctx.Query("access_token")); t != "" {
		return t
	}
	if t := accessTokenFromReferer(string(ctx.Request().Header.Referer())); t != "" {
		return t
	}
	if t := strings.TrimSpace(ctx.Cookies(panelAuthCookie)); t != "" {
		return t
	}
	return ""
}

func accessTokenFromReferer(referer string) string {
	referer = strings.TrimSpace(referer)
	if referer == "" {
		return ""
	}
	u, err := url.Parse(referer)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(u.Query().Get("access_token"))
}

func (cc *Controller) rememberPanelAuth(ctx fiber.Ctx, token string) {
	token = strings.TrimSpace(token)
	if token == "" {
		return
	}
	ctx.Cookie(&fiber.Cookie{
		Name:     panelAuthCookie,
		Value:    token,
		Path:     Prefix,
		HTTPOnly: true,
		Secure:   strings.EqualFold(ctx.Protocol(), "https"),
		SameSite: "Lax",
		MaxAge:   60 * 60 * 12,
	})
}

func (cc *Controller) authDataFromJWT(raw string) (*goauth.AuthData, error) {
	secret := cc.jwtSecret()
	if secret == "" {
		return nil, errUnauthorized
	}

	token, err := jwt.Parse(raw, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errUnauthorized
		}
		return []byte(secret), nil
	})
	if err != nil || token == nil || !token.Valid {
		return nil, errUnauthorized
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errUnauthorized
	}

	userID := stringClaim(claims, "user_id")
	if userID == "" {
		return nil, errUnauthorized
	}
	roles := []string{}
	switch v := claims["roles"].(type) {
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				roles = append(roles, s)
			}
		}
	case []string:
		roles = append(roles, v...)
	}

	return &goauth.AuthData{
		SessionID: stringClaim(claims, "session_id"),
		UserID:    userID,
		Roles:     roles,
	}, nil
}

func (cc *Controller) jwtSecret() string {
	if auth := cc.app.Authorization(); auth != nil {
		if inner := auth.Unwrap(); inner != nil {
			if s := strings.TrimSpace(inner.JWTSecret); s != "" {
				return s
			}
		}
	}
	if cfg := cc.app.ServerConfig(); cfg != nil {
		return strings.TrimSpace(cfg.JWT_SECRET)
	}
	return ""
}

func stringClaim(claims jwt.MapClaims, key string) string {
	v, ok := claims[key]
	if !ok || v == nil {
		return ""
	}
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

// stripAccessTokenQuery removes access_token from a request URI before proxying
// upstream so the JWT is not leaked to websockify/noVNC.
func stripAccessTokenQuery(requestURI string) string {
	pathPart := requestURI
	query := ""
	if before, after, ok := strings.Cut(requestURI, "?"); ok {
		pathPart = before
		query = after
	}
	if query == "" || !strings.Contains(query, "access_token=") {
		return requestURI
	}
	parts := strings.Split(query, "&")
	kept := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.HasPrefix(p, "access_token=") {
			continue
		}
		kept = append(kept, p)
	}
	if len(kept) == 0 {
		return pathPart
	}
	return pathPart + "?" + strings.Join(kept, "&")
}

// withAccessToken appends access_token to a relative /novnc client URL and to the
// websockify path query so redirects from /novnc keep JWT auth for the first paint.
func withAccessToken(clientURL, token string) string {
	token = strings.TrimSpace(token)
	if token == "" || strings.TrimSpace(clientURL) == "" {
		return clientURL
	}
	u, err := url.Parse(clientURL)
	if err != nil {
		return clientURL
	}
	q := u.Query()
	q.Set("access_token", token)
	if pathParam := q.Get("path"); pathParam != "" {
		ws, err := url.Parse(pathParam)
		if err == nil {
			wsQ := ws.Query()
			wsQ.Set("access_token", token)
			ws.RawQuery = wsQ.Encode()
			// path is a relative websockify?... value, not a full URL.
			q.Set("path", strings.TrimPrefix(ws.String(), "/"))
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}
