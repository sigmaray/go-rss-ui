package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"go-rss-ui/services"
)

func ClearAllTables(c *gin.Context) {
	if !requireCypressMode(c) {
		return
	}

	session := sessions.Default(c)
	if err := services.ClearAllCoreTables(); err != nil {
		addFlashError(session, "Failed to clear tables: "+err.Error())
		saveSession(session)
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	addFlashSuccess(session, "All tables cleared successfully")
	saveSession(session)
	c.Redirect(http.StatusFound, "/tools")
}

func ClearTable(c *gin.Context) {
	if !requireCypressMode(c) {
		return
	}

	session := sessions.Default(c)
	tableName := c.PostForm("name")
	if tableName == "" {
		addFlashError(session, "Table name is required")
		saveSession(session)
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	if err := services.ClearCoreTable(tableName); err != nil {
		addFlashError(session, err.Error())
		saveSession(session)
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	addFlashSuccess(session, fmt.Sprintf("Table '%s' cleared successfully", tableName))
	saveSession(session)
	c.Redirect(http.StatusFound, "/tools")
}
