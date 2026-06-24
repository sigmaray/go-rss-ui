package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go-rss-ui/database"
	"go-rss-ui/models"
)

// ListItems godoc
// @Summary      List items
// @Description  Returns paginated list of RSS items
// @Tags         items
// @Produce      json
// @Param        page     query     int  false  "Page number"  default(1)
// @Param        feed_id  query     int  false  "Filter by feed ID"
// @Success      200      {object}  PaginatedItemsResponse
// @Failure      401      {object}  ErrorResponse
// @Security     CookieAuth
// @Router       /items [get]
func ListItems(c *gin.Context) {
	var items []models.Item
	model := database.DB.Model(&models.Item{}).Preload("Feed")

	if feedID := c.Query("feed_id"); feedID != "" {
		model = model.Where("feed_id = ?", feedID)
	}

	model = model.Order("created_at DESC")
	page := database.Paginator.With(model).Request(c.Request).Response(&items)

	respItems := make([]ItemResponse, 0, len(items))
	for _, item := range items {
		respItems = append(respItems, toItemResponse(item, true))
	}

	c.JSON(http.StatusOK, PaginatedItemsResponse{
		Page:       page.Page,
		TotalPages: page.TotalPages,
		Total:      page.Total,
		Items:      respItems,
	})
}

// GetItem godoc
// @Summary      Get item
// @Description  Returns a single RSS item by ID
// @Tags         items
// @Produce      json
// @Param        id   path      int  true  "Item ID"
// @Success      200  {object}  ItemResponse
// @Failure      400  {object}  ErrorResponse
// @Failure      401  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse
// @Security     CookieAuth
// @Router       /items/{id} [get]
func GetItem(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var item models.Item
	if err := database.DB.Preload("Feed").First(&item, id).Error; err != nil {
		notFound(c, "item")
		return
	}

	c.JSON(http.StatusOK, toItemResponse(item, true))
}
