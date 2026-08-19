package authorization

import (
	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
)

type controller struct {
	app *config.AppClients
}

func NewController(appClients *config.AppClients) *controller {
	return &controller{app: appClients}
}

func SetupRoutesView(app fiber.Router, appClients *config.AppClients) {
	cc := NewController(appClients)
	app.Get("/sign-in", cc.SignInView)
}

func SetupRoutesAPI(app fiber.Router, appClients *config.AppClients) {
	cc := NewController(appClients)
	auth := appClients.Authorization()
	api := app.Group("/authorization")
	api.Get("/providers", auth.GetProviders)
	api.Get("/provider/:provider", providerMiddleware(appClients))
	api.Get("/provider/:provider/callback", auth.HandleCallback)

	api.Post("/signin", cc.SignInAPI)
	api.Post("/local-signin", cc.LocalSignInAPI)
	api.Get("/local-signin", cc.LocalSignInAPI)
	api.Post("/forgot-password", cc.ForgotPasswordAPI)
	api.Post("/check", cc.CheckApi)
	api.Get("/branding", cc.BrandingAPI)
}

func providerMiddleware(appClients *config.AppClients) fiber.Handler {
	auth := appClients.Authorization()
	return func(c fiber.Ctx) error {
		// if c.Query("connect") == "1" {
		// // 	cfg := appClients.ServerConfig()
		// // 	scopes := microsoftsdk.ConnectScopes()
		// // 	if resourceID := strings.TrimSpace(c.Query("resource_id")); resourceID != "" {
		// // 		if res := appClients.Resources(); res != nil {
		// // 			if row, err := res.GetResource(c.Context(), resourceID); err == nil {
		// // 				scopes = microsoftsdk.ScopesFromResourceConfig(row.Config)
		// // 			}
		// // 		}
		// // 	}
		// // 	providers := []goauth.Provider{
		// // 		azuread.New(azuread.Options{
		// // 			ClientID:            os.Getenv("AZURE_AD_CLIENT_ID"),
		// // 			ClientSecret:        os.Getenv("AZURE_AD_CLIENT_SECRET"),
		// // 			TenantID:            os.Getenv("AZURE_AD_TENANT_ID"),
		// // 			Scopes:              scopes,
		// // 			AuthorizationParams: microsoftsdk.ConnectAuthorizationParams(),
		// // 		}),
		// // 	}
		// 	newauth, err := goauth.New(&goauth.Config{
		// 		JWTSecret:         cfg.JWT_SECRET,
		// 		AuthURL:           cfg.AUTH_URL,
		// 		DB:                appClients.Postgres(),
		// 		Providers:         providers,
		// 		Redis:             appClients.Redis(),
		// 		ResolveUser:       config.NewResolveUser(appClients.Postgres()),
		// 		OnProviderConnect: config.NewProviderConnectHandler(appClients.Resources()),
		// 	})

		// 	if err != nil {
		// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
		// 			"message": "Failed to create new authorization",
		// 			"code":    "ERROR",
		// 			"error":   err.Error(),
		// 		})
		// 	}

		// 	return goauthfiberv3.New(newauth).HandleSignIn(c)
		// }
		return auth.HandleSignIn(c)
	}
}
