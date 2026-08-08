package frontend

import (
	"embed"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"
)

//go:embed static/*
var staticFS embed.FS

func GetStatic() embed.FS {
	return staticFS
}

func UseStatic() fiber.Handler {
	return static.New("static", static.Config{
		FS:     staticFS,
		Browse: false,
	})
}
