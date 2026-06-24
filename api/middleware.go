package api

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"go-rss-ui/database"
	"go-rss-ui/models"
)

func APIAuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		userID := session.Get("user")
		if userID == nil {
			c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "authentication required"})
			c.Abort()
			return
		}

		var userIDUint uint
		switch v := userID.(type) {
		case uint:
			userIDUint = v
		case float64:
			userIDUint = uint(v)
		default:
			c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "invalid session"})
			c.Abort()
			return
		}

		var user models.User
		if err := database.DB.First(&user, userIDUint).Error; err != nil {
			session.Clear()
			_ = session.Save()
			c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "authentication required"})
			c.Abort()
			return
		}

		c.Set("userID", userIDUint)
		c.Set("username", user.Username)
		c.Next()
	}
}
