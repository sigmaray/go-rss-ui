package commands

import (
	"go-rss-ui/app"
	"go-rss-ui/services"
)

func CommandClearUsers() {
	rowsAffected, err := services.ClearUsers()
	if err != nil {
		app.Logger.Fatal().Err(err).Msg("Failed to clear users table")
	}

	app.Logger.Info().Int64("rows_affected", rowsAffected).Msg("Successfully cleared users table")
}

func CommandSeedUsers() {
	SeedUsers()
}

func SeedUsers() {
	result, err := services.EnsureAdminUser()
	if err != nil {
		app.Logger.Fatal().Err(err).Msg("Failed to ensure admin user")
	}

	if result.Created {
		app.Logger.Info().Msg("Admin user 'admin' created with password 'password'")
		return
	}

	app.Logger.Info().Msg("Admin user already exists")
}
