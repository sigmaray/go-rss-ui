package services

import (
	"errors"

	"go-rss-ui/database"
	"go-rss-ui/models"
	"gorm.io/gorm"
)

type SeedFeedsResult struct {
	Created int
	Existed int
	Errors  int
}

func DefaultFeedURLs() []string {
	return []string{
		"https://feeds.bbci.co.uk/news/rss.xml",
		"http://rss.cnn.com/rss/cnn_topstories.rss",
		"https://www.wired.com/feed/rss",
		"https://habr.com/ru/rss/articles/?fl=ru",
		"https://www.telegraph.co.uk/rss.xml",
		"https://abcnews.go.com/abcnews/topstories",
		"https://feeds.content.dowjones.io/public/rss/RSSWorldNews",
		"https://feeds.skynews.com/feeds/rss/home.xml",
		"https://www.theguardian.com/international/rss",
		"https://feeds.nbcnews.com/nbcnews/public/news",
		"https://www.theguardian.com/world/rss",
		"https://xkcd.com/atom.xml",
		"https://fedoramagazine.org/feed/",
		"https://planet.gnome.org//rss20.xml",
		"https://hacks.mozilla.org/feed/",
		"https://www.reddit.com/r/all/new/.rss",
	}
}

func SeedFeeds() SeedFeedsResult {
	return SeedFeedsWithURLs(DefaultFeedURLs())
}

func SeedFeedsWithURLs(feedURLs []string) SeedFeedsResult {
	ensurePrimaryDatabase()

	result := SeedFeedsResult{}

	for _, feedURL := range feedURLs {
		var feed models.Feed
		dbResult := database.DB.Where("url = ?", feedURL).First(&feed)

		switch {
		case errors.Is(dbResult.Error, gorm.ErrRecordNotFound):
			if err := database.DB.Create(&models.Feed{URL: feedURL}).Error; err != nil {
				result.Errors++
			} else {
				result.Created++
			}
		case dbResult.Error != nil:
			result.Errors++
		default:
			result.Existed++
		}
	}

	return result
}
