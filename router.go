package main

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"go-rss-ui/config"
	"go-rss-ui/handlers"
)

func setupRouter() *gin.Engine {
	router := gin.Default()

	router.HTMLRender = loadTemplates("./templates")
	router.Static("/static", "./static")

	store := cookie.NewStore([]byte(config.GetSessionSecret()))
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 30,
		HttpOnly: true,
		Secure:   config.GetSessionSecure(),
		SameSite: http.SameSiteLaxMode,
	})

	router.Use(sessions.Sessions("mysession", store))
	router.Use(handlers.AddAuthInfo())

	router.GET("/", handlers.Home)
	router.GET("/feeds", handlers.PublicFeeds)

	admin := router.Group("/admin")
	admin.Use(handlers.AuthRequired())
	{
		admin.GET("/", handlers.AdminIndex)
		admin.GET("/users", handlers.UsersIndex)
		admin.GET("/users/new", handlers.ShowCreateUserForm)
		admin.POST("/users", handlers.CreateUser)
		admin.GET("/users/:id/edit", handlers.ShowEditUserForm)
		admin.POST("/users/:id/edit", handlers.EditUser)
		admin.POST("/users/:id/delete", handlers.DeleteUser)

		admin.GET("/feeds", handlers.FeedsIndex)
		admin.GET("/feeds/new", handlers.ShowCreateFeedForm)
		admin.POST("/feeds", handlers.CreateFeed)
		admin.GET("/feeds/:id/edit", handlers.ShowEditFeedForm)
		admin.POST("/feeds/:id/edit", handlers.EditFeed)
		admin.GET("/feeds/:id", handlers.ShowFeed)
		admin.POST("/feeds/:id/fetch", handlers.FetchSingleFeed)
		admin.POST("/feeds/:id/delete", handlers.DeleteFeed)
		admin.POST("/feeds/delete-all", handlers.DeleteAllFeeds)
		admin.POST("/feeds/seed", handlers.SeedFeeds)

		admin.GET("/items", handlers.ItemsIndex)
		admin.GET("/items/:id", handlers.ShowItem)
		admin.POST("/items/fetch", handlers.FetchFeedItems)
		admin.POST("/items/delete-all", handlers.DeleteAllItems)

		admin.GET("/feed-fetching-log", handlers.ShowLogs)
		admin.GET("/zerolog", handlers.ShowZerolog)
		admin.GET("/chart", handlers.ShowChart)
		admin.GET("/info", handlers.ShowInfo)
		admin.POST("/info/dump-db-structure", handlers.DumpDBStructureAdmin)
	}

	router.GET("/login", handlers.ShowLogin)
	router.POST("/login", handlers.Login)
	router.POST("/logout", handlers.Logout)

	if config.IsCypressMode() {
		router.GET("/test_feeds/*filepath", handlers.TestFeeds)

		tools := router.Group("/tools")
		tools.GET("", handlers.ShowTools)
		tools.POST("/clear-all-tables", handlers.ClearAllTables)
		tools.POST("/clear-table", handlers.ClearTable)
		tools.POST("/seed-users", handlers.SeedUsers)
		tools.POST("/seed-users-and-login", handlers.SeedUsersAndLogin)
		tools.POST("/seed-feeds", handlers.SeedFeeds)
		tools.POST("/drop-db", handlers.DropDB)
		tools.POST("/drop-all-tables", handlers.DropAllTables)
		tools.POST("/create-db", handlers.CreateDB)
		tools.POST("/automigrate", handlers.Migrate)
		tools.POST("/migrate", handlers.Migrate)
		tools.POST("/dump-db-structure", handlers.DumpDBStructure)
		tools.POST("/execute-sql", handlers.ExecuteSQL)
	}

	return router
}
