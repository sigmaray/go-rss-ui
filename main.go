package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/gin-contrib/multitemplate"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/mmcdole/gofeed"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// LogEntry represents a single log entry
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"` // "success" or "error"
	FeedURL   string    `json:"feed_url"`
	Message   string    `json:"message"`
}

const (
	fetchLogsRedisKey    = "app:fetch-logs"
	itemsCreatedStatsKey = "app:items:created:stats"
	maxLogSize           = 1000
)

// Global logger and Redis client
var (
	appLogger   *zerolog.Logger
	redisClient *redis.Client
	redisCtx    context.Context
)

// RedisLogWriter implements io.Writer to write logs to Redis
type RedisLogWriter struct {
	client *redis.Client
	ctx    context.Context
	key    string
	maxLen int64
}

func (w *RedisLogWriter) Write(p []byte) (n int, err error) {
	// Parse JSON to check log level
	var logData map[string]interface{}
	if err := json.Unmarshal(p, &logData); err != nil {
		return len(p), nil // Ignore parsing errors
	}

	// Filter only info and error levels
	level, ok := logData["level"].(string)
	if !ok {
		return len(p), nil
	}

	if level == "info" || level == "error" {
		// Use pipeline for atomicity
		pipe := w.client.Pipeline()
		pipe.LPush(w.ctx, w.key, string(p))
		pipe.LTrim(w.ctx, w.key, 0, w.maxLen-1)
		_, _ = pipe.Exec(w.ctx) // Ignore Redis errors to not block logging
	}

	return len(p), nil
}

// initLogger initializes zerolog with Redis writer
func initLogger() {
	redisCtx = context.Background()

	// Create Redis client
	redisAddr := GetRedisAddr()
	password := GetRedisPassword()

	opts := &redis.Options{
		Addr: redisAddr,
	}
	if password != "" {
		opts.Password = password
	}

	redisClient = redis.NewClient(opts)

	// Test Redis connection
	_, err := redisClient.Ping(redisCtx).Result()
	if err != nil {
		// If Redis is not available, log to stdout only
		tempLogger := zerolog.New(os.Stdout).With().
			Timestamp().
			Str("service", "go-rss-ui").
			Logger()
		tempLogger.Warn().Err(err).Msg("Failed to connect to Redis, logging to stdout only")
		appLogger = &tempLogger
		return
	}

	// Create Redis writer
	redisWriter := &RedisLogWriter{
		client: redisClient,
		ctx:    redisCtx,
		key:    "app:logs",
		maxLen: 1000,
	}

	// MultiWriter: log to both stdout and Redis
	multi := io.MultiWriter(
		zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339},
		redisWriter,
	)

	logger := zerolog.New(multi).With().
		Timestamp().
		Str("service", "go-rss-ui").
		Logger()

	appLogger = &logger
	appLogger.Info().Msg("Logger initialized with Redis support")
}

// addLogEntry adds a log entry to Redis storage
// Maintains maximum of 1000 entries by removing oldest entries
func addLogEntry(logType, feedURL, message string) {
	if redisClient == nil {
		// If Redis is not available, skip logging
		return
	}

	entry := LogEntry{
		Timestamp: time.Now(),
		Type:      logType,
		FeedURL:   feedURL,
		Message:   message,
	}

	// Serialize to JSON
	entryJSON, err := json.Marshal(entry)
	if err != nil {
		// If serialization fails, log error but don't block
		appLogger.Error().Err(err).Msg("Failed to serialize log entry")
		return
	}

	// Use pipeline for atomicity
	pipe := redisClient.Pipeline()
	pipe.LPush(redisCtx, fetchLogsRedisKey, string(entryJSON))
	pipe.LTrim(redisCtx, fetchLogsRedisKey, 0, maxLogSize-1)
	_, _ = pipe.Exec(redisCtx) // Ignore Redis errors to not block execution
}

// getLogEntries returns all log entries from Redis
func getLogEntries() []LogEntry {
	var entries []LogEntry

	if redisClient == nil {
		// If Redis is not available, return empty slice
		return entries
	}

	// Get logs from Redis (up to 1000 entries)
	logs, err := redisClient.LRange(redisCtx, fetchLogsRedisKey, 0, maxLogSize-1).Result()
	if err != nil {
		// If error occurs, log it but return empty slice
		appLogger.Error().Err(err).Msg("Failed to get fetch logs from Redis")
		return entries
	}

	// Parse JSON logs
	// Redis LPUSH adds to the left, so LRANGE 0 999 returns newest first
	for _, logJSON := range logs {
		var entry LogEntry
		if err := json.Unmarshal([]byte(logJSON), &entry); err != nil {
			// If parsing fails, skip this entry
			appLogger.Error().Err(err).Msg("Failed to parse fetch log entry")
			continue
		}
		entries = append(entries, entry)
	}

	return entries
}

// incrementItemsCreatedStats increments the counter for items created in the current hour
func incrementItemsCreatedStats() {
	if redisClient == nil {
		return
	}

	// Get current hour in format: YYYY-MM-DD-HH
	now := time.Now()
	hourKey := fmt.Sprintf("%s:%s", itemsCreatedStatsKey, now.Format("2006-01-02-15"))

	// Increment counter for this hour
	redisClient.Incr(redisCtx, hourKey)

	// Set expiration to 48 hours (to keep data for 24 hours + buffer)
	redisClient.Expire(redisCtx, hourKey, 48*time.Hour)
}

// getItemsCreatedStats returns statistics for items created in the last 24 hours
// Returns a map where key is hour (format: "YYYY-MM-DD HH:00") and value is count
func getItemsCreatedStats() map[string]int64 {
	stats := make(map[string]int64)

	if redisClient == nil {
		return stats
	}

	// Get keys for last 24 hours
	now := time.Now()
	for i := 0; i < 24; i++ {
		hourTime := now.Add(-time.Duration(i) * time.Hour)
		hourKey := fmt.Sprintf("%s:%s", itemsCreatedStatsKey, hourTime.Format("2006-01-02-15"))

		// Get count for this hour
		count, err := redisClient.Get(redisCtx, hourKey).Int64()
		if err == nil {
			// Format hour for display: "YYYY-MM-DD HH:00"
			displayKey := hourTime.Format("2006-01-02 15:00")
			stats[displayKey] = count
		}
	}

	return stats
}

// isUniqueConstraintError checks if the error is a unique constraint violation
func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	// Check for PostgreSQL unique constraint violation
	// PostgreSQL error code 23505 is "unique_violation"
	return strings.Contains(errStr, "duplicate key") ||
		strings.Contains(errStr, "unique constraint") ||
		strings.Contains(errStr, "23505") ||
		strings.Contains(errStr, "unique constraint failed")
}

func loadTemplates(templatesDir string) multitemplate.Renderer {
	r := multitemplate.NewRenderer()

	// Define custom template functions
	funcMap := template.FuncMap{
		"hasPrefix": strings.HasPrefix,
	}

	layouts, err := filepath.Glob(templatesDir + "/layouts/*.html")
	if err != nil {
		panic(err.Error())
	}

	// Load partials
	partials, err := filepath.Glob(templatesDir + "/partials/*.html")
	if err != nil {
		panic(err.Error())
	}

	// includes, err := filepath.Glob(templatesDir + "/includes/*.html")
	includes, err := filepath.Glob(templatesDir + "/*.html")
	if err != nil {
		panic(err.Error())
	}

	// Generate our templates map from our layouts/ and includes/ directories
	for _, include := range includes {
		layoutCopy := make([]string, len(layouts))
		copy(layoutCopy, layouts)
		// Include partials in each template
		files := append(layoutCopy, include)
		files = append(files, partials...)
		// Use AddFromFilesFuncs to add templates with custom functions
		r.AddFromFilesFuncs(filepath.Base(include), funcMap, files...)
	}

	return r
}

// generatePageNumbers generates a slice of page numbers for pagination
// Returns a slice where -1 represents ellipsis
func generatePageNumbers(currentPage, totalPages int64) []interface{} {
	var pages []interface{}
	if totalPages <= 7 {
		// Show all pages if 7 or fewer
		for i := int64(1); i <= totalPages; i++ {
			pages = append(pages, i)
		}
	} else {
		// Show first page
		pages = append(pages, int64(1))

		// Calculate start and end
		start := currentPage - 2
		if start < 2 {
			start = 2
		}
		end := currentPage + 2
		if end > totalPages-1 {
			end = totalPages - 1
		}

		// Add ellipsis if needed
		if start > 2 {
			pages = append(pages, int64(-1)) // -1 means ellipsis
		}

		// Add pages around current
		for i := start; i <= end; i++ {
			pages = append(pages, i)
		}

		// Add ellipsis if needed
		if end < totalPages-1 {
			pages = append(pages, int64(-1)) // -1 means ellipsis
		}

		// Show last page
		pages = append(pages, totalPages)
	}
	return pages
}

// showStartupInfo displays information about available commands and what the application does
func showStartupInfo() {
	port := GetServerPort()
	fmt.Println("=" + strings.Repeat("=", 70) + "=")
	fmt.Println("  Go RSS UI Application")
	fmt.Println("=" + strings.Repeat("=", 70) + "=")
	fmt.Println()
	fmt.Printf("Starting web server on http://localhost:%s\n", port)
	fmt.Println()
	fmt.Println("When you run the application without a command, it starts the web server.")
	fmt.Printf("You can access the application in your browser at http://localhost:%s\n", port)
	fmt.Println()
	fmt.Println("Available CLI commands:")
	fmt.Println()
	fmt.Println("  clear-users  - Clear all data from users table")
	fmt.Println("  seed-users   - Create a standard admin user")
	fmt.Println("  seed-feeds   - Create default RSS feeds")
	fmt.Println("  fetch-feeds  - Fetch and process all RSS feeds (creates/updates items)")
	fmt.Println("  execute-sql  - Execute SQL query (provide query as argument or via stdin)")
	fmt.Println("                Example: go run . execute-sql \"SELECT * FROM feeds\"")
	fmt.Println("  migrate      - Create tables in database using AutoMigrate")
	fmt.Println("  drop-db      - Delete the application database")
	fmt.Println("  drop-all-tables - Drop all tables in the database")
	fmt.Println("  create-db    - Create the application database")
	fmt.Println("  dump-db-structure - Dump database structure to structure.sql file")
	fmt.Println()
	fmt.Println("Usage examples:")
	fmt.Println("  go run .                    - Start web server (default)")
	fmt.Println("  go run . seed-users         - Create admin user")
	fmt.Println("  go run . fetch-feeds        - Fetch all RSS feeds")
	fmt.Println("  go run . execute-sql \"...\"  - Execute SQL query")
	fmt.Println()
	fmt.Println("=" + strings.Repeat("=", 70) + "=")
	fmt.Println()
}

func main() {
	// Load environment variables from .env file
	LoadConfig()

	// Initialize logger with Redis support
	initLogger()

	// Check for command-line arguments
	if len(os.Args) > 1 {
		command := os.Args[1]
		switch command {
		case "clear-users":
			CommandClearUsers()
		case "seed-users":
			CommandSeedUsers()
		case "seed-feeds":
			CommandSeedFeeds()
		case "migrate":
			CommandMigrate()
		case "drop-db":
			CommandDropDB()
		case "drop-all-tables":
			CommandDropAllTables()
		case "create-db":
			CommandCreateDB()
		case "fetch-feeds":
			CommandFetchFeeds()
		case "execute-sql":
			CommandExecuteSQL()
		case "dump-db-structure":
			CommandDumpDBStructure()
		default:
			fmt.Println("Unknown command:", command)
			fmt.Println("\nAvailable commands:")
			fmt.Println("  clear-users  - Clear all data from users table")
			fmt.Println("  seed-users   - Create a standard admin user")
			fmt.Println("  seed-feeds   - Create default RSS feeds")
			fmt.Println("  fetch-feeds  - Fetch and process all RSS feeds")
			fmt.Println("  execute-sql  - Execute SQL query (provide query as argument or via stdin)")
			fmt.Println("  migrate      - Create tables in database using AutoMigrate")
			fmt.Println("  drop-db      - Delete the application database")
			fmt.Println("  drop-all-tables - Drop all tables in the database")
			fmt.Println("  create-db    - Create the application database")
			fmt.Println("  dump-db-structure - Dump database structure to structure.sql file")
			os.Exit(1)
		}
		return
	}

	// Show informational message about available commands
	showStartupInfo()

	// Run the web server
	ConnectDatabase()
	// Seed()

	// Start background feed fetcher if enabled
	if GetBackgroundFetchEnabled() {
		go startBackgroundFeedFetcher()
	} else {
		appLogger.Info().Msg("Background feed fetcher is disabled")
	}

	r := gin.Default()

	r.HTMLRender = loadTemplates("./templates")

	r.Static("/static", "./static")

	// Custom handler for test_feeds that checks for error endpoints first
	// This single handler handles both error endpoints and static files
	r.GET("/test_feeds/*filepath", func(c *gin.Context) {
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
	})

	store := cookie.NewStore([]byte("secret"))
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 30, // 30 days
		HttpOnly: true,
		Secure:   false,                // Set to false for HTTP, true for HTTPS
		SameSite: http.SameSiteLaxMode, // Changed from None to Lax for HTTP compatibility
	})
	r.Use(sessions.Sessions("mysession", store))
	r.Use(AddAuthInfo())

	r.GET("/", func(c *gin.Context) {
		var items []Item
		model := DB.Model(&Item{}).Preload("Feed").Order("created_at DESC")

		// Filter by feed_id if provided
		if feedID := c.Query("feed_id"); feedID != "" {
			model = model.Where("feed_id = ?", feedID)
		}

		page := Paginator.With(model).Request(c.Request).Response(&items)

		// Sanitize HTML content for each item and convert to template.HTML
		type ItemWithSanitizedContent struct {
			Item
			SanitizedContent template.HTML
		}
		itemsWithSanitized := make([]ItemWithSanitizedContent, len(items))
		for i, item := range items {
			itemsWithSanitized[i] = ItemWithSanitizedContent{
				Item:             item,
				SanitizedContent: template.HTML(SanitizeHTML(item.Content)),
			}
		}

		// Get last 20 feeds that have items (ordered by most recent item created_at)
		// Get feed IDs with their latest item timestamp, then fetch the feeds
		type FeedWithLatestItem struct {
			FeedID              uint      `gorm:"column:feed_id"`
			LatestItemCreatedAt time.Time `gorm:"column:latest_item_created_at"`
		}
		var feedIDsWithLatest []FeedWithLatestItem
		DB.Raw(`
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

		// Fetch feeds in the same order
		var recentFeeds []Feed
		if len(feedIDs) > 0 {
			// Create a map to preserve order
			feedMap := make(map[uint]Feed)
			DB.Where("id IN ?", feedIDs).Find(&recentFeeds)
			for _, feed := range recentFeeds {
				feedMap[feed.ID] = feed
			}
			// Reorder according to feedIDs order
			recentFeeds = make([]Feed, 0, len(feedIDs))
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
			"title":       "My RSS App",
			"items":       itemsWithSanitized,
			"recentFeeds": recentFeeds,
			"feedID":      c.Query("feed_id"),
		}

		// Add pagination data
		data = addPaginationData(data, page, paginationURL, "items")

		data = getTemplateData(c, data)
		c.HTML(http.StatusOK, "index.html", data)
	})

	r.GET("/feeds", func(c *gin.Context) {
		var feeds []Feed
		model := DB.Model(&Feed{}).Order("created_at DESC")
		page := Paginator.With(model).Request(c.Request).Response(&feeds)

		data := gin.H{
			"title": "All Feeds",
			"feeds": feeds,
		}

		// Add pagination data
		data = addPaginationData(data, page, "/feeds", "feeds")

		data = getTemplateData(c, data)
		c.HTML(http.StatusOK, "all_feeds.html", data)
	})

	admin := r.Group("/admin")
	{
		admin.Use(AuthRequired())
		admin.GET("/", func(c *gin.Context) {
			c.Redirect(http.StatusFound, "/admin/users")
		})
		admin.GET("/users", adminIndex)
		admin.GET("/users/new", showCreateUserForm)
		admin.POST("/users", createUser)
		admin.GET("/users/:id/edit", showEditUserForm)
		admin.POST("/users/:id/edit", editUser)
		admin.POST("/users/:id/delete", deleteUser)

		// Feeds routes
		admin.GET("/feeds", adminFeedsIndex)
		admin.GET("/feeds/:id", showFeed)
		admin.GET("/feeds/new", showCreateFeedForm)
		admin.POST("/feeds", createFeed)
		admin.POST("/feeds/:id/fetch", fetchSingleFeed)
		admin.POST("/feeds/:id/delete", deleteFeed)
		admin.POST("/feeds/delete-all", deleteAllFeeds)
		admin.POST("/feeds/seed", seedFeeds)

		// Items routes
		admin.GET("/items", adminItemsIndex)
		admin.GET("/items/:id", showItem)
		admin.POST("/items/fetch", fetchFeedItems)
		admin.POST("/items/delete-all", deleteAllItems)

		// Logs routes
		admin.GET("/feed-fetching-log", showLogs)
		admin.GET("/zerolog", showZerolog)

		// Chart route
		admin.GET("/chart", showChart)

		// Info route
		admin.GET("/info", showInfo)
		admin.POST("/info/dump-db-structure", dumpDBStructureAdmin)
	}

	r.GET("/login", showLogin)
	r.POST("/login", login)
	r.POST("/logout", logout)

	// Tools routes (only available when CYPRESS=true)
	if IsCypressMode() {
		tools := r.Group("/tools")
		tools.GET("", showTools)
		tools.POST("/clear-all-tables", clearAllTables)
		tools.POST("/clear-table", clearTable)
		tools.POST("/seed-users", seedUsers)
		tools.POST("/seed-users-and-login", seedUsersAndLogin)
		tools.POST("/seed-feeds", seedFeeds)
		tools.POST("/drop-db", dropDB)
		tools.POST("/drop-all-tables", dropAllTables)
		tools.POST("/create-db", createDB)
		tools.POST("/migrate", migrate)
		tools.POST("/dump-db-structure", dumpDBStructure)
		tools.POST("/execute-sql", executeSQL)
	}

	port := GetServerPort()
	if err := r.Run(":" + port); err != nil {
		appLogger.Fatal().Err(err).Msg("Failed to start server")
	}
}

func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		userID := session.Get("user")
		if userID == nil {
			appLogger.Debug().Msg("User not logged in")
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		// Check if user exists in database
		var user User
		userIDUint, ok := userID.(uint)
		if !ok {
			// Try to convert from float64 (JSON numbers are often float64)
			if userIDFloat, ok := userID.(float64); ok {
				userIDUint = uint(userIDFloat)
			} else {
				appLogger.Warn().Msg("Invalid user ID type in session")
				session.Clear()
				if err := session.Save(); err != nil {
					appLogger.Error().Err(err).Msg("Error saving session")
				}
				c.Redirect(http.StatusFound, "/login")
				c.Abort()
				return
			}
		}

		result := DB.First(&user, userIDUint)
		if result.Error != nil {
			appLogger.Warn().Uint("user_id", userIDUint).Msg("User not found in database, invalidating session")
			session.Clear()
			if err := session.Save(); err != nil {
				appLogger.Error().Err(err).Msg("Error saving session")
			}
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		c.Next()
	}
}

// AddAuthInfo adds authentication info, flash messages, and CYPRESS mode to context for all requests
func AddAuthInfo() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)

		// Handle authentication
		userID := session.Get("user")
		if userID != nil {
			// Convert user ID to uint
			var userIDUint uint
			if id, ok := userID.(uint); ok {
				userIDUint = id
			} else if idFloat, ok := userID.(float64); ok {
				userIDUint = uint(idFloat)
			} else {
				c.Set("isAuthenticated", false)
			}

			if userIDUint > 0 {
				// Load user from database to get username
				var user User
				if err := DB.First(&user, userIDUint).Error; err == nil {
					c.Set("isAuthenticated", true)
					c.Set("username", user.Username)
					c.Set("userID", userIDUint)
				} else {
					c.Set("isAuthenticated", false)
				}
			}
		} else {
			c.Set("isAuthenticated", false)
		}

		// Get flash messages and add to context
		successMsg, errorMsg := getFlashMessages(session)
		if successMsg != "" {
			c.Set("success", successMsg)
		}
		if errorMsg != "" {
			c.Set("error", errorMsg)
		}
		// Save session after reading flash messages
		if err := session.Save(); err != nil {
			log.Printf("Error saving session in AddAuthInfo: %v", err)
		}

		// Add CYPRESS mode info
		c.Set("isCypressMode", IsCypressMode())

		c.Next()
	}
}

// getTemplateData collects all template data from context (auth, flash messages, CYPRESS mode)
func getTemplateData(c *gin.Context, data gin.H) gin.H {
	if data == nil {
		data = gin.H{}
	}

	// Add authentication info
	if isAuth, exists := c.Get("isAuthenticated"); exists {
		data["isAuthenticated"] = isAuth
	}
	if username, exists := c.Get("username"); exists {
		data["username"] = username
	}

	// Add flash messages
	if success, exists := c.Get("success"); exists {
		data["success"] = success
	}
	if error, exists := c.Get("error"); exists {
		data["error"] = error
	}

	// Add CYPRESS mode info
	if isCypressMode, exists := c.Get("isCypressMode"); exists {
		data["isCypressMode"] = isCypressMode
	}

	// Add current path for active menu highlighting
	data["currentPath"] = c.Request.URL.Path

	return data
}

// addPaginationData adds pagination data (page, pages, prevPage, nextPage) to the data map
// It extracts Page and TotalPages fields from the page object using reflection
// baseURL is the base URL for pagination links (e.g., "/admin/users")
// entityName is the name of the entity for display (e.g., "users")
func addPaginationData(data gin.H, page interface{}, baseURL, entityName string) gin.H {
	if data == nil {
		data = gin.H{}
	}

	// Use reflection to access Page and TotalPages fields
	v := reflect.ValueOf(page)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		// If not a struct, just add the page object
		data["page"] = page
		return data
	}

	// Get Page field
	pageField := v.FieldByName("Page")
	totalPagesField := v.FieldByName("TotalPages")

	if !pageField.IsValid() || !totalPagesField.IsValid() {
		// If fields don't exist, just add the page object
		data["page"] = page
		return data
	}

	// Convert to int64
	var pageNum, totalPages int64
	if pageField.Kind() == reflect.Int64 {
		pageNum = pageField.Int()
	} else if pageField.Kind() == reflect.Int {
		pageNum = int64(pageField.Int())
	}

	if totalPagesField.Kind() == reflect.Int64 {
		totalPages = totalPagesField.Int()
	} else if totalPagesField.Kind() == reflect.Int {
		totalPages = int64(totalPagesField.Int())
	}

	// Ensure page is at least 1
	if pageNum < 1 {
		pageNum = 1
	}

	prevPage := pageNum - 1
	if prevPage < 1 {
		prevPage = 0 // 0 means no previous page
	}

	nextPage := pageNum + 1
	if nextPage > totalPages {
		nextPage = 0 // 0 means no next page
	}

	data["page"] = page
	data["pages"] = generatePageNumbers(pageNum, totalPages)
	data["prevPage"] = prevPage
	data["nextPage"] = nextPage
	data["paginationBaseURL"] = baseURL
	data["paginationEntityName"] = entityName

	return data
}

// Helper functions for flash messages using simple strings instead of maps
func addFlashSuccess(session sessions.Session, message string) {
	session.AddFlash("success:" + message)
}

func addFlashError(session sessions.Session, message string) {
	session.AddFlash("error:" + message)
}

func getFlashMessages(session sessions.Session) (successMsg, errorMsg string) {
	flashes := session.Flashes()
	for _, flash := range flashes {
		if flashStr, ok := flash.(string); ok {
			if len(flashStr) > 8 && flashStr[:8] == "success:" {
				successMsg = flashStr[8:]
			} else if len(flashStr) > 6 && flashStr[:6] == "error:" {
				errorMsg = flashStr[6:]
			}
		}
	}
	return successMsg, errorMsg
}

func showLogin(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", getTemplateData(c, gin.H{
		"title": "Login",
	}))
}

func login(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	var user User
	result := DB.Where("username = ?", username).First(&user)
	if result.Error != nil {
		c.HTML(http.StatusUnauthorized, "login.html", getTemplateData(c, gin.H{
			"title":   "Login",
			"message": "Invalid credentials",
		}))
		return
	}

	if !user.CheckPassword(password) {
		c.HTML(http.StatusUnauthorized, "login.html", getTemplateData(c, gin.H{
			"title":   "Login",
			"message": "Invalid credentials",
		}))
		return
	}

	session := sessions.Default(c)
	session.Set("user", user.ID)
	if err := session.Save(); err != nil {
		log.Printf("Error saving session: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save session"})
		return
	}

	c.Redirect(http.StatusFound, "/admin/users")
}

func logout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	if err := session.Save(); err != nil {
		log.Printf("Error saving session: %v", err)
	}
	c.Redirect(http.StatusFound, "/")
}

func adminIndex(c *gin.Context) {
	var users []User
	model := DB.Model(&User{}).Order("created_at DESC")
	page := Paginator.With(model).Request(c.Request).Response(&users)

	data := gin.H{
		"title": "User Management",
		"users": page.Items,
	}

	// Add pagination data
	data = addPaginationData(data, page, "/admin/users", "users")

	// Check for error in query parameter (for backward compatibility)
	if queryError := c.Query("error"); queryError != "" {
		if _, exists := c.Get("error"); !exists {
			data["error"] = queryError
		}
	}

	data = getTemplateData(c, data)
	c.HTML(http.StatusOK, "users.html", data)
}

func showCreateUserForm(c *gin.Context) {
	data := getTemplateData(c, gin.H{
		"title": "Create New User",
	})
	c.HTML(http.StatusOK, "create_user.html", data)
}

func showEditUserForm(c *gin.Context) {
	id := c.Param("id")

	var user User
	if err := DB.First(&user, id).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/users?error=User+not+found")
		return
	}

	data := getTemplateData(c, gin.H{
		"title": "Edit User",
		"user":  user,
	})
	c.HTML(http.StatusOK, "edit_user.html", data)
}

func createUser(c *gin.Context) {
	// Create input struct from form data
	input := UserInput{
		Username: c.PostForm("username"),
		Password: c.PostForm("password"),
	}

	// Validate input using validator/v10
	if err := ValidateStruct(input); err != nil {
		data := getTemplateData(c, gin.H{
			"title": "Create New User",
			"error": FormatValidationErrors(err),
		})
		c.HTML(http.StatusBadRequest, "create_user.html", data)
		return
	}

	// Check if username already exists
	var existingUser User
	if err := DB.Where("username = ?", input.Username).First(&existingUser).Error; err == nil {
		// User with this username already exists
		data := getTemplateData(c, gin.H{
			"title": "Create New User",
			"error": "Username already exists",
		})
		c.HTML(http.StatusBadRequest, "create_user.html", data)
		return
	}

	user := User{Username: input.Username, Password: input.Password}
	if err := DB.Create(&user).Error; err != nil {
		// Check if error is due to unique constraint violation
		if isUniqueConstraintError(err) {
			data := getTemplateData(c, gin.H{
				"title": "Create New User",
				"error": "Username already exists",
			})
			c.HTML(http.StatusBadRequest, "create_user.html", data)
			return
		}
		data := getTemplateData(c, gin.H{
			"title": "Create New User",
			"error": "Failed to create user: " + err.Error(),
		})
		c.HTML(http.StatusInternalServerError, "create_user.html", data)
		return
	}

	session := sessions.Default(c)
	addFlashSuccess(session, "User created successfully")
	if err := session.Save(); err != nil {
		log.Printf("Error saving session in createUser: %v", err)
	}
	c.Redirect(http.StatusFound, "/admin/users")
}

func editUser(c *gin.Context) {
	id := c.Param("id")
	username := c.PostForm("username")
	password := c.PostForm("password")

	var user User
	if err := DB.First(&user, id).Error; err != nil {
		data := getTemplateData(c, gin.H{
			"title": "Edit User",
			"error": "User not found",
			"user":  user,
		})
		c.HTML(http.StatusNotFound, "edit_user.html", data)
		return
	}

	// Use current username if not provided, otherwise use new one
	usernameToValidate := username
	if usernameToValidate == "" {
		usernameToValidate = user.Username
	}

	// Create input struct for validation
	input := UserInputUpdate{
		Username: usernameToValidate,
		Password: password,
	}

	// Validate input using validator/v10
	if err := ValidateStruct(input); err != nil {
		data := getTemplateData(c, gin.H{
			"title": "Edit User",
			"error": FormatValidationErrors(err),
			"user":  user,
		})
		c.HTML(http.StatusBadRequest, "edit_user.html", data)
		return
	}

	if username != "" {
		// Check if new username is already taken by another user
		var existingUser User
		if err := DB.Where("username = ? AND id != ?", username, id).First(&existingUser).Error; err == nil {
			// Username is already taken by another user
			data := getTemplateData(c, gin.H{
				"title": "Edit User",
				"error": "Username already exists",
				"user":  user,
			})
			c.HTML(http.StatusBadRequest, "edit_user.html", data)
			return
		}
		user.Username = username
	}
	if password != "" {
		user.Password = password
	}

	if err := DB.Save(&user).Error; err != nil {
		// Check if error is due to unique constraint violation
		if isUniqueConstraintError(err) {
			data := getTemplateData(c, gin.H{
				"title": "Edit User",
				"error": "Username already exists",
				"user":  user,
			})
			c.HTML(http.StatusBadRequest, "edit_user.html", data)
			return
		}
		data := getTemplateData(c, gin.H{
			"title": "Edit User",
			"error": "Failed to update user: " + err.Error(),
			"user":  user,
		})
		c.HTML(http.StatusInternalServerError, "edit_user.html", data)
		return
	}

	c.Redirect(http.StatusFound, "/admin/users")
}

func deleteUser(c *gin.Context) {
	id := c.Param("id")
	session := sessions.Default(c)

	var user User
	if err := DB.First(&user, id).Error; err != nil {
		addFlashError(session, "User not found")
		if err := session.Save(); err != nil {
			log.Printf("Error saving session: %v", err)
		}
		c.Redirect(http.StatusFound, "/admin/users")
		return
	}

	if err := DB.Unscoped().Delete(&user).Error; err != nil {
		addFlashError(session, "Failed to delete user: "+err.Error())
		if err := session.Save(); err != nil {
			log.Printf("Error saving session: %v", err)
		}
		c.Redirect(http.StatusFound, "/admin/users")
		return
	}

	addFlashSuccess(session, "User deleted successfully")
	if err := session.Save(); err != nil {
		log.Printf("Error saving session: %v", err)
	}
	c.Redirect(http.StatusFound, "/admin/users")
}

// Feed handlers
func adminFeedsIndex(c *gin.Context) {
	var feeds []Feed
	model := DB.Model(&Feed{}).Order("created_at DESC")
	page := Paginator.With(model).Request(c.Request).Response(&feeds)

	data := gin.H{
		"title": "Feed Management",
		"feeds": page.Items,
	}

	// Add pagination data
	data = addPaginationData(data, page, "/admin/feeds", "feeds")

	data = getTemplateData(c, data)
	c.HTML(http.StatusOK, "feeds.html", data)
}

func showCreateFeedForm(c *gin.Context) {
	data := getTemplateData(c, gin.H{
		"title": "Create New Feed",
	})
	c.HTML(http.StatusOK, "create_feed.html", data)
}

func createFeed(c *gin.Context) {
	// Create input struct from form data
	input := FeedInput{
		URL: c.PostForm("url"),
	}

	// Validate input using validator/v10
	if err := ValidateStruct(input); err != nil {
		data := getTemplateData(c, gin.H{
			"title": "Create New Feed",
			"error": FormatValidationErrors(err),
		})
		c.HTML(http.StatusBadRequest, "create_feed.html", data)
		return
	}

	feed := Feed{URL: input.URL}
	if err := DB.Create(&feed).Error; err != nil {
		data := getTemplateData(c, gin.H{
			"title": "Create New Feed",
			"error": "Failed to create feed: " + err.Error(),
		})
		c.HTML(http.StatusInternalServerError, "create_feed.html", data)
		return
	}

	session := sessions.Default(c)
	addFlashSuccess(session, "Feed created successfully")
	if err := session.Save(); err != nil {
		log.Printf("Error saving session in createFeed: %v", err)
	}
	c.Redirect(http.StatusFound, "/admin/feeds")
}

func fetchSingleFeed(c *gin.Context) {
	id := c.Param("id")
	session := sessions.Default(c)

	var feed Feed
	if err := DB.First(&feed, id).Error; err != nil {
		addFlashError(session, "Feed not found")
		if err := session.Save(); err != nil {
			log.Printf("Error saving session: %v", err)
		}
		c.Redirect(http.StatusFound, "/admin/feeds")
		return
	}

	itemsCreated, itemsUpdated, err := processSingleFeed(feed.ID)
	if err != nil {
		addFlashError(session, fmt.Sprintf("Failed to fetch feed: %v", err))
		if err := session.Save(); err != nil {
			log.Printf("Error saving session: %v", err)
		}
		c.Redirect(http.StatusFound, "/admin/feeds")
		return
	}

	successMsg := fmt.Sprintf("Feed fetched successfully: %d items created, %d items updated", itemsCreated, itemsUpdated)
	addFlashSuccess(session, successMsg)
	if err := session.Save(); err != nil {
		log.Printf("Error saving session: %v", err)
	}
	c.Redirect(http.StatusFound, "/admin/feeds")
}

func deleteFeed(c *gin.Context) {
	id := c.Param("id")
	session := sessions.Default(c)

	var feed Feed
	if err := DB.First(&feed, id).Error; err != nil {
		addFlashError(session, "Feed not found")
		if err := session.Save(); err != nil {
			log.Printf("Error saving session: %v", err)
		}
		c.Redirect(http.StatusFound, "/admin/feeds")
		return
	}

	// Items will be deleted automatically due to CASCADE constraint
	if err := DB.Unscoped().Delete(&feed).Error; err != nil {
		addFlashError(session, "Failed to delete feed: "+err.Error())
		if err := session.Save(); err != nil {
			log.Printf("Error saving session: %v", err)
		}
		c.Redirect(http.StatusFound, "/admin/feeds")
		return
	}

	addFlashSuccess(session, "Feed deleted successfully")
	if err := session.Save(); err != nil {
		log.Printf("Error saving session: %v", err)
	}
	c.Redirect(http.StatusFound, "/admin/feeds")
}

func deleteAllFeeds(c *gin.Context) {
	session := sessions.Default(c)

	// Delete all feeds (items will be deleted automatically due to CASCADE constraint)
	result := DB.Unscoped().Delete(&Feed{}, "1 = 1")
	if result.Error != nil {
		addFlashError(session, "Failed to delete all feeds")
		if err := session.Save(); err != nil {
			log.Printf("Error saving session: %v", err)
		}
		c.Redirect(http.StatusFound, "/admin/feeds")
		return
	}

	addFlashSuccess(session, "All feeds deleted successfully")
	if err := session.Save(); err != nil {
		log.Printf("Error saving session: %v", err)
	}
	c.Redirect(http.StatusFound, "/admin/feeds")
}

func seedFeeds(c *gin.Context) {
	// Check if this is called from /tools (CYPRESS mode required)
	if c.Request.URL.Path == "/tools/seed-feeds" {
		if !IsCypressMode() {
			c.JSON(http.StatusForbidden, gin.H{"error": "Tools are only available when CYPRESS=true"})
			return
		}
	}

	session := sessions.Default(c)

	// Use the unified SeedFeeds function
	result := SeedFeeds()

	successMsg := fmt.Sprintf("Seeded feeds: %d created", result.Created)
	if result.Existed > 0 {
		successMsg += fmt.Sprintf(", %d already existed", result.Existed)
	}
	if result.Errors > 0 {
		successMsg += fmt.Sprintf(", %d errors", result.Errors)
	}

	addFlashSuccess(session, successMsg)
	if err := session.Save(); err != nil {
		log.Printf("Error saving session: %v", err)
	}

	// Redirect based on where the request came from
	if c.Request.URL.Path == "/tools/seed-feeds" {
		c.Redirect(http.StatusFound, "/tools")
	} else {
		c.Redirect(http.StatusFound, "/admin/feeds")
	}
}

func showFeed(c *gin.Context) {
	id := c.Param("id")
	session := sessions.Default(c)

	var feed Feed
	if err := DB.First(&feed, id).Error; err != nil {
		addFlashError(session, "Feed not found")
		if err := session.Save(); err != nil {
			log.Printf("Error saving session: %v", err)
		}
		c.Redirect(http.StatusFound, "/admin/feeds")
		return
	}

	// Get items for this feed with pagination
	var items []Item
	model := DB.Model(&Item{}).Where("feed_id = ?", feed.ID).Order("created_at DESC")
	page := Paginator.With(model).Request(c.Request).Response(&items)

	data := gin.H{
		"title": "Feed Details",
		"feed":  feed,
		"items": page.Items,
	}

	// Add pagination data
	data = addPaginationData(data, page, fmt.Sprintf("/admin/feeds/%s", id), "items")

	data = getTemplateData(c, data)
	c.HTML(http.StatusOK, "feed.html", data)
}

// Item handlers
func adminItemsIndex(c *gin.Context) {
	var items []Item
	model := DB.Model(&Item{}).Preload("Feed")

	// Filter by feed if provided
	if feedID := c.Query("feed_id"); feedID != "" {
		model = model.Where("feed_id = ?", feedID)
	}

	model = model.Order("created_at DESC")
	page := Paginator.With(model).Request(c.Request).Response(&items)

	data := gin.H{
		"title": "Items",
		"items": page.Items,
	}

	// Add pagination data
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

func showLogs(c *gin.Context) {
	entries := getLogEntries()
	// Redis LPUSH adds to the left, so LRANGE 0 999 returns newest first
	// No need to reverse - newest logs are already at the beginning

	data := getTemplateData(c, gin.H{
		"title":   "Feed Fetching Log",
		"entries": entries,
	})
	c.HTML(http.StatusOK, "logs.html", data)
}

// ZerologEntry represents a log entry from Redis
type ZerologEntry struct {
	Level   string                 `json:"level"`
	Time    string                 `json:"time"`
	Service string                 `json:"service,omitempty"`
	Message string                 `json:"message"`
	FeedURL string                 `json:"feed_url,omitempty"`
	Error   string                 `json:"error,omitempty"`
	Extra   map[string]interface{} `json:"-"`
	RawJSON string                 `json:"-"`
}

func showZerolog(c *gin.Context) {
	var entries []ZerologEntry

	// Get filter level from query parameter, default to "all"
	filterLevel := c.DefaultQuery("level", "all")

	if redisClient == nil {
		data := getTemplateData(c, gin.H{
			"title":       "Zerolog Logs",
			"entries":     entries,
			"filterLevel": filterLevel,
			"error":       "Redis client is not available",
		})
		c.HTML(http.StatusOK, "zerolog.html", data)
		return
	}

	// Get logs from Redis (up to 1000 entries)
	logs, err := redisClient.LRange(redisCtx, "app:logs", 0, 999).Result()
	if err != nil {
		appLogger.Error().Err(err).Msg("Failed to get logs from Redis")
		data := getTemplateData(c, gin.H{
			"title":       "Zerolog Logs",
			"entries":     entries,
			"filterLevel": filterLevel,
			"error":       fmt.Sprintf("Failed to get logs from Redis: %v", err),
		})
		c.HTML(http.StatusOK, "zerolog.html", data)
		return
	}

	// Parse JSON logs
	// Redis LPUSH adds to the left, so LRANGE 0 999 returns newest first
	// No need to reverse - newest logs are already at the beginning
	for _, logJSON := range logs {
		var entry ZerologEntry
		if err := json.Unmarshal([]byte(logJSON), &entry); err != nil {
			// If parsing fails, create a basic entry
			entry = ZerologEntry{
				Level:   "unknown",
				Message: "Failed to parse log entry",
				RawJSON: logJSON,
			}
		} else {
			entry.RawJSON = logJSON
		}

		// Filter by level if not "all"
		if filterLevel == "all" || strings.EqualFold(entry.Level, filterLevel) {
			entries = append(entries, entry)
		}
	}

	data := getTemplateData(c, gin.H{
		"title":       "Zerolog Logs",
		"entries":     entries,
		"filterLevel": filterLevel,
	})
	c.HTML(http.StatusOK, "zerolog.html", data)
}

func showChart(c *gin.Context) {
	stats := getItemsCreatedStats()

	// Generate all 24 hours (even if no data) for complete chart
	now := time.Now()
	labels := make([]string, 24)
	data := make([]int64, 24)

	// Fill from oldest to newest (last 24 hours)
	for i := 0; i < 24; i++ {
		hourTime := now.Add(-time.Duration(23-i) * time.Hour)
		hourLabel := hourTime.Format("2006-01-02 15:00")
		labels[i] = hourLabel

		// Get count from stats if available
		if count, ok := stats[hourLabel]; ok {
			data[i] = count
		} else {
			data[i] = 0
		}
	}

	chartData := gin.H{
		"labels": labels,
		"data":   data,
	}

	pageData := getTemplateData(c, gin.H{
		"title":     "Items Created Chart",
		"chartData": chartData,
	})

	c.HTML(http.StatusOK, "chart.html", pageData)
}

// EnvVarInfo contains information about an environment variable
type EnvVarInfo struct {
	Name        string
	Value       string
	Description string
}

func showInfo(c *gin.Context) {
	// Get statistics
	var feedsCount int64
	DB.Model(&Feed{}).Count(&feedsCount)

	var itemsCount int64
	DB.Model(&Item{}).Count(&itemsCount)

	// Get last successful fetch
	var lastSuccessFeed Feed
	var lastSuccessTime *time.Time
	DB.Where("last_successfully_fetched_at IS NOT NULL").
		Order("last_successfully_fetched_at DESC").
		First(&lastSuccessFeed)
	if lastSuccessFeed.LastSuccessfullyFetchedAt != nil {
		lastSuccessTime = lastSuccessFeed.LastSuccessfullyFetchedAt
	}

	// Get last error fetch
	var lastErrorFeed Feed
	var lastErrorTime *time.Time
	var lastError string
	DB.Where("last_error_at IS NOT NULL").
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
			Value:       getEnvOrDefault("RSS_DB_HOST", "localhost (default)"),
			Description: "PostgreSQL database host",
		},
		{
			Name:        "RSS_DB_USER",
			Value:       getEnvOrDefault("RSS_DB_USER", "postgres (default)"),
			Description: "PostgreSQL database user",
		},
		{
			Name:        "RSS_DB_PASSWORD",
			Value:       maskPassword(getEnvOrDefault("RSS_DB_PASSWORD", "postgres (default)")),
			Description: "PostgreSQL database password",
		},
		{
			Name:        "RSS_DB_NAME",
			Value:       getEnvOrDefault("RSS_DB_NAME", "go_rss_ui_2 (default)"),
			Description: "PostgreSQL database name",
		},
		{
			Name:        "RSS_DB_PORT",
			Value:       getEnvOrDefault("RSS_DB_PORT", "5432 (default)"),
			Description: "PostgreSQL database port",
		},
		{
			Name:        "RSS_DB_SSLMODE",
			Value:       getEnvOrDefault("RSS_DB_SSLMODE", "disable (default)"),
			Description: "PostgreSQL SSL mode",
		},
		{
			Name:        "RSS_DB_TIMEZONE",
			Value:       getEnvOrDefault("RSS_DB_TIMEZONE", "Asia/Shanghai (default)"),
			Description: "PostgreSQL timezone",
		},
		{
			Name:        "RSS_BACKGROUND_FETCH_ENABLED",
			Value:       getEnvValueOrDefault("RSS_BACKGROUND_FETCH_ENABLED", "true (default)"),
			Description: "Enable/disable background feed fetching",
		},
		{
			Name:        "RSS_BACKGROUND_FETCH_INTERVAL",
			Value:       fmt.Sprintf("%d (default: 60)", GetBackgroundFetchInterval()),
			Description: "Background feed fetch interval in seconds",
		},
		{
			Name:        "RSS_CYPRESS",
			Value:       getEnvValueOrDefault("RSS_CYPRESS", "false (default)"),
			Description: "Enable Cypress mode (enables /tools page for testing)",
		},
		{
			Name:        "RSS_PORT",
			Value:       getEnvOrDefault("RSS_PORT", "8082 (default)"),
			Description: "Server port",
		},
		{
			Name:        "RSS_REDIS_HOST",
			Value:       getEnvOrDefault("RSS_REDIS_HOST", "localhost (default)"),
			Description: "Redis host",
		},
		{
			Name:        "RSS_REDIS_PORT",
			Value:       getEnvOrDefault("RSS_REDIS_PORT", "6379 (default)"),
			Description: "Redis port",
		},
		{
			Name:        "RSS_REDIS_PASSWORD",
			Value:       maskPassword(getEnvOrDefault("RSS_REDIS_PASSWORD", "(empty)")),
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

func showItem(c *gin.Context) {
	id := c.Param("id")

	var item Item
	if err := DB.Preload("Feed").First(&item, id).Error; err != nil {
		// Show 404 page instead of redirecting
		data := getTemplateData(c, gin.H{
			"title": "404 - Item Not Found",
			"error": "Item not found",
		})
		c.HTML(http.StatusNotFound, "item.html", data)
		return
	}

	// Sanitize HTML content before displaying (defense in depth - already sanitized when saved)
	sanitizedDescription := SanitizeHTML(item.Description)
	sanitizedContent := SanitizeHTML(item.Content)

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

func deleteAllItems(c *gin.Context) {
	session := sessions.Default(c)
	result := DB.Delete(&Item{}, "1 = 1")
	if result.Error != nil {
		addFlashError(session, "Failed to delete all items")
		if err := session.Save(); err != nil {
			log.Printf("Error saving session: %v", err)
		}
		c.Redirect(http.StatusFound, "/admin/items")
		return
	}
	addFlashSuccess(session, "All items deleted successfully")
	if err := session.Save(); err != nil {
		log.Printf("Error saving session: %v", err)
	}
	c.Redirect(http.StatusFound, "/admin/items")
}

func processAllFeeds() (itemsCreated, itemsUpdated, errors int) {
	return processFeedsWithFilter(true)
}

func processFeedsWithFilter(includeTest bool) (itemsCreated, itemsUpdated, errors int) {
	var feeds []Feed
	if includeTest {
		// Include all feeds (for manual fetch)
		DB.Find(&feeds)
	} else {
		// Exclude test feeds from background fetching (feeds with /test_feeds/ in URL)
		DB.Where("url NOT LIKE ?", "%/test_feeds/%").Find(&feeds)
	}

	if len(feeds) == 0 {
		return 0, 0, 0
	}

	// Counters with mutex for thread safety
	var mu sync.Mutex
	itemsCreated = 0
	itemsUpdated = 0
	errors = 0

	// Worker pool: limit to 10 concurrent goroutines
	const maxWorkers = 10
	feedChan := make(chan Feed, len(feeds))
	var wg sync.WaitGroup

	// Start 10 worker goroutines
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fp := gofeed.NewParser()

			for feed := range feedChan {
				parsedFeed, err := fp.ParseURL(feed.URL)
				if err != nil {
					appLogger.Error().Str("feed_url", feed.URL).Err(err).Msg("Error parsing feed")
					// Update feed with error information
					now := time.Now()
					feed.LastError = err.Error()
					feed.LastErrorAt = &now
					DB.Save(&feed)
					// Add error log entry
					addLogEntry("error", feed.URL, fmt.Sprintf("Failed to fetch feed: %v", err))
					mu.Lock()
					errors++
					mu.Unlock()
					continue
				}

				// Update feed title and description if available
				if parsedFeed.Title != "" {
					feed.Title = parsedFeed.Title
				}
				if parsedFeed.Description != "" {
					feed.Description = parsedFeed.Description
				}
				// Update successful fetch timestamp and clear error
				now := time.Now()
				feed.LastSuccessfullyFetchedAt = &now
				feed.LastError = ""
				feed.LastErrorAt = nil
				DB.Save(&feed)

				// Local counters for this feed
				feedCreated := 0
				feedUpdated := 0

				// Process items for this feed
				for _, item := range parsedFeed.Items {
					// Determine GUID
					guid := item.GUID
					if guid == "" {
						guid = item.Link
					}

					// Parse published date
					var publishedAt *time.Time
					if item.PublishedParsed != nil {
						publishedAt = item.PublishedParsed
					} else if item.UpdatedParsed != nil {
						publishedAt = item.UpdatedParsed
					}

					// Check if item already exists by GUID
					var existingItem Item
					result := DB.Where("guid = ? AND feed_id = ?", guid, feed.ID).First(&existingItem)

					if result.Error != nil {
						// Item doesn't exist, create it
						// Sanitize HTML content before saving
						description := SanitizeHTML(item.Description)
						content := SanitizeHTML(getItemContent(item))
						newItem := Item{
							FeedID:      feed.ID,
							Title:       item.Title,
							Link:        item.Link,
							Description: description,
							Content:     content,
							Author:      getItemAuthor(item),
							PublishedAt: publishedAt,
							GUID:        guid,
						}
						if err := DB.Create(&newItem).Error; err != nil {
							appLogger.Error().Err(err).Str("feed_url", feed.URL).Msg("Error creating item")
							mu.Lock()
							errors++
							mu.Unlock()
						} else {
							feedCreated++
							mu.Lock()
							itemsCreated++
							mu.Unlock()
							// Track item creation statistics
							incrementItemsCreatedStats()
						}
					} else {
						// Item exists, update it
						// Sanitize HTML content before saving
						description := SanitizeHTML(item.Description)
						content := SanitizeHTML(getItemContent(item))
						existingItem.Title = item.Title
						existingItem.Link = item.Link
						existingItem.Description = description
						existingItem.Content = content
						existingItem.Author = getItemAuthor(item)
						if publishedAt != nil {
							existingItem.PublishedAt = publishedAt
						}
						if err := DB.Save(&existingItem).Error; err != nil {
							appLogger.Error().Err(err).Str("feed_url", feed.URL).Msg("Error updating item")
							mu.Lock()
							errors++
							mu.Unlock()
						} else {
							feedUpdated++
							mu.Lock()
							itemsUpdated++
							mu.Unlock()
						}
					}
				}

				// Add success log entry with created and updated counts
				addLogEntry("success", feed.URL, fmt.Sprintf("Successfully fetched feed: %d created, %d updated", feedCreated, feedUpdated))
			}
		}()
	}

	// Send all feeds to the channel
	for _, feed := range feeds {
		feedChan <- feed
	}
	close(feedChan)

	// Wait for all workers to finish
	wg.Wait()

	return itemsCreated, itemsUpdated, errors
}

// processSingleFeed processes a single feed by ID and returns created, updated, and error count
func processSingleFeed(feedID uint) (itemsCreated, itemsUpdated int, err error) {
	var feed Feed
	if err := DB.First(&feed, feedID).Error; err != nil {
		return 0, 0, err
	}

	fp := gofeed.NewParser()
	parsedFeed, err := fp.ParseURL(feed.URL)
	if err != nil {
		appLogger.Error().Str("feed_url", feed.URL).Err(err).Msg("Error parsing feed")
		// Update feed with error information
		now := time.Now()
		feed.LastError = err.Error()
		feed.LastErrorAt = &now
		DB.Save(&feed)
		// Add error log entry
		addLogEntry("error", feed.URL, fmt.Sprintf("Failed to fetch feed: %v", err))
		return 0, 0, err
	}

	// Update feed title and description if available
	if parsedFeed.Title != "" {
		feed.Title = parsedFeed.Title
	}
	if parsedFeed.Description != "" {
		feed.Description = parsedFeed.Description
	}
	// Update successful fetch timestamp and clear error
	now := time.Now()
	feed.LastSuccessfullyFetchedAt = &now
	feed.LastError = ""
	feed.LastErrorAt = nil
	DB.Save(&feed)

	// Local counters for this feed
	feedCreated := 0
	feedUpdated := 0

	// Process items for this feed
	for _, item := range parsedFeed.Items {
		// Determine GUID
		guid := item.GUID
		if guid == "" {
			guid = item.Link
		}

		// Parse published date
		var publishedAt *time.Time
		if item.PublishedParsed != nil {
			publishedAt = item.PublishedParsed
		} else if item.UpdatedParsed != nil {
			publishedAt = item.UpdatedParsed
		}

		// Check if item already exists by GUID
		var existingItem Item
		result := DB.Where("guid = ? AND feed_id = ?", guid, feed.ID).First(&existingItem)

		if result.Error != nil {
			// Item doesn't exist, create it
			// Sanitize HTML content before saving
			description := SanitizeHTML(item.Description)
			content := SanitizeHTML(getItemContent(item))
			newItem := Item{
				FeedID:      feed.ID,
				Title:       item.Title,
				Link:        item.Link,
				Description: description,
				Content:     content,
				Author:      getItemAuthor(item),
				PublishedAt: publishedAt,
				GUID:        guid,
			}
			if err := DB.Create(&newItem).Error; err != nil {
				appLogger.Error().Err(err).Str("feed_url", feed.URL).Msg("Error creating item")
			} else {
				feedCreated++
				// Track item creation statistics
				incrementItemsCreatedStats()
			}
		} else {
			// Item exists, update it
			// Sanitize HTML content before saving
			description := SanitizeHTML(item.Description)
			content := SanitizeHTML(getItemContent(item))
			existingItem.Title = item.Title
			existingItem.Link = item.Link
			existingItem.Description = description
			existingItem.Content = content
			existingItem.Author = getItemAuthor(item)
			if publishedAt != nil {
				existingItem.PublishedAt = publishedAt
			}
			if err := DB.Save(&existingItem).Error; err != nil {
				appLogger.Error().Err(err).Str("feed_url", feed.URL).Msg("Error updating item")
			} else {
				feedUpdated++
			}
		}
	}

	// Add success log entry with created and updated counts
	addLogEntry("success", feed.URL, fmt.Sprintf("Successfully fetched feed: %d created, %d updated", feedCreated, feedUpdated))

	return feedCreated, feedUpdated, nil
}

func fetchFeedItems(c *gin.Context) {
	session := sessions.Default(c)
	var feeds []Feed
	DB.Find(&feeds)

	if len(feeds) == 0 {
		addFlashError(session, "No feeds available")
		if err := session.Save(); err != nil {
			log.Printf("Error saving session: %v", err)
		}
		c.Redirect(http.StatusFound, "/admin/items")
		return
	}

	itemsCreated, itemsUpdated, errors := processAllFeeds()

	successMsg := fmt.Sprintf("Fetched items: %d created, %d updated", itemsCreated, itemsUpdated)
	if errors > 0 {
		successMsg += fmt.Sprintf(", %d errors", errors)
	}
	addFlashSuccess(session, successMsg)
	if err := session.Save(); err != nil {
		log.Printf("Error saving session in fetchFeedItems: %v", err)
	}
	c.Redirect(http.StatusFound, "/admin/items")
}

// startBackgroundFeedFetcher starts a background goroutine that fetches feeds at configured interval
func startBackgroundFeedFetcher() {
	interval := GetBackgroundFetchInterval()
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	// Fetch immediately on startup
	appLogger.Info().Int("interval", interval).Msg("Starting background feed fetcher")
	itemsCreated, itemsUpdated, errors := processAllFeeds()
	appLogger.Info().
		Int("items_created", itemsCreated).
		Int("items_updated", itemsUpdated).
		Int("errors", errors).
		Msg("Initial feed fetch completed")

	// Then fetch at configured interval
	for range ticker.C {
		appLogger.Info().Msg("Background feed fetch started")
		itemsCreated, itemsUpdated, errors := processAllFeeds()
		appLogger.Info().
			Int("items_created", itemsCreated).
			Int("items_updated", itemsUpdated).
			Int("errors", errors).
			Msg("Background feed fetch completed")
	}
}

func getItemContent(item *gofeed.Item) string {
	if item.Content != "" {
		return item.Content
	}
	return item.Description
}

func getItemAuthor(item *gofeed.Item) string {
	if item.Author != nil && item.Author.Name != "" {
		return item.Author.Name
	}
	if len(item.Authors) > 0 && item.Authors[0].Name != "" {
		return item.Authors[0].Name
	}
	return ""
}

// Tools handlers (only available when CYPRESS=true)

func showTools(c *gin.Context) {
	if !IsCypressMode() {
		c.JSON(http.StatusForbidden, gin.H{"error": "Tools page is only available when CYPRESS=true"})
		return
	}

	data := getTemplateData(c, gin.H{
		"title": "Tools",
	})
	c.HTML(http.StatusOK, "tools.html", data)
}

func clearAllTables(c *gin.Context) {
	if !IsCypressMode() {
		c.JSON(http.StatusForbidden, gin.H{"error": "Tools are only available when CYPRESS=true"})
		return
	}

	session := sessions.Default(c)

	// Clear all tables
	if err := DB.Exec("TRUNCATE TABLE items CASCADE").Error; err != nil {
		addFlashError(session, "Failed to clear items: "+err.Error())
		if err := session.Save(); err != nil {
			log.Printf("Error saving session: %v", err)
		}
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	if err := DB.Exec("TRUNCATE TABLE feeds CASCADE").Error; err != nil {
		addFlashError(session, "Failed to clear feeds: "+err.Error())
		if err := session.Save(); err != nil {
			log.Printf("Error saving session: %v", err)
		}
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	if err := DB.Exec("TRUNCATE TABLE users CASCADE").Error; err != nil {
		addFlashError(session, "Failed to clear users: "+err.Error())
		if err := session.Save(); err != nil {
			log.Printf("Error saving session: %v", err)
		}
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	addFlashSuccess(session, "All tables cleared successfully")
	if err := session.Save(); err != nil {
		log.Printf("Error saving session: %v", err)
	}
	c.Redirect(http.StatusFound, "/tools")
}

func clearTable(c *gin.Context) {
	if !IsCypressMode() {
		c.JSON(http.StatusForbidden, gin.H{"error": "Tools are only available when CYPRESS=true"})
		return
	}

	session := sessions.Default(c)
	tableName := c.PostForm("name")

	if tableName == "" {
		addFlashError(session, "Table name is required")
		if err := session.Save(); err != nil {
			log.Printf("Error saving session: %v", err)
		}
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	// Validate table name to prevent SQL injection
	validTables := map[string]bool{
		"users": true,
		"feeds": true,
		"items": true,
	}

	if !validTables[strings.ToLower(tableName)] {
		addFlashError(session, "Invalid table name. Allowed tables: users, feeds, items")
		if err := session.Save(); err != nil {
			log.Printf("Error saving session: %v", err)
		}
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	// Clear the specified table
	// Use parameterized query to prevent SQL injection
	tableNameLower := strings.ToLower(tableName)
	var sqlQuery string
	switch tableNameLower {
	case "users":
		sqlQuery = "TRUNCATE TABLE users CASCADE"
	case "feeds":
		sqlQuery = "TRUNCATE TABLE feeds CASCADE"
	case "items":
		sqlQuery = "TRUNCATE TABLE items CASCADE"
	default:
		addFlashError(session, "Invalid table name")
		if err := session.Save(); err != nil {
			log.Printf("Error saving session: %v", err)
		}
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	if err := DB.Exec(sqlQuery).Error; err != nil {
		addFlashError(session, fmt.Sprintf("Failed to clear table %s: %s", tableName, err.Error()))
		if err := session.Save(); err != nil {
			log.Printf("Error saving session: %v", err)
		}
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	addFlashSuccess(session, fmt.Sprintf("Table '%s' cleared successfully", tableName))
	if err := session.Save(); err != nil {
		log.Printf("Error saving session: %v", err)
	}
	c.Redirect(http.StatusFound, "/tools")
}

func seedUsers(c *gin.Context) {
	if !IsCypressMode() {
		c.JSON(http.StatusForbidden, gin.H{"error": "Tools are only available when CYPRESS=true"})
		return
	}

	session := sessions.Default(c)

	// Create admin user
	var user User
	result := DB.Where("username = ?", "admin").First(&user)
	if result.Error == gorm.ErrRecordNotFound {
		adminUser := User{Username: "admin", Password: "password"}
		if err := DB.Create(&adminUser).Error; err != nil {
			addFlashError(session, "Failed to create admin user: "+err.Error())
			if err := session.Save(); err != nil {
				log.Printf("Error saving session: %v", err)
			}
			c.Redirect(http.StatusFound, "/tools")
			return
		}
		addFlashSuccess(session, "Admin user 'admin' created with password 'password'")
	} else if result.Error != nil {
		addFlashError(session, "Failed to check for existing user: "+result.Error.Error())
		if err := session.Save(); err != nil {
			log.Printf("Error saving session: %v", err)
		}
		c.Redirect(http.StatusFound, "/tools")
		return
	} else {
		addFlashSuccess(session, "Admin user already exists")
	}

	if err := session.Save(); err != nil {
		log.Printf("Error saving session: %v", err)
	}
	c.Redirect(http.StatusFound, "/tools")
}

func seedUsersAndLogin(c *gin.Context) {
	if !IsCypressMode() {
		c.JSON(http.StatusForbidden, gin.H{"error": "Tools are only available when CYPRESS=true"})
		return
	}

	session := sessions.Default(c)

	// Create admin user (same logic as seedUsers)
	var user User
	result := DB.Where("username = ?", "admin").First(&user)
	if result.Error == gorm.ErrRecordNotFound {
		adminUser := User{Username: "admin", Password: "password"}
		if err := DB.Create(&adminUser).Error; err != nil {
			addFlashError(session, "Failed to create admin user: "+err.Error())
			if err := session.Save(); err != nil {
				log.Printf("Error saving session: %v", err)
			}
			c.Redirect(http.StatusFound, "/tools")
			return
		}
		// Reload user to get the ID
		if err := DB.Where("username = ?", "admin").First(&user).Error; err != nil {
			addFlashError(session, "Failed to find created user: "+err.Error())
			if err := session.Save(); err != nil {
				log.Printf("Error saving session: %v", err)
			}
			c.Redirect(http.StatusFound, "/tools")
			return
		}
	} else if result.Error != nil {
		addFlashError(session, "Failed to check for existing user: "+result.Error.Error())
		if err := session.Save(); err != nil {
			log.Printf("Error saving session: %v", err)
		}
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	// Login as admin user
	session.Set("user", user.ID)
	if err := session.Save(); err != nil {
		log.Printf("Error saving session in seedUsersAndLogin: %v", err)
		addFlashError(session, "Failed to save session")
		if err := session.Save(); err != nil {
			log.Printf("Error saving session: %v", err)
		}
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	// Set success message based on whether user was created or already existed
	if result.Error == gorm.ErrRecordNotFound {
		addFlashSuccess(session, "Admin user created and logged in successfully")
	} else {
		addFlashSuccess(session, "Logged in as admin successfully")
	}
	if err := session.Save(); err != nil {
		log.Printf("Error saving session: %v", err)
	}
	c.Redirect(http.StatusFound, "/admin/users")
}

func executeSQL(c *gin.Context) {
	if !IsCypressMode() {
		c.JSON(http.StatusForbidden, gin.H{"error": "Tools are only available when CYPRESS=true"})
		return
	}

	sqlQuery := c.PostForm("sql")
	if sqlQuery == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "SQL query is required"})
		return
	}

	// Execute SQL query
	var results []map[string]interface{}
	rows, err := DB.Raw(sqlQuery).Rows()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Error closing rows: %v", err)
		}
	}()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Scan rows
	for rows.Next() {
		// Create a slice of interface{} to hold column values
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Create a map for this row
		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			// Convert []byte to string for better JSON representation
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Return JSON response
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"columns": columns,
		"rows":    results,
		"count":   len(results),
	})
}

func dropDB(c *gin.Context) {
	if !IsCypressMode() {
		c.JSON(http.StatusForbidden, gin.H{"error": "Tools are only available when CYPRESS=true"})
		return
	}

	session := sessions.Default(c)
	dbname := getDBName()
	adminDSN := getAdminDSN()

	// Connect to postgres database using GORM
	db, err := gorm.Open(postgres.Open(adminDSN), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Info),
	})
	if err != nil {
		addFlashError(session, "Failed to connect to postgres database: "+err.Error())
		if err := session.Save(); err != nil {
			log.Printf("Error saving session: %v", err)
		}
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	// Get underlying sql.DB
	sqlDB, err := db.DB()
	if err != nil {
		addFlashError(session, "Failed to get database connection: "+err.Error())
		if err := session.Save(); err != nil {
			log.Printf("Error saving session: %v", err)
		}
		c.Redirect(http.StatusFound, "/tools")
		return
	}
	defer func() {
		if err := sqlDB.Close(); err != nil {
			log.Printf("Error closing database connection: %v", err)
		}
	}()

	// Terminate all connections to the target database
	_, err = sqlDB.Exec(fmt.Sprintf(`
		SELECT pg_terminate_backend(pg_stat_activity.pid)
		FROM pg_stat_activity
		WHERE pg_stat_activity.datname = '%s'
		AND pid <> pg_backend_pid();
	`, dbname))
	if err != nil {
		log.Printf("Warning: Failed to terminate connections: %v", err)
	}

	// Drop the database (quote identifier to handle special characters)
	_, err = sqlDB.Exec(fmt.Sprintf(`DROP DATABASE IF EXISTS "%s"`, dbname))
	if err != nil {
		addFlashError(session, "Failed to drop database: "+err.Error())
		if err := session.Save(); err != nil {
			log.Printf("Error saving session: %v", err)
		}
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	addFlashSuccess(session, fmt.Sprintf("Database '%s' dropped successfully", dbname))
	if err := session.Save(); err != nil {
		log.Printf("Error saving session: %v", err)
	}
	c.Redirect(http.StatusFound, "/tools")
}

func dropAllTables(c *gin.Context) {
	if !IsCypressMode() {
		c.JSON(http.StatusForbidden, gin.H{"error": "Tools are only available when CYPRESS=true"})
		return
	}

	session := sessions.Default(c)

	result, err := DropAllTables()
	if err != nil {
		addFlashError(session, err.Error())
		if err := session.Save(); err != nil {
			log.Printf("Error saving session: %v", err)
		}
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	if len(result.TableNames) == 0 {
		addFlashSuccess(session, "No tables found in database")
		if err := session.Save(); err != nil {
			log.Printf("Error saving session: %v", err)
		}
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	if len(result.Errors) > 0 {
		errorMsg := fmt.Sprintf("Dropped %d table(s), but encountered errors: %s", result.DroppedCount, strings.Join(result.Errors, "; "))
		addFlashError(session, errorMsg)
	} else {
		addFlashSuccess(session, fmt.Sprintf("Successfully dropped %d table(s)", result.DroppedCount))
	}

	if err := session.Save(); err != nil {
		log.Printf("Error saving session: %v", err)
	}
	c.Redirect(http.StatusFound, "/tools")
}

func createDB(c *gin.Context) {
	if !IsCypressMode() {
		c.JSON(http.StatusForbidden, gin.H{"error": "Tools are only available when CYPRESS=true"})
		return
	}

	session := sessions.Default(c)
	dbname := getDBName()
	adminDSN := getAdminDSN()

	// Connect to postgres database using GORM
	db, err := gorm.Open(postgres.Open(adminDSN), &gorm.Config{})
	if err != nil {
		addFlashError(session, "Failed to connect to postgres database: "+err.Error())
		if err := session.Save(); err != nil {
			log.Printf("Error saving session: %v", err)
		}
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	// Get underlying sql.DB
	sqlDB, err := db.DB()
	if err != nil {
		addFlashError(session, "Failed to get database connection: "+err.Error())
		if err := session.Save(); err != nil {
			log.Printf("Error saving session: %v", err)
		}
		c.Redirect(http.StatusFound, "/tools")
		return
	}
	defer func() {
		if err := sqlDB.Close(); err != nil {
			log.Printf("Error closing database connection: %v", err)
		}
	}()

	// Check if database already exists
	var exists bool
	err = sqlDB.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)",
		dbname,
	).Scan(&exists)
	if err != nil {
		addFlashError(session, "Failed to check if database exists: "+err.Error())
		if err := session.Save(); err != nil {
			log.Printf("Error saving session: %v", err)
		}
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	if exists {
		addFlashSuccess(session, fmt.Sprintf("Database '%s' already exists", dbname))
		if err := session.Save(); err != nil {
			log.Printf("Error saving session: %v", err)
		}
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	// Create the database (quote identifier to handle special characters)
	_, err = sqlDB.Exec(fmt.Sprintf(`CREATE DATABASE "%s"`, dbname))
	if err != nil {
		addFlashError(session, "Failed to create database: "+err.Error())
		if err := session.Save(); err != nil {
			log.Printf("Error saving session: %v", err)
		}
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	addFlashSuccess(session, fmt.Sprintf("Database '%s' created successfully", dbname))
	if err := session.Save(); err != nil {
		log.Printf("Error saving session: %v", err)
	}
	c.Redirect(http.StatusFound, "/tools")
}

func migrate(c *gin.Context) {
	if !IsCypressMode() {
		c.JSON(http.StatusForbidden, gin.H{"error": "Tools are only available when CYPRESS=true"})
		return
	}

	session := sessions.Default(c)
	dsn := getAppDSN()

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		addFlashError(session, "Failed to connect to database: "+err.Error())
		if err := session.Save(); err != nil {
			log.Printf("Error saving session: %v", err)
		}
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	// Run AutoMigrate for all models
	err = db.AutoMigrate(&User{}, &Feed{}, &Item{})
	if err != nil {
		addFlashError(session, "Failed to migrate database: "+err.Error())
		if err := session.Save(); err != nil {
			log.Printf("Error saving session: %v", err)
		}
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	addFlashSuccess(session, "Database migration completed successfully")
	if err := session.Save(); err != nil {
		log.Printf("Error saving session: %v", err)
	}
	c.Redirect(http.StatusFound, "/tools")
}

func dumpDBStructure(c *gin.Context) {
	if !IsCypressMode() {
		c.JSON(http.StatusForbidden, gin.H{"error": "Tools are only available when CYPRESS=true"})
		return
	}

	session := sessions.Default(c)
	ConnectDatabase()

	err := DumpDBStructure()
	if err != nil {
		addFlashError(session, "Failed to dump database structure: "+err.Error())
		if err := session.Save(); err != nil {
			log.Printf("Error saving session: %v", err)
		}
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	addFlashSuccess(session, "Database structure dumped successfully to structure.sql")
	if err := session.Save(); err != nil {
		log.Printf("Error saving session: %v", err)
	}
	c.Redirect(http.StatusFound, "/tools")
}

func dumpDBStructureAdmin(c *gin.Context) {
	session := sessions.Default(c)
	ConnectDatabase()

	err := DumpDBStructure()
	if err != nil {
		addFlashError(session, "Failed to dump database structure: "+err.Error())
		if err := session.Save(); err != nil {
			log.Printf("Error saving session: %v", err)
		}
		c.Redirect(http.StatusFound, "/admin/info")
		return
	}

	addFlashSuccess(session, "Database structure dumped successfully to structure.sql")
	if err := session.Save(); err != nil {
		log.Printf("Error saving session: %v", err)
	}
	c.Redirect(http.StatusFound, "/admin/info")
}
