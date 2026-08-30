package app

import (
	"fmt"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/helmet"
	"github.com/gofiber/fiber/v3/middleware/static"
	"github.com/z46-dev/sigil/src/app/api"
	"github.com/z46-dev/sigil/src/config"
)

var app *fiber.App

// New builds the HTTP application without opening a listener.
func New() (server *fiber.App) {
	server = fiber.New(fiber.Config{
		ProxyHeader:        fiber.HeaderXForwardedFor,
		TrustProxy:         len(config.Config.WebServer.TrustedProxies) > 0,
		TrustProxyConfig:   fiber.TrustProxyConfig{Proxies: config.Config.WebServer.TrustedProxies},
		EnableIPValidation: true,
	})

	server.Use(helmet.New(helmet.Config{
		ContentSecurityPolicy:     "default-src 'self'; script-src 'self' 'wasm-unsafe-eval' https://cdnjs.cloudflare.com; style-src 'self'; connect-src 'self'; img-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'",
		CrossOriginEmbedderPolicy: "unsafe-none",
		XFrameOptions:             "DENY",
	}))

	api.Init(server)

	if config.Config.WebServer.ServePublicDir {
		server.Get("/*", static.New("./client/public"))
	}

	return
}

// Start builds and serves the configured HTTP application.
func Start() (err error) {
	app = New()

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

// Redirects traffic from the from port to the to port. If fromTLS is true, the from server will serve with TLS. If toTLS is true, the redirect will use HTTPS.
func StartRedirect(from, to string, fromTLS, toTLS bool) (err error) {
	if fromTLS && config.Config.WebServer.TLSDir == "" {
		err = fmt.Errorf("TLS is requested but no TLS directory is configured")
		return
	}

	var app *fiber.App = fiber.New()

	app.Get("*", func(c fiber.Ctx) (e error) {
		var scheme string = "http"
		if toTLS {
			scheme = "https"
		}

		e = c.Redirect().Status(fiber.StatusMovedPermanently).To(fmt.Sprintf("%s://%s%s", scheme, to, c.Path()))
		return
	})

	var listenConfig fiber.ListenConfig
	if fromTLS {
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

	err = app.Listen(from, listenConfig)
	return
}
