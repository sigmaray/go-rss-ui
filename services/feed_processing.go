package services

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/mmcdole/gofeed"
	"go-rss-ui/app"
	"go-rss-ui/config"
	"go-rss-ui/database"
	"go-rss-ui/models"
	"go-rss-ui/validation"
	"gorm.io/gorm"
)

func ProcessAllFeeds() (itemsCreated, itemsUpdated, errorsCount int) {
	return ProcessFeedsWithFilter(true)
}

func ProcessFeedsWithFilter(includeTest bool) (itemsCreated, itemsUpdated, errorsCount int) {
	ensurePrimaryDatabase()

	var feeds []models.Feed
	query := database.DB
	if includeTest {
		query.Find(&feeds)
	} else {
		query.Where("url NOT LIKE ?", "%/test_feeds/%").Find(&feeds)
	}

	if len(feeds) == 0 {
		return 0, 0, 0
	}

	const maxWorkers = 10

	var mu sync.Mutex
	feedChan := make(chan models.Feed, len(feeds))
	var wg sync.WaitGroup

	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for feed := range feedChan {
				created, updated, feedErrors, _ := processFeed(feed)

				mu.Lock()
				itemsCreated += created
				itemsUpdated += updated
				errorsCount += feedErrors
				mu.Unlock()
			}
		}()
	}

	for _, feed := range feeds {
		feedChan <- feed
	}
	close(feedChan)

	wg.Wait()

	return itemsCreated, itemsUpdated, errorsCount
}

func ProcessSingleFeed(feedID uint) (itemsCreated, itemsUpdated int, err error) {
	ensurePrimaryDatabase()

	var feed models.Feed
	if err := database.DB.First(&feed, feedID).Error; err != nil {
		return 0, 0, err
	}

	itemsCreated, itemsUpdated, _, err = processFeed(feed)
	return itemsCreated, itemsUpdated, err
}

func StartBackgroundFeedFetcher() {
	interval := config.GetBackgroundFetchInterval()
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	app.Logger.Info().Int("interval", interval).Msg("Starting background feed fetcher")
	itemsCreated, itemsUpdated, errorsCount := ProcessAllFeeds()
	app.Logger.Info().
		Int("items_created", itemsCreated).
		Int("items_updated", itemsUpdated).
		Int("errors", errorsCount).
		Msg("Initial feed fetch completed")

	for range ticker.C {
		app.Logger.Info().Msg("Background feed fetch started")
		itemsCreated, itemsUpdated, errorsCount = ProcessAllFeeds()
		app.Logger.Info().
			Int("items_created", itemsCreated).
			Int("items_updated", itemsUpdated).
			Int("errors", errorsCount).
			Msg("Background feed fetch completed")
	}
}

func processFeed(feed models.Feed) (itemsCreated, itemsUpdated, errorsCount int, err error) {
	parsedFeed, err := gofeed.NewParser().ParseURL(feed.URL)
	if err != nil {
		recordFeedFetchError(&feed, err)
		return 0, 0, 1, err
	}

	itemsCreated, itemsUpdated, errorsCount = processParsedFeed(&feed, parsedFeed)
	return itemsCreated, itemsUpdated, errorsCount, nil
}

func processParsedFeed(feed *models.Feed, parsedFeed *gofeed.Feed) (itemsCreated, itemsUpdated, errorsCount int) {
	if err := updateFeedAfterSuccess(feed, parsedFeed); err != nil {
		app.Logger.Error().Err(err).Str("feed_url", feed.URL).Msg("Error saving feed metadata")
		errorsCount++
	}

	for _, item := range parsedFeed.Items {
		created, updated, err := upsertFeedItem(feed.ID, item)
		if err != nil {
			app.Logger.Error().Err(err).Str("feed_url", feed.URL).Msg("Error processing item")
			errorsCount++
			continue
		}

		if created {
			itemsCreated++
			IncrementItemsCreatedStats()
		}
		if updated {
			itemsUpdated++
		}
	}

	AddLogEntry(
		"success",
		feed.URL,
		fmt.Sprintf("Successfully fetched feed: %d created, %d updated", itemsCreated, itemsUpdated),
	)

	return itemsCreated, itemsUpdated, errorsCount
}

func updateFeedAfterSuccess(feed *models.Feed, parsedFeed *gofeed.Feed) error {
	if parsedFeed.Title != "" {
		feed.Title = parsedFeed.Title
	}
	if parsedFeed.Description != "" {
		feed.Description = parsedFeed.Description
	}

	now := time.Now()
	feed.LastSuccessfullyFetchedAt = &now
	feed.LastError = ""
	feed.LastErrorAt = nil

	return database.DB.Save(feed).Error
}

func recordFeedFetchError(feed *models.Feed, err error) {
	app.Logger.Error().Str("feed_url", feed.URL).Err(err).Msg("Error parsing feed")

	now := time.Now()
	feed.LastError = err.Error()
	feed.LastErrorAt = &now
	if saveErr := database.DB.Save(feed).Error; saveErr != nil {
		app.Logger.Error().Str("feed_url", feed.URL).Err(saveErr).Msg("Error saving feed fetch failure")
	}

	AddLogEntry("error", feed.URL, fmt.Sprintf("Failed to fetch feed: %v", err))
}

func upsertFeedItem(feedID uint, item *gofeed.Item) (created, updated bool, err error) {
	guid := item.GUID
	if guid == "" {
		guid = item.Link
	}

	var publishedAt *time.Time
	if item.PublishedParsed != nil {
		publishedAt = item.PublishedParsed
	} else if item.UpdatedParsed != nil {
		publishedAt = item.UpdatedParsed
	}

	description := validation.SanitizeHTML(item.Description)
	content := validation.SanitizeHTML(getItemContent(item))
	author := getItemAuthor(item)

	var existingItem models.Item
	result := database.DB.Where("guid = ? AND feed_id = ?", guid, feedID).First(&existingItem)

	switch {
	case errors.Is(result.Error, gorm.ErrRecordNotFound):
		newItem := models.Item{
			FeedID:      feedID,
			Title:       item.Title,
			Link:        item.Link,
			Description: description,
			Content:     content,
			Author:      author,
			PublishedAt: publishedAt,
			GUID:        guid,
		}
		if err := database.DB.Create(&newItem).Error; err != nil {
			return false, false, err
		}
		return true, false, nil
	case result.Error != nil:
		return false, false, result.Error
	default:
		existingItem.Title = item.Title
		existingItem.Link = item.Link
		existingItem.Description = description
		existingItem.Content = content
		existingItem.Author = author
		if publishedAt != nil {
			existingItem.PublishedAt = publishedAt
		}
		if err := database.DB.Save(&existingItem).Error; err != nil {
			return false, false, err
		}
		return false, true, nil
	}
}

func getItemContent(item *gofeed.Item) string {
	if item.Content != "" {
		return item.Content
	}
	return item.Description
}

func getItemAuthor(item *gofeed.Item) string {
	if item.Author != nil && item.Author.Name != "" {
		return item.Author.Name
	}
	if len(item.Authors) > 0 && item.Authors[0].Name != "" {
		return item.Authors[0].Name
	}
	return ""
}
