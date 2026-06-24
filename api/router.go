package api

import (
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// RegisterRoutes mounts REST API and Swagger UI on the router.
func RegisterRoutes(router *gin.Engine) {
	v1 := router.Group("/api/v1")
	{
		v1.POST("/auth/login", Login)
		v1.POST("/auth/logout", Logout)

		protected := v1.Group("")
		protected.Use(APIAuthRequired())
		{
			protected.GET("/users", ListUsers)
			protected.POST("/users", CreateUser)
			protected.GET("/users/:id", GetUser)
			protected.PUT("/users/:id", UpdateUser)
			protected.DELETE("/users/:id", DeleteUser)

			protected.GET("/feeds", ListFeeds)
			protected.POST("/feeds", CreateFeed)
			protected.GET("/feeds/:id", GetFeed)
			protected.PUT("/feeds/:id", UpdateFeed)
			protected.DELETE("/feeds/:id", DeleteFeed)
			protected.POST("/feeds/:id/fetch", FetchFeed)

			protected.GET("/items", ListItems)
			protected.GET("/items/:id", GetItem)
		}
	}

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}
