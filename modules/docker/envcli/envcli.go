package envcli

import (
	"errors"

	"github.com/docker/docker/client"
	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/modules/docker/environments"
	"github.com/izetmolla/containerws/packages/dockerclient"
)

// Engine returns a Docker client for ?environment_id= (or the default environment).
func Engine(app *config.AppClients, c fiber.Ctx) (*client.Client, error) {
	_, cli, err := environments.ClientFromQuery(app.DB(), c.Query("environment_id"))
	return cli, err
}

// Respond maps Docker / Fiber errors to an API response.
func Respond(app *config.AppClients, c fiber.Ctx, err error) error {
	r := app.Render()
	var fe *fiber.Error
	if errors.As(err, &fe) {
		return r.Api(c, r.WithError(err), r.WithStatus(fe.Code), r.WithErrorCode("DOCKER_ENV"))
	}
	code, msg := dockerclient.MapError(err)
	return r.Api(c, r.WithError(errors.New(msg)), r.WithStatus(code), r.WithErrorCode("DOCKER_ERROR"))
}
