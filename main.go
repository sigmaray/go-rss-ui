package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-contrib/multitemplate"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
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

		// Items routes
		admin.GET("/items", adminItemsIndex)
		admin.GET("/items/:id", showItem)
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

	data := addAuthToData(c, gin.H{
		"title": "Feed Management",
		"feeds": feeds,
	})

	if errorMsg := c.Query("error"); errorMsg != "" {
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

// Item handlers
func adminItemsIndex(c *gin.Context) {
	var items []Item
	query := DB.Preload("Feed")

	// Filter by feed if provided
	if feedID := c.Query("feed_id"); feedID != "" {
		query = query.Where("feed_id = ?", feedID)
	}

	query.Order("created_at DESC").Find(&items)

	data := addAuthToData(c, gin.H{
		"title": "Items",
		"items": items,
	})

	if errorMsg := c.Query("error"); errorMsg != "" {
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
