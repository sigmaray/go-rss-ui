package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gin-contrib/multitemplate"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/mmcdole/gofeed"
	"gorm.io/gorm"
)

func loadTemplates(templatesDir string) multitemplate.Renderer {
	r := multitemplate.NewRenderer()

	layouts, err := filepath.Glob(templatesDir + "/layouts/*.html")
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
		files := append(layoutCopy, include)
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
		case "seed":
			CommandSeed()
		case "migrate":
			CommandMigrate()
		case "drop-db":
			CommandDropDB()
		case "create-db":
			CommandCreateDB()
		default:
			fmt.Println("Unknown command:", command)
			fmt.Println("\nAvailable commands:")
			fmt.Println("  clear-users  - Clear all data from users table")
			fmt.Println("  seed         - Create a standard admin user")
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
		c.HTML(http.StatusOK, "index.html", gin.H{
			"title": "My RSS App",
		})
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

	// Tools routes (only available when CYPRESS=true)
	if IsCypressMode() {
		tools := r.Group("/tools")
		tools.GET("", showTools)
		tools.POST("/clear-database", clearDatabase)
		tools.POST("/seed-users", seedUsers)
		tools.POST("/seed-feeds", seedFeeds)
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

// AddAuthInfo adds authentication info to context for all requests
func AddAuthInfo() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
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
				c.Next()
				return
			}

			// Load user from database to get username
			var user User
			if err := DB.First(&user, userIDUint).Error; err == nil {
				c.Set("isAuthenticated", true)
				c.Set("username", user.Username)
				c.Set("userID", userIDUint)
			} else {
				c.Set("isAuthenticated", false)
			}
		} else {
			c.Set("isAuthenticated", false)
		}
		c.Next()
	}
}

// addAuthToData adds authentication info and CYPRESS mode to template data
func addAuthToData(c *gin.Context, data gin.H) gin.H {
	if data == nil {
		data = gin.H{}
	}
	if isAuth, exists := c.Get("isAuthenticated"); exists {
		data["isAuthenticated"] = isAuth
	}
	if username, exists := c.Get("username"); exists {
		data["username"] = username
	}
	// Add CYPRESS mode info
	data["isCypressMode"] = IsCypressMode()
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
	c.HTML(http.StatusOK, "login.html", gin.H{
		"title": "Login",
	})
}

func login(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	var user User
	result := DB.Where("username = ?", username).First(&user)
	if result.Error != nil {
		c.HTML(http.StatusUnauthorized, "login.html", gin.H{
			"title":   "Login",
			"message": "Invalid credentials",
		})
		return
	}

	if !user.CheckPassword(password) {
		c.HTML(http.StatusUnauthorized, "login.html", gin.H{
			"title":   "Login",
			"message": "Invalid credentials",
		})
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

	// Ensure page is at least 1
	if page.Page < 1 {
		page.Page = 1
	}

	prevPage := page.Page - 1
	if prevPage < 1 {
		prevPage = 0 // 0 means no previous page
	}

	nextPage := page.Page + 1
	if nextPage > page.TotalPages {
		nextPage = 0 // 0 means no next page
	}

	session := sessions.Default(c)

	// Get flash messages BEFORE creating data to ensure they're preserved
	successMsg, errorMsg := getFlashMessages(session)
	if err := session.Save(); err != nil {
		log.Printf("Error saving session in adminIndex: %v", err)
	}

	data := gin.H{
		"title":    "User Management",
		"users":    page.Items,
		"page":     page,
		"pages":    generatePageNumbers(page.Page, page.TotalPages),
		"prevPage": prevPage,
		"nextPage": nextPage,
	}

	// Add auth info
	data = addAuthToData(c, data)

	// Add flash messages to data
	if successMsg != "" {
		data["success"] = successMsg
	}
	if errorMsg != "" {
		data["error"] = errorMsg
	}

	// Check for error in query parameter (for backward compatibility)
	if queryError := c.Query("error"); queryError != "" && errorMsg == "" {
		data["error"] = queryError
	}

	c.HTML(http.StatusOK, "users.html", data)
}

func showCreateUserForm(c *gin.Context) {
	data := addAuthToData(c, gin.H{
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

	data := addAuthToData(c, gin.H{
		"title": "Edit User",
		"user":  user,
	})
	c.HTML(http.StatusOK, "edit_user.html", data)
}

func createUser(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	if username == "" || password == "" {
		data := addAuthToData(c, gin.H{
			"title": "Create New User",
			"error": "Username and password are required",
		})
		c.HTML(http.StatusBadRequest, "create_user.html", data)
		return
	}

	user := User{Username: username, Password: password}
	if err := DB.Create(&user).Error; err != nil {
		data := addAuthToData(c, gin.H{
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
		data := addAuthToData(c, gin.H{
			"title": "Edit User",
			"error": "User not found",
			"user":  user,
		})
		c.HTML(http.StatusNotFound, "edit_user.html", data)
		return
	}

	if username != "" {
		user.Username = username
	}
	if password != "" {
		user.Password = password
	}

	if err := DB.Save(&user).Error; err != nil {
		data := addAuthToData(c, gin.H{
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

	if err := DB.Delete(&user).Error; err != nil {
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

	// Ensure page is at least 1
	if page.Page < 1 {
		page.Page = 1
	}

	prevPage := page.Page - 1
	if prevPage < 1 {
		prevPage = 0 // 0 means no previous page
	}

	nextPage := page.Page + 1
	if nextPage > page.TotalPages {
		nextPage = 0 // 0 means no next page
	}

	session := sessions.Default(c)
	data := addAuthToData(c, gin.H{
		"title":    "Feed Management",
		"feeds":    page.Items,
		"page":     page,
		"pages":    generatePageNumbers(page.Page, page.TotalPages),
		"prevPage": prevPage,
		"nextPage": nextPage,
	})

	// Get flash messages
	successMsg, errorMsg := getFlashMessages(session)
	if err := session.Save(); err != nil {
		log.Printf("Error saving session in adminFeedsIndex: %v", err)
	}
	if successMsg != "" {
		data["success"] = successMsg
	}
	if errorMsg != "" {
		data["error"] = errorMsg
	}

	c.HTML(http.StatusOK, "feeds.html", data)
}

func showCreateFeedForm(c *gin.Context) {
	data := addAuthToData(c, gin.H{
		"title": "Create New Feed",
	})
	c.HTML(http.StatusOK, "create_feed.html", data)
}

func createFeed(c *gin.Context) {
	feedURL := c.PostForm("url")

	if feedURL == "" {
		data := addAuthToData(c, gin.H{
			"title": "Create New Feed",
			"error": "URL is required",
		})
		c.HTML(http.StatusBadRequest, "create_feed.html", data)
		return
	}

	feed := Feed{URL: feedURL}
	if err := DB.Create(&feed).Error; err != nil {
		data := addAuthToData(c, gin.H{
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

	// Delete associated items first
	DB.Where("feed_id = ?", feed.ID).Delete(&Item{})

	if err := DB.Delete(&feed).Error; err != nil {
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

	// Delete all items first (due to foreign key constraint)
	result := DB.Delete(&Item{}, "1 = 1")
	if result.Error != nil {
		addFlashError(session, "Failed to delete items")
		session.Save()
		c.Redirect(http.StatusFound, "/admin/feeds")
		return
	}

	// Delete all feeds
	result = DB.Delete(&Feed{}, "1 = 1")
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

	defaultFeeds := []string{
		"https://feeds.bbci.co.uk/news/rss.xml",
		"http://rss.cnn.com/rss/cnn_topstories.rss",
		"https://www.wired.com/feed/rss",
	}

	feedsCreated := 0
	feedsExisted := 0
	errors := 0

	for _, feedURL := range defaultFeeds {
		var feed Feed
		result := DB.Where("url = ?", feedURL).First(&feed)
		if result.Error == gorm.ErrRecordNotFound {
			feed := Feed{URL: feedURL}
			if err := DB.Create(&feed).Error; err != nil {
				log.Printf("Failed to create feed %s: %v", feedURL, err)
				errors++
			} else {
				feedsCreated++
			}
		} else if result.Error != nil {
			log.Printf("Failed to check for existing feed %s: %v", feedURL, result.Error)
			errors++
		} else {
			feedsExisted++
		}
	}

	successMsg := fmt.Sprintf("Seeded feeds: %d created", feedsCreated)
	if feedsExisted > 0 {
		successMsg += fmt.Sprintf(", %d already existed", feedsExisted)
	}
	if errors > 0 {
		successMsg += fmt.Sprintf(", %d errors", errors)
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

	// Ensure page is at least 1
	if page.Page < 1 {
		page.Page = 1
	}

	prevPage := page.Page - 1
	if prevPage < 1 {
		prevPage = 0 // 0 means no previous page
	}

	nextPage := page.Page + 1
	if nextPage > page.TotalPages {
		nextPage = 0 // 0 means no next page
	}

	session := sessions.Default(c)

	// Get flash messages BEFORE creating data to ensure they're preserved
	successMsg, errorMsg := getFlashMessages(session)
	if err := session.Save(); err != nil {
		log.Printf("Error saving session in adminItemsIndex: %v", err)
	}

	data := addAuthToData(c, gin.H{
		"title":    "Items",
		"items":    page.Items,
		"page":     page,
		"pages":    generatePageNumbers(page.Page, page.TotalPages),
		"prevPage": prevPage,
		"nextPage": nextPage,
	})

	// Add flash messages to data
	if successMsg != "" {
		data["success"] = successMsg
	}
	if errorMsg != "" {
		data["error"] = errorMsg
	}

	c.HTML(http.StatusOK, "items.html", data)
}

func showItem(c *gin.Context) {
	id := c.Param("id")

	var item Item
	if err := DB.Preload("Feed").First(&item, id).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/items?error=Item+not+found")
		return
	}

	data := addAuthToData(c, gin.H{
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
							mu.Lock()
							itemsUpdated++
							mu.Unlock()
						}
					}
				}
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

	data := addAuthToData(c, gin.H{
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
