package config

import (
	"fmt"

	"github.com/z46-dev/goconf"
)

type Configuration struct {
	WebServer struct {
		Address              string   `toml:"address" default:":8080" validate:"required"` // Listen address for the web application server e.g. ":8080" or "0.0.0.0:8080"
		TLSDir               string   `toml:"tls_dir" default:""`                          // Directory containing a crt and a key file for TLS. Leave empty to use HTTP instead of HTTPS.
		ServePublicDir       bool     `toml:"serve_public_dir" default:"true"`             // Serve the public directory for static files. Set to false to disable serving static files.
		ServerNetworkSignals bool     `toml:"server_network_signals" default:"false"`      // Use a privacy-preserving coarse network prefix as matching evidence.
		NetworkSignalKey     string   `toml:"network_signal_key" default:""`               // Private HMAC key used to pseudonymize network prefixes.
		IPDataDirectory      string   `toml:"ip_data_directory" default:"data"`            // Directory containing compressed GeoLite MMDB archives.
		TrustedProxies       []string `toml:"trusted_proxies"`                             // Proxy IPs or CIDRs allowed to supply forwarded client information.
		RedirectServers      []struct {
			From    string `toml:"from" default:"" validate:"required"` // The address to listen on for redirects
			To      string `toml:"to" default:"" validate:"required"`   // The target address to redirect to
			FromTLS bool   `toml:"from_tls" default:"false"`            // Whether to use TLS for the from server
			ToTLS   bool   `toml:"to_tls" default:"true"`               // Whether to use TLS for the redirect target
		} `toml:"redirect_servers"` // List of redirect servers
	} `toml:"web_server"` // Web server configuration

	Database struct {
		File string `toml:"file" default:"sigil.db" validate:"required"` // Path to the database file
	} `toml:"database"` // Database configuration

	IPIntelligence struct {
		DatabaseFile     string `toml:"database_file" default:"ip-intelligence.db" validate:"required"` // Separate durable cache for IP feed indicators.
		ThreatFoxAuthKey string `toml:"threatfox_auth_key" default:""`                                  // Optional abuse.ch ThreatFox API authentication key.
	} `toml:"ip_intelligence"`
}

var Config Configuration

func LoadConfig(path string) (err error) {
	Config, err = goconf.LoadConfig[Configuration](path, goconf.WithIndentSpaces(4), goconf.WithNewFileBehavior(goconf.NewFileBehaviorCreateAndTry))
	if err == nil && Config.WebServer.ServerNetworkSignals && len(Config.WebServer.NetworkSignalKey) < 32 {
		err = fmt.Errorf("network_signal_key must contain at least 32 characters when server_network_signals is enabled")
	}

	return
}
