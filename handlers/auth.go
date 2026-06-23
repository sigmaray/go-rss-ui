package handlers

import (
	"log"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"go-rss-ui/database"
	"go-rss-ui/models"
)

func ShowLogin(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", getTemplateData(c, gin.H{
		"title": "Login",
	}))
}

func Login(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	var user models.User
	result := database.DB.Where("username = ?", username).First(&user)
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

func Logout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	saveSession(session)
	c.Redirect(http.StatusFound, "/")
}
