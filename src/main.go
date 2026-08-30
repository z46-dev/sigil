package main

import (
	"github.com/z46-dev/golog"
	"github.com/z46-dev/sigil/service/ipevaluation"
	"github.com/z46-dev/sigil/src/app"
	"github.com/z46-dev/sigil/src/config"
	"github.com/z46-dev/sigil/src/db"
)

var log *golog.Logger = golog.New().Prefix("[SIGIL]", golog.BoldPurple).Timestamp()

func main() {
	log.Info("Starting...")

	var err error

	if err = config.LoadConfig("config.toml"); err != nil {
		log.Panicf("Failed to load config: %v\n", err)
	}

	if err = db.Init(log); err != nil {
		log.Panicf("Failed to initialize database: %v\n", err)
	}

	if config.Config.WebServer.ServerNetworkSignals {
		if err = ipevaluation.Init(config.Config.WebServer.IPDataDirectory, config.Config.IPIntelligence.DatabaseFile, config.Config.IPIntelligence.ThreatFoxAuthKey); err != nil {
			log.Panicf("Failed to initialize IP evaluations: %v\n", err)
		}
		defer ipevaluation.Close()
	}

	for _, redirect := range config.Config.WebServer.RedirectServers {
		log.Infof("Redirecting from %s to %s (from_tls: %v, to_tls: %v)", redirect.From, redirect.To, redirect.FromTLS, redirect.ToTLS)
		go func() {
			var e error
			if e = app.StartRedirect(redirect.From, redirect.To, redirect.FromTLS, redirect.ToTLS); e != nil {
				log.Panicf("Failed to start redirect from %s to %s: %v\n", redirect.From, redirect.To, e)
			}
		}()
	}

	if err = app.Start(); err != nil {
		log.Panicf("Failed to start app: %v\n", err)
	}
}
