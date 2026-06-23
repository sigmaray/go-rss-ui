package commands

import (
	"go-rss-ui/app"
	"go-rss-ui/services"
)

type SeedFeedsResult = services.SeedFeedsResult

func CommandSeedFeeds() {
	result := SeedFeeds()
	app.Logger.Info().
		Int("created", result.Created).
		Int("existed", result.Existed).
		Int("errors", result.Errors).
		Msg("Seeded feeds")
}

func GetDefaultFeeds() []string {
	return services.DefaultFeedURLs()
}

func SeedFeeds() SeedFeedsResult {
	return services.SeedFeeds()
}

func SeedFeedsWithURLs(feedURLs []string) SeedFeedsResult {
	return services.SeedFeedsWithURLs(feedURLs)
}

func CommandFetchFeeds() {
	app.Logger.Info().Msg("Starting feed fetch...")
	itemsCreated, itemsUpdated, errorsCount := services.ProcessAllFeeds()
	app.Logger.Info().
		Int("items_created", itemsCreated).
		Int("items_updated", itemsUpdated).
		Int("errors", errorsCount).
		Msg("Feed fetch completed")
}
