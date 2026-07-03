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
		"https://rss.nytimes.com/services/xml/rss/nyt/HomePage.xml",
		"https://www.cbsnews.com/feeds/rss/main.rss",
		"https://www.theguardian.com/world/rss",
		"https://feeds.nbcnews.com/nbcnews/public/news",
		"https://www.theguardian.com/international/rss",
		"https://feeds.skynews.com/feeds/rss/home.xml",
		"https://feeds.content.dowjones.io/public/rss/RSSWorldNews",
		"https://abcnews.go.com/abcnews/topstories",
		"https://www.telegraph.co.uk/rss.xml",
		"https://www.wired.com/feed/rss",
		"http://rss.cnn.com/rss/cnn_world.rss",
		"http://rss.cnn.com/rss/cnn_topstories.rss",
		"http://feeds.bbci.co.uk/news/world/rss.xml",
		"http://feeds.bbci.co.uk/news/rss.xml",
		"https://habr.com/ru/rss/all/all/?fl=ru",
		"https://www.ixbt.com/export/news.rss",
		"https://ru.euronews.com/rss",
		"https://www.euronews.com/rss",
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
