package handlers

import (
	"log"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"go-rss-ui/app"
	"go-rss-ui/config"
	"go-rss-ui/database"
	"go-rss-ui/models"
)

func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		userID := session.Get("user")
		if userID == nil {
			app.Logger.Debug().Msg("User not logged in")
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		// Check if user exists in database
		var user models.User
		userIDUint, ok := userID.(uint)
		if !ok {
			// Try to convert from float64 (JSON numbers are often float64)
			if userIDFloat, ok := userID.(float64); ok {
				userIDUint = uint(userIDFloat)
			} else {
				app.Logger.Warn().Msg("Invalid user ID type in session")
				session.Clear()
				saveSession(session)
				c.Redirect(http.StatusFound, "/login")
				c.Abort()
				return
			}
		}

		result := database.DB.First(&user, userIDUint)
		if result.Error != nil {
			app.Logger.Warn().Uint("user_id", userIDUint).Msg("User not found in database, invalidating session")
			session.Clear()
			saveSession(session)
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
				var user models.User
				if err := database.DB.First(&user, userIDUint).Error; err == nil {
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
		c.Set("isCypressMode", config.IsCypressMode())

		c.Next()
	}
}
