package handlers

import (
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"go-rss-ui/database"
	"go-rss-ui/models"
	"go-rss-ui/services"
	"go-rss-ui/validation"
)

// Item handlers
func ItemsIndex(c *gin.Context) {
	var items []models.Item
	model := database.DB.Model(&models.Item{}).Preload("Feed")

	// Filter by feed if provided
	if feedID := c.Query("feed_id"); feedID != "" {
		model = model.Where("feed_id = ?", feedID)
	}

	model = model.Order("created_at DESC")
	page := database.Paginator.With(model).Request(c.Request).Response(&items)

	data := gin.H{
		"title": "Items",
		"items": page.Items,
	}

	data = addPaginationData(data, page, "/admin/items", "items")

	// Check for error in query parameter (for backward compatibility)
	if queryError := c.Query("error"); queryError != "" {
		if _, exists := c.Get("error"); !exists {
			data["error"] = queryError
		}
	}

	data = getTemplateData(c, data)
	c.HTML(http.StatusOK, "items.html", data)
}

func ShowItem(c *gin.Context) {
	id := c.Param("id")

	var item models.Item
	if err := database.DB.Preload("Feed").First(&item, id).Error; err != nil {
		// Show 404 page instead of redirecting
		data := getTemplateData(c, gin.H{
			"title": "404 - Item Not Found",
			"error": "Item not found",
		})
		c.HTML(http.StatusNotFound, "item.html", data)
		return
	}

	// Sanitize HTML content before displaying (defense in depth - already sanitized when saved)
	sanitizedDescription := validation.SanitizeHTML(item.Description)
	sanitizedContent := validation.SanitizeHTML(item.Content)

	// Convert Description and Content to template.HTML for safe HTML rendering
	itemData := gin.H{
		"ID":          item.ID,
		"FeedID":      item.FeedID,
		"Title":       item.Title,
		"Link":        item.Link,
		"Author":      item.Author,
		"PublishedAt": item.PublishedAt,
		"CreatedAt":   item.CreatedAt,
		"UpdatedAt":   item.UpdatedAt,
		"Feed":        item.Feed,
		"Description": template.HTML(sanitizedDescription),
		"Content":     template.HTML(sanitizedContent),
	}

	data := getTemplateData(c, gin.H{
		"title": item.Title,
		"item":  itemData,
	})
	c.HTML(http.StatusOK, "item.html", data)
}

func DeleteAllItems(c *gin.Context) {
	session := sessions.Default(c)
	result := database.DB.Delete(&models.Item{}, "1 = 1")
	if result.Error != nil {
		addFlashError(session, "Failed to delete all items")
		saveSession(session)
		c.Redirect(http.StatusFound, "/admin/items")
		return
	}
	addFlashSuccess(session, "All items deleted successfully")
	saveSession(session)
	c.Redirect(http.StatusFound, "/admin/items")
}

func FetchFeedItems(c *gin.Context) {
	session := sessions.Default(c)
	var feeds []models.Feed
	database.DB.Find(&feeds)

	if len(feeds) == 0 {
		addFlashError(session, "No feeds available")
		saveSession(session)
		c.Redirect(http.StatusFound, "/admin/items")
		return
	}

	itemsCreated, itemsUpdated, errors := services.ProcessAllFeeds()

	successMsg := fmt.Sprintf("Fetched items: %d created, %d updated", itemsCreated, itemsUpdated)
	if errors > 0 {
		successMsg += fmt.Sprintf(", %d errors", errors)
	}
	addFlashSuccess(session, successMsg)
	if err := session.Save(); err != nil {
		log.Printf("Error saving session in FetchFeedItems: %v", err)
	}
	c.Redirect(http.StatusFound, "/admin/items")
}
