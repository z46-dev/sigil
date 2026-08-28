package config

import "github.com/z46-dev/goconf"

type Configuration struct {
	WebServer struct {
		Address        string `toml:"address" default:":8080" validate:"required"` // Listen address for the web application server e.g. ":8080" or "0.0.0.0:8080"
		TLSDir         string `toml:"tls_dir" default:""`                          // Directory containing a crt and a key file for TLS. Leave empty to use HTTP instead of HTTPS.
		ServePublicDir bool   `toml:"serve_public_dir" default:"true"`             // Serve the public directory for static files. Set to false to disable serving static files.
	} `toml:"web_server"` // Web server configuration

	Database struct {
		File string `toml:"file" default:"sigil.db" validate:"required"` // Path to the database file
	} `toml:"database"` // Database configuration
}

var Config Configuration

func LoadConfig(path string) (err error) {
	Config, err = goconf.LoadConfig[Configuration](path, goconf.WithIndentSpaces(4), goconf.WithNewFileBehavior(goconf.NewFileBehaviorCreateAndTry))
	return
}
