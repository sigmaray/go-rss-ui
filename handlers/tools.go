package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go-rss-ui/config"
)

func ShowTools(c *gin.Context) {
	if !requireCypressMode(c) {
		return
	}

	data := getTemplateData(c, gin.H{
		"title": "Tools",
	})
	c.HTML(http.StatusOK, "tools.html", data)
}

func requireCypressMode(c *gin.Context) bool {
	if config.IsCypressMode() {
		return true
	}

	c.JSON(http.StatusForbidden, gin.H{"error": "Tools are only available when CYPRESS=true"})
	return false
}
