package cloudshell

import (
	"github.com/gofiber/contrib/v3/websocket"
	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
)

type controller struct {
	app *config.AppClients
}

func NewController(appClients *config.AppClients) *controller {
	return &controller{app: appClients}
}

func SetupRoutesAPI(router fiber.Router, appClients *config.AppClients) {
	cc := NewController(appClients)
	SetSessionDB(appClients.DB())
	api := router.Group("/cloudshell")

	api.Get("/session", cc.GetSessionAPI)
	api.Get("/sessions", cc.ListSessionsAPI)
	api.Delete("/sessions/:id", cc.KillSessionAPI)

	api.Use("/ws", func(c fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			if err := cc.bindWSUser(c); err != nil {
				return err
			}
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	api.Get("/ws", websocket.New(cc.HandleTerminalWS, websocket.Config{
		RecoverHandler: func(conn *websocket.Conn) {
			_ = conn.WriteJSON(map[string]any{
				"type":    "error",
				"message": "terminal session crashed",
			})
			_ = conn.Close()
		},
	}))
}
