package app

import (
	"github.com/UNHCSC/pve-acl/config"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/template/html/v2"
	"github.com/z46-dev/golog"
)

var appLog *golog.Logger = golog.New().Prefix("[Please call app.InitAndListen() with the main logger]", golog.BoldRed)

func InitAndListen(parentLog *golog.Logger) (app *fiber.App, err error) {
	appLog = parentLog.SpawnChild().Prefix("[APP]", golog.BoldPurple)

	var templateEngine *html.Engine = html.New("./client/views", ".html")
	templateEngine.Reload(config.Config.WebServer.ReloadTemplatesOnEachRender)

	app = fiber.New(fiber.Config{
		Views: templateEngine,
	})

	// Statics
	app.Static("/static", "./client/static")

	// Pages

	// API
	var (
		api   fiber.Router = app.Group("/api")
		apiV1 fiber.Router = api.Group("/v1")
	)

	// API v1
	var (
		apiV1Auth   fiber.Router = apiV1.Group("/auth")
		apiV1Enums  fiber.Router = apiV1.Group("/enums")
		apiV1Users  fiber.Router = apiV1.Group("/users")
		apiV1Groups fiber.Router = apiV1.Group("/groups")
		apiV1Assets fiber.Router = apiV1.Group("/assets")
	)

	// API v1 auth
	apiV1Auth.Post("/login", _noop)
	apiV1Auth.Post("/logout", _noop)
	apiV1Auth.Get("/status", _noop)

	// API v1 enums
	apiV1Enums.Get("/permissions", _noop)
	apiV1Enums.Get("/assettypes", _noop)

	// API v1 users
	apiV1Users.Get("/me", _noop)
	apiV1Users.Get("/search", _noop)
	apiV1Users.Get("/some/:usernames", _noop)
	apiV1Users.Post("/update/:usernames", _noop)

	// API v1 groups
	apiV1Groups.Get("/search", _noop)
	apiV1Groups.Get("/some/:groupnames", _noop)
	apiV1Groups.Post("/update/:groupnames", _noop)

	// API v1 assets
	apiV1Assets.Get("/search", _noop)
	apiV1Assets.Get("/some/:assetnames", _noop)
	apiV1Assets.Post("/update/:assetnames", _noop)
	apiV1Assets.Post("/create", _noop)

	return
}

func _noop(*fiber.Ctx) (err error) {
	return
}
