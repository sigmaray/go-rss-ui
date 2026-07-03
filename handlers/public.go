package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go-rss-ui/database"
	"go-rss-ui/models"
	"go-rss-ui/validation"
)

func Home(c *gin.Context) {
	var items []models.Item
	model := database.DB.Model(&models.Item{}).Preload("Feed").Order("created_at DESC")

	// Filter by feed_id if provided
	if feedID := c.Query("feed_id"); feedID != "" {
		model = model.Where("feed_id = ?", feedID)
	}

	page := database.Paginator.With(model).Request(c.Request).Response(&items)

	// Sanitize HTML content for each item and convert to template.HTML
	type ItemWithSanitizedContent struct {
		models.Item
		SanitizedContent template.HTML
	}
	itemsWithSanitized := make([]ItemWithSanitizedContent, len(items))
	for i, item := range items {
		itemsWithSanitized[i] = ItemWithSanitizedContent{
			Item:             item,
			SanitizedContent: template.HTML(validation.SanitizeHTML(item.Content)),
		}
	}

	// Get last 20 feeds that have items (ordered by most recent item created_at)
	// Get feed IDs with their latest item timestamp, then fetch the feeds
	type FeedWithLatestItem struct {
		FeedID              uint      `gorm:"column:feed_id"`
		LatestItemCreatedAt time.Time `gorm:"column:latest_item_created_at"`
	}
	var feedIDsWithLatest []FeedWithLatestItem
	database.DB.Raw(`
		SELECT feed_id, MAX(created_at) as latest_item_created_at
		FROM items
		GROUP BY feed_id
		ORDER BY latest_item_created_at DESC
		LIMIT 20
	`).Scan(&feedIDsWithLatest)

	// Extract feed IDs
	feedIDs := make([]uint, len(feedIDsWithLatest))
	for i, f := range feedIDsWithLatest {
		feedIDs[i] = f.FeedID
	}

	var recentFeeds []models.Feed
	if len(feedIDs) > 0 {
		// Create a map to preserve order
		feedMap := make(map[uint]models.Feed)
		database.DB.Where("id IN ?", feedIDs).Find(&recentFeeds)
		for _, feed := range recentFeeds {
			feedMap[feed.ID] = feed
		}
		// Reorder according to feedIDs order
		recentFeeds = make([]models.Feed, 0, len(feedIDs))
		for _, id := range feedIDs {
			if feed, ok := feedMap[id]; ok {
				recentFeeds = append(recentFeeds, feed)
			}
		}
	}

	// Build pagination URL with feed_id if present
	paginationURL := "/"
	if feedID := c.Query("feed_id"); feedID != "" {
		paginationURL = fmt.Sprintf("/?feed_id=%s", feedID)
	}

	data := gin.H{
		"title":       "RSS Feeds",
		"items":       itemsWithSanitized,
		"recentFeeds": recentFeeds,
		"feedID":      c.Query("feed_id"),
	}

	// Add pagination data
	data = addPaginationData(data, page, paginationURL, "items")

	data = getTemplateData(c, data)
	c.HTML(http.StatusOK, "index.html", data)
}

func PublicFeeds(c *gin.Context) {
	var feeds []models.Feed
	model := database.DB.Model(&models.Feed{}).Order("created_at DESC")
	page := database.Paginator.With(model).Request(c.Request).Response(&feeds)

	data := gin.H{
		"title": "All Feeds",
		"feeds": feeds,
	}

	// Add pagination data
	data = addPaginationData(data, page, "/feeds", "feeds")

	data = getTemplateData(c, data)
	c.HTML(http.StatusOK, "all_feeds.html", data)
}

func TestFeeds(c *gin.Context) {
	filepath := c.Param("filepath")

	// Handle error endpoints
	if filepath == "/error404.xml" {
		c.Header("Content-Type", "text/plain")
		c.String(http.StatusNotFound, "404 Not Found")
		return
	}
	if filepath == "/error500.xml" {
		c.Header("Content-Type", "text/plain")
		c.String(http.StatusInternalServerError, "500 Internal Server Error")
		return
	}

	// Serve static file for all other paths
	c.File("./test_feeds" + filepath)
}
