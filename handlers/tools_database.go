package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"go-rss-ui/services"
)

func DropDB(c *gin.Context) {
	if !requireCypressMode(c) {
		return
	}

	session := sessions.Default(c)
	dbname := services.GetDBName()

	if err := services.DropDatabase(); err != nil {
		addFlashError(session, "Failed to drop database: "+err.Error())
		saveSession(session)
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	addFlashSuccess(session, fmt.Sprintf("Database '%s' dropped successfully", dbname))
	saveSession(session)
	c.Redirect(http.StatusFound, "/tools")
}

func DropAllTables(c *gin.Context) {
	if !requireCypressMode(c) {
		return
	}

	session := sessions.Default(c)
	result, err := services.DropAllTables()
	if err != nil {
		addFlashError(session, err.Error())
		saveSession(session)
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	if len(result.TableNames) == 0 {
		addFlashSuccess(session, "No tables found in database")
		saveSession(session)
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	if len(result.Errors) > 0 {
		errorMsg := fmt.Sprintf(
			"Dropped %d table(s), but encountered errors: %s",
			result.DroppedCount,
			strings.Join(result.Errors, "; "),
		)
		addFlashError(session, errorMsg)
	} else {
		addFlashSuccess(session, fmt.Sprintf("Successfully dropped %d table(s)", result.DroppedCount))
	}

	saveSession(session)
	c.Redirect(http.StatusFound, "/tools")
}

func CreateDB(c *gin.Context) {
	if !requireCypressMode(c) {
		return
	}

	session := sessions.Default(c)
	dbname := services.GetDBName()

	created, err := services.CreateDatabase()
	if err != nil {
		addFlashError(session, "Failed to create database: "+err.Error())
		saveSession(session)
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	if created {
		addFlashSuccess(session, fmt.Sprintf("Database '%s' created successfully", dbname))
	} else {
		addFlashSuccess(session, fmt.Sprintf("Database '%s' already exists", dbname))
	}

	saveSession(session)
	c.Redirect(http.StatusFound, "/tools")
}

func Migrate(c *gin.Context) {
	if !requireCypressMode(c) {
		return
	}

	session := sessions.Default(c)
	if err := services.MigrateDatabase(); err != nil {
		addFlashError(session, "Failed to migrate database: "+err.Error())
		saveSession(session)
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	addFlashSuccess(session, "Database migration completed successfully")
	saveSession(session)
	c.Redirect(http.StatusFound, "/tools")
}

func DumpDBStructure(c *gin.Context) {
	if !requireCypressMode(c) {
		return
	}

	session := sessions.Default(c)
	if err := services.DumpDBStructure(); err != nil {
		addFlashError(session, "Failed to dump database structure: "+err.Error())
		saveSession(session)
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	addFlashSuccess(session, "Database structure dumped successfully to structure.sql")
	saveSession(session)
	c.Redirect(http.StatusFound, "/tools")
}
