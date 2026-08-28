package app

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/csrf"
	"github.com/gofiber/fiber/v3/middleware/static"
	"github.com/z46-dev/sigil/src/app/api"
	"github.com/z46-dev/sigil/src/config"
)

var app *fiber.App

func Start() (err error) {
	app = fiber.New(fiber.Config{})

	app.Use(cors.New(cors.Config{
		AllowOrigins: config.Config.WebServer.CORSAllowedOrigins,
		AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization"},
	}))

	if config.Config.WebServer.EnableCSRF {
		app.Use(csrf.New(csrf.Config{
			Next: func(ctx fiber.Ctx) bool {
				return strings.HasPrefix(ctx.Path(), "/api")
			},
		}))
	}

	api.Init(app)

	if config.Config.WebServer.ServePublicDir {
		app.Get("/*", static.New("./client/public"))
	}

	var listenConfig fiber.ListenConfig
	if config.Config.WebServer.TLSDir != "" {
		var (
			certPath, keyPath string
			found             bool
		)

		if certPath, keyPath, found = DiscoverTLSKeys(config.Config.WebServer.TLSDir); !found {
			err = fiber.ErrInternalServerError
			return
		}

		listenConfig.CertFile = certPath
		listenConfig.CertKeyFile = keyPath
	}

	err = app.Listen(config.Config.WebServer.Address, listenConfig)
	return
}
