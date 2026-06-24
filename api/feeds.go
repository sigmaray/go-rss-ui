package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"go-rss-ui/database"
	"go-rss-ui/models"
	"go-rss-ui/services"
	"go-rss-ui/validation"
)

// ListFeeds godoc
// @Summary      List feeds
// @Description  Returns paginated list of RSS feeds
// @Tags         feeds
// @Produce      json
// @Param        page  query     int  false  "Page number"  default(1)
// @Success      200   {object}  PaginatedFeedsResponse
// @Failure      401   {object}  ErrorResponse
// @Security     CookieAuth
// @Router       /feeds [get]
func ListFeeds(c *gin.Context) {
	var feeds []models.Feed
	model := database.DB.Model(&models.Feed{}).Order("created_at DESC")
	page := database.Paginator.With(model).Request(c.Request).Response(&feeds)

	items := make([]FeedResponse, 0, len(feeds))
	for _, feed := range feeds {
		items = append(items, toFeedResponse(feed))
	}

	c.JSON(http.StatusOK, PaginatedFeedsResponse{
		Page:       page.Page,
		TotalPages: page.TotalPages,
		Total:      page.Total,
		Items:      items,
	})
}

// GetFeed godoc
// @Summary      Get feed
// @Description  Returns a single feed by ID
// @Tags         feeds
// @Produce      json
// @Param        id   path      int  true  "Feed ID"
// @Success      200  {object}  FeedResponse
// @Failure      400  {object}  ErrorResponse
// @Failure      401  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse
// @Security     CookieAuth
// @Router       /feeds/{id} [get]
func GetFeed(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var feed models.Feed
	if err := database.DB.First(&feed, id).Error; err != nil {
		notFound(c, "feed")
		return
	}

	c.JSON(http.StatusOK, toFeedResponse(feed))
}

// CreateFeed godoc
// @Summary      Create feed
// @Description  Add a new RSS feed URL
// @Tags         feeds
// @Accept       json
// @Produce      json
// @Param        feed  body      validation.FeedInput  true  "Feed data"
// @Success      201   {object}  FeedResponse
// @Failure      400   {object}  ErrorResponse
// @Failure      401   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Security     CookieAuth
// @Router       /feeds [post]
func CreateFeed(c *gin.Context) {
	var input validation.FeedInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	if err := validation.ValidateStruct(input); err != nil {
		validationError(c, err)
		return
	}

	var existingFeed models.Feed
	if err := database.DB.Where("url = ?", input.URL).First(&existingFeed).Error; err == nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "feed URL already exists"})
		return
	} else if !isNotFound(err) {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	feed := models.Feed{URL: input.URL}
	if err := database.DB.Create(&feed).Error; err != nil {
		if isUniqueConstraintError(err) {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "feed URL already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, toFeedResponse(feed))
}

// UpdateFeed godoc
// @Summary      Update feed
// @Description  Update feed URL
// @Tags         feeds
// @Accept       json
// @Produce      json
// @Param        id    path      int                   true  "Feed ID"
// @Param        feed  body      validation.FeedInput  true  "Feed data"
// @Success      200   {object}  FeedResponse
// @Failure      400   {object}  ErrorResponse
// @Failure      401   {object}  ErrorResponse
// @Failure      404   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Security     CookieAuth
// @Router       /feeds/{id} [put]
func UpdateFeed(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var feed models.Feed
	if err := database.DB.First(&feed, id).Error; err != nil {
		notFound(c, "feed")
		return
	}

	var input validation.FeedInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	if err := validation.ValidateStruct(input); err != nil {
		validationError(c, err)
		return
	}

	var existingFeed models.Feed
	if err := database.DB.Where("url = ? AND id != ?", input.URL, id).First(&existingFeed).Error; err == nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "feed URL already exists"})
		return
	} else if !isNotFound(err) {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	feed.URL = input.URL
	if err := database.DB.Save(&feed).Error; err != nil {
		if isUniqueConstraintError(err) {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "feed URL already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, toFeedResponse(feed))
}

// DeleteFeed godoc
// @Summary      Delete feed
// @Description  Permanently delete a feed and its items
// @Tags         feeds
// @Produce      json
// @Param        id   path      int  true  "Feed ID"
// @Success      204  "No Content"
// @Failure      400  {object}  ErrorResponse
// @Failure      401  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Security     CookieAuth
// @Router       /feeds/{id} [delete]
func DeleteFeed(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var feed models.Feed
	if err := database.DB.First(&feed, id).Error; err != nil {
		notFound(c, "feed")
		return
	}

	if err := database.DB.Unscoped().Delete(&feed).Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// FetchFeed godoc
// @Summary      Fetch feed
// @Description  Fetch and process a single RSS feed
// @Tags         feeds
// @Produce      json
// @Param        id   path      int  true  "Feed ID"
// @Success      200  {object}  FetchFeedResponse
// @Failure      400  {object}  ErrorResponse
// @Failure      401  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Security     CookieAuth
// @Router       /feeds/{id}/fetch [post]
func FetchFeed(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var feed models.Feed
	if err := database.DB.First(&feed, id).Error; err != nil {
		notFound(c, "feed")
		return
	}

	itemsCreated, itemsUpdated, err := services.ProcessSingleFeed(feed.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: fmt.Sprintf("failed to fetch feed: %v", err)})
		return
	}

	c.JSON(http.StatusOK, FetchFeedResponse{
		ItemsCreated: itemsCreated,
		ItemsUpdated: itemsUpdated,
	})
}
