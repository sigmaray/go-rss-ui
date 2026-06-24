// @title           Go RSS UI API
// @version         1.0
// @description     REST API for RSS feed management application
// @host            localhost:8082
// @BasePath        /api/v1
// @securityDefinitions.apikey CookieAuth
// @in              cookie
// @name            mysession
package main

import (
	"go-rss-ui/app"
	"go-rss-ui/config"
	"go-rss-ui/database"
	"go-rss-ui/services"
)

func main() {
	config.Load()
	app.InitLogger()

	if runCLICommand() {
		return
	}

	showStartupInfo()
	database.Connect()

	if config.GetBackgroundFetchEnabled() {
		go services.StartBackgroundFeedFetcher()
	} else {
		app.Logger.Info().Msg("Background feed fetcher is disabled")
	}

	router := setupRouter()
	port := config.GetServerPort()
	if err := router.Run(":" + port); err != nil {
		app.Logger.Fatal().Err(err).Msg("Failed to start server")
	}
}
