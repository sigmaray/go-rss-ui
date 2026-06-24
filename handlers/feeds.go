package handlers

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"go-rss-ui/config"
	"go-rss-ui/database"
	"go-rss-ui/models"
	"go-rss-ui/services"
	"go-rss-ui/validation"
)

// Feed handlers
func FeedsIndex(c *gin.Context) {
	var feeds []models.Feed
	model := database.DB.Model(&models.Feed{}).Order("created_at DESC")
	page := database.Paginator.With(model).Request(c.Request).Response(&feeds)

	data := gin.H{
		"title": "Feed Management",
		"feeds": page.Items,
	}

	// Add pagination data
	data = addPaginationData(data, page, "/admin/feeds", "feeds")

	data = getTemplateData(c, data)
	c.HTML(http.StatusOK, "feeds/index.html", data)
}

func ShowCreateFeedForm(c *gin.Context) {
	data := getTemplateData(c, gin.H{
		"title": "Create New Feed",
	})
	c.HTML(http.StatusOK, "feeds/create.html", data)
}

func CreateFeed(c *gin.Context) {
	// Create input struct from form data
	input := validation.FeedInput{
		URL: c.PostForm("url"),
	}

	// Validate input using validator/v10
	if err := validation.ValidateStruct(input); err != nil {
		data := getTemplateData(c, gin.H{
			"title": "Create New Feed",
			"error": validation.FormatValidationErrors(err),
		})
		c.HTML(http.StatusBadRequest, "feeds/create.html", data)
		return
	}

	// Check if feed URL already exists
	var existingFeed models.Feed
	if err := database.DB.Where("url = ?", input.URL).First(&existingFeed).Error; err == nil {
		data := getTemplateData(c, gin.H{
			"title": "Create New Feed",
			"error": "Feed URL already exists",
		})
		c.HTML(http.StatusBadRequest, "feeds/create.html", data)
		return
	}

	feed := models.Feed{URL: input.URL}
	if err := database.DB.Create(&feed).Error; err != nil {
		if isUniqueConstraintError(err) {
			data := getTemplateData(c, gin.H{
				"title": "Create New Feed",
				"error": "Feed URL already exists",
			})
			c.HTML(http.StatusBadRequest, "feeds/create.html", data)
			return
		}
		data := getTemplateData(c, gin.H{
			"title": "Create New Feed",
			"error": "Failed to create feed: " + err.Error(),
		})
		c.HTML(http.StatusInternalServerError, "feeds/create.html", data)
		return
	}

	session := sessions.Default(c)
	addFlashSuccess(session, "Feed created successfully")
	if err := session.Save(); err != nil {
		log.Printf("Error saving session in CreateFeed: %v", err)
	}
	c.Redirect(http.StatusFound, "/admin/feeds")
}

func FetchSingleFeed(c *gin.Context) {
	id := c.Param("id")
	session := sessions.Default(c)

	var feed models.Feed
	if err := database.DB.First(&feed, id).Error; err != nil {
		addFlashError(session, "Feed not found")
		saveSession(session)
		c.Redirect(http.StatusFound, "/admin/feeds")
		return
	}

	itemsCreated, itemsUpdated, err := services.ProcessSingleFeed(feed.ID)
	if err != nil {
		addFlashError(session, fmt.Sprintf("Failed to fetch feed: %v", err))
		saveSession(session)
		c.Redirect(http.StatusFound, "/admin/feeds")
		return
	}

	successMsg := fmt.Sprintf("Feed fetched successfully: %d items created, %d items updated", itemsCreated, itemsUpdated)
	addFlashSuccess(session, successMsg)
	saveSession(session)
	c.Redirect(http.StatusFound, "/admin/feeds")
}

func DeleteFeed(c *gin.Context) {
	id := c.Param("id")
	session := sessions.Default(c)

	var feed models.Feed
	if err := database.DB.First(&feed, id).Error; err != nil {
		addFlashError(session, "Feed not found")
		saveSession(session)
		c.Redirect(http.StatusFound, "/admin/feeds")
		return
	}

	// Items will be deleted automatically due to CASCADE constraint
	if err := database.DB.Unscoped().Delete(&feed).Error; err != nil {
		addFlashError(session, "Failed to delete feed: "+err.Error())
		saveSession(session)
		c.Redirect(http.StatusFound, "/admin/feeds")
		return
	}

	addFlashSuccess(session, "Feed deleted successfully")
	saveSession(session)
	c.Redirect(http.StatusFound, "/admin/feeds")
}

func DeleteAllFeeds(c *gin.Context) {
	session := sessions.Default(c)

	// Delete all feeds (items will be deleted automatically due to CASCADE constraint)
	result := database.DB.Unscoped().Delete(&models.Feed{}, "1 = 1")
	if result.Error != nil {
		addFlashError(session, "Failed to delete all feeds")
		saveSession(session)
		c.Redirect(http.StatusFound, "/admin/feeds")
		return
	}

	addFlashSuccess(session, "All feeds deleted successfully")
	saveSession(session)
	c.Redirect(http.StatusFound, "/admin/feeds")
}

func SeedFeeds(c *gin.Context) {
	// Check if this is called from /tools (CYPRESS mode required)
	if c.Request.URL.Path == "/tools/seed-feeds" {
		if !config.IsCypressMode() {
			c.JSON(http.StatusForbidden, gin.H{"error": "Tools are only available when CYPRESS=true"})
			return
		}
	}

	session := sessions.Default(c)

	result := services.SeedFeeds()

	successMsg := fmt.Sprintf("Seeded feeds: %d created", result.Created)
	if result.Existed > 0 {
		successMsg += fmt.Sprintf(", %d already existed", result.Existed)
	}
	if result.Errors > 0 {
		successMsg += fmt.Sprintf(", %d errors", result.Errors)
	}

	addFlashSuccess(session, successMsg)
	saveSession(session)

	// Redirect based on where the request came from
	if c.Request.URL.Path == "/tools/seed-feeds" {
		c.Redirect(http.StatusFound, "/tools")
	} else {
		c.Redirect(http.StatusFound, "/admin/feeds")
	}
}

func ShowFeed(c *gin.Context) {
	id := c.Param("id")
	session := sessions.Default(c)

	var feed models.Feed
	if err := database.DB.First(&feed, id).Error; err != nil {
		addFlashError(session, "Feed not found")
		saveSession(session)
		c.Redirect(http.StatusFound, "/admin/feeds")
		return
	}

	// Get items for this feed with pagination
	var items []models.Item
	model := database.DB.Model(&models.Item{}).Where("feed_id = ?", feed.ID).Order("created_at DESC")
	page := database.Paginator.With(model).Request(c.Request).Response(&items)

	data := gin.H{
		"title": "Feed Details",
		"feed":  feed,
		"items": page.Items,
	}

	// Add pagination data
	data = addPaginationData(data, page, fmt.Sprintf("/admin/feeds/%s", id), "items")

	data = getTemplateData(c, data)
	c.HTML(http.StatusOK, "feeds/show.html", data)
}
