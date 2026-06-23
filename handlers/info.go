package handlers

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"go-rss-ui/config"
	"go-rss-ui/database"
	"go-rss-ui/models"
	"go-rss-ui/services"
)

// EnvVarInfo contains information about an environment variable
type EnvVarInfo struct {
	Name        string
	Value       string
	Description string
}

func ShowInfo(c *gin.Context) {
	// Get statistics
	var feedsCount int64
	database.DB.Model(&models.Feed{}).Count(&feedsCount)

	var itemsCount int64
	database.DB.Model(&models.Item{}).Count(&itemsCount)

	// Get last successful fetch
	var lastSuccessFeed models.Feed
	var lastSuccessTime *time.Time
	database.DB.Where("last_successfully_fetched_at IS NOT NULL").
		Order("last_successfully_fetched_at DESC").
		First(&lastSuccessFeed)
	if lastSuccessFeed.LastSuccessfullyFetchedAt != nil {
		lastSuccessTime = lastSuccessFeed.LastSuccessfullyFetchedAt
	}

	// Get last error fetch
	var lastErrorFeed models.Feed
	var lastErrorTime *time.Time
	var lastError string
	database.DB.Where("last_error_at IS NOT NULL").
		Order("last_error_at DESC").
		First(&lastErrorFeed)
	if lastErrorFeed.LastErrorAt != nil {
		lastErrorTime = lastErrorFeed.LastErrorAt
		lastError = lastErrorFeed.LastError
	}

	// Get environment variables
	envVars := []EnvVarInfo{
		{
			Name:        "RSS_DATABASE_URL",
			Value:       maskPassword(os.Getenv("RSS_DATABASE_URL")),
			Description: "Complete PostgreSQL connection string (takes precedence over individual RSS_DB_* variables)",
		},
		{
			Name:        "RSS_DB_HOST",
			Value:       config.GetEnvOrDefault("RSS_DB_HOST", "localhost (default)"),
			Description: "PostgreSQL database host",
		},
		{
			Name:        "RSS_DB_USER",
			Value:       config.GetEnvOrDefault("RSS_DB_USER", "postgres (default)"),
			Description: "PostgreSQL database user",
		},
		{
			Name:        "RSS_DB_PASSWORD",
			Value:       maskPassword(config.GetEnvOrDefault("RSS_DB_PASSWORD", "postgres (default)")),
			Description: "PostgreSQL database password",
		},
		{
			Name:        "RSS_DB_NAME",
			Value:       config.GetEnvOrDefault("RSS_DB_NAME", "go_rss_ui_2 (default)"),
			Description: "PostgreSQL database name",
		},
		{
			Name:        "RSS_DB_PORT",
			Value:       config.GetEnvOrDefault("RSS_DB_PORT", "5432 (default)"),
			Description: "PostgreSQL database port",
		},
		{
			Name:        "RSS_DB_SSLMODE",
			Value:       config.GetEnvOrDefault("RSS_DB_SSLMODE", "disable (default)"),
			Description: "PostgreSQL SSL mode",
		},
		{
			Name:        "RSS_DB_TIMEZONE",
			Value:       config.GetEnvOrDefault("RSS_DB_TIMEZONE", "Asia/Shanghai (default)"),
			Description: "PostgreSQL timezone",
		},
		{
			Name:        "RSS_BACKGROUND_FETCH_ENABLED",
			Value:       getEnvValueOrDefault("RSS_BACKGROUND_FETCH_ENABLED", "true (default)"),
			Description: "Enable/disable background feed fetching",
		},
		{
			Name:        "RSS_BACKGROUND_FETCH_INTERVAL",
			Value:       fmt.Sprintf("%d (default: 60)", config.GetBackgroundFetchInterval()),
			Description: "Background feed fetch interval in seconds",
		},
		{
			Name:        "RSS_CYPRESS",
			Value:       getEnvValueOrDefault("RSS_CYPRESS", "false (default)"),
			Description: "Enable Cypress mode (enables /tools page for testing)",
		},
		{
			Name:        "RSS_PORT",
			Value:       config.GetEnvOrDefault("RSS_PORT", "8082 (default)"),
			Description: "Server port",
		},
		{
			Name:        "RSS_REDIS_HOST",
			Value:       config.GetEnvOrDefault("RSS_REDIS_HOST", "localhost (default)"),
			Description: "Redis host",
		},
		{
			Name:        "RSS_REDIS_PORT",
			Value:       config.GetEnvOrDefault("RSS_REDIS_PORT", "6379 (default)"),
			Description: "Redis port",
		},
		{
			Name:        "RSS_REDIS_PASSWORD",
			Value:       maskPassword(config.GetEnvOrDefault("RSS_REDIS_PASSWORD", "(empty)")),
			Description: "Redis password",
		},
	}

	data := getTemplateData(c, gin.H{
		"title":           "System Information",
		"feedsCount":      feedsCount,
		"itemsCount":      itemsCount,
		"lastSuccessTime": lastSuccessTime,
		"lastSuccessFeed": lastSuccessFeed,
		"lastErrorTime":   lastErrorTime,
		"lastError":       lastError,
		"lastErrorFeed":   lastErrorFeed,
		"envVars":         envVars,
	})
	c.HTML(http.StatusOK, "info.html", data)
}

func DumpDBStructureAdmin(c *gin.Context) {
	session := sessions.Default(c)

	err := services.DumpDBStructure()
	if err != nil {
		addFlashError(session, "Failed to dump database structure: "+err.Error())
		saveSession(session)
		c.Redirect(http.StatusFound, "/admin/info")
		return
	}

	addFlashSuccess(session, "Database structure dumped successfully to structure.sql")
	saveSession(session)
	c.Redirect(http.StatusFound, "/admin/info")
}

// maskPassword masks password in connection strings
func maskPassword(value string) string {
	if value == "" {
		return "(not set)"
	}
	// Handle postgres://user:password@host format
	if strings.HasPrefix(value, "postgres://") || strings.HasPrefix(value, "postgresql://") {
		parts := strings.Split(value, "@")
		if len(parts) == 2 {
			authPart := parts[0]
			if strings.Contains(authPart, ":") {
				authParts := strings.Split(authPart, ":")
				if len(authParts) >= 3 {
					// postgres://user:password@host
					return fmt.Sprintf("%s:%s:***@%s", authParts[0], authParts[1], parts[1])
				}
			}
		}
	}
	// If it's a connection string with password=, mask the password
	if strings.Contains(value, "password=") {
		parts := strings.Split(value, " ")
		for i, part := range parts {
			if strings.HasPrefix(part, "password=") {
				parts[i] = "password=***"
				break
			}
		}
		return strings.Join(parts, " ")
	}
	// Otherwise, just mask the whole value if it looks like a password
	if len(value) > 0 && value != "(not set)" {
		return "***"
	}
	return value
}

// getEnvValueOrDefault returns the value or default string
func getEnvValueOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
