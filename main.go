package main

import (
	"fmt"
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
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// LogEntry represents a single log entry
type LogEntry struct {
	Timestamp time.Time
	Type      string // "success" or "error"
	FeedURL   string
	Message   string
}

// In-memory log storage with max 1000 entries
var (
	logEntries []LogEntry
	logMutex   sync.RWMutex
	maxLogSize = 1000
)

// addLogEntry adds a log entry to the in-memory log storage
// Maintains maximum of 1000 entries by removing oldest entries
func addLogEntry(logType, feedURL, message string) {
	logMutex.Lock()
	defer logMutex.Unlock()

	entry := LogEntry{
		Timestamp: time.Now(),
		Type:      logType,
		FeedURL:   feedURL,
		Message:   message,
	}

	logEntries = append(logEntries, entry)

	// Keep only the last maxLogSize entries
	if len(logEntries) > maxLogSize {
		logEntries = logEntries[len(logEntries)-maxLogSize:]
	}
}

// getLogEntries returns a copy of all log entries
func getLogEntries() []LogEntry {
	logMutex.RLock()
	defer logMutex.RUnlock()

	// Return a copy to prevent external modifications
	entries := make([]LogEntry, len(logEntries))
	copy(entries, logEntries)
	return entries
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

	fmt.Println(includes)

	// Generate our templates map from our layouts/ and includes/ directories
	for _, include := range includes {
		layoutCopy := make([]string, len(layouts))
		copy(layoutCopy, layouts)
		fmt.Println(layouts)
		// Include partials in each template
		files := append(layoutCopy, include)
		files = append(files, partials...)
		fmt.Println(files)
		r.AddFromFiles(filepath.Base(include), files...)
	}
	fmt.Println(r)
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

func main() {
	// Load environment variables from .env file
	LoadConfig()

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
		case "create-db":
			CommandCreateDB()
		case "fetch-feeds":
			CommandFetchFeeds()
		case "execute-sql":
			CommandExecuteSQL()
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
			fmt.Println("  create-db    - Create the application database")
			os.Exit(1)
		}
		return
	}

	// Run the web server
	ConnectDatabase()
	// Seed()

	// Start background feed fetcher if enabled
	if GetBackgroundFetchEnabled() {
		go startBackgroundFeedFetcher()
	} else {
		log.Println("Background feed fetcher is disabled")
	}

	r := gin.Default()

	r.HTMLRender = loadTemplates("./templates")

	r.Static("/static", "./static")
	r.Static("/test_feeds", "./test_feeds")

	store := cookie.NewStore([]byte("secret"))
	r.Use(sessions.Sessions("mysession", store))
	r.Use(AddAuthInfo())

	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", getTemplateData(c, gin.H{
			"title": "My RSS App",
		}))
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
	}

	r.GET("/login", showLogin)
	r.POST("/login", login)
	r.POST("/logout", logout)

	// Logs route (requires authentication)
	r.GET("/logs", AuthRequired(), showLogs)

	// Info route (requires authentication)
	r.GET("/info", AuthRequired(), showInfo)

	// Tools routes (only available when CYPRESS=true)
	if IsCypressMode() {
		tools := r.Group("/tools")
		tools.GET("", showTools)
		tools.POST("/clear-database", clearDatabase)
		tools.POST("/seed-users", seedUsers)
		tools.POST("/seed-feeds", seedFeeds)
		tools.POST("/drop-db", dropDB)
		tools.POST("/create-db", createDB)
		tools.POST("/migrate", migrate)
		tools.POST("/execute-sql", executeSQL)
	}

	r.Run(":8082")
}

func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		userID := session.Get("user")
		if userID == nil {
			log.Println("User not logged in")
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
				log.Println("Invalid user ID type in session")
				session.Clear()
				session.Save()
				c.Redirect(http.StatusFound, "/login")
				c.Abort()
				return
			}
		}

		result := DB.First(&user, userIDUint)
		if result.Error != nil {
			log.Printf("User with ID %d not found in database, invalidating session", userIDUint)
			session.Clear()
			session.Save()
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
	session.Save()
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
	username := c.PostForm("username")
	password := c.PostForm("password")

	if username == "" || password == "" {
		data := getTemplateData(c, gin.H{
			"title": "Create New User",
			"error": "Username and password are required",
		})
		c.HTML(http.StatusBadRequest, "create_user.html", data)
		return
	}

	// Check if username already exists
	var existingUser User
	if err := DB.Where("username = ?", username).First(&existingUser).Error; err == nil {
		// User with this username already exists
		data := getTemplateData(c, gin.H{
			"title": "Create New User",
			"error": "Username already exists",
		})
		c.HTML(http.StatusBadRequest, "create_user.html", data)
		return
	}

	user := User{Username: username, Password: password}
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
		session.Save()
		c.Redirect(http.StatusFound, "/admin/users")
		return
	}

	if err := DB.Unscoped().Delete(&user).Error; err != nil {
		addFlashError(session, "Failed to delete user: "+err.Error())
		session.Save()
		c.Redirect(http.StatusFound, "/admin/users")
		return
	}

	addFlashSuccess(session, "User deleted successfully")
	session.Save()
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
	feedURL := c.PostForm("url")

	if feedURL == "" {
		data := getTemplateData(c, gin.H{
			"title": "Create New Feed",
			"error": "URL is required",
		})
		c.HTML(http.StatusBadRequest, "create_feed.html", data)
		return
	}

	feed := Feed{URL: feedURL}
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
		session.Save()
		c.Redirect(http.StatusFound, "/admin/feeds")
		return
	}

	itemsCreated, itemsUpdated, err := processSingleFeed(feed.ID)
	if err != nil {
		addFlashError(session, fmt.Sprintf("Failed to fetch feed: %v", err))
		session.Save()
		c.Redirect(http.StatusFound, "/admin/feeds")
		return
	}

	successMsg := fmt.Sprintf("Feed fetched successfully: %d items created, %d items updated", itemsCreated, itemsUpdated)
	addFlashSuccess(session, successMsg)
	session.Save()
	c.Redirect(http.StatusFound, "/admin/feeds")
}

func deleteFeed(c *gin.Context) {
	id := c.Param("id")
	session := sessions.Default(c)

	var feed Feed
	if err := DB.First(&feed, id).Error; err != nil {
		addFlashError(session, "Feed not found")
		session.Save()
		c.Redirect(http.StatusFound, "/admin/feeds")
		return
	}

	// Items will be deleted automatically due to CASCADE constraint
	if err := DB.Unscoped().Delete(&feed).Error; err != nil {
		addFlashError(session, "Failed to delete feed: "+err.Error())
		session.Save()
		c.Redirect(http.StatusFound, "/admin/feeds")
		return
	}

	addFlashSuccess(session, "Feed deleted successfully")
	session.Save()
	c.Redirect(http.StatusFound, "/admin/feeds")
}

func deleteAllFeeds(c *gin.Context) {
	session := sessions.Default(c)

	// Delete all feeds (items will be deleted automatically due to CASCADE constraint)
	result := DB.Unscoped().Delete(&Feed{}, "1 = 1")
	if result.Error != nil {
		addFlashError(session, "Failed to delete all feeds")
		session.Save()
		c.Redirect(http.StatusFound, "/admin/feeds")
		return
	}

	addFlashSuccess(session, "All feeds deleted successfully")
	session.Save()
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
	session.Save()

	// Redirect based on where the request came from
	if c.Request.URL.Path == "/tools/seed-feeds" {
		c.Redirect(http.StatusFound, "/tools")
	} else {
		c.Redirect(http.StatusFound, "/admin/feeds")
	}
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
	// Reverse order to show newest first
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}

	data := getTemplateData(c, gin.H{
		"title":   "Logs",
		"entries": entries,
	})
	c.HTML(http.StatusOK, "logs.html", data)
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
			Name:        "DATABASE_URL",
			Value:       maskPassword(os.Getenv("DATABASE_URL")),
			Description: "Complete PostgreSQL connection string (takes precedence over individual DB_* variables)",
		},
		{
			Name:        "DB_HOST",
			Value:       getEnvOrDefault("DB_HOST", "localhost (default)"),
			Description: "PostgreSQL database host",
		},
		{
			Name:        "DB_USER",
			Value:       getEnvOrDefault("DB_USER", "postgres (default)"),
			Description: "PostgreSQL database user",
		},
		{
			Name:        "DB_PASSWORD",
			Value:       maskPassword(getEnvOrDefault("DB_PASSWORD", "postgres (default)")),
			Description: "PostgreSQL database password",
		},
		{
			Name:        "DB_NAME",
			Value:       getEnvOrDefault("DB_NAME", "go_rss_ui_2 (default)"),
			Description: "PostgreSQL database name",
		},
		{
			Name:        "DB_PORT",
			Value:       getEnvOrDefault("DB_PORT", "5432 (default)"),
			Description: "PostgreSQL database port",
		},
		{
			Name:        "DB_SSLMODE",
			Value:       getEnvOrDefault("DB_SSLMODE", "disable (default)"),
			Description: "PostgreSQL SSL mode",
		},
		{
			Name:        "DB_TIMEZONE",
			Value:       getEnvOrDefault("DB_TIMEZONE", "Asia/Shanghai (default)"),
			Description: "PostgreSQL timezone",
		},
		{
			Name:        "BACKGROUND_FETCH_ENABLED",
			Value:       getEnvValueOrDefault("BACKGROUND_FETCH_ENABLED", "true (default)"),
			Description: "Enable/disable background feed fetching",
		},
		{
			Name:        "BACKGROUND_FETCH_INTERVAL",
			Value:       fmt.Sprintf("%d (default: 60)", GetBackgroundFetchInterval()),
			Description: "Background feed fetch interval in seconds",
		},
		{
			Name:        "CYPRESS",
			Value:       getEnvValueOrDefault("CYPRESS", "false (default)"),
			Description: "Enable Cypress mode (enables /tools page for testing)",
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

	data := getTemplateData(c, gin.H{
		"title": item.Title,
		"item":  item,
	})
	c.HTML(http.StatusOK, "item.html", data)
}

func deleteAllItems(c *gin.Context) {
	session := sessions.Default(c)
	result := DB.Delete(&Item{}, "1 = 1")
	if result.Error != nil {
		addFlashError(session, "Failed to delete all items")
		session.Save()
		c.Redirect(http.StatusFound, "/admin/items")
		return
	}
	addFlashSuccess(session, "All items deleted successfully")
	session.Save()
	c.Redirect(http.StatusFound, "/admin/items")
}

// processFeeds fetches and processes all feeds, returns statistics
func processFeeds() (itemsCreated, itemsUpdated, errors int) {
	return processFeedsWithFilter(false)
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
					log.Printf("Error parsing feed %s: %v", feed.URL, err)
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
						newItem := Item{
							FeedID:      feed.ID,
							Title:       item.Title,
							Link:        item.Link,
							Description: item.Description,
							Content:     getItemContent(item),
							Author:      getItemAuthor(item),
							PublishedAt: publishedAt,
							GUID:        guid,
						}
						if err := DB.Create(&newItem).Error; err != nil {
							log.Printf("Error creating item: %v", err)
							mu.Lock()
							errors++
							mu.Unlock()
						} else {
							feedCreated++
							mu.Lock()
							itemsCreated++
							mu.Unlock()
						}
					} else {
						// Item exists, update it
						existingItem.Title = item.Title
						existingItem.Link = item.Link
						existingItem.Description = item.Description
						existingItem.Content = getItemContent(item)
						existingItem.Author = getItemAuthor(item)
						if publishedAt != nil {
							existingItem.PublishedAt = publishedAt
						}
						if err := DB.Save(&existingItem).Error; err != nil {
							log.Printf("Error updating item: %v", err)
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
		log.Printf("Error parsing feed %s: %v", feed.URL, err)
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
			newItem := Item{
				FeedID:      feed.ID,
				Title:       item.Title,
				Link:        item.Link,
				Description: item.Description,
				Content:     getItemContent(item),
				Author:      getItemAuthor(item),
				PublishedAt: publishedAt,
				GUID:        guid,
			}
			if err := DB.Create(&newItem).Error; err != nil {
				log.Printf("Error creating item: %v", err)
			} else {
				feedCreated++
			}
		} else {
			// Item exists, update it
			existingItem.Title = item.Title
			existingItem.Link = item.Link
			existingItem.Description = item.Description
			existingItem.Content = getItemContent(item)
			existingItem.Author = getItemAuthor(item)
			if publishedAt != nil {
				existingItem.PublishedAt = publishedAt
			}
			if err := DB.Save(&existingItem).Error; err != nil {
				log.Printf("Error updating item: %v", err)
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
		session.Save()
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
	log.Printf("Starting background feed fetcher (interval: %d seconds)", interval)
	itemsCreated, itemsUpdated, errors := processAllFeeds()
	log.Printf("Initial feed fetch completed: %d created, %d updated, %d errors", itemsCreated, itemsUpdated, errors)

	// Then fetch at configured interval
	for range ticker.C {
		log.Println("Background feed fetch started")
		itemsCreated, itemsUpdated, errors := processAllFeeds()
		log.Printf("Background feed fetch completed: %d created, %d updated, %d errors", itemsCreated, itemsUpdated, errors)
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

func clearDatabase(c *gin.Context) {
	if !IsCypressMode() {
		c.JSON(http.StatusForbidden, gin.H{"error": "Tools are only available when CYPRESS=true"})
		return
	}

	session := sessions.Default(c)

	// Clear all tables
	if err := DB.Exec("TRUNCATE TABLE items CASCADE").Error; err != nil {
		addFlashError(session, "Failed to clear items: "+err.Error())
		session.Save()
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	if err := DB.Exec("TRUNCATE TABLE feeds CASCADE").Error; err != nil {
		addFlashError(session, "Failed to clear feeds: "+err.Error())
		session.Save()
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	if err := DB.Exec("TRUNCATE TABLE users CASCADE").Error; err != nil {
		addFlashError(session, "Failed to clear users: "+err.Error())
		session.Save()
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	addFlashSuccess(session, "Database cleared successfully")
	session.Save()
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
			session.Save()
			c.Redirect(http.StatusFound, "/tools")
			return
		}
		addFlashSuccess(session, "Admin user 'admin' created with password 'password'")
	} else if result.Error != nil {
		addFlashError(session, "Failed to check for existing user: "+result.Error.Error())
		session.Save()
		c.Redirect(http.StatusFound, "/tools")
		return
	} else {
		addFlashSuccess(session, "Admin user already exists")
	}

	session.Save()
	c.Redirect(http.StatusFound, "/tools")
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
	defer rows.Close()

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
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		addFlashError(session, "Failed to connect to postgres database: "+err.Error())
		session.Save()
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	// Get underlying sql.DB
	sqlDB, err := db.DB()
	if err != nil {
		addFlashError(session, "Failed to get database connection: "+err.Error())
		session.Save()
		c.Redirect(http.StatusFound, "/tools")
		return
	}
	defer sqlDB.Close()

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
		session.Save()
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	addFlashSuccess(session, fmt.Sprintf("Database '%s' dropped successfully", dbname))
	session.Save()
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
		session.Save()
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	// Get underlying sql.DB
	sqlDB, err := db.DB()
	if err != nil {
		addFlashError(session, "Failed to get database connection: "+err.Error())
		session.Save()
		c.Redirect(http.StatusFound, "/tools")
		return
	}
	defer sqlDB.Close()

	// Check if database already exists
	var exists bool
	err = sqlDB.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)",
		dbname,
	).Scan(&exists)
	if err != nil {
		addFlashError(session, "Failed to check if database exists: "+err.Error())
		session.Save()
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	if exists {
		addFlashSuccess(session, fmt.Sprintf("Database '%s' already exists", dbname))
		session.Save()
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	// Create the database (quote identifier to handle special characters)
	_, err = sqlDB.Exec(fmt.Sprintf(`CREATE DATABASE "%s"`, dbname))
	if err != nil {
		addFlashError(session, "Failed to create database: "+err.Error())
		session.Save()
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	addFlashSuccess(session, fmt.Sprintf("Database '%s' created successfully", dbname))
	session.Save()
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
		session.Save()
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	// Run AutoMigrate for all models
	err = db.AutoMigrate(&User{}, &Feed{}, &Item{})
	if err != nil {
		addFlashError(session, "Failed to migrate database: "+err.Error())
		session.Save()
		c.Redirect(http.StatusFound, "/tools")
		return
	}

	addFlashSuccess(session, "Database migration completed successfully")
	session.Save()
	c.Redirect(http.StatusFound, "/tools")
}
