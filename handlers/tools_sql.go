package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go-rss-ui/services"
)

func ExecuteSQL(c *gin.Context) {
	if !requireCypressMode(c) {
		return
	}

	sqlQuery := c.PostForm("sql")
	if sqlQuery == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "SQL query is required"})
		return
	}

	result, err := services.RunSQLQuery(sqlQuery)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"columns": result.Columns,
		"rows":    result.Rows,
		"count":   len(result.Rows),
	})
}
