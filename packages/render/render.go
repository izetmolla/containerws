package render

import (
	"context"
	"embed"

	"github.com/gofiber/fiber/v3"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type GeneralDataFunc func(c fiber.Ctx, reqCtx context.Context, moduleName string, forApi bool) (map[string]any, error)
type Render struct {
	Config
}

type ConfigFunc func(cfg *Config)
type Config struct {
	db              *gorm.DB
	redis           *redis.Client
	moduleName      string
	withGeneralData GeneralDataFunc
	assets          embed.FS
}

func defaultConfig() *Config {
	return &Config{
		moduleName:      "containerws",
		withGeneralData: nil,
		assets:          embed.FS{},
	}
}

func New(opts ...ConfigFunc) *Render {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}
	return &Render{
		Config: *cfg,
	}
}

func (r *Render) WithDB(db *gorm.DB) *Render {
	r.db = db
	return r
}

func (r *Render) WithRedis(redis *redis.Client) *Render {
	r.redis = redis
	return r
}

func (r *Render) WithModuleName(moduleName string) *Render {
	r.moduleName = moduleName
	return r
}
func (r *Render) WithGeneralData(withGeneralData GeneralDataFunc) *Render {
	r.withGeneralData = withGeneralData
	return r
}
func (r *Render) WithAssets(assets embed.FS) *Render {
	r.assets = assets
	return r
}

func (r *Render) WithErrorStatus(request int) RenderOptionsFunc {
	return func(o *RenderOptions) {
		o.errorStatus = request
	}
}
