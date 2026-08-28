package main

import (
	"github.com/z46-dev/golog"
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

	if err = app.Start(); err != nil {
		log.Panicf("Failed to start app: %v\n", err)
	}
}
