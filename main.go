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

	// Start background feed fetcher
	go startBackgroundFeedFetcher()

	r := gin.Default()

	r.HTMLRender = loadTemplates("./templates")

	r.Static("/static", "./static")

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

	r.Run(":8082")
}

func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		user := session.Get("user")
		if user == nil {
			log.Println("User not logged in")
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
		user := session.Get("user")
		if user != nil {
			c.Set("isAuthenticated", true)
			c.Set("username", user)
		} else {
			c.Set("isAuthenticated", false)
		}
		c.Next()
	}
}

// addAuthToData adds authentication info to template data
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
	return data
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
	session.Set("user", user.Username)
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
	DB.Find(&users)

	data := gin.H{
		"title": "User Management",
		"users": users,
	}

	// Add auth info
	data = addAuthToData(c, data)

	// Check for error in query parameter
	if errorMsg := c.Query("error"); errorMsg != "" {
		data["error"] = errorMsg
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

	var user User
	if err := DB.First(&user, id).Error; err != nil {
		c.HTML(http.StatusNotFound, "users.html", gin.H{
			"title": "User Management",
			"error": "User not found",
			"users": []User{},
		})
		return
	}

	if err := DB.Delete(&user).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "users.html", gin.H{
			"title": "User Management",
			"error": "Failed to delete user: " + err.Error(),
			"users": []User{},
		})
		return
	}

	c.Redirect(http.StatusFound, "/admin/users")
}

// Feed handlers
func adminFeedsIndex(c *gin.Context) {
	var feeds []Feed
	DB.Find(&feeds)

	session := sessions.Default(c)
	data := addAuthToData(c, gin.H{
		"title": "Feed Management",
		"feeds": feeds,
	})

	// Get flash messages
	flashes := session.Flashes()
	for _, flash := range flashes {
		flashMap := flash.(map[string]interface{})
		if msgType, ok := flashMap["type"].(string); ok {
			if msg, ok := flashMap["message"].(string); ok {
				if msgType == "error" {
					data["error"] = msg
				} else if msgType == "success" {
					data["success"] = msg
				}
			}
		}
	}
	session.Save()

	c.HTML(http.StatusOK, "feeds.html", data)
}

func showCreateFeedForm(c *gin.Context) {
	data := addAuthToData(c, gin.H{
		"title": "Create New Feed",
	})
	c.HTML(http.StatusOK, "create_feed.html", data)
}

func createFeed(c *gin.Context) {
	url := c.PostForm("url")

	if url == "" {
		data := addAuthToData(c, gin.H{
			"title": "Create New Feed",
			"error": "URL is required",
		})
		c.HTML(http.StatusBadRequest, "create_feed.html", data)
		return
	}

	feed := Feed{URL: url}
	if err := DB.Create(&feed).Error; err != nil {
		data := addAuthToData(c, gin.H{
			"title": "Create New Feed",
			"error": "Failed to create feed: " + err.Error(),
		})
		c.HTML(http.StatusInternalServerError, "create_feed.html", data)
		return
	}

	c.Redirect(http.StatusFound, "/admin/feeds")
}

func deleteFeed(c *gin.Context) {
	id := c.Param("id")

	var feed Feed
	if err := DB.First(&feed, id).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/feeds?error=Feed+not+found")
		return
	}

	// Delete associated items first
	DB.Where("feed_id = ?", feed.ID).Delete(&Item{})

	if err := DB.Delete(&feed).Error; err != nil {
		c.Redirect(http.StatusFound, "/admin/feeds?error=Failed+to+delete+feed")
		return
	}

	c.Redirect(http.StatusFound, "/admin/feeds")
}

func deleteAllFeeds(c *gin.Context) {
	session := sessions.Default(c)

	// Delete all items first (due to foreign key constraint)
	result := DB.Delete(&Item{}, "1 = 1")
	if result.Error != nil {
		session.AddFlash(map[string]interface{}{"type": "error", "message": "Failed to delete items"})
		session.Save()
		c.Redirect(http.StatusFound, "/admin/feeds")
		return
	}

	// Delete all feeds
	result = DB.Delete(&Feed{}, "1 = 1")
	if result.Error != nil {
		session.AddFlash(map[string]interface{}{"type": "error", "message": "Failed to delete all feeds"})
		session.Save()
		c.Redirect(http.StatusFound, "/admin/feeds")
		return
	}

	session.AddFlash(map[string]interface{}{"type": "success", "message": "All feeds deleted successfully"})
	session.Save()
	c.Redirect(http.StatusFound, "/admin/feeds")
}

func seedFeeds(c *gin.Context) {
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

	session.AddFlash(map[string]interface{}{"type": "success", "message": successMsg})
	session.Save()
	c.Redirect(http.StatusFound, "/admin/feeds")
}

// Item handlers
func adminItemsIndex(c *gin.Context) {
	var items []Item
	query := DB.Preload("Feed")

	// Filter by feed if provided
	if feedID := c.Query("feed_id"); feedID != "" {
		query = query.Where("feed_id = ?", feedID)
	}

	query.Order("created_at DESC").Find(&items)

	session := sessions.Default(c)
	data := addAuthToData(c, gin.H{
		"title": "Items",
		"items": items,
	})

	// Get flash messages
	flashes := session.Flashes()
	for _, flash := range flashes {
		flashMap := flash.(map[string]interface{})
		if msgType, ok := flashMap["type"].(string); ok {
			if msg, ok := flashMap["message"].(string); ok {
				if msgType == "error" {
					data["error"] = msg
				} else if msgType == "success" {
					data["success"] = msg
				}
			}
		}
	}
	session.Save()

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
		session.AddFlash(map[string]interface{}{"type": "error", "message": "Failed to delete all items"})
		session.Save()
		c.Redirect(http.StatusFound, "/admin/items")
		return
	}
	session.AddFlash(map[string]interface{}{"type": "success", "message": "All items deleted successfully"})
	session.Save()
	c.Redirect(http.StatusFound, "/admin/items")
}

// processFeeds fetches and processes all feeds, returns statistics
func processFeeds() (itemsCreated, itemsUpdated, errors int) {
	var feeds []Feed
	DB.Find(&feeds)

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
		session.AddFlash(map[string]interface{}{"type": "error", "message": "No feeds available"})
		session.Save()
		c.Redirect(http.StatusFound, "/admin/items")
		return
	}

	itemsCreated, itemsUpdated, errors := processFeeds()

	successMsg := fmt.Sprintf("Fetched items: %d created, %d updated", itemsCreated, itemsUpdated)
	if errors > 0 {
		successMsg += fmt.Sprintf(", %d errors", errors)
	}
	session.AddFlash(map[string]interface{}{"type": "success", "message": successMsg})
	session.Save()
	c.Redirect(http.StatusFound, "/admin/items")
}

// startBackgroundFeedFetcher starts a background goroutine that fetches feeds every 2 minutes
func startBackgroundFeedFetcher() {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	// Fetch immediately on startup
	log.Println("Starting background feed fetcher")
	itemsCreated, itemsUpdated, errors := processFeeds()
	log.Printf("Initial feed fetch completed: %d created, %d updated, %d errors", itemsCreated, itemsUpdated, errors)

	// Then fetch every 2 minutes
	for range ticker.C {
		log.Println("Background feed fetch started")
		itemsCreated, itemsUpdated, errors := processFeeds()
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
