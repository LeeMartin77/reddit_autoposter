package main

import (
	"context"
	"errors"

	"github.com/leemartin77/reddit_autoposter/internal/config"
	"github.com/leemartin77/reddit_autoposter/internal/webapp"
	"github.com/rs/zerolog/log"
	"github.com/sethvargo/go-envconfig"
)

func main() {

	var config config.Configuration
	if err := envconfig.Process(context.Background(), &config); err != nil {
		log.Fatal().Err(err).Msg("An error was encountered parsing environment variables")
	}

	site, err := webapp.NewWebsite(config)

	if err != nil {
		log.Fatal().Err(err).Msg("api failed to initialise")
	}

	if err := site.Run(); err != nil {
		if errors.Is(err, webapp.ErrShutdown) {
			log.Error().Err(err).Msg("api failed to shutdown")
		} else {
			log.Fatal().Err(err).Msg("api failed to start")
		}
	}
}
