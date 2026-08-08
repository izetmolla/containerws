package render

import "github.com/gofiber/fiber/v3"

/*
RenderError is a function that renders an error page.
params:
- err: error to render
- listenAddress: address to listen on
returns:
- error: error if any
*/
func RenderError(err error, listenAddress string) error {
	app := fiber.New()
	ServeError(app, err)
	return app.Listen(listenAddress)
}

// ServeError registers handlers that respond with the error message on every request.
func ServeError(app *fiber.App, err error) {
	if app == nil || err == nil {
		return
	}
	message := err.Error()
	app.All("*", func(c fiber.Ctx) error {
		return c.SendString(message)
	})
}
